// Adapt Traicr's merged, normalized records into session entries and Turns.
// Native source files remain authoritative and are available through the details page.

export type EventMetadata = {
	transcript_message?: string;
	transcript_block?: number;
	transcript_order?: number;
	transcript_parent?: string;
	session?: SessionNative;
	state?: { stopReason?: string; type?: string };
	usage?: {
		inputTokens?: number;
		outputTokens?: number;
		cacheReadInputTokens?: number;
		cacheCreationInputTokens?: number;
	};
	hidden?: boolean;
	source_pointer?: string;
	run?: { status?: string; result?: { exitCode?: number; [key: string]: unknown } };
	[key: string]: unknown;
};

export type SessionNative = {
	title?: string;
	created?: string;
	agentMode?: string;
	archived?: unknown;
	activatedSkills?: unknown;
	meta?: { features?: unknown; executorType?: unknown };
	env?: { initial?: { workingDirectory?: string } };
	[key: string]: unknown;
};

export type TranscriptAttachment = {
	name?: string;
	media_type?: string;
	path?: string;
	url?: string;
	source_pointer?: string;
	inline?: boolean;
	archived_path?: string;
	size?: number;
	revisionId?: number;
	[key: string]: unknown;
};

export type TranscriptEvent = {
	id: number;
	key: string;
	parent_key?: string;
	branch?: string;
	kind: string;
	role?: string;
	model?: string;
	provider?: string;
	tool?: string;
	call_id?: string;
	timestamp?: string;
	text: string;
	revision_id?: number;
	aliases?: string[];
	attachments?: TranscriptAttachment[];
	metadata?: EventMetadata;
};

export type TranscriptMetadata = {
	traceId: string;
	nativeId: string;
	harness: string;
	title?: string;
	cwd?: string;
	revision?: string;
};

export type ContentBlock =
	| { type: 'text'; text: string }
	| { type: 'hidden'; text: string }
	| { type: 'thinking'; thinking: string }
	| { type: 'toolCall'; id: string; name: string; arguments: Record<string, unknown>; details?: EventMetadata }
	| ({ type: 'attachment' } & TranscriptAttachment);

export type SessionMessage = {
	role: string;
	content: ContentBlock[];
	model?: string;
	provider?: string;
	stopReason?: string;
	nativeDetails?: EventMetadata;
	usage?: { input?: number; output?: number; cacheRead?: number; cacheWrite?: number };
	toolCallId?: string;
	toolName?: string;
	run?: EventMetadata['run'];
	isError?: boolean;
};

export type EntrySource = {
	eventId: number;
	key: string;
	revisionId?: number;
	pointer?: string;
	details?: EventMetadata;
};

export type SessionEntry = {
	id: string;
	parentId: string | null;
	type: string;
	timestamp?: string;
	sources?: EntrySource[];
	message?: SessionMessage;
	modelId?: string;
	provider?: string;
	summary?: string;
	tokensBefore?: number;
	thinkingLevel?: string;
	customType?: string;
	display?: boolean;
	content?: string;
};

export type TranscriptHeader = {
	id: string;
	harness: string;
	title?: string;
	timestamp?: string;
	cwd?: string;
	native?: SessionNative;
};

export type TranscriptSession = {
	header: TranscriptHeader;
	entries: SessionEntry[];
	leafId?: string;
	eventEntries: Map<string, string>;
};

export type ConversationTurn = {
	id: string;
	user?: SessionEntry;
	follow: SessionEntry[];
};

function nativeMessageKey(event: { id?: number; key?: string; metadata?: EventMetadata }) {
	if (event.metadata?.transcript_message) return event.metadata.transcript_message;
	const key = event.key || String(event.id);
	const native = key.slice(key.indexOf(':') + 1);
	if (native.startsWith('sha256:')) return key;
	return native.replace(/:\d+$/, '');
}

function blockIndex(event: TranscriptEvent) {
	if (Number.isInteger(event.metadata?.transcript_block)) return event.metadata!.transcript_block as number;
	const match = event.key?.match(/:(\d+)$/);
	return match ? Number(match[1]) : event.id;
}

