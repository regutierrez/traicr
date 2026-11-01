import { redirect } from '@sveltejs/kit';
import type { PageLoad } from './$types';

// The records view became Overview.
export const load: PageLoad = ({ params }) => redirect(307, `/traces/${params.id}`);
