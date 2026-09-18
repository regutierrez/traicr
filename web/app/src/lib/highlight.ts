function escapeRegExp(value: string) {
	return value.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
}

export function highlightParts(text: string, query: string, mode: string) {
	if (!text) return [{ text: '', mark: false }];
	if (!query) return [{ text, mark: false }];
	let pattern = '';
	if (mode === 'regex') {
		pattern = query;
	} else if (mode === 'exact') {
		pattern = escapeRegExp(query);
	} else {
		const terms = query.match(/[\p{L}\p{N}_]+/gu) ?? [];
		if (terms.length === 0) return [{ text, mark: false }];
		pattern = terms.map(escapeRegExp).join('|');
	}
	let expression: RegExp;
	try {
		expression = new RegExp(pattern, mode === 'regex' || mode === 'exact' ? 'g' : 'gi');
	} catch {
		return [{ text, mark: false }];
	}
	const parts: { text: string; mark: boolean }[] = [];
	let last = 0;
	for (const match of text.matchAll(expression)) {
		const index = match.index ?? 0;
		if (match[0] === '' || index < last) continue;
		if (index > last) parts.push({ text: text.slice(last, index), mark: false });
		parts.push({ text: match[0], mark: true });
		last = index + match[0].length;
	}
	if (last < text.length) parts.push({ text: text.slice(last), mark: false });
	return parts.length ? parts : [{ text, mark: false }];
}