// buildTranscriptSession groups content blocks into turns and retains parent branches.
export function buildTranscriptSession(events: TranscriptEvent[], metadata: TranscriptMetadata): TranscriptSession {
	const native = events.find((event) => event.metadata?.session)?.metadata?.session;
	const groups = new Map<string, TranscriptEvent[]>();
	for (const event of events) {
		const key = nativeMessageKey(event);
		if (!groups.has(key)) groups.set(key, []);
		groups.get(key)!.push(event);
	}
	const ordered = [...groups.values()].sort((a, b) => {
		if (Number.isInteger(a[0].metadata?.transcript_order) && Number.isInteger(b[0].metadata?.transcript_order)) {
			return a[0].metadata!.transcript_order! - b[0].metadata!.transcript_order!;
		}
		return (
			(a[0].timestamp || '').localeCompare(b[0].timestamp || '') ||
			a.reduce((id, e) => Math.min(id, e.id), Infinity) - b.reduce((id, e) => Math.min(id, e.id), Infinity)
		);
	});
	const entries: SessionEntry[] = [];
	const eventEntries = new Map<string, string>();
	const groupEntries = new Map<string, { first: SessionEntry; last: SessionEntry }>();
	const groupParents = new Map<string, { key?: string; nativeParent?: string; previous: string | null }>();
	let previous: string | null = null;
	for (const group of ordered) {
		group.sort((a, b) => blockIndex(a) - blockIndex(b) || a.id - b.id);
		const first = group[0];
		const groupKey = nativeMessageKey(first);
		let messageEntry: SessionEntry | null = null;
		let usageAssigned = false;
		const added: SessionEntry[] = [];
		function append(entry: SessionEntry) {
			entry.id = String(entry.id);
			entry.parentId = added.at(-1)?.id || null;
			entries.push(entry);
			added.push(entry);
			return entry;
		}
		for (const event of group) {
			const text = String(event.text || '');
			const details = event.metadata || {};
			let entry: SessionEntry;
			if (
				['message', 'reasoning', 'tool_call', 'attachment'].includes(event.kind) &&
				(!event.role || ['user', 'assistant'].includes(event.role) || event.kind === 'tool_call')
			) {
				const role = event.role === 'user' ? 'user' : 'assistant';
				if (!messageEntry || messageEntry.message?.role !== role) {
					let stopReason = details.state?.stopReason || details.state?.type || (metadata.harness === 'amp' ? undefined : 'stop');
					if (details.state?.type === 'cancelled') stopReason = 'aborted';
					else if (details.state?.type === 'error') stopReason = 'error';
					else if (details.state?.stopReason === 'tool_use') stopReason = 'toolUse';
					else if (details.state?.stopReason === 'end_turn') stopReason = 'stop';
					messageEntry = append({
						id: String(event.id),
						parentId: null,
						type: 'message',
						timestamp: event.timestamp,
						message: {
							role,
							content: [],
							model: event.model,
							provider: event.provider,
							stopReason,
							nativeDetails: details,
							usage:
								!usageAssigned && details.usage
									? {
											input: details.usage.inputTokens,
											output: details.usage.outputTokens,
											cacheRead: details.usage.cacheReadInputTokens,
											cacheWrite: details.usage.cacheCreationInputTokens
										}
									: undefined
						}
					});
					if (role === 'assistant' && details.usage) usageAssigned = true;
				}
				entry = messageEntry;
				if (event.kind === 'reasoning') entry.message!.content.push({ type: 'thinking', thinking: text });
				else if (event.kind === 'tool_call') {
					let args: unknown;
					try {
						args = JSON.parse(text);
					} catch {
						args = { raw: text };
					}
					if (!args || typeof args !== 'object') args = { raw: text };
					entry.message!.content.push({
						type: 'toolCall',
						id: event.call_id || event.key,
						name: event.tool || 'tool',
						arguments: args as Record<string, unknown>,
						details
					});
				} else if (event.kind === 'attachment') {
					entry.message!.content.push(
						...(event.attachments || []).map((a) => ({ type: 'attachment' as const, ...a, revisionId: event.revision_id }))
					);
				} else entry.message!.content.push({ type: details.hidden ? 'hidden' : 'text', text });
			} else if (event.kind === 'tool_result') {
				const existing = added.find((e) => e.message?.role === 'toolResult' && e.message.toolCallId === event.call_id);
				entry =
					existing ||
					append({
						id: String(event.id),
						parentId: null,
						type: 'message',
						timestamp: event.timestamp,
						message: {
							role: 'toolResult',
							toolCallId: event.call_id,
							toolName: event.tool,
							content: [],
							run: details.run,
							isError:
								details.run?.status === 'error' ||
								(typeof details.run?.result?.exitCode === 'number' && details.run.result.exitCode !== 0)
						}
					});
				entry.message!.content.push({ type: 'text', text });
				entry.message!.content.push(
					...(event.attachments || []).map((a) => ({ type: 'attachment' as const, ...a, revisionId: event.revision_id }))
				);
				messageEntry = null;
			} else {
				messageEntry = null;
				if (event.kind === 'model_change')
					entry = append({
						id: String(event.id),
						parentId: null,
						type: 'model_change',
						timestamp: event.timestamp,
						modelId: event.model || text,
						provider: event.provider || ''
					});
				else if (event.kind === 'compaction' || event.kind === 'branch_summary')
					entry = append({
						id: String(event.id),
						parentId: null,
						type: event.kind,
						timestamp: event.timestamp,
						summary: text,
						tokensBefore: 0
					});
				else if (event.kind === 'thinking_level_change')
					entry = append({
						id: String(event.id),
						parentId: null,
						type: event.kind,
						timestamp: event.timestamp,
						thinkingLevel: text
					});
				else if (event.kind === 'session_info' || event.kind === 'label')
					entry = append({ id: String(event.id), parentId: null, type: event.kind, timestamp: event.timestamp });
				else
					entry = append({
						id: String(event.id),
						parentId: null,
						type: 'custom_message',
						timestamp: event.timestamp,
						customType: event.kind,
						display: true,
						content: text
					});
			}
			entry.sources ||= [];
			entry.sources.push({
				eventId: event.id,
				key: event.key,
				revisionId: event.revision_id,
				pointer: details.source_pointer,
				details
			});
			eventEntries.set(String(event.id), entry.id);
			eventEntries.set(event.key, entry.id);
			for (const alias of event.aliases || []) eventEntries.set(alias, entry.id);
		}
		if (added.length) {
			groupEntries.set(groupKey, { first: added[0], last: added.at(-1)! });
			groupParents.set(groupKey, { key: first.parent_key, nativeParent: first.metadata?.transcript_parent, previous });
			previous = added.at(-1)!.id;
		}
	}
	const entryMap = new Map(entries.map((e) => [e.id, e]));
	for (const [key, group] of groupEntries) {
		const parent = groupParents.get(key)!;
		const nativeParent = parent.nativeParent || (parent.key && nativeMessageKey({ key: parent.key }));
		group.first.parentId =
			(nativeParent ? groupEntries.get(nativeParent)?.last.id : undefined) ||
			(parent.key ? eventEntries.get(parent.key) : undefined) ||
			parent.previous;
	}
	// A malformed native graph must not hang parent walkers.
	const visited = new Set<string>();
	for (const entry of entries) {
		if (visited.has(entry.id)) continue;
		const seen = new Set([entry.id]);
		let node: SessionEntry | undefined = entry;
		while (node.parentId && entryMap.has(node.parentId) && !visited.has(node.parentId)) {
			if (seen.has(node.parentId)) {
				node.parentId = null;
				break;
			}
			seen.add(node.parentId);
			node = entryMap.get(node.parentId);
			if (!node) break;
		}
		for (const id of seen) visited.add(id);
	}
	return {
		header: {
			id: metadata.nativeId,
			harness: metadata.harness,
			title: native?.title || metadata.title,
			timestamp: native?.created || entries.find((e) => e.timestamp)?.timestamp,
			cwd: native?.env?.initial?.workingDirectory || metadata.cwd,
			native
		},
		entries,
		leafId: entries.at(-1)?.id,
		eventEntries
	};
}

