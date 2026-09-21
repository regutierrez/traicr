import { leafCount } from './tree';
import { indexToolGroup, type ToolKind } from './tools';
import type { TranscriptEntry, Turn } from './types';

export type StreamFilter = 'all' | 'prompts' | 'responses' | 'thinking' | 'tools' | 'compaction' | 'branches';

export type StreamCounts = {
	prompts: number;
	responses: number;
	thinking: number;
	tools: number;
	compaction: number;
	branches: number;
	toolKinds: { id: string; kind: ToolKind; label: string; count: number }[];
};

export function streamCounts(entries: TranscriptEntry[], graph: TranscriptEntry[] = entries): StreamCounts {
	let prompts = 0;
	let responses = 0;
	let thinking = 0;
	let tools = 0;
	let compaction = 0;
	const kinds = new Map<string, { id: string; kind: ToolKind; label: string; count: number }>();
	for (const entry of entries) {
		if (entry.message?.role === 'user') prompts += 1;
		if (entry.message?.role === 'assistant') {
			const content = entry.message.content ?? [];
			if (content.some((block) => block.type === 'text')) responses += 1;
			for (const block of content) {
				if (block.type === 'thinking') thinking += 1;
				if (block.type === 'toolCall') {
					tools += 1;
					const group = indexToolGroup(block.name);
					const current = kinds.get(group.id);
					if (current) current.count += 1;
					else kinds.set(group.id, { ...group, count: 1 });
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
		toolKinds: [...kinds.values()].sort((a, b) => b.count - a.count || a.label.localeCompare(b.label))
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
	if (filter === 'all' || filter === 'branches') return true;
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
