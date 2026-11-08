import { readJSON } from '$lib/api';
import { withQuery } from '$lib/links';
import type { Patch } from '$lib/types';
import type { PageLoad } from './$types';

export const load: PageLoad = async ({ fetch, params, url }) => {
	const revision = url.searchParams.get('revision') ?? '';
	const result = await readJSON<{ patches: Patch[] }>(fetch, withQuery(`/api/v1/traces/${params.id}/patches`, revision ? { revision } : {}), url);
	return { patches: result.ok ? result.data.patches : [], patchError: result.ok ? '' : result.message };
};
