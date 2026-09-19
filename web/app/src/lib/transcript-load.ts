import { buildTranscriptSession, type TranscriptEvent, type TranscriptMetadata, type TranscriptSession } from './transcript-data';

export type TranscriptLoad =
	| { ok: true; session: TranscriptSession; eventCount: number }
	| { ok: false; message: string; reload: boolean };

type EventPage = { events?: TranscriptEvent[]; next_cursor?: string };

function eventsURL(metadata: TranscriptMetadata, cursor: string) {
	const params = new URLSearchParams({ limit: '200' });
	if (metadata.harness === 'amp') {
		params.set('revision', metadata.revision || '');
		if (cursor) params.set('cursor', cursor);
		return `/traces/${encodeURIComponent(metadata.traceId)}/transcript?${params}`;
	}
	if (cursor) params.set('cursor', cursor);
	return `/api/v1/traces/${encodeURIComponent(metadata.traceId)}/events?${params}`;
}

export async function loadTranscriptEvents(
	fetchFn: typeof globalThis.fetch,
	metadata: TranscriptMetadata
): Promise<TranscriptLoad> {
	const events: TranscriptEvent[] = [];
	let cursor = '';
	const cursors = new Set<string>();
	while (true) {
		const response = await fetchFn(eventsURL(metadata, cursor), {
			headers: { Accept: 'application/json' },
			credentials: 'same-origin'
		});
		if (response.status === 409) {
			return {
				ok: false,
				reload: true,
				message: 'Transcript changed while loading. Reload the page to read the updated history.'
			};
		}
		if (!response.ok || !response.headers.get('content-type')?.includes('application/json')) {
			return { ok: false, reload: false, message: 'Transcript load failed. Reload the page and sign in again.' };
		}
		const page = (await response.json()) as EventPage;
		const next = page.next_cursor || '';
		if (next && cursors.has(next)) {
			return { ok: false, reload: false, message: 'Transcript pagination returned a repeated cursor.' };
		}
		for (const event of page.events ?? []) events.push(event);
		if (!next) break;
		cursor = next;
		cursors.add(cursor);
	}
	return { ok: true, session: buildTranscriptSession(events, metadata), eventCount: events.length };
}
