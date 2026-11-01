import { resolve } from '$app/paths';

// resolve() knows route ids, not query strings; append those here so every link is built the same way.
export function withQuery(path: string, params: Record<string, string> | URLSearchParams) {
	const query = new URLSearchParams(params).toString();
	return query ? `${path}?${query}` : path;
}

export function transcriptHref(traceID: number) {
	return resolve('/traces/[id]', { id: String(traceID) });
}

// The trace lets the files page link back to its session; no API maps a revision to its trace.
export function revisionSourcesHref(revisionID: number, traceID?: number) {
	const path = resolve('/revisions/[id]/sources', { id: String(revisionID) });
	return traceID ? withQuery(path, { trace: String(traceID) }) : path;
}

// The Go server answers this route with the original bytes as a download. Links to it need data-sveltekit-reload.
export function sourceFileHref(revisionID: number, path: string) {
	// A Go route, not a SvelteKit one, so resolve() does not know it.
	return withQuery(`/revisions/${encodeURIComponent(String(revisionID))}/file`, { path });
}
