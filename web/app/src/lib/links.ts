import { resolve } from '$app/paths';

// resolve() knows route ids, not query strings; append those here so every link is built the same way.
export function withQuery(path: string, params: Record<string, string> | URLSearchParams) {
	const query = new URLSearchParams(params).toString();
	return query ? `${path}?${query}` : path;
}

export function transcriptHref(traceID: number) {
	return resolve('/traces/[id]', { id: String(traceID) });
}

export function recordsHref(traceID: number, params: Record<string, string> = {}) {
	return withQuery(resolve('/traces/[id]/records', { id: String(traceID) }), params);
}

export function revisionSourcesHref(revisionID: number) {
	return resolve('/revisions/[id]/sources', { id: String(revisionID) });
}

// The preview page and the download share one route; the Go server sends bytes only when download=1.
export function sourceFileHref(revisionID: number, path: string, options: { download?: boolean } = {}) {
	const params: Record<string, string> = { path };
	if (options.download) params.download = '1';
	return withQuery(resolve('/revisions/[id]/file', { id: String(revisionID) }), params);
}
