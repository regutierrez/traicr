import { readJSON } from '$lib/api';
import type { Machine, TranscriptCardPage } from '$lib/types';
import type { PageLoad } from './$types';

export const load: PageLoad = async ({ fetch, url }) => {
	const [result, machines] = await Promise.all([
		readJSON<TranscriptCardPage>(fetch, `/api/v1/cards?${url.searchParams.toString()}`),
		readJSON<Machine[]>(fetch, '/api/v1/machines')
	]);
	const known = machines.ok ? (machines.data ?? []) : [];
	if (!result.ok) return { cards: [], machines: known, error: result.message, next: '', partial: false };
	return {
		cards: result.data.cards ?? [],
		machines: known,
		error: '',
		next: result.data.next_cursor ?? '',
		partial: result.data.partial ?? false
	};
};
