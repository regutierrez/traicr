import { expect, test } from 'vitest';
import { attachmentView } from './attachments';
import { renderMarkdown, sanitizeMarkdownUrl } from './markdown';
import { childCards, leftoverCards } from './children';
import { parseSkillBlock } from './skill';
import { buildTranscriptSession } from './session';
import { isStructuredOutput, requestedEdit, resultFiles, resultText, toolChipLabel, toolChipParts, toolStatus, toolSummary } from './tools';
import { defaultLeafId, findNewestLeaf, getPath, graphHasFork, graphLeaves } from './tree';
import { displayModel, foldWork, groupTurns, isToolOnlyMessage, pathSessionFacts, streamCounts, streamRows, activeTime, timeSpan, toolNameCounts, toolResults, usageTotals, withoutSuggestedReplies, workSummary, promptOutline } from './turns';
import type { ToolCallBlock, TranscriptEntry } from './types';

test('shared tools keep original names and display shell and file operations consistently', () => {
	for (const name of ['bash', 'Bash', 'shell_command']) {
		const call = { type: 'toolCall', id: 'call', name, arguments: { command: 'echo <value>' } } as ToolCallBlock;
		expect(toolSummary(call)).toContain(name);
		expect(toolSummary(call)).toContain('echo <value>');
	}
	expect(toolSummary({ type: 'toolCall', id: 'r', name: 'read', arguments: { file_path: 'src/main.js' } })).toBe('read · src/main.js');
	expect(toolSummary({ type: 'toolCall', id: 'w', name: 'write', arguments: { path: 'index.html' } })).toBe('write · index.html');
	expect(toolChipLabel({ type: 'toolCall', id: 'r', name: 'read', arguments: { file_path: 'src/main.js' } })).toBe('Read main.js');
	expect(toolChipLabel({ type: 'toolCall', id: 's', name: 'shell_command', arguments: { command: 'go test' } })).toBe('Ran go test');
	expect(toolChipLabel({ type: 'toolCall', id: 'b', name: 'bash', arguments: { command: 'git status --short; git branch --show-current' } })).toBe(
		'Ran git status --short; git branch --show-current'
	);
	expect(toolChipLabel({ type: 'toolCall', id: 'w', name: 'write_stdin', arguments: {} })).toBe('Used Write Stdin');
	expect(toolChipLabel({ type: 'toolCall', id: 'p', name: 'apply_patch', arguments: { path: 'range.go' } })).toBe('Edit range.go');
	// Amp's own file tools are edits and writes, not unknown tools.
	expect(toolChipLabel({ type: 'toolCall', id: 'e', name: 'edit_file', arguments: { path: '/tmp/a/body.md' } })).toBe('Edit body.md');
	expect(toolChipLabel({ type: 'toolCall', id: 'c', name: 'create_file', arguments: { path: 'new.go' } })).toBe('Write new.go');
	expect(resultFiles({ diff: '````diff\n--- a\n+++ b\n-old\n+new\n````' })[0].diff.map((line) => line.kind)).toEqual(['removed', 'added', 'removed', 'added']);
	expect(toolChipLabel({ type: 'toolCall', id: 't', name: 'create_thread', arguments: { title: 'Review boundary documentation' } })).toBe(
		'Thread Review boundary documentation'
	);
	expect(toolChipLabel({ type: 'toolCall', id: 'o', name: 'oracle', arguments: { task: 'Explain the failing assertion' } })).toBe(
		'Oracle · Explain the failing assertion'
	);
	expect(toolChipParts({ type: 'toolCall', id: 's', name: 'shell_command', arguments: { command: 'go test' } })).toEqual({
		verb: 'Ran',
		rest: 'go test',
		icon: 'terminal'
	});
	expect(toolChipParts({ type: 'toolCall', id: 'w', name: 'write_stdin', arguments: {} })).toEqual({
		verb: 'Used',
		rest: 'Write Stdin',
		icon: 'wrench'
	});
	const edit = requestedEdit({ oldText: 'old <tag>', newText: 'new & value' });
	expect(edit[0]).toEqual({ kind: 'removed', text: '-old <tag>' });
	expect(edit[1]).toEqual({ kind: 'added', text: '+new & value' });
});

