import { readJSON } from '$lib/api';
import type { ArchiveStatus } from '$lib/types';
import type { PageLoad } from './$types';

// The session list beside this page searches; the page itself shows how the archive is doing.
export const load: PageLoad = async ({ fetch, url }) => {
	const status = await readJSON<ArchiveStatus>(fetch, '/api/v1/status', url);
	return status.ok ? { health: status.data, error: '' } : { health: null, error: status.message };
};
