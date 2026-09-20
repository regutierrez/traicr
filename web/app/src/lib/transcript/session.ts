import { defaultLeafId } from './tree';
import type {
	ContentBlock,
	EventMetadata,
	Message,
	SessionMetadata,
	TranscriptEntry,
	TranscriptEvent,
	TranscriptSession
} from './types';

function nativeMessageKey(event: TranscriptEvent) {
	if (event.metadata?.transcript_message) return event.metadata.transcript_message;
	const key = event.key || String(event.id);
	const native = key.slice(key.indexOf(':') + 1);
	if (native.startsWith('sha256:')) return key;
	return native.replace(/:\d+$/, '');
}

function blockIndex(event: TranscriptEvent) {
	if (Number.isInteger(event.metadata?.transcript_block)) return event.metadata?.transcript_block as number;
	const match = event.key?.match(/:(\d+)$/);
	return match ? Number(match[1]) : event.id;
}

function parseArgs(text: string): Record<string, unknown> {
	let args: unknown;
	try {
		args = JSON.parse(text);
	} catch {
		args = { raw: text };
	}
	if (!args || typeof args !== 'object') return { raw: text };
	return args as Record<string, unknown>;
}

function stopReasonFor(details: EventMetadata, harness: string) {
	let stopReason = details.state?.stopReason || details.state?.type || (harness === 'amp' ? undefined : 'stop');
	if (details.state?.type === 'cancelled') stopReason = 'aborted';
	else if (details.state?.type === 'error') stopReason = 'error';
	else if (details.state?.stopReason === 'tool_use') stopReason = 'toolUse';
	else if (details.state?.stopReason === 'end_turn') stopReason = 'stop';
	return stopReason;
}