test('Amp recorded tool states preserve failure and running precedence', () => {
	const cases: [TranscriptEntry['message'] | undefined, boolean | undefined, string][] = [
		[{ role: 'toolResult', content: [], isError: true, run: { status: 'done', result: { running: true } } }, true, 'failed'],
		[{ role: 'toolResult', content: [], run: { status: 'done', result: { running: true } } }, true, 'running'],
		[{ role: 'toolResult', content: [], run: { status: 'cancelled' } }, true, 'cancelled'],
		[{ role: 'toolResult', content: [] }, false, 'unknown'],
		[undefined, false, 'incomplete arguments'],
		[undefined, true, 'result not collected']
	];
	for (const [result, complete, status] of cases) {
		const call = { type: 'toolCall', id: 'call', name: 'tool', arguments: {}, details: { complete } } as ToolCallBlock;
		expect(toolStatus(call, result ? { id: '2', parentId: null, type: 'message', message: result } : undefined)).toBe(status);
	}
});

test('JSON tool output stays structured text', () => {
	expect(isStructuredOutput('{\n  "tool": "get_guide",\n  "description": "Call `doop-instructions` once."\n}')).toBe(true);
	expect(isStructuredOutput('Call `doop-instructions` once.')).toBe(false);
});

test('an empty Cursor call id detaches the result into a second chip', () => {
	const detached = buildTranscriptSession(
		[
			{ id: 1, key: 'message:bubble-guide', kind: 'message', role: 'assistant', text: "I'll pull the Doop guide" },
			{
				id: 2,
				key: 'tool_call:bubble-guide',
				kind: 'tool_call',
				role: 'assistant',
				tool: 'GetMcpTools',
				text: '"{\\"server\\":\\"user-doop\\",\\"toolName\\":\\"get_guide\\"}"'
			},
			{ id: 3, key: 'tool_result:bubble-guide', kind: 'tool_result', role: 'assistant', tool: 'GetMcpTools', text: '{"content":"guide schema"}' }
		],
		{ harness: 'cursor', nativeId: 'ddf4e792-18a5-4ad4-9004-8b7e6d5008ee', traceId: '1' }
	);
	const detachedPath = getPath(detached.entries, detached.leafId ?? '');
	const call = detachedPath.flatMap((entry) => entry.message?.content ?? []).find((block) => block.type === 'toolCall');
	expect(call && call.type === 'toolCall' ? call.id : '').toBe('tool_call:bubble-guide');
	expect(call && call.type === 'toolCall' ? call.arguments : {}).toEqual({
		raw: '"{\\"server\\":\\"user-doop\\",\\"toolName\\":\\"get_guide\\"}"'
	});
	expect(toolResults(detachedPath).has(call && call.type === 'toolCall' ? call.id : '')).toBe(false);
	expect(toolStatus(call as ToolCallBlock, undefined)).toBe('result not collected');
	const detachedRows = streamRows(detachedPath).flatMap((row) => row.entries);
	expect(detachedRows.some((entry) => entry.message?.role === 'toolResult' && entry.message.toolName === 'GetMcpTools')).toBe(true);

	const linked = buildTranscriptSession(
		[
			{ id: 1, key: 'message:bubble-guide', kind: 'message', role: 'assistant', text: "I'll pull the Doop guide" },
			{
				id: 2,
				key: 'tool_call:bubble-guide',
				kind: 'tool_call',
				role: 'assistant',
				call_id: 'toolu_guide',
				tool: 'GetMcpTools',
				text: '{"server":"user-doop","toolName":"get_guide"}'
			},
			{
				id: 3,
				key: 'tool_result:bubble-guide',
				kind: 'tool_result',
				role: 'assistant',
				call_id: 'toolu_guide',
				tool: 'GetMcpTools',
				text: '{"content":"guide schema"}'
			}
		],
		{ harness: 'cursor', nativeId: 'ddf4e792-18a5-4ad4-9004-8b7e6d5008ee', traceId: '1' }
	);
	const linkedPath = getPath(linked.entries, linked.leafId ?? '');
	const linkedCall = linkedPath.flatMap((entry) => entry.message?.content ?? []).find((block) => block.type === 'toolCall');
	const linkedResult = toolResults(linkedPath).get(linkedCall && linkedCall.type === 'toolCall' ? linkedCall.id : '');
	expect(linkedCall && linkedCall.type === 'toolCall' ? linkedCall.arguments : {}).toEqual({
		server: 'user-doop',
		toolName: 'get_guide'
	});
	expect(resultText(linkedResult)).toBe('{"content":"guide schema"}');
	expect(streamRows(linkedPath).some((row) => row.entries.some((entry) => entry.message?.role === 'toolResult'))).toBe(false);
});

