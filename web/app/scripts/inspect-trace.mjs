// Prints a trace the way the transcript page shows it: the same API pages and
// the same adapter code, run in Node. See AGENTS.md.
//
//   npm run inspect-trace -- <trace-id> [--revision <id>] [--json] [--full]
//
// The server and admin token come from TRAICR_URL and TRAICR_TOKEN, or from
// the collector config `traicr login` saved.
import { readFileSync } from 'node:fs';
import { homedir } from 'node:os';
import { join } from 'node:path';
import { parseArgs } from 'node:util';
import { runnerImport } from 'vite';

const { values, positionals } = parseArgs({
	allowPositionals: true,
	options: { revision: { type: 'string', default: '' }, json: { type: 'boolean' }, full: { type: 'boolean' } }
});
if (positionals.length !== 1 || !/^\d+$/.test(positionals[0])) {
	console.error('usage: npm run inspect-trace -- <trace-id> [--revision <id>] [--json] [--full]');
	process.exit(2);
}
const traceId = positionals[0];

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

// The page's own fetches are relative; send them to the server with the token.
const fetcher = (path, init = {}) =>
	fetch(server + path, { ...init, headers: { ...init.headers, Authorization: `Bearer ${token}` } });

const { module: transcript } = await runnerImport('/src/lib/transcript/index.ts', {
	configFile: false,
	root: new URL('..', import.meta.url).pathname,
	logLevel: 'silent'
});
const { loadTranscriptSession, getPath, streamRows, toolResults, toolChipLabel, toolStatus, customNote, leafCount } = transcript;

const response = await fetcher(`/api/v1/traces/${traceId}`);
if (!response.ok) {
	console.error(`GET /api/v1/traces/${traceId}: ${response.status} ${await response.text()}`);
	process.exit(1);
}
const trace = await response.json();
const session = await loadTranscriptSession(
	{
		traceId,
		nativeId: trace.native_trace_id,
		harness: trace.harness,
		title: trace.title,
		cwd: trace.working_directory,
		revision: values.revision
	},
	fetcher
);
const path = getPath(session.entries, session.leafId);
const results = toolResults(path);
const rows = streamRows(path);

const clip = (text) => {
	const flat = String(text ?? '').replace(/\s+/g, ' ').trim();
	return values.full || flat.length <= 160 ? flat : `${flat.slice(0, 159)}…`;
};

// One line per thing the page draws, prefixed with the event IDs behind it.
function describe(entry) {
	const events = (entry.sources ?? []).map((source) => source.eventId).join(',');
	const lines = [];
	const message = entry.message;
	if (message?.role === 'toolResult') {
		lines.push(`unpaired tool result ${message.toolName || ''}: ${clip(message.content.map((block) => block.text ?? '').join(' '))}`);
	} else if (message) {
		for (const block of message.content) {
			if (block.type === 'text') lines.push(`${message.role}: ${clip(block.text)}`);
			else if (block.type === 'hidden') lines.push(`hidden context: ${clip(block.text)}`);
			else if (block.type === 'thinking') lines.push(`thought: ${clip(block.thinking) || '(text not exported)'}`);
			else if (block.type === 'attachment') lines.push(`attachment: ${block.name || block.path || block.url || ''}`);
			else if (block.type === 'toolCall') {
				const result = results.get(block.id);
				const output = result?.message?.content.map((item) => item.text ?? '').join(' ');
				lines.push(`${toolChipLabel(block)} [${toolStatus(block, result)}]${output ? ` → ${clip(output)}` : ''}`);
			}
		}
		if (message.stopReason && !['stop', 'toolUse', 'complete'].includes(message.stopReason)) {
			lines.push(`response state: ${message.stopReason}`);
		}
	} else if (entry.type === 'compaction' || entry.type === 'branch_summary') {
		lines.push(`${entry.type}: ${clip(entry.summary)}`);
	} else {
		const note = customNote(entry);
		if (note) lines.push(`note ${note.title}: ${clip(note.body)}`);
	}
	return lines.map((line) => `  [${events}] ${line}`);
}

const { timing, anomalies } = session;
if (values.json) {
	const summary = { id: trace.id, harness: trace.harness, title: trace.title };
	const shown = rows.map((row) => ({ chrome: row.chrome, lines: row.entries.flatMap(describe) }));
	console.log(JSON.stringify({ trace: summary, timing, anomalies, entries: session.entries.length, path: path.length, rows: shown }, null, 2));
} else {
	console.log(`Trace ${trace.id} · ${trace.harness} · ${trace.title || trace.native_trace_id}`);
	console.log(`Loaded ${timing.events} events in ${timing.pages} pages · fetch ${Math.round(timing.fetchMs)} ms · adapter ${Math.round(timing.adapterMs)} ms`);
	console.log(`${session.entries.length} entries · ${leafCount(session.entries)} branches · shown path ${path.length} entries in ${rows.length} rows`);
	console.log(anomalies.length ? 'Anomalies:' : 'Anomalies: none');
	for (const anomaly of anomalies) {
		const more = anomaly.count > anomaly.eventIds.length ? ',…' : '';
		console.log(`  ${anomaly.kind}: ${anomaly.message} · events ${anomaly.eventIds.join(',')}${more}`);
	}
	let turn = 0;
	for (const row of rows) {
		if (row.chrome === 'user') console.log(`\n# Turn ${++turn}`);
		for (const line of row.entries.flatMap(describe)) console.log(line);
	}
}