// Groups content blocks into message entries and retains parent branches.
export function buildTranscriptSession(events: TranscriptEvent[], metadata: SessionMetadata): TranscriptSession {
	const native = events.find((event) => event.metadata?.session)?.metadata?.session;
	const groups = new Map<string, TranscriptEvent[]>();
	for (const event of events) {
		const key = nativeMessageKey(event);
		const group = groups.get(key);
		if (group) group.push(event);
		else groups.set(key, [event]);
	}
	const ordered = [...groups.values()].sort((a, b) => {
		if (Number.isInteger(a[0].metadata?.transcript_order) && Number.isInteger(b[0].metadata?.transcript_order)) {
			return (a[0].metadata?.transcript_order ?? 0) - (b[0].metadata?.transcript_order ?? 0);
		}
		return (
			(a[0].timestamp || '').localeCompare(b[0].timestamp || '') ||
			a.reduce((id, event) => Math.min(id, event.id), Infinity) - b.reduce((id, event) => Math.min(id, event.id), Infinity)
		);
	});
	const entries: TranscriptEntry[] = [];
	const eventEntries = new Map<string, string>();
	const groupEntries = new Map<string, { first: TranscriptEntry; last: TranscriptEntry }>();
	const groupParents = new Map<string, { key?: string; nativeParent?: string; previous: string | null }>();
	let previous: string | null = null;

	for (const group of ordered) {
		group.sort((a, b) => blockIndex(a) - blockIndex(b) || a.id - b.id);
		const first = group[0];
		const groupKey = nativeMessageKey(first);
		let messageEntry: TranscriptEntry | null = null;
		let usageAssigned = false;
		const added: TranscriptEntry[] = [];

		function append(entry: TranscriptEntry) {
			entry.id = String(entry.id);
			entry.parentId = added.at(-1)?.id || null;
			entries.push(entry);
			added.push(entry);
			return entry;
		}

		for (const event of group) {
			const text = String(event.text || '');
			const details = event.metadata || {};
			let entry: TranscriptEntry;
			if (
				['message', 'reasoning', 'tool_call', 'attachment'].includes(event.kind) &&
				(!event.role || ['user', 'assistant'].includes(event.role) || event.kind === 'tool_call')
			) {
				const role = event.role === 'user' ? 'user' : 'assistant';
				if (!messageEntry || messageEntry.message?.role !== role) {
					const stopReason = stopReasonFor(details, metadata.harness);
					const message: Message = {
						role,
						content: [],
						model: event.model,
						provider: event.provider,
						stopReason,
						nativeDetails: details
					};
					if (!usageAssigned && details.usage) {
						message.usage = {
							input: details.usage.inputTokens,
							output: details.usage.outputTokens,
							cacheRead: details.usage.cacheReadInputTokens,
							cacheWrite: details.usage.cacheCreationInputTokens
						};
					}
					messageEntry = append({
						id: String(event.id),
						parentId: null,
						type: 'message',
						timestamp: event.timestamp,
						message
					});
					if (role === 'assistant' && details.usage) usageAssigned = true;
				}
				entry = messageEntry;
				const content = entry.message?.content as ContentBlock[];
				if (event.kind === 'reasoning') content.push({ type: 'thinking', thinking: text });
				else if (event.kind === 'tool_call') {
					const args = parseArgs(text);
					content.push({
						type: 'toolCall',
						id: event.call_id || event.key,
						name: event.tool || 'tool',
						arguments: args,
						details
					});
				} else if (event.kind === 'attachment') {
					content.push(
						...(event.attachments || []).map((attachment) => ({
							...attachment,
							type: 'attachment' as const,
							revisionId: event.revision_id
						}))
					);
				} else content.push({ type: details.hidden ? 'hidden' : 'text', text });
			} else if (event.kind === 'tool_result') {
				const existing = added.find((item) => item.message?.role === 'toolResult' && item.message.toolCallId === event.call_id);
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
				entry.message?.content.push({ type: 'text', text });
				entry.message?.content.push(
					...(event.attachments || []).map((attachment) => ({
						...attachment,
						type: 'attachment' as const,
						revisionId: event.revision_id
					}))
				);
				messageEntry = null;
			} else {
				messageEntry = null;
				if (event.kind === 'model_change') {
					entry = append({
						id: String(event.id),
						parentId: null,
						type: 'model_change',
						timestamp: event.timestamp,
						modelId: event.model || text,
						provider: event.provider || ''
					});
				} else if (event.kind === 'compaction' || event.kind === 'branch_summary') {
					entry = append({
						id: String(event.id),
						parentId: null,
						type: event.kind,
						timestamp: event.timestamp,
						summary: text,
						tokensBefore: 0
					});
				} else if (event.kind === 'thinking_level_change') {
					entry = append({
						id: String(event.id),
						parentId: null,
						type: event.kind,
						timestamp: event.timestamp,
						thinkingLevel: text
					});
				} else if (event.kind === 'session_info' || event.kind === 'label') {
					entry = append({
						id: String(event.id),
						parentId: null,
						type: event.kind,
						timestamp: event.timestamp
					});
				} else if (event.kind === 'custom') {
					entry = append({
						id: String(event.id),
						parentId: null,
						type: 'custom_message',
						timestamp: event.timestamp,
						customType: text || 'custom',
						display: true,
						content: ''
					});
				} else {
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
			groupEntries.set(groupKey, { first: added[0], last: added.at(-1) as TranscriptEntry });
			groupParents.set(groupKey, {
				key: first.parent_key,
				nativeParent: first.metadata?.transcript_parent,
				previous
			});
			previous = added.at(-1)?.id ?? null;
		}
	}

	const entryMap = new Map(entries.map((entry) => [entry.id, entry]));
	for (const [key, group] of groupEntries) {
		const parent = groupParents.get(key);
		if (!parent) continue;
		const nativeParent = parent.nativeParent || (parent.key && nativeMessageKey({ key: parent.key } as TranscriptEvent));
		group.first.parentId = (nativeParent && groupEntries.get(nativeParent)?.last.id) || (parent.key && eventEntries.get(parent.key)) || parent.previous;
	}

	const visited = new Set<string>();
	for (const entry of entries) {
		if (visited.has(entry.id)) continue;
		const seen = new Set([entry.id]);
		let node: TranscriptEntry | undefined = entry;
		while (node?.parentId && entryMap.has(node.parentId) && !visited.has(node.parentId)) {
			if (seen.has(node.parentId)) {
				node.parentId = null;
				break;
			}
			seen.add(node.parentId);
			node = entryMap.get(node.parentId);
		}
		for (const id of seen) visited.add(id);
	}

	return {
		header: {
			id: metadata.nativeId,
			harness: metadata.harness,
			title: native?.title || metadata.title,
			timestamp: native?.created || entries.find((entry) => entry.timestamp)?.timestamp,
			cwd: native?.env?.initial?.workingDirectory || metadata.cwd,
			native
		},
		entries,
		leafId: defaultLeafId(entries),
		eventEntries
	};
}