test('empty reasoning stays empty so the UI can explain the omitted text', () => {
	const data = buildTranscriptSession(
		[
			{ id: 1, key: 'reasoning:a:0', kind: 'reasoning', role: 'assistant', text: '' },
			{ id: 2, key: 'tool_call:b:0', parent_key: 'reasoning:a:0', kind: 'tool_call', role: 'assistant', call_id: 'agent', tool: 'Agent', text: '{"description":"Review tests"}' },
			{ id: 3, key: 'tool_result:c:0', parent_key: 'tool_call:b:0', kind: 'tool_result', call_id: 'agent', text: 'Coverage report' }
		],
		{ harness: 'claude-code', nativeId: 's', traceId: '1' }
	);
	const thinking = data.entries[0].message?.content.find((block) => block.type === 'thinking');
	expect(thinking && 'thinking' in thinking ? thinking.thinking : 'missing').toBe('');
	expect(resultText(data.entries[2])).toBe('Coverage report');
});

test('Pi skills parse without treating the tag as HTML', () => {
	const skill = parseSkillBlock('<skill name="review" location="/skills/review">\nCheck <script> safely.\n</skill>\n\nReview the changes.');
	expect(skill).toEqual({
		name: 'review',
		location: '/skills/review',
		content: 'Check <script> safely.',
		userMessage: 'Review the changes.'
	});
});

test('archived images use local previews and keep large originals downloadable', () => {
	const image = {
		name: 'screenshot.png',
		media_type: 'image/png',
		url: 'https://ampcode.com/user-content/attachments/fixture.png',
		archived_path: 'source/attachments/hash',
		size: 10 * 1024 * 1024,
		revisionId: 9,
		source_pointer: '/messages/0/content/0'
	};
	const view = attachmentView(image);
	expect(view.kind).toBe('archived');
	if (view.kind === 'archived') {
		expect(view.preview).toContain('/revisions/9/attachment?pointer=');
		expect(view.download).toContain('/revisions/9/file?path=source%2Fattachments%2Fhash&download=1');
		expect(view.oversized).toBe(false);
	}
	const large = attachmentView({ ...image, size: image.size + 1 });
	expect(large.kind === 'archived' && large.oversized && !large.preview).toBe(true);
	expect(attachmentView({ url: 'javascript:alert(1)', path: 'unsafe.png' }).kind).toBe('missing');
	expect(attachmentView({ url: 'https://example.com/private.png', path: 'private.png' })).toMatchObject({
		kind: 'external',
		href: 'https://example.com/private.png'
	});
});

test('markdown images become links and unsafe schemes are dropped', () => {
	expect(sanitizeMarkdownUrl('javascript:alert(1)')).toBeNull();
	const html = renderMarkdown('See ![secret](https://example.com/x.png) and <script>bad</script>');
	expect(html).toContain('[Image: secret]');
	expect(html).toContain('https://example.com/x.png');
	expect(html).not.toContain('<img');
	expect(html).not.toContain('<script>');
});

