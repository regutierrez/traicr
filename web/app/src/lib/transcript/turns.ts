import { leafCount } from './tree';
import { toolKind, toolKindLabel, type ToolKind } from './tools';
import type { TranscriptEntry, Turn } from './types';

export type StreamFilter = 'all' | 'prompts' | 'responses' | 'thinking' | 'tools' | 'compaction';

export const streamFilters: StreamFilter[] = ['all', 'prompts', 'responses', 'thinking', 'tools', 'compaction'];

export type StreamCounts = {
	prompts: number;
	responses: number;
	thinking: number;
	tools: number;
	compaction: number;
	branches: number;
	toolKinds: { kind: ToolKind; label: string; count: number }[];
};

export function streamCounts(entries: TranscriptEntry[], graph: TranscriptEntry[] = entries): StreamCounts {
	let prompts = 0;
	let responses = 0;
	let thinking = 0;
	let tools = 0;
	let compaction = 0;
	const kinds = new Map<ToolKind, number>();
	for (const entry of entries) {
		if (entry.message?.role === 'user') prompts += 1;
		if (entry.message?.role === 'assistant') {
			const content = entry.message.content ?? [];
			if (content.some((block) => block.type === 'text')) responses += 1;
			for (const block of content) {
				if (block.type === 'thinking') thinking += 1;
				if (block.type === 'toolCall') {
					tools += 1;
					const kind = toolKind(block.name);
					kinds.set(kind, (kinds.get(kind) ?? 0) + 1);
				}
			}
		}
		if (entry.type === 'compaction') compaction += 1;
	}
	return {
		prompts,
		responses,
		thinking,
		tools,
		compaction,
		branches: leafCount(graph),
		toolKinds: [...kinds.entries()]
			.map(([kind, count]) => ({ kind, label: toolKindLabel[kind], count }))
			.sort((a, b) => a.label.localeCompare(b.label))
	};
}

export function isToolOnlyMessage(entry: TranscriptEntry) {
	const message = entry.message;
	if (!message || message.role !== 'assistant') return false;
	const content = message.content ?? [];
	return content.length > 0 && content.every((block) => block.type === 'toolCall');
}

export function groupTurns(path: TranscriptEntry[]): Turn[] {
	const turns: Turn[] = [];
	let current: Turn | null = null;

	for (const entry of path) {
		if (
			entry.type === 'session_info' ||
			entry.type === 'label' ||
			entry.type === 'model_change' ||
			entry.type === 'thinking_level_change'
		) {
			continue;
		}
		const isUser = entry.message?.role === 'user';
		if (isUser || !current) {
			current = { id: entry.id, role: isUser ? 'user' : turnRole(entry), entries: [entry] };
			turns.push(current);
			continue;
		}
		current.entries.push(entry);
	}
	return turns;
}

export function chatRows(grouped: Turn[]): Turn[] {
	return grouped.flatMap((turn) => {
		if (turn.role !== 'user') return [turn];
		const split = turn.entries.findIndex((entry) => entry.message?.role !== 'user');
		if (split <= 0) return [turn];
		return [
			{ id: turn.id, role: 'user', entries: turn.entries.slice(0, split) },
			{ id: turn.entries[split].id, role: 'assistant', entries: turn.entries.slice(split) }
		];
	});
}

export type StreamChrome = 'user' | 'agent' | 'quiet';

export type StreamRow = {
	id: string;
	chrome: StreamChrome;
	entries: TranscriptEntry[];
};

function entryChrome(entry: TranscriptEntry): StreamChrome {
	const message = entry.message;
	if (message?.role === 'user') return 'user';
	if (message?.role === 'assistant') {
		const speech = (message.content ?? []).some((block) => block.type === 'text' || block.type === 'attachment');
		return speech ? 'agent' : 'quiet';
	}
	return 'quiet';
}

function isPairedToolResult(entry: TranscriptEntry, calls: Set<string>) {
	const id = entry.message?.toolCallId;
	return entry.message?.role === 'toolResult' && Boolean(id && calls.has(id));
}

