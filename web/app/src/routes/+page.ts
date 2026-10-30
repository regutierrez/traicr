import { readJSON } from '$lib/api';
import type { TranscriptCardPage } from '$lib/types';
import type { PageLoad } from './$types';

export const load: PageLoad = async ({ fetch, url }) => {
	const result = await readJSON<TranscriptCardPage>(fetch, `/api/v1/cards?${url.searchParams.toString()}`);
	if (!result.ok) return { cards: [], error: result.message, next: '', partial: false };
	return { cards: result.data.cards ?? [], error: '', next: result.data.next_cursor ?? '', partial: result.data.partial ?? false };
};
