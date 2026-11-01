import { browser } from '$app/environment';

// Whether the session list shows beside a trace. The layout draws the list and the
// trace header holds its toggle, so the state lives here; hiding it is remembered.
export const sidebar = $state({ open: browser ? localStorage.getItem('traicr.sidebar') !== 'hidden' : true });

export function toggleSidebar() {
	sidebar.open = !sidebar.open;
	localStorage.setItem('traicr.sidebar', sidebar.open ? 'shown' : 'hidden');
}