export function streamRows(path: TranscriptEntry[]): StreamRow[] {
	const calls = visibleToolCalls(path);
	return chatRows(groupTurns(path)).flatMap((turn) => {
		if (turn.role === 'user') return [{ id: turn.id, chrome: 'user' as const, entries: turn.entries }];
		return turn.entries
			.filter((entry) => !isPairedToolResult(entry, calls))
			.map((entry) => ({ id: entry.id, chrome: entryChrome(entry), entries: [entry] }));
	});
}

export type WorkRow = {
	id: string;
	chrome: 'work';
	entries: TranscriptEntry[];
	steps: StreamRow[];
};

/** Shorter runs stay visible: one "Ran go test" row says more than "Ran 1 command". */
const MIN_FOLDED_STEPS = 3;

function stepCount(rows: StreamRow[]) {
	let steps = 0;
	for (const row of rows) {
		for (const entry of row.entries) {
			for (const block of entry.message?.content ?? []) {
				if (block.type === 'toolCall' || block.type === 'thinking') steps += 1;
			}
		}
	}
	return steps;
}

/**
 * Folds each run of quiet rows (thinking and tool calls between replies) with at
 * least three steps into one work row. One Amp message can hold several steps. A row that `keepVisible` accepts, such as one that opens a child
 * trace, ends the run and stays in the stream.
 */
export function foldWork(rows: StreamRow[], keepVisible: (row: StreamRow) => boolean = () => false): (StreamRow | WorkRow)[] {
	const folded: (StreamRow | WorkRow)[] = [];
	let run: StreamRow[] = [];
	const flush = () => {
		if (stepCount(run) >= MIN_FOLDED_STEPS) {
			folded.push({ id: `work:${run[0].id}`, chrome: 'work', entries: run.flatMap((row) => row.entries), steps: run });
		} else {
			folded.push(...run);
		}
		run = [];
	};
	for (const row of rows) {
		if (row.chrome === 'quiet' && !keepVisible(row)) {
			run.push(row);
			continue;
		}
		flush();
		folded.push(row);
	}
	flush();
	return folded;
}

const workPhrases: Record<ToolKind | 'thinking', [string, string]> = {
	exec: ['ran 1 command', 'ran # commands'],
	read: ['read 1 file', 'read # files'],
	edit: ['made 1 edit', 'made # edits'],
	write: ['wrote 1 file', 'wrote # files'],
	web: ['fetched 1 page', 'fetched # pages'],
	browser: ['took 1 browser step', 'took # browser steps'],
	other: ['used 1 tool', 'used # tools'],
	thinking: ['thought once', 'thought # times']
};

const workOrder: (ToolKind | 'thinking')[] = ['exec', 'read', 'edit', 'write', 'web', 'browser', 'other', 'thinking'];

function formatDuration(ms: number) {
	const seconds = Math.round(ms / 1000);
	if (seconds < 1) return 'under a second';
	if (seconds < 60) return `${seconds}s`;
	const minutes = Math.floor(seconds / 60);
	if (minutes < 60) return seconds % 60 ? `${minutes}m ${seconds % 60}s` : `${minutes}m`;
	return minutes % 60 ? `${Math.floor(minutes / 60)}h ${minutes % 60}m` : `${Math.floor(minutes / 60)}h`;
}

/**
 * "Worked for 2m 14s · Ran 12 commands, read 3 files, and thought 4 times".
 * The duration runs from the first step to the last recorded tool result; it is
 * left out when the export has no timestamps.
 */