test('linear graphs do not grow a tree and forks do', () => {
	const linear = buildTranscriptSession(
		[
			{ id: 1, key: 'message:u:0', kind: 'message', role: 'user', text: 'Start' },
			{ id: 2, key: 'message:a:0', parent_key: 'message:u:0', kind: 'message', role: 'assistant', text: 'Reply' }
		],
		{ harness: 'pi', nativeId: 's', traceId: '1' }
	);
	expect(graphHasFork(linear.entries)).toBe(false);
	const forked = buildTranscriptSession(
		[
			{ id: 1, key: 'message:u:0', kind: 'message', role: 'user', text: 'Start' },
			{ id: 2, key: 'message:a:0', parent_key: 'message:u:0', kind: 'message', role: 'assistant', text: 'First' },
			{ id: 3, key: 'message:b:0', parent_key: 'message:u:0', kind: 'message', role: 'assistant', text: 'Second' }
		],
		{ harness: 'pi', nativeId: 's', traceId: '1' }
	);
	expect(graphHasFork(forked.entries)).toBe(true);
	expect(groupTurns(linear.entries).length).toBe(1);
	expect(groupTurns(linear.entries)[0].entries.length).toBe(2);
	expect(streamCounts(linear.entries)).toMatchObject({ prompts: 1, responses: 1, branches: 1 });
	expect(streamCounts(forked.entries).branches).toBe(2);
	expect(graphLeaves(forked.entries)).toHaveLength(2);
	expect(forked.leafId).toBeTruthy();
	const selected = getPath(forked.entries, forked.leafId ?? '');
	expect(streamCounts(selected, forked.entries)).toMatchObject({ prompts: 1, responses: 1, branches: 2 });
});

test('default leaf prefers the conversation path over a later branch summary', () => {
	const forked = buildTranscriptSession(
		[
			{ id: 1, key: 'message:u:0', kind: 'message', role: 'user', text: 'Read the fixture.', timestamp: '2026-01-01T00:00:01Z' },
			{
				id: 2,
				key: 'message:a:0',
				parent_key: 'message:u:0',
				kind: 'tool_call',
				role: 'assistant',
				call_id: 'call-1',
				tool: 'read',
				text: '{"path":"fixture.txt"}',
				timestamp: '2026-01-01T00:00:02Z'
			},
			{ id: 3, key: 'tool_result:r:0', parent_key: 'message:a:0', kind: 'tool_result', call_id: 'call-1', tool: 'read', text: 'ok', timestamp: '2026-01-01T00:00:03Z' },
			{ id: 4, key: 'compaction:c:0', parent_key: 'tool_result:r:0', kind: 'compaction', text: 'Earlier fixture activity summarized.', timestamp: '2026-01-01T00:00:05Z' },
			{ id: 5, key: 'branch:b:0', parent_key: 'message:u:0', kind: 'branch_summary', text: 'Alternate fixture branch.', timestamp: '2026-01-01T00:00:06Z' }
		],
		{ harness: 'pi', nativeId: 's', traceId: '1' }
	);
	expect(forked.leafId).toBe(defaultLeafId(forked.entries));
	expect(forked.entries.find((entry) => entry.id === forked.leafId)?.type).toBe('compaction');
	expect(findNewestLeaf(forked.entries, forked.entries[0].id)).toBe(forked.leafId);
});

