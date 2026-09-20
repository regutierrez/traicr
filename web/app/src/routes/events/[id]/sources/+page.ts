import { readJSON } from '$lib/api';
import type { PageLoad } from './$types';

type Source = { observation_id?: number; revision_id: number };

export const load: PageLoad = async ({ fetch, params }) => {
	const result = await readJSON<Source[]>(fetch, `/api/v1/events/${params.id}/sources`);
	if (!result.ok) return { sources: [], error: result.message, id: params.id };
	return { sources: result.data ?? [], error: '', id: params.id };
};
