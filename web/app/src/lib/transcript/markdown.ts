import hljs from 'highlight.js/lib/core';
import bash from 'highlight.js/lib/languages/bash';
import c from 'highlight.js/lib/languages/c';
import cpp from 'highlight.js/lib/languages/cpp';
import csharp from 'highlight.js/lib/languages/csharp';
import css from 'highlight.js/lib/languages/css';
import dockerfile from 'highlight.js/lib/languages/dockerfile';
import go from 'highlight.js/lib/languages/go';
import java from 'highlight.js/lib/languages/java';
import javascript from 'highlight.js/lib/languages/javascript';
import json from 'highlight.js/lib/languages/json';
import markdown from 'highlight.js/lib/languages/markdown';
import php from 'highlight.js/lib/languages/php';
import python from 'highlight.js/lib/languages/python';
import ruby from 'highlight.js/lib/languages/ruby';
import rust from 'highlight.js/lib/languages/rust';
import scss from 'highlight.js/lib/languages/scss';
import sql from 'highlight.js/lib/languages/sql';
import typescript from 'highlight.js/lib/languages/typescript';
import xml from 'highlight.js/lib/languages/xml';
import yaml from 'highlight.js/lib/languages/yaml';
import { Marked } from 'marked';

hljs.registerLanguage('bash', bash);
hljs.registerLanguage('c', c);
hljs.registerLanguage('cpp', cpp);
hljs.registerLanguage('csharp', csharp);
hljs.registerLanguage('css', css);
hljs.registerLanguage('dockerfile', dockerfile);
hljs.registerLanguage('go', go);
hljs.registerLanguage('html', xml);
hljs.registerLanguage('java', java);
hljs.registerLanguage('javascript', javascript);
hljs.registerLanguage('json', json);
hljs.registerLanguage('markdown', markdown);
hljs.registerLanguage('php', php);
hljs.registerLanguage('python', python);
hljs.registerLanguage('ruby', ruby);
hljs.registerLanguage('rust', rust);
hljs.registerLanguage('scss', scss);
hljs.registerLanguage('sql', sql);
hljs.registerLanguage('typescript', typescript);
hljs.registerLanguage('xml', xml);
hljs.registerLanguage('yaml', yaml);

const LANGUAGE_BY_EXT: Record<string, string> = {
	ts: 'typescript',
	tsx: 'typescript',
	js: 'javascript',
	jsx: 'javascript',
	py: 'python',
	rb: 'ruby',
	rs: 'rust',
	go: 'go',
	java: 'java',
	c: 'c',
	cpp: 'cpp',
	h: 'c',
	hpp: 'cpp',
	cs: 'csharp',
	php: 'php',
	sh: 'bash',
	bash: 'bash',
	zsh: 'bash',
	sql: 'sql',
	html: 'html',
	css: 'css',
	scss: 'scss',
	json: 'json',
	yaml: 'yaml',
	yml: 'yaml',
	xml: 'xml',
	md: 'markdown',
	dockerfile: 'dockerfile'
};

export function escapeHtml(value: unknown) {
	return String(value ?? '').replace(/[&<>"']/g, (char) => {
		return { '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[char] ?? char;
	});
}

export function sanitizeMarkdownUrl(value: string) {
	const href = String(value || '').trim().replace(/[\x00-\x1f\x7f]/g, '');
	if (!href) return href;
	const scheme = href.match(/^([A-Za-z][A-Za-z0-9+.-]*):/);
	if (scheme && !/^(https?|mailto|tel|ftp)$/i.test(scheme[1])) return null;
	return href;
}

export function highlightCode(text: string, language?: string) {
	if (language && hljs.getLanguage(language)) {
		try {
			return hljs.highlight(text, { language }).value;
		} catch {
			return escapeHtml(text);
		}
	}
	try {
		return hljs.highlightAuto(text).value;
	} catch {
		return escapeHtml(text);
	}
}

export function languageForPath(path?: string) {
	if (!path) return '';
	const ext = path.split('.').pop()?.toLowerCase() ?? '';
	return LANGUAGE_BY_EXT[ext] || '';
}

const marked = new Marked({
	breaks: true,
	gfm: true,
	tokenizer: {
		html() {
			return undefined;
		},
		tag() {
			return undefined;
		}
	},
	renderer: {
		link({ href, title, tokens }) {
			const safe = sanitizeMarkdownUrl(href);
			if (safe === null) return this.parser.parseInline(tokens);
			const titleAttr = title ? ` title="${escapeHtml(title)}"` : '';
			return `<a href="${escapeHtml(safe)}" rel="noreferrer"${titleAttr}>${this.parser.parseInline(tokens)}</a>`;
		},
		image({ href, text }) {
			const safe = sanitizeMarkdownUrl(href);
			if (safe === null) return escapeHtml(text || '');
			return `<a rel="noreferrer" href="${escapeHtml(safe)}">[Image: ${escapeHtml(text || href)}]</a>`;
		},
		code({ text, lang }) {
			return `<pre><code class="hljs">${highlightCode(text, lang)}</code></pre>`;
		},
		codespan({ text }) {
			return `<code>${escapeHtml(text)}</code>`;
		}
	}
});

export function renderMarkdown(text: string) {
	return marked.parse(text) as string;
}