test('stream rows keep the user bubble off the following agent turn and tools', () => {
	const entries: TranscriptEntry[] = [
		{
			id: 'u',
			parentId: null,
			type: 'message',
			message: { role: 'user', content: [{ type: 'text', text: 'Review the branch' }] }
		},
		{
			id: 'a',
			parentId: 'u',
			type: 'message',
			message: { role: 'assistant', content: [{ type: 'text', text: 'I will check the diff.' }] }
		},
		{
			id: 't',
			parentId: 'a',
			type: 'message',
			message: {
				role: 'assistant',
				content: [{ type: 'toolCall', id: 'c', name: 'bash', arguments: { command: 'git status' } }]
			}
		},
		{
			id: 'm',
			parentId: 't',
			type: 'message',
			message: {
				role: 'assistant',
				content: [
					{ type: 'text', text: 'Looks clean.' },
					{ type: 'toolCall', id: 'c2', name: 'read', arguments: { path: 'README.md' } }
				]
			}
		}
	];
	expect(groupTurns(entries)).toHaveLength(1);
	expect(streamRows(entries).map((row) => [row.chrome, row.entries.length])).toEqual([
		['user', 1],
		['agent', 1],
		['quiet', 1],
		['agent', 1]
	]);
	expect(streamRows(entries)[0].entries[0].id).toBe('u');
	expect(streamRows(entries)[1].entries[0].message?.content[0]).toMatchObject({ type: 'text' });
	expect(
		streamRows([
			...entries,
			{
				id: 'r',
				parentId: 't',
				type: 'message',
				message: { role: 'toolResult', toolCallId: 'c', content: [{ type: 'text', text: 'ok' }] }
			}
		]).map((row) => row.id)
	).toEqual(['u', 'a', 't', 'm']);
});

test('streamRows keep the first path message first, not a window around the leaf', () => {
	const path = Array.from({ length: 250 }, (_, index) => ({
		id: String(index + 1),
		parentId: index === 0 ? null : String(index),
		type: 'message' as const,
		message: {
			role: index % 2 === 0 ? 'user' : 'assistant',
			content: [{ type: 'text' as const, text: `message ${index + 1}` }]
		}
	})) as TranscriptEntry[];
	const rows = streamRows(path);
	expect(rows[0]).toMatchObject({ chrome: 'user', id: '1' });
	expect(rows[0].entries[0].id).toBe('1');
	expect(rows.at(-1)).toMatchObject({ chrome: 'agent', id: '250' });
	expect(rows).toHaveLength(250);
});

test('model and thinking facts leave the stream and keep the latest display name', () => {
	const path = [
		{
			id: 'm',
			parentId: null,
			type: 'model_change',
			provider: 'openai-codex',
			modelId: 'gpt-6-astra'
		},
		{ id: 't', parentId: 'm', type: 'thinking_level_change', thinkingLevel: '' },
		{
			id: 'u',
			parentId: 't',
			type: 'message',
			message: { role: 'user', content: [{ type: 'text', text: 'hello' }] }
		},
		{ id: 'later', parentId: 'u', type: 'thinking_level_change', thinkingLevel: 'high' }
	] as TranscriptEntry[];
	expect(displayModel('openai-codex', 'gpt-6-astra')).toBe('GPT 6 Astra');
	expect(displayModel('anthropic', 'claude-opus-5-5')).toBe('Claude Opus 5.5');
	expect(displayModel('anthropic', 'claude-sonnet-4-20250514')).toBe('Claude Sonnet 4 20250514');
	expect(pathSessionFacts(path)).toEqual({ model: 'GPT 6 Astra', thinkingLevel: 'high' });
	expect(streamRows(path).map((row) => row.id)).toEqual(['u']);
});

test('tool-only assistant messages stay quiet rows', () => {
	expect(
		isToolOnlyMessage({
			id: '2',
			parentId: '1',
			type: 'message',
			message: { role: 'assistant', content: [{ type: 'toolCall', id: 'c', name: 'read', arguments: {} }] }
		})
	).toBe(true);
	expect(
		isToolOnlyMessage({
			id: '2',
			parentId: '1',
			type: 'message',
			message: { role: 'assistant', content: [{ type: 'text', text: 'I will inspect it' }] }
		})
	).toBe(false);
});

test('leftover child traces stay unplaced when no visible card consumes them', () => {
	const kids = [{ id: 9, native_trace_id: 'T-1', title: 'Child' }];
	expect(leftoverCards([], new Map(), kids)).toEqual(kids);
	expect(
		leftoverCards(
			[
				{
					id: '1',
					parentId: null,
					type: 'message',
					message: {
						role: 'assistant',
						content: [{ type: 'toolCall', id: 'c', name: 'create_thread', arguments: { threadID: 'T-1' } }]
					}
				}
			],
			new Map(),
			kids
		)
	).toEqual([]);
});

