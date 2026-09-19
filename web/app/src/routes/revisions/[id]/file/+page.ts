import { readJSON } from '$lib/api';
import type { FilePreview } from '$lib/types';
import type { PageLoad } from './$types';

export const load: PageLoad = async ({ fetch, params, url }) => {
	const path = url.searchParams.get('path') ?? '';
	if (!path) return { preview: null, error: '', id: params.id };
	const result = await readJSON<FilePreview>(
		fetch,
		`/api/v1/revisions/${params.id}/file?preview=1&path=${encodeURIComponent(path)}`
	);
	if (!result.ok) return { preview: null, error: result.message, id: params.id };
	return { preview: result.data, error: '', id: params.id };
};
