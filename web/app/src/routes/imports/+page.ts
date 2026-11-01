import { readJSON } from '$lib/api';
import type { ArchiveStatus, ImportPage, Machine } from '$lib/types';
import type { PageLoad } from './$types';

export const load: PageLoad = async ({ fetch, url }) => {
	const cursor = url.searchParams.get('cursor') ?? '';
	const [result, machines, status] = await Promise.all([
		readJSON<ImportPage>(fetch, `/api/v1/imports?cursor=${encodeURIComponent(cursor)}`),
		readJSON<Machine[]>(fetch, '/api/v1/machines'),
		readJSON<ArchiveStatus>(fetch, '/api/v1/status')
	]);
	const known = machines.ok ? (machines.data ?? []) : [];
	// The page still works without the health line, so a status failure only hides it.
	const health = status.ok ? status.data : null;
	if (!result.ok) return { imports: [], machines: known, health, error: result.message, next: '' };
	return { imports: result.data.imports ?? [], machines: known, health, error: '', next: result.data.next_cursor ?? '' };
};
