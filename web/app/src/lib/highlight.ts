const MAX_QUERY = 200;
const MAX_HAYSTACK = 32_768;
const MAX_MATCHES = 64;

function escapeRegExp(value: string) {
	return value.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
}

// User regexes run against snippet text in the session list. Nested or stacked
// quantifiers are the usual catastrophic-backtracking shape; refuse those and
// cap query / haystack / match count so a crafted pattern cannot hang the tab.
function unsafeRegex(pattern: string): boolean {
	if (pattern.length > MAX_QUERY) return true;
	// (…quantifier…)quantifier, e.g. (a+)+ — not a bare (foo)+.
	if (/\((?:[^()\\]|\\.)*[+*?{](?:[^()\\]|\\.)*\)[+*?{]/.test(pattern)) return true;
	// Stacked greedy quantifiers, e.g. a++ / a+*. Lazy +? / *? stay allowed.
	return /[+*][+*]/.test(pattern);
}

export function highlightParts(text: string, query: string, mode: string) {
	if (!text) return [{ text: '', mark: false }];
	if (!query) return [{ text, mark: false }];
	if (query.length > MAX_QUERY) return [{ text, mark: false }];

	let haystack = text;
	let overflow = '';
	if (mode === 'regex' && text.length > MAX_HAYSTACK) {
		haystack = text.slice(0, MAX_HAYSTACK);
		overflow = text.slice(MAX_HAYSTACK);
	}

	let pattern = '';
	if (mode === 'regex') {
		if (unsafeRegex(query)) return [{ text, mark: false }];
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
	let matches = 0;
	for (const match of haystack.matchAll(expression)) {
		const index = match.index ?? 0;
		if (match[0] === '' || index < last) continue;
		if (index > last) parts.push({ text: haystack.slice(last, index), mark: false });
		parts.push({ text: match[0], mark: true });
		last = index + match[0].length;
		matches += 1;
		if (matches >= MAX_MATCHES) break;
	}
	const tail = haystack.slice(last) + overflow;
	if (tail) parts.push({ text: tail, mark: false });
	return parts.length ? parts : [{ text, mark: false }];
}
