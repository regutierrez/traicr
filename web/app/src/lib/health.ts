import { resolve } from '$app/paths';
import { count } from '$lib/format';
import type { ArchiveStatus } from '$lib/types';

// The overview and the Archive page read /api/v1/status the same way.

/** Problems first, so a glance at the top says whether anything needs a look. Empty when all is well. */
export function healthProblems(health: ArchiveStatus, includeHistoricalFailures = true) {
	const lines: { text: string; tone: 'warn' | 'bad' | '' }[] = [];
	if (health.running_import_id) lines.push({ text: `Import #${health.running_import_id} is running`, tone: '' });
	if (includeHistoricalFailures && health.imports.with_failed_traces) lines.push({ text: `${count(health.imports.with_failed_traces, 'import')} had failed traces`, tone: 'bad' });
	if (health.imports.incomplete) lines.push({ text: `${count(health.imports.incomplete, 'import')} stopped before finishing`, tone: 'warn' });
	if (health.titles.enabled && health.titles.pending) lines.push({ text: `${count(health.titles.pending, 'title')} waiting to be named`, tone: '' });
	return lines;
}

/** How a machine's newest import ended, linked to that import. Null when the machine never imported. */
export function lastImportResult(health: ArchiveStatus | null, machineID: string) {
	const machine = health?.machines.find((item) => item.id === machineID);
	if (!health || !machine) return null;
	const href = resolve('/imports/[id]', { id: String(machine.last_import_id) });
	if (machine.last_import_id === health.running_import_id) return { href, text: 'running', tone: 'running' };
	if (!machine.last_import_complete) return { href, text: 'stopped before finishing', tone: 'warn' };
	if (machine.last_import_failed_traces) return { href, text: `${machine.last_import_failed_traces} failed`, tone: 'bad' };
	return { href, text: 'OK', tone: 'ok' };
}
