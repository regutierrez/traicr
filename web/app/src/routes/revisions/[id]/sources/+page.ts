import { readJSON, stopIfMissing } from '$lib/api';
import type { SourceFile, Trace } from '$lib/types';
import type { PageLoad } from './$types';

export const load: PageLoad = async ({ fetch, params, url }) => {
	const traceID = url.searchParams.get('trace');
	const [result, owner] = await Promise.all([
		readJSON<SourceFile[]>(fetch, `/api/v1/revisions/${params.id}/sources`),
		traceID ? readJSON<Trace>(fetch, `/api/v1/traces/${encodeURIComponent(traceID)}`) : null
	]);
	stopIfMissing(result, 'This revision is not in the archive. Its session may have been deleted.');
	// Only a trace that owns this revision may label the page and its way back.
	const trace = owner?.ok && owner.data.revisions?.some((revision) => String(revision.id) === params.id) ? owner.data : null;
	if (!result.ok) return { files: [], trace, error: result.message, id: params.id };
	return { files: result.data ?? [], trace, error: '', id: params.id };
};
