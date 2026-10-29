import { defaultLeafId, getPath, leafCount } from './tree';
import { visibleToolCalls } from './turns';
import type {
	AnomalyKind,
	ContentBlock,
	EventMetadata,
	Message,
	SessionMetadata,
	TranscriptAnomaly,
	TranscriptEntry,
	TranscriptEvent,
	TranscriptSession
} from './types';

const ANOMALY_SAMPLE = 20;

// Collects anomalies by kind, keeping a count, a sample of event IDs, and how
// often each detail (such as an event kind) occurred.
function anomalyLog() {
	const found = new Map<AnomalyKind, { eventIds: number[]; count: number; details: Map<string, number> }>();
	return {
		add(kind: AnomalyKind, eventId: number | undefined, detail = '') {
			let item = found.get(kind);
			if (!item) found.set(kind, (item = { eventIds: [], count: 0, details: new Map() }));
			item.count += 1;
			if (eventId !== undefined && item.eventIds.length < ANOMALY_SAMPLE) item.eventIds.push(eventId);
			if (detail) item.details.set(detail, (item.details.get(detail) ?? 0) + 1);
		},
		list(describe: (kind: AnomalyKind, count: number, details: string) => string): TranscriptAnomaly[] {
			return [...found].map(([kind, item]) => {
				const details = [...item.details].map(([detail, count]) => `${detail} (${count})`).join(', ');
				return { kind, message: describe(kind, item.count, details), eventIds: item.eventIds, count: item.count };
			});
		}
	};
}

function describeAnomaly(kind: AnomalyKind, count: number, details: string, entries: number, branches: number) {
	switch (kind) {
		case 'unpaired_tool_result':
			return `${count} tool results on the shown path have no call on that path and render as "Unpaired tool result"`;
		case 'unrendered_event':
			return `${count} events have no dedicated view and render as raw notes: ${details}`;
		case 'tool_args_not_json':
			return `${count} tool calls have arguments that are not a JSON object and show as { raw: text }: ${details}`;
		case 'missing_parent':
			return `${count} messages name a parent that is not loaded and follow the previous message instead`;
		case 'parent_cycle':
			return `${count} parent links formed a loop and were cut`;
		case 'off_path':
			return `${count} of ${entries} entries are not on the shown path (${branches} branches)`;
	}
}

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

// Returns null when the text is not a JSON object.
function parseArgs(text: string): Record<string, unknown> | null {
	let args: unknown;
	try {
		args = JSON.parse(text);
	} catch {
		return null;
	}
	if (!args || typeof args !== 'object') return null;
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
	const sorted = [...groups.values()].sort((a, b) => {
		if (Number.isInteger(a[0].metadata?.transcript_order) && Number.isInteger(b[0].metadata?.transcript_order)) {
			return (a[0].metadata?.transcript_order ?? 0) - (b[0].metadata?.transcript_order ?? 0);
		}
		return (
			(a[0].timestamp || '').localeCompare(b[0].timestamp || '') ||
			a.reduce((id, event) => Math.min(id, event.id), Infinity) - b.reduce((id, event) => Math.min(id, event.id), Infinity)
		);
	});
	// Clocks can put a child a few milliseconds before its parent. Put every recorded
	// parent first, so a parentless group never falls back onto its own descendant.
	const groupOfKey = new Map<string, TranscriptEvent[]>();
	for (const group of sorted) for (const event of group) groupOfKey.set(event.key, group);
	const ordered: TranscriptEvent[][] = [];
	const placed = new Set<TranscriptEvent[]>();
	for (const start of sorted) {
		const chain: TranscriptEvent[][] = [];
		for (let group: TranscriptEvent[] | undefined = start; group && !placed.has(group); ) {
			placed.add(group);
			chain.push(group);
			const first: TranscriptEvent = group.find((event) => event.parent_key || event.metadata?.transcript_parent) ?? group[0];
			const native: string | undefined = first.metadata?.transcript_parent;
			group = (native && groups.get(native)) || (first.parent_key ? groupOfKey.get(first.parent_key) : undefined);
		}
		ordered.push(...chain.reverse());
	}
	const entries: TranscriptEntry[] = [];
	const eventEntries = new Map<string, string>();
	const groupEntries = new Map<string, { first: TranscriptEntry; last: TranscriptEntry }>();
	const groupParents = new Map<string, { key?: string; nativeParent?: string; previous: string | null }>();
	const anomalies = anomalyLog();
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
				(!event.role || ['user', 'assistant'].includes(event.role) || event.kind === 'tool_call' || (event.kind === 'message' && event.role === 'system'))
			) {
				const role = event.role === 'user' || event.role === 'system' ? event.role : 'assistant';
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
					// Calls without arguments record nothing or null; that is not a parsing problem.
					if (!args && text.trim() && text.trim() !== 'null') anomalies.add('tool_args_not_json', event.id, event.tool || 'tool');
					content.push({
						type: 'toolCall',
						id: event.call_id || event.key,
						name: event.tool || 'tool',
						arguments: args ?? { raw: text },
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
				} else content.push({ type: details.hidden || role === 'system' ? 'hidden' : 'text', text });
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
						type: 'custom',
						timestamp: event.timestamp,
						customType: text || 'custom',
						display: true,
						content: ''
					});
				} else {
					// Pi's own custom_message and context_edit records are meant to render as notes.
					if (event.kind !== 'custom_message' && event.kind !== 'context_edit') {
						anomalies.add('unrendered_event', event.id, event.role ? `${event.kind}/${event.role}` : event.kind);
					}
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
		const found = (nativeParent && groupEntries.get(nativeParent)?.last.id) || (parent.key && eventEntries.get(parent.key));
		if (nativeParent && !found) anomalies.add('missing_parent', group.first.sources?.[0]?.eventId);
		group.first.parentId = found || parent.previous;
	}

	const visited = new Set<string>();
	for (const entry of entries) {
		if (visited.has(entry.id)) continue;
		const seen = new Set([entry.id]);
		let node: TranscriptEntry | undefined = entry;
		while (node?.parentId && entryMap.has(node.parentId) && !visited.has(node.parentId)) {
			if (seen.has(node.parentId)) {
				anomalies.add('parent_cycle', node.sources?.[0]?.eventId);
				node.parentId = null;
				break;
			}
			seen.add(node.parentId);
			node = entryMap.get(node.parentId);
		}
		for (const id of seen) visited.add(id);
	}

	const leafId = defaultLeafId(entries);
	const path = getPath(entries, leafId);
	const calls = visibleToolCalls(path);
	for (const entry of path) {
		const message = entry.message;
		if (message?.role === 'toolResult' && !(message.toolCallId && calls.has(message.toolCallId))) {
			anomalies.add('unpaired_tool_result', entry.sources?.[0]?.eventId);
		}
	}
	const onPath = new Set(path.map((entry) => entry.id));
	for (const entry of entries) {
		if (!onPath.has(entry.id)) anomalies.add('off_path', entry.sources?.[0]?.eventId);
	}
	const branches = leafCount(entries);

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
		leafId,
		eventEntries,
		anomalies: anomalies.list((kind, count, details) => describeAnomaly(kind, count, details, entries.length, branches))
	};
}
