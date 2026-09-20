import type { TranscriptEntry, Turn } from './types';

export function groupTurns(path: TranscriptEntry[]): Turn[] {
	const turns: Turn[] = [];
	let current: Turn | null = null;

	for (const entry of path) {
		if (entry.type === 'session_info' || entry.type === 'label') continue;
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

function turnRole(entry: TranscriptEntry): Turn['role'] {
	if (entry.message?.role === 'user') return 'user';
	if (entry.message?.role === 'assistant' || entry.message?.role === 'toolResult') return 'assistant';
	if (
		entry.type === 'model_change' ||
		entry.type === 'thinking_level_change' ||
		entry.type === 'compaction' ||
		entry.type === 'branch_summary' ||
		entry.type === 'custom_message'
	) {
		return 'system';
	}
	return 'assistant';
}

export const PAGE_SIZE = 200;

export function pageWindow(path: TranscriptEntry[], targetId: string, start?: number) {
	const targetIndex = Math.max(0, path.findIndex((entry) => entry.id === targetId));
	const from = start ?? Math.max(0, targetIndex - 100);
	const to = Math.min(path.length, from + PAGE_SIZE);
	return { from, to, hasEarlier: from > 0, hasLater: to < path.length };
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

export function toolResults(path: TranscriptEntry[]) {
	const results = new Map<string, TranscriptEntry>();
	for (const entry of path) {
		if (entry.message?.role === 'toolResult' && entry.message.toolCallId) {
			results.set(entry.message.toolCallId, entry);
		}
	}
	return results;
}
