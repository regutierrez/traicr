import { expect, test } from 'vitest';
import { attachmentView } from './attachments';
import { renderMarkdown, sanitizeMarkdownUrl } from './markdown';
import { parseSkillBlock } from './skill';
import { buildTranscriptSession } from './session';
import { requestedEdit, resultText, toolStatus, toolSummary } from './tools';
import { graphHasFork } from './tree';
import { groupTurns } from './turns';
import type { ToolCallBlock, TranscriptEntry } from './types';

test('shared tools keep original names and display shell and file operations consistently', () => {
	for (const name of ['bash', 'Bash', 'shell_command']) {
		const call = { type: 'toolCall', id: 'call', name, arguments: { command: 'echo <value>' } } as ToolCallBlock;
		expect(toolSummary(call)).toContain(name);
		expect(toolSummary(call)).toContain('echo <value>');
	}
	expect(toolSummary({ type: 'toolCall', id: 'r', name: 'read', arguments: { file_path: 'src/main.js' } })).toBe('read · src/main.js');
	expect(toolSummary({ type: 'toolCall', id: 'w', name: 'write', arguments: { path: 'index.html' } })).toBe('write · index.html');
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
});