export function workSummary(entries: TranscriptEntry[], results: Map<string, TranscriptEntry> = new Map()) {
	const counts = new Map<ToolKind | 'thinking', number>();
	const times: number[] = [];
	const addTime = (value?: string) => {
		const time = value ? Date.parse(value) : NaN;
		if (Number.isFinite(time)) times.push(time);
	};
	for (const entry of entries) {
		addTime(entry.timestamp);
		for (const block of entry.message?.content ?? []) {
			if (block.type === 'thinking') counts.set('thinking', (counts.get('thinking') ?? 0) + 1);
			if (block.type === 'toolCall') {
				const kind = toolKind(block.name);
				counts.set(kind, (counts.get(kind) ?? 0) + 1);
				addTime(results.get(block.id)?.timestamp);
			}
		}
	}
	const phrases = workOrder
		.filter((kind) => counts.get(kind))
		.map((kind) => {
			const value = counts.get(kind) ?? 0;
			const [one, many] = workPhrases[kind];
			return value === 1 ? one : many.replace('#', String(value));
		});
	let sentence = phrases.length > 1 ? `${phrases.slice(0, -1).join(', ')}${phrases.length > 2 ? ',' : ''} and ${phrases.at(-1)}` : (phrases[0] ?? '');
	sentence = sentence.charAt(0).toUpperCase() + sentence.slice(1);
	const duration = times.length > 1 ? formatDuration(Math.max(...times) - Math.min(...times)) : '';
	return { duration: duration ? `Worked for ${duration}` : '', sentence };
}

function turnRole(entry: TranscriptEntry): Turn['role'] {
	if (entry.message?.role === 'user') return 'user';
	if (entry.message?.role === 'assistant' || entry.message?.role === 'toolResult') return 'assistant';
	if (
		entry.type === 'model_change' ||
		entry.type === 'thinking_level_change' ||
		entry.type === 'compaction' ||
		entry.type === 'branch_summary' ||
		entry.type === 'custom' ||
		entry.type === 'custom_message'
	) {
		return 'system';
	}
	return 'assistant';
}

export function visibleToolCalls(path: TranscriptEntry[]) {
	const ids = new Set<string>();
	for (const entry of path) {
		for (const block of entry.message?.content ?? []) {
			if (block.type === 'toolCall') ids.add(block.id);
		}
	}
	return ids;
}

export function entryMatchesFilter(entry: TranscriptEntry, filter: StreamFilter) {
	if (filter === 'all') return true;
	if (filter === 'prompts') return entry.message?.role === 'user';
	if (filter === 'responses') {
		return Boolean(entry.message?.role === 'assistant' && entry.message.content.some((block) => block.type === 'text'));
	}
	if (filter === 'thinking') return Boolean(entry.message?.content.some((block) => block.type === 'thinking'));
	if (filter === 'tools') {
		return Boolean(
			entry.message?.role === 'toolResult' || entry.message?.content.some((block) => block.type === 'toolCall')
		);
	}
	if (filter === 'compaction') return entry.type === 'compaction';
	return true;
}

export type SessionFacts = {
	model?: string;
	thinkingLevel?: string;
};

export function displayModel(provider?: string, modelId?: string) {
	const raw = (modelId || '').trim();
	if (!raw) return '';
	const name = raw.includes('/') ? raw.slice(raw.lastIndexOf('/') + 1) : raw;
	return name
		.replace(/^gpt-/i, 'GPT ')
		.replace(/^claude-/i, 'Claude ')
		// "opus-5-5" is version 5.5; a dated suffix such as "4-20250514" is not.
		.replace(/\b(\d)-(\d)\b/g, '$1.$2')
		.replace(/-/g, ' ')
		.replace(/\b([a-z])/g, (letter) => letter.toUpperCase())
		.replace(/\bGpt\b/g, 'GPT');
}

export type CustomNote = {
	title: string;
	body: string;
};

export function customNote(entry: TranscriptEntry): CustomNote | null {
	if (entry.type === 'custom') {
		const extension = (entry.customType || '').trim();
		return extension ? { title: `(used pi-extension ${extension})`, body: (entry.content || '').trim() } : null;
	}
	if (entry.type !== 'custom_message') return null;
	if (entry.customType === 'unknown') return { title: 'Unsupported', body: entry.content || '' };
	if (entry.customType === 'context_edit') {
		const title = entry.content ? 'Replaced an earlier entry in model context' : 'Removed an earlier entry from model context';
		return { title, body: entry.content || '' };
	}
	return { title: (entry.customType || '').trim() || entry.type, body: entry.content || '' };
}

