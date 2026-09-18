import { readJSON } from '$lib/api';
import type { CardPage } from '$lib/types';
import type { PageLoad } from './$types';

export const load: PageLoad = async ({ fetch, url }) => {
	const result = await readJSON<CardPage>(fetch, `/api/v1/cards?${url.searchParams.toString()}`);
	if (!result.ok) return { cards: [], error: result.message, next: '' };
	return { cards: result.data.cards ?? [], error: '', next: result.data.next_cursor ?? '' };
};
