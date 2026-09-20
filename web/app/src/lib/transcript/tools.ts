import type { ToolCallBlock, ToolResultPayload, TranscriptEntry } from './types';

export type ToolStatus =
	| 'failed'
	| 'running'
	| 'cancelled'
	| 'done'
	| 'unknown'
	| 'incomplete arguments'
	| 'result not collected';

export type DiffLine = { kind: 'added' | 'removed' | 'context'; text: string };

export function toolStatus(call: ToolCallBlock, result?: TranscriptEntry): ToolStatus {
	const message = result?.message;
	const run = message?.run;
	const payload = run?.result;
	if (message?.isError) return 'failed';
	if (payload?.running) return 'running';
	if (run?.status) return run.status as ToolStatus;
	if (result) return 'unknown';
	if (call.details?.complete === false) return 'incomplete arguments';
	return 'result not collected';
}

export function isShellTool(name: string) {
	return ['shell_command', 'bash', 'shell'].includes(name.toLowerCase());
}

export function isFileTool(name: string) {
	return ['read', 'write', 'edit', 'ls', 'glob', 'grep', 'find'].includes(name.toLowerCase());
}

export function toolPath(args: Record<string, unknown>) {
	const path = args.file_path ?? args.path;
	return typeof path === 'string' ? path : '';
}

export function toolCommand(args: Record<string, unknown>) {
	return typeof args.command === 'string' ? args.command : '';
}

export function toolSummary(call: ToolCallBlock) {
	const name = call.name;
	const args = call.arguments || {};
	if (isShellTool(name)) {
		const command = toolCommand(args).replace(/[\n\t]/g, ' ').trim();
		return command ? `${name} · ${command.slice(0, 72)}` : name;
	}
	const path = toolPath(args);
	if (path) return `${name} · ${path}`;
	if (call.name === 'skill' && typeof args.name === 'string') return `skill · ${args.name}`;
	if (typeof args.description === 'string') return `${name} · ${args.description}`;
	return name;
}

export function requestedEdit(args: Record<string, unknown>): DiffLine[] {
	const before = args.oldText ?? args.old_string;
	const after = args.newText ?? args.new_string;
	if (typeof before !== 'string' || typeof after !== 'string') return [];
	return [
		...before.split('\n').map((text) => ({ kind: 'removed' as const, text: `-${text}` })),
		...after.split('\n').map((text) => ({ kind: 'added' as const, text: `+${text}` }))
	];
}

export function diffLines(text: string): DiffLine[] {
	return text.split('\n').map((line) => ({
		kind: line.startsWith('+') ? 'added' : line.startsWith('-') ? 'removed' : 'context',
		text: line
	}));
}

export function resultText(result?: TranscriptEntry) {
	return (result?.message?.content ?? [])
		.filter((block) => block.type === 'text')
		.map((block) => ('text' in block ? block.text : ''))
		.join('\n');
}

export function resultFiles(payload?: ToolResultPayload) {
	if (!Array.isArray(payload?.files)) return [];
	return payload.files.flatMap((file) => {
		if (!file || typeof file !== 'object') return [];
		const item = file as { uri?: string; path?: string; diff?: string; additions?: number; deletions?: number };
		return [
			{
				label: item.uri || item.path || 'Changed file',
				additions: item.additions,
				deletions: item.deletions,
				diff: typeof item.diff === 'string' ? diffLines(item.diff) : []
			}
		];
	});
}

export function threadValues(args: Record<string, unknown>, payload?: ToolResultPayload) {
	const values: { id: string; label: string; name: string }[] = [];
	for (const key of ['thread', 'threadID', 'thread_id']) {
		const value = args[key];
		if (typeof value === 'string' && value) {
			values.push({
				id: normalizeThreadID(value),
				label: key === 'thread' || key === 'thread_id' ? 'Referenced thread' : 'Referenced thread',
				name: String(args.name ?? '')
			});
		}
	}
	if (typeof payload?.threadID === 'string' && payload.threadID) {
		values.push({ id: normalizeThreadID(payload.threadID), label: 'Spawned thread', name: '' });
	}
	return values;
}

export function normalizeThreadID(id: string) {
	const match = id.match(/^https:\/\/ampcode\.com\/threads\/([^/?#]+)/);
	return match ? match[1] : id;
}

export function ampThreadURL(id: string) {
	return id.startsWith('T-') ? `https://ampcode.com/threads/${encodeURIComponent(id)}` : '';
}

export function resolveThreadHref(id: string) {
	return `/traces/resolve?native_id=${encodeURIComponent(id)}`;
}
