import { buildTranscriptSession } from './session';
import type { LoadedSession, SessionMetadata, TranscriptEvent } from './types';

export const CHANGED_MESSAGE = 'Transcript changed while loading. Reload the page to read the updated history.';
export const LOAD_FAILED_MESSAGE = 'Transcript load failed. Reload the page and sign in again.';

function statusFor(events: TranscriptEvent[], metadata: SessionMetadata, hasMore: boolean) {
	if (!events.length) {
		return 'No normalized messages. Open Session details to inspect the retained source files.';
	}
	const view = metadata.revision ? 'Selected revision' : 'Merged archive';
	const more = hasMore ? ' · More records available below' : ' · All records loaded';
	return `${view} · ${events.length} records loaded${more}. Original records and parsing warnings are in Session details.`;
}

function assemble(events: TranscriptEvent[], metadata: SessionMetadata, cursor: string, seen: string[]): LoadedSession {
	return {
		...buildTranscriptSession(events, metadata),
		hasMore: !!cursor,
		status: statusFor(events, metadata, !!cursor),
		cursor,
		events,
		seenCursors: seen,
		metadata
	};
}

export async function fetchTranscriptPage(
	metadata: SessionMetadata,
	cursor: string,
	seen: string[],
	fetcher: typeof fetch = fetch
) {
	const route = metadata.harness === 'amp' ? 'transcript' : 'events';
	const url = `/traces/${encodeURIComponent(metadata.traceId)}/${route}?limit=200&revision=${encodeURIComponent(metadata.revision || '')}&cursor=${encodeURIComponent(cursor)}`;
	const response = await fetcher(url, { headers: { Accept: 'application/json' } });
	if (response.status === 409) throw new Error(CHANGED_MESSAGE);
	if (!response.ok || !response.headers.get('content-type')?.includes('application/json')) {
		throw new Error(LOAD_FAILED_MESSAGE);
	}
	const page = (await response.json()) as { events?: TranscriptEvent[]; next_cursor?: string };
	const next = page.next_cursor || '';
	if (next && seen.includes(next)) throw new Error('Transcript pagination returned a repeated cursor.');
	return { events: page.events ?? [], cursor: next, seen: [...seen, next] };
}

export async function loadMoreSession(session: LoadedSession, fetcher: typeof fetch = fetch): Promise<LoadedSession> {
	const page = await fetchTranscriptPage(session.metadata, session.cursor, session.seenCursors, fetcher);
	return assemble([...session.events, ...page.events], session.metadata, page.cursor, page.seen);
}

export async function loadTranscriptSession(
	metadata: SessionMetadata,
	fetcher: typeof fetch = fetch
): Promise<LoadedSession> {
	let events: TranscriptEvent[] = [];
	let cursor = '';
	let seen: string[] = [];
	const first = await fetchTranscriptPage(metadata, cursor, seen, fetcher);
	events = first.events;
	cursor = first.cursor;
	seen = first.seen;
	let data = assemble(events, metadata, cursor, seen);
	if (metadata.harness !== 'amp') {
		while (data.hasMore) {
			data = await loadMoreSession(data, fetcher);
		}
	}
	return data;
}
