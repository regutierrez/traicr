import { describe, expect, it } from 'vitest';
import { healthProblems } from './health';
import type { ArchiveStatus } from './types';

const health: ArchiveStatus = {
	machines: [],
	imports: { total: 71, with_failed_traces: 35, incomplete: 0 },
	traces: 5101,
	events: 1102546,
	titles: { enabled: false, pending: 0 },
	storage: { database_bytes: 0, objects_bytes: 0 }
};

describe('healthProblems', () => {
	it('keeps historical failures in Archive but hides them on the homepage', () => {
		expect(healthProblems(health)).toEqual([{ text: '35 imports had failed traces', tone: 'bad' }]);
		expect(healthProblems(health, false)).toEqual([]);
	});

	it('still reports running and interrupted imports on the homepage', () => {
		expect(healthProblems({ ...health, running_import_id: 72, imports: { ...health.imports, incomplete: 2 } }, false)).toEqual([
			{ text: 'Import #72 is running', tone: '' },
			{ text: '2 imports stopped before finishing', tone: 'warn' }
		]);
	});
});
