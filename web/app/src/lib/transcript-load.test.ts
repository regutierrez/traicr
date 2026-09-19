import assert from 'node:assert/strict';
import { test } from 'node:test';
import { loadTranscriptEvents } from './transcript-load';
import type { TranscriptMetadata } from './transcript-data';

const metadata: TranscriptMetadata = {
	nativeId: 'session',
	harness: 'pi',
	title: 'A whole conversation',
	traceId: '1'
};

test('Amp pages /traces/{id}/transcript until no cursor and treats 409 as reload', async () => {
	let calls = 0;
	const fetchFn: typeof fetch = async (input) => {
		calls += 1;
		const url = String(input);
		assert.ok(url.startsWith('/traces/1/transcript?'));
		assert.equal(new URL(url, 'https://example.com').searchParams.get('revision'), '7');
		if (calls === 1) {
			return new Response(
				JSON.stringify({
					events: [
						{
							id: 1,
							key: 'message:m:0',
							kind: 'message',
							role: 'assistant',
							text: 'Block 1',
							metadata: { transcript_message: 'm', transcript_order: 0, transcript_block: 1 }
						}
					],
					next_cursor: 'second-page'
				}),
				{ headers: { 'content-type': 'application/json' } }
			);
		}
		return new Response(JSON.stringify({ error: { code: 'transcript_changed' } }), {
			status: 409,
			headers: { 'content-type': 'application/json' }
		});
	};
	const result = await loadTranscriptEvents(fetchFn, { ...metadata, harness: 'amp', revision: '7' });
	assert.equal(result.ok, false);
	if (!result.ok) {
		assert.equal(result.reload, true);
		assert.match(result.message, /Transcript changed.*Reload/);
	}
	assert.equal(calls, 2);
});

test('other harnesses page /api/v1/traces/{id}/events until no cursor', async () => {
	let calls = 0;
	const fetchFn: typeof fetch = async (input) => {
		calls += 1;
		const url = String(input);
		assert.ok(url.startsWith('/api/v1/traces/1/events?'));
		return new Response(
			JSON.stringify({
				events: [{ id: calls, key: `message:m${calls}:0`, kind: 'message', role: 'user', text: `Page ${calls}` }],
				next_cursor: calls === 1 ? 'second-page' : ''
			}),
			{ headers: { 'content-type': 'application/json' } }
		);
	};
	const result = await loadTranscriptEvents(fetchFn, metadata);
	assert.equal(result.ok, true);
	if (result.ok) {
		assert.equal(result.eventCount, 2);
		assert.equal(result.session.entries.length, 2);
	}
	assert.equal(calls, 2);
});