test('a Claude Code subagent card sits on the Agent call that started it', () => {
	const kids = [
		{ id: 9, native_trace_id: 'a06a6831ff92a4124', title: 'Review the migration' },
		{ id: 10, native_trace_id: 'b17b7942ee03b5235', title: 'Other subagent' }
	];
	const call: ToolCallBlock = { type: 'toolCall', id: 'toolu_1', name: 'Agent', arguments: { prompt: 'review it' } };
	const entry: TranscriptEntry = { id: '1', parentId: null, type: 'message', message: { role: 'assistant', content: [call] } };
	const result = (text: string): TranscriptEntry => ({ id: '2', parentId: '1', type: 'message', message: { role: 'toolResult', toolCallId: 'toolu_1', content: [{ type: 'text', text }] } });
	const launched = result('Async agent launched successfully.\nagentId: a06a6831ff92a4124 (internal ID - do not mention to user.)');
	expect(childCards(entry, call, launched, kids)).toEqual([
		expect.objectContaining({ id: 'a06a6831ff92a4124', title: 'Review the migration', collected: true, traceId: 9, meta: 'Subagent' })
	]);
	// The second child has no Agent call that names it, so it stays a parent-level card.
	expect(leftoverCards([entry], new Map([['toolu_1', launched]]), kids)).toEqual([kids[1]]);
	// An ID the archive never received still gets a card that says so, named by the call.
	const described = { ...call, arguments: { description: 'Build looks 05 and 09' } };
	expect(childCards(entry, described, result('agentId: ffffffff0000aaaa1'), kids)).toEqual([
		expect.objectContaining({ collected: false, title: 'Build looks 05 and 09' })
	]);
	// Other tools that print agentId are not subagent launches.
	expect(childCards(entry, { ...call, name: 'Bash' }, launched, kids)).toEqual([]);
});

test('runs of three or more quiet rows fold into one work row with a summary', () => {
	const step = (id: string, name: string, at: string) =>
		({
			id,
			parentId: null,
			type: 'message',
			timestamp: at,
			message: { role: 'assistant', content: [{ type: 'toolCall', id: `c${id}`, name, arguments: {} }] }
		}) as TranscriptEntry;
	const thought = { id: 't', parentId: null, type: 'message', timestamp: '2026-01-01T00:00:01Z', message: { role: 'assistant', content: [{ type: 'thinking', thinking: 'hm' }] } } as TranscriptEntry;
	const reply = { id: 'r', parentId: null, type: 'message', message: { role: 'assistant', content: [{ type: 'text', text: 'done' }] } } as TranscriptEntry;
	const user = { id: 'u', parentId: null, type: 'message', message: { role: 'user', content: [{ type: 'text', text: 'go' }] } } as TranscriptEntry;
	const rows = streamRows([
		user,
		step('1', 'bash', '2026-01-01T00:00:00Z'),
		step('2', 'read', '2026-01-01T00:00:02Z'),
		step('3', 'bash', '2026-01-01T00:00:03Z'),
		reply,
		step('4', 'bash', '2026-01-01T00:00:04Z'),
		thought
	]);
	const folded = foldWork(rows);
	expect(folded.map((row) => row.chrome)).toEqual(['user', 'work', 'agent', 'quiet', 'quiet']);
	expect(folded[1].entries.map((entry) => entry.id)).toEqual(['1', '2', '3']);

	// Steps count, not rows: one message holding three steps folds on its own.
	const packed = { id: 'p', parentId: null, type: 'message', message: { role: 'assistant', content: [thought.message!.content[0], ...step('5', 'bash', '').message!.content, ...step('6', 'read', '').message!.content] } } as TranscriptEntry;
	expect(foldWork(streamRows([user, packed, reply])).map((row) => row.chrome)).toEqual(['user', 'work', 'agent']);

	// A row that must stay visible splits the run, and the shorter halves stay unfolded.
	expect(foldWork(rows, (row) => row.id === '2').map((row) => row.chrome)).toEqual(['user', 'quiet', 'quiet', 'quiet', 'agent', 'quiet', 'quiet']);

	// The last result's timestamp ends the work, not the last call's.
	const results = new Map([['c3', { id: 'x', parentId: null, type: 'message', timestamp: '2026-01-01T00:02:05Z' } as TranscriptEntry]]);
	expect(workSummary([...folded[1].entries, thought], results)).toEqual({
		duration: 'Worked for 2m 5s',
		sentence: 'Ran 2 commands, read 1 file, and thought once'
	});
	expect(workSummary([step('9', 'edit', ''), step('8', 'edit', '')])).toEqual({ duration: '', sentence: 'Made 2 edits' });
});

