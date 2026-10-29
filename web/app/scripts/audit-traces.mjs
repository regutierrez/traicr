// Runs the transcript adapter over many traces and prints one JSON line per
// trace, then a summary per harness. Use it to find which traces the page
// shows badly, and to compare two servers before and after a change.
//
//   npm run --silent audit-traces -- [--harness pi] [--limit 100] [ids...]
//
// Server and token come from TRAICR_URL and TRAICR_TOKEN, as for inspect-trace.
import { readFileSync } from 'node:fs';
import { homedir } from 'node:os';
import { join } from 'node:path';
import { parseArgs } from 'node:util';
import { runnerImport } from 'vite';

const { values, positionals } = parseArgs({
	allowPositionals: true,
	options: { harness: { type: 'string', default: '' }, limit: { type: 'string', default: '0' } }
});

function savedLogin() {
	const dir =
		process.env.TRAICR_CONFIG_DIR ||
		(process.platform === 'darwin'
			? join(homedir(), 'Library', 'Application Support', 'traicr')
			: join(process.env.XDG_CONFIG_HOME || join(homedir(), '.config'), 'traicr'));
	try {
		return JSON.parse(readFileSync(join(dir, 'collector.json'), 'utf8'));
	} catch {
		return {};
	}
}
const saved = savedLogin();
const server = (process.env.TRAICR_URL || saved.server_url || '').replace(/\/$/, '');
const token = process.env.TRAICR_TOKEN || saved.token || '';
if (!server || !token) {
	console.error('Set TRAICR_URL and TRAICR_TOKEN, or run `traicr login <url>` first.');
	process.exit(2);
}
const fetcher = (path, init = {}) =>
	fetch(server + path, { ...init, headers: { ...init.headers, Authorization: `Bearer ${token}` } });
async function getJSON(path) {
	const response = await fetcher(path);
	if (!response.ok) throw new Error(`GET ${path}: ${response.status} ${await response.text()}`);
	return response.json();
}

const { module: transcript } = await runnerImport('/src/lib/transcript/index.ts', {
	configFile: false,
	root: new URL('..', import.meta.url).pathname,
	logLevel: 'silent'
});
const { loadTranscriptSession, getPath, streamRows, customNote, leafCount, toolResults } = transcript;

let ids = positionals;
if (!ids.length) {
	const limit = Number(values.limit) || Infinity;
	let cursor = '';
	do {
		const query = new URLSearchParams({ limit: '200' });
		if (values.harness) query.set('harness', values.harness);
		if (cursor) query.set('cursor', cursor);
		const page = await getJSON(`/api/v1/cards?${query}`);
		ids.push(...page.cards.map((card) => String(card.id)));
		cursor = page.next_cursor || '';
	} while (cursor && ids.length < limit);
	ids = ids.slice(0, limit).sort((a, b) => Number(a) - Number(b));
}

// Things a reader sees as broken: a message with no text and nothing else,
// and a raw note that only names a record kind.
function emptyBlocks(entry) {
	const content = entry.message?.content ?? [];
	if (entry.message?.role === 'toolResult') return 0;
	const visible = content.filter((block) => block.type !== 'text' || block.text.trim());
	return content.length && !visible.length && !entry.message?.nativeDetails?.state ? 1 : 0;
}

const totals = new Map();
for (const id of ids) {
	let line;
	try {
		const trace = await getJSON(`/api/v1/traces/${id}`);
		const session = await loadTranscriptSession(
			{ traceId: id, nativeId: trace.native_trace_id, harness: trace.harness, title: trace.title, cwd: trace.working_directory },
			fetcher
		);
		const path = getPath(session.entries, session.leafId);
		const notes = {};
		const seen = new Set();
		let empty = 0;
		let repeated = 0;
		for (const entry of path) {
			empty += emptyBlocks(entry);
			// The same long message shown twice usually means a copy was parsed as a new message.
			const text = (entry.message?.content ?? [])
				.map((block) => (block.type === 'text' ? block.text : ''))
				.join('')
				.trim();
			if (text.length > 40 && entry.message?.role !== 'toolResult') {
				const key = `${entry.message.role}\u0000${text}`;
				if (seen.has(key)) repeated += 1;
				seen.add(key);
			}
			const note = entry.message ? null : customNote(entry);
			if (note && entry.type === 'custom_message') notes[note.title] = (notes[note.title] ?? 0) + 1;
		}
		// A call whose result exists in the trace but not on the shown path reads as "result not collected".
		const shownResults = toolResults(path);
		const allResults = toolResults(session.entries);
		let hiddenResults = 0;
		for (const entry of path) {
			for (const block of entry.message?.content ?? []) {
				if (block.type === 'toolCall' && !shownResults.has(block.id) && allResults.has(block.id)) hiddenResults += 1;
			}
		}
		const revision = trace.revisions?.[0] ?? {};
		line = {
			id: Number(id),
			harness: trace.harness,
			titled: Boolean(trace.title && trace.title !== trace.native_trace_id),
			cwd: Boolean(trace.working_directory),
			status: revision.status,
			warnings: revision.diagnostics?.length ?? 0,
			events: session.timing.events,
			entries: session.entries.length,
			branches: leafCount(session.entries),
			path: path.length,
			rows: streamRows(path).length,
			empty,
			repeated,
			hiddenResults,
			notes,
			anomalies: Object.fromEntries(session.anomalies.map((anomaly) => [anomaly.kind, anomaly.count])),
			fetchMs: Math.round(session.timing.fetchMs),
			adapterMs: Math.round(session.timing.adapterMs)
		};
	} catch (error) {
		line = { id: Number(id), error: String(error.message || error).slice(0, 300) };
	}
	console.log(JSON.stringify(line));
	const harness = line.harness || 'error';
	const total = totals.get(harness) ?? {
		traces: 0,
		untitled: 0,
		noCwd: 0,
		notNormalized: 0,
		noEvents: 0,
		empty: 0,
		repeated: 0,
		hiddenResults: 0,
		withAnomalies: 0,
		anomalies: {}
	};
	total.traces += 1;
	if (line.error) {
		totals.set(harness, total);
		continue;
	}
	if (!line.titled) total.untitled += 1;
	if (!line.cwd) total.noCwd += 1;
	if (line.status !== 'normalized') total.notNormalized += 1;
	if (!line.events) total.noEvents += 1;
	total.empty += line.empty;
	total.repeated += line.repeated;
	total.hiddenResults += line.hiddenResults;
	if (Object.keys(line.anomalies).length) total.withAnomalies += 1;
	for (const [kind, count] of Object.entries(line.anomalies)) total.anomalies[kind] = (total.anomalies[kind] ?? 0) + count;
	totals.set(harness, total);
}
for (const [harness, total] of totals) console.log(JSON.stringify({ summary: harness, ...total }));
