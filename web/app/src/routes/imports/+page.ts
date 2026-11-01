import { readJSON } from '$lib/api';
import type { ImportPage, Machine } from '$lib/types';
import type { PageLoad } from './$types';

export const load: PageLoad = async ({ fetch, url }) => {
	const cursor = url.searchParams.get('cursor') ?? '';
	const [result, machines] = await Promise.all([
		readJSON<ImportPage>(fetch, `/api/v1/imports?cursor=${encodeURIComponent(cursor)}`),
		readJSON<Machine[]>(fetch, '/api/v1/machines')
	]);
	const known = machines.ok ? (machines.data ?? []) : [];
	if (!result.ok) return { imports: [], machines: known, error: result.message, next: '' };
	return { imports: result.data.imports ?? [], machines: known, error: '', next: result.data.next_cursor ?? '' };
};