test('Overview totals sum usage per reply, keep the peak call window, and rank tools by use', () => {
	const reply = (id: string, usage: Record<string, number>, at: string, calls: [string, string][] = []) =>
		({
			id,
			parentId: null,
			type: 'message',
			timestamp: at,
			message: { role: 'assistant', usage, content: calls.map(([callId, name]) => ({ type: 'toolCall', id: callId, name, arguments: {} })) }
		}) as TranscriptEntry;
	const entries = [
		{ id: 'u', parentId: null, type: 'message', timestamp: '2026-01-01T00:00:00Z', message: { role: 'user', usage: { input: 999 }, content: [] } } as TranscriptEntry,
		reply('a', { input: 10, output: 5, cacheRead: 100, cacheWrite: 7, context: 117, contextWindow: 200 }, '2026-01-01T00:01:00Z', [['c1', 'read'], ['c2', 'bash']]),
		reply('b', { input: 1, output: 2, context: 300, contextWindow: 1000 }, '2026-01-01T00:02:30Z', [['c3', 'bash']]),
		reply('c', { input: 4, context: 250, contextWindow: 200 }, '2026-01-01T00:03:00Z'),
		{ id: 'r', parentId: null, type: 'message', message: { role: 'toolResult', toolCallId: 'c3', isError: true, content: [] } } as TranscriptEntry
	];
	// The user's usage is ignored, and the peak window comes from the peak call, not the last one.
	expect(usageTotals(entries)).toEqual({ input: 15, output: 7, cacheRead: 100, cacheWrite: 7, peakContext: 300, contextWindow: 1000, messages: 3 });
	expect(usageTotals(entries.slice(0, 1))).toBeNull();
	expect(toolNameCounts(entries).map((tool) => [tool.name, tool.count, tool.errors])).toEqual([
		['bash', 2, 1],
		['read', 1, 0]
	]);
	expect(timeSpan(entries)).toMatchObject({ start: '2026-01-01T00:00:00.000Z', end: '2026-01-01T00:03:00.000Z', duration: '3m' });
});