export function pathSessionFacts(path: TranscriptEntry[]): SessionFacts {
	let model = '';
	let thinkingLevel = '';
	for (const entry of path) {
		if (entry.type === 'model_change') {
			model = displayModel(entry.provider, entry.modelId) || model;
		}
		if (entry.type === 'thinking_level_change') {
			const level = entry.thinkingLevel?.trim();
			if (level) thinkingLevel = level;
		}
		if (entry.message?.model) {
			model = displayModel(entry.message.provider, entry.message.model) || model;
		}
	}
	return {
		...(model ? { model } : {}),
		...(thinkingLevel ? { thinkingLevel } : {})
	};
}

export function toolResults(path: TranscriptEntry[]) {
	const results = new Map<string, TranscriptEntry>();
	for (const entry of path) {
		if (entry.message?.role === 'toolResult' && entry.message.toolCallId) {
			results.set(entry.message.toolCallId, entry);
		}
	}
	return results;
}

/** Tool calls on the path by recorded tool name, most used first. Errors count results marked as errors. */
export function toolNameCounts(path: TranscriptEntry[], results: Map<string, TranscriptEntry> = toolResults(path)) {
	const counts = new Map<string, { name: string; kind: ToolKind; count: number; errors: number }>();
	for (const entry of path) {
		for (const block of entry.message?.content ?? []) {
			if (block.type !== 'toolCall') continue;
			const item = counts.get(block.name) ?? { name: block.name, kind: toolKind(block.name), count: 0, errors: 0 };
			item.count += 1;
			if (results.get(block.id)?.message?.isError) item.errors += 1;
			counts.set(block.name, item);
		}
	}
	return [...counts.values()].sort((a, b) => b.count - a.count || a.name.localeCompare(b.name));
}

export type UsageTotals = {
	input: number;
	output: number;
	cacheRead: number;
	cacheWrite: number;
	/** The largest context one call sent, and the window it sent it to. */
	peakContext: number;
	contextWindow: number;
	/** Assistant messages that recorded usage. */
	messages: number;
};

/**
 * Sums recorded usage over assistant messages. The adapter puts a message's usage
 * on that message once, so blocks are not counted twice. Null when nothing recorded usage.
 */
export function usageTotals(entries: TranscriptEntry[]): UsageTotals | null {
	const totals: UsageTotals = { input: 0, output: 0, cacheRead: 0, cacheWrite: 0, peakContext: 0, contextWindow: 0, messages: 0 };
	for (const entry of entries) {
		const usage = entry.message?.role === 'assistant' ? entry.message.usage : undefined;
		if (!usage) continue;
		totals.messages += 1;
		totals.input += usage.input ?? 0;
		totals.output += usage.output ?? 0;
		totals.cacheRead += usage.cacheRead ?? 0;
		totals.cacheWrite += usage.cacheWrite ?? 0;
		if ((usage.context ?? 0) > totals.peakContext) {
			totals.peakContext = usage.context ?? 0;
			totals.contextWindow = usage.contextWindow ?? 0;
		}
	}
	return totals.messages ? totals : null;
}

/** First and last recorded timestamps, or null when the export has none. */
export function timeSpan(entries: TranscriptEntry[]) {
	let first = Infinity;
	let last = -Infinity;
	for (const entry of entries) {
		const time = entry.timestamp ? Date.parse(entry.timestamp) : NaN;
		if (!Number.isFinite(time)) continue;
		first = Math.min(first, time);
		last = Math.max(last, time);
	}
	return Number.isFinite(first) ? { start: new Date(first).toISOString(), end: new Date(last).toISOString(), duration: formatDuration(last - first) } : null;
}
