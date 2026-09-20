import { readJSON } from '$lib/api';
import type { ImportReport } from '$lib/types';
import type { PageLoad } from './$types';

export const load: PageLoad = async ({ fetch, params }) => {
	const result = await readJSON<ImportReport>(fetch, `/api/v1/imports/${params.id}`);
	if (!result.ok) return { report: null, error: result.message };
	return { report: result.data, error: '' };
};
