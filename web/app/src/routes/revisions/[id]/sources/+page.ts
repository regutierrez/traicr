import { readJSON } from '$lib/api';
import type { SourceFile } from '$lib/types';
import type { PageLoad } from './$types';

export const load: PageLoad = async ({ fetch, params }) => {
	const result = await readJSON<SourceFile[]>(fetch, `/api/v1/revisions/${params.id}/sources`);
	if (!result.ok) return { files: [], error: result.message, id: params.id };
	return { files: result.data ?? [], error: '', id: params.id };
};
