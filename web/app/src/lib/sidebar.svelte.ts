import { browser } from '$app/environment';

// Whether the session list shows beside the page. The list holds its own toggle and the rail
// holds it while the list is hidden, so the state lives here; hiding it is remembered.
export const sidebar = $state({ open: browser ? localStorage.getItem('traicr.sidebar') !== 'hidden' : true });

export function toggleSidebar() {
	sidebar.open = !sidebar.open;
	localStorage.setItem('traicr.sidebar', sidebar.open ? 'shown' : 'hidden');
}

/** The /api/v1/cards filters the session list offers, besides the query and its mode. */
export const searchFilters = ['harness', 'model', 'machine', 'repository', 'after', 'before'] as const;

/**
 * The instant the reader's calendar day YYYY-MM-DD starts, or its last millisecond with `end`.
 * The API reads a bare date as a UTC day; the list's day headings and the overview's ranges are local days.
 */
export function localDayBound(date: string, end = false) {
	const [year, month, day] = date.split('-').map(Number);
	return new Date(new Date(year, month - 1, day + (end ? 1 : 0)).getTime() - (end ? 1 : 0)).toISOString();
}

// What the session list searches for. It lives outside the list so pages without the list,
// such as Archive, do not reset it.
export const search = $state<Record<'q' | 'mode' | (typeof searchFilters)[number], string>>({
	q: '',
	mode: 'fulltext',
	harness: '',
	model: '',
	machine: '',
	repository: '',
	after: '',
	before: ''
});
