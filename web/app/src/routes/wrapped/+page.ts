import type { PageLoad } from './$types';

// ?year=2025 recaps that local calendar year; ?year=all recaps the whole archive. Default: this year.
// The page fetches the recap itself, so a large archive shows a loading message instead of a stuck link.
export const load: PageLoad = ({ url }) => {
	const thisYear = new Date().getFullYear();
	const asked = url.searchParams.get('year');
	return { year: asked === 'all' ? 0 : Number(asked) || thisYear, thisYear };
};