test('housekeeping tools never trigger a fold and stay out of the summary; automation messages are not prompts', () => {
	const call = (id: string, name: string) =>
		({ id, parentId: null, type: 'message', message: { role: 'assistant', content: [{ type: 'toolCall', id: `c${id}`, name, arguments: {} }] } }) as TranscriptEntry;
	const user = { id: 'u', parentId: null, type: 'message', message: { role: 'user', content: [{ type: 'text', text: 'go' }] } } as TranscriptEntry;
	const automation = {
		id: 'n',
		parentId: null,
		type: 'message',
		message: { role: 'user', nativeDetails: { message_meta: { fromAutomation: true } }, content: [{ type: 'text', text: 'A subagent ended.' }] }
	} as unknown as TranscriptEntry;
	const reply = { id: 'r', parentId: null, type: 'message', message: { role: 'assistant', content: [{ type: 'text', text: 'ok' }] } } as TranscriptEntry;

	// Housekeeping is not a step: two real steps beside it stay visible, three fold and take it along.
	const folded = foldWork(
		streamRows([user, call('1', 'set_thread_status'), call('2', 'bash'), call('3', 'read'), reply, call('4', 'set_thread_status'), call('5', 'bash'), call('6', 'read'), call('7', 'bash')])
	);
	expect(folded.map((row) => row.chrome)).toEqual(['user', 'quiet', 'quiet', 'quiet', 'agent', 'work']);
	expect(folded[5].entries.map((entry) => entry.id)).toEqual(['4', '5', '6', '7']);
	expect(workSummary(folded[5].entries).sentence).toBe('Ran 2 commands and read 1 file');
	expect(workSummary([call('5', 'set_thread_status'), call('6', 'tool_search')]).sentence).toBe('2 housekeeping steps');

	const rows = streamRows([user, reply, automation, reply]);
	expect(rows.map((row) => row.chrome)).toEqual(['user', 'agent', 'automation', 'agent']);
	expect(streamCounts([user, automation]).prompts).toBe(1);

	// Overview hides the Thoughts card when no thought kept its text.
	const thoughts = { id: 't', parentId: null, type: 'message', message: { role: 'assistant', content: [{ type: 'thinking', thinking: '' }, { type: 'thinking', thinking: 'why' }] } } as TranscriptEntry;
	expect(streamCounts([thoughts])).toMatchObject({ thinking: 2, thinkingWithText: 1 });
});

test('active time sums each turn and leaves out idle time between prompts', () => {
	const at = (id: string, role: string, time: string) =>
		({ id, parentId: null, type: 'message', timestamp: time, message: { role, content: [{ type: 'text', text: id }] } }) as TranscriptEntry;
	const path = [
		at('u1', 'user', '2026-01-01T00:00:00Z'),
		at('a1', 'assistant', '2026-01-01T00:02:00Z'),
		at('u2', 'user', '2026-01-02T00:00:00Z'),
		at('a2', 'assistant', '2026-01-02T00:00:30Z')
	];
	expect(activeTime(path)).toBe('2m 30s');
	expect(activeTime([at('u', 'user', '')])).toBe('');
});

test("replies drop Amp's suggested-replies block but keep the tag when it is mentioned in prose", () => {
	const reply = 'Done. The `<suggested-replies>` tag is stripped by Amp.\n\n<suggested-replies>\n**Ship it**\nRevert it\n</suggested-replies>';
	expect(withoutSuggestedReplies(reply)).toBe('Done. The `<suggested-replies>` tag is stripped by Amp.');
	expect(withoutSuggestedReplies('<suggested-replies>\nOK\n</suggested-replies>')).toBe('');
});

test('the prompt outline pairs each prompt with the last reply of its turn, as plain text', () => {
	const say = (id: string, role: string, text: string) =>
		({ id, parentId: null, type: 'message', message: { role, content: [{ type: 'text', text }] } }) as TranscriptEntry;
	const rows = foldWork(
		streamRows([
			say('u1', 'user', '## Fix the **login** bug'),
			say('a1', 'assistant', 'Looking.'),
			say('a2', 'assistant', 'Fixed in `auth.ts`.\n\n<suggested-replies>\nShip it\n</suggested-replies>'),
			say('u2', 'user', '<skill name="review" location="/skills/review">\nReview steps\n</skill>\n\nreview it'),
			say('u3', 'user', 'thanks'),
			say('a3', 'assistant', 'Summary:\n\n| Part | Status |\n|---|:---:|\n| API | done |')
		])
	);
	expect(promptOutline(rows)).toEqual([
		{ id: 'u1', prompt: 'Fix the login bug', at: '', reply: 'Fixed in auth.ts.', replyId: 'a2' },
		{ id: 'u2', prompt: 'review it', at: '', reply: '', replyId: '' },
		{ id: 'u3', prompt: 'thanks', at: '', reply: 'Summary: Part Status API done', replyId: 'a3' }
	]);
});
