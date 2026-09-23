import { expect, test } from 'vitest';
import { attachmentView } from './attachments';
import { renderMarkdown, sanitizeMarkdownUrl } from './markdown';
import { leftoverCards } from './children';
import { parseSkillBlock } from './skill';
import { buildTranscriptSession } from './session';
import { isStructuredOutput, requestedEdit, resultText, toolChipLabel, toolChipParts, toolStatus, toolSummary } from './tools';
import { defaultLeafId, findNewestLeaf, getPath, graphHasFork, graphLeaves } from './tree';
import { displayModel, groupTurns, isToolOnlyMessage, pathSessionFacts, streamCounts, streamRows, toolResults } from './turns';
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
