import { redirect } from '@sveltejs/kit';
import type { PageLoad } from './$types';

// Machines became a section of the Archive page.
export const load: PageLoad = () => redirect(307, '/imports');
