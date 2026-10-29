import { buildTranscriptSession } from './session';
import type { LoadedSession, SessionMetadata, TranscriptEvent } from './types';

export const CHANGED_MESSAGE = 'Transcript changed while loading. Reload the page to read the updated history.';
export const LOAD_FAILED_MESSAGE = 'Transcript load failed. Reload the page and sign in again.';

// Amp orders blocks by their native message order, which only the transcript
// route applies. Other harnesses read Events in chronological order.
export function transcriptPageURL(metadata: SessionMetadata, cursor: string) {
	const route = metadata.harness === 'amp' ? 'transcript' : 'events';
	const query = new URLSearchParams({ limit: '200', cursor });
	if (metadata.harness === 'amp') query.set('revision', metadata.revision || '');
	return `/api/v1/traces/${encodeURIComponent(metadata.traceId)}/${route}?${query}`;
}

async function fetchTranscriptPage(metadata: SessionMetadata, cursor: string, fetcher: typeof fetch) {
	const response = await fetcher(transcriptPageURL(metadata, cursor), { headers: { Accept: 'application/json' } });
	if (response.status === 409) throw new Error(CHANGED_MESSAGE);
	if (!response.ok || !response.headers.get('content-type')?.includes('application/json')) {
		throw new Error(LOAD_FAILED_MESSAGE);
	}
	const page = (await response.json()) as { events?: TranscriptEvent[]; next_cursor?: string };
	return { events: page.events ?? [], cursor: page.next_cursor || '' };
}

// Loads every page, then builds the session once. Building after each page
// made large sessions cost the square of their size.
export async function loadTranscriptSession(
	metadata: SessionMetadata,
	fetcher: typeof fetch = fetch
): Promise<LoadedSession> {
	const events: TranscriptEvent[] = [];
	const seen = new Set<string>();
	let cursor = '';
	let pages = 0;
	const fetchStarted = performance.now();
	do {
		const page = await fetchTranscriptPage(metadata, cursor, fetcher);
		pages += 1;
		events.push(...page.events);
		cursor = page.cursor;
		if (cursor && seen.has(cursor)) throw new Error('Transcript pagination returned a repeated cursor.');
		seen.add(cursor);
	} while (cursor);
	const fetchMs = performance.now() - fetchStarted;
	const adapterStarted = performance.now();
	const session = buildTranscriptSession(events, metadata);
	const adapterMs = performance.now() - adapterStarted;
	return { ...session, events, metadata, timing: { fetchMs, adapterMs, pages, events: events.length } };
}

// Sends the load timing and adapter anomalies to the server log, where
// GET /api/v1/logs returns them. Diagnostics must never break the page.
export async function reportLoad(session: LoadedSession) {
	const report = {
		revision: session.metadata.revision || '',
		fetch_ms: session.timing.fetchMs,
		adapter_ms: session.timing.adapterMs,
		pages: session.timing.pages,
		events: session.timing.events,
		anomalies: session.anomalies.map(({ kind, count, message, eventIds }) => ({ kind, count, message, event_ids: eventIds }))
	};
	try {
		// window.fetch, not SvelteKit's load fetch, so the token matches the cookie this POST sends.
		const csrf = await fetch('/api/v1/csrf', { credentials: 'same-origin', headers: { accept: 'application/json' } });
		const token = ((await csrf.json()) as { csrf?: string }).csrf ?? '';
		await fetch(`/traces/${encodeURIComponent(session.metadata.traceId)}/load-report`, {
			method: 'post',
			body: new URLSearchParams({ csrf: token, report: JSON.stringify(report) }),
			credentials: 'same-origin'
		});
	} catch {
		// The report is diagnostics only.
	}
}
