import { readJSON } from '$lib/api';
import type { ImportPage } from '$lib/types';
import type { PageLoad } from './$types';

export const load: PageLoad = async ({ fetch, url }) => {
	const cursor = url.searchParams.get('cursor') ?? '';
	const result = await readJSON<ImportPage>(fetch, `/api/v1/imports?cursor=${encodeURIComponent(cursor)}`);
	if (!result.ok) return { imports: [], error: result.message, next: '' };
	return { imports: result.data.imports ?? [], error: '', next: result.data.next_cursor ?? '' };
};