export function graphHasFork(entries: SessionEntry[]) {
	const children = new Map<string, number>();
	for (const entry of entries) {
		if (!entry.parentId) continue;
		const count = (children.get(entry.parentId) ?? 0) + 1;
		if (count >= 2) return true;
		children.set(entry.parentId, count);
	}
	return false;
}

export function pathToEntry(entries: SessionEntry[], id: string) {
	const byId = new Map(entries.map((entry) => [entry.id, entry]));
	if (!id || !byId.has(id)) return entries;
	const path: SessionEntry[] = [];
	const seen = new Set<string>();
	let current = byId.get(id);
	while (current && !seen.has(current.id)) {
		seen.add(current.id);
		path.push(current);
		current = current.parentId ? byId.get(current.parentId) : undefined;
	}
	return path.reverse();
}

export function turnsFromEntries(entries: SessionEntry[]): ConversationTurn[] {
	const turns: ConversationTurn[] = [];
	for (const entry of entries) {
		if (entry.type === 'session_info' || entry.type === 'label') continue;
		if (entry.type === 'message' && entry.message?.role === 'user') {
			turns.push({ id: entry.id, user: entry, follow: [] });
			continue;
		}
		const current = turns.at(-1);
		if (!current) turns.push({ id: entry.id, follow: [entry] });
		else current.follow.push(entry);
	}
	return turns;
}

export function rootsOf(entries: SessionEntry[]) {
	const ids = new Set(entries.map((entry) => entry.id));
	return entries.filter((entry) => !entry.parentId || !ids.has(entry.parentId));
}

export function childrenOf(entries: SessionEntry[], parentId: string) {
	return entries.filter((entry) => entry.parentId === parentId);
}

export function entryLabel(entry: SessionEntry) {
	if (entry.type === 'message' && entry.message) {
		if (entry.message.role === 'toolResult') return `tool · ${entry.message.toolName || 'result'}`;
		const block = entry.message.content.find((item) => item.type === 'text' && 'text' in item && item.text);
		const thinking = entry.message.content.find((item) => item.type === 'thinking');
		const tool = entry.message.content.find((item) => item.type === 'toolCall');
		const text =
			(block && block.type === 'text' && block.text) ||
			(thinking && thinking.type === 'thinking' && thinking.thinking) ||
			(tool && tool.type === 'toolCall' && tool.name) ||
			entry.message.role;
		return `${entry.message.role}: ${truncate(String(text))}`;
	}
	if (entry.type === 'model_change') return `model · ${entry.modelId || ''}`.trim();
	if (entry.summary) return truncate(entry.summary);
	return entry.customType || entry.type;
}

function truncate(text: string, limit = 72) {
	const compact = text.replace(/\s+/g, ' ').trim();
	return compact.length > limit ? `${compact.slice(0, limit)}…` : compact || 'untitled';
}
