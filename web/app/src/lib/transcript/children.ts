import type { TraceRef } from '../types';
import { ampThreadURL, normalizeThreadID } from './tools';
import type { ToolCallBlock, TranscriptEntry } from './types';

export type ChildCard = {
	id: string;
	title: string;
	collected: boolean;
	traceId?: number;
	href?: string;
	originalHref?: string;
	meta: string;
};

export function childCards(
	entry: TranscriptEntry,
	call: ToolCallBlock | undefined,
	result: TranscriptEntry | undefined,
	children: TraceRef[]
): ChildCard[] {
	const ids = new Set<string>();
	const cards: ChildCard[] = [];
	const consider = (raw: string | undefined, label: string) => {
		if (!raw) return;
		const id = normalizeThreadID(raw);
		if (!id || ids.has(id)) return;
		ids.add(id);
		const match = children.find((child) => child.native_trace_id === id);
		cards.push({
			id,
			title: match?.title || id,
			collected: Boolean(match),
			traceId: match?.id,
			originalHref: ampThreadURL(id) || undefined,
			meta: label
		});
	};

	if (call) {
		for (const key of ['thread', 'threadID', 'thread_id']) {
			const value = call.arguments?.[key];
			if (typeof value === 'string') consider(value, call.name === 'send_message_to_thread' ? 'Message to thread' : 'Referenced thread');
		}
	}
	const payload = result?.message?.run?.result;
	if (typeof payload?.threadID === 'string') {
		consider(payload.threadID, call?.name === 'create_thread' ? 'Spawned thread' : 'Referenced thread');
	}
	const from = entry.message?.nativeDetails?.message_meta?.fromExecutorThreadID;
	if (from) consider(from, 'From thread');
	return cards;
}

export function unplacedChildren(children: TraceRef[], placed: Set<string>) {
	return children.filter((child) => !placed.has(child.native_trace_id));
}
