import { readJSON } from '$lib/api';
import type { Machine } from '$lib/types';
import type { PageLoad } from './$types';

export const load: PageLoad = async ({ fetch }) => {
	const result = await readJSON<Machine[]>(fetch, '/api/v1/machines');
	if (!result.ok) return { machines: [], error: result.message };
	return { machines: result.data ?? [], error: '' };
};
