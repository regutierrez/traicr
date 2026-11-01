<script lang="ts">
	import { renderMarkdown } from '$lib/transcript/markdown';
	import 'highlight.js/styles/github-dark.css';

	let { text = '' }: { text?: string } = $props();
	let html = $derived(renderMarkdown(text));
</script>

<div class="md">{@html html}</div>

<style>
	.md {
		overflow-wrap: anywhere;
		font-size: inherit;
		line-height: inherit;
	}

	.md :global(> :first-child) {
		margin-top: 0;
	}

	.md :global(> :last-child) {
		margin-bottom: 0;
	}

	.md :global(p),
	.md :global(ul),
	.md :global(ol),
	.md :global(pre),
	.md :global(blockquote) {
		margin: 0.65rem 0;
	}

	/* Tailwind's reset removes list markers; prose needs them back. */
	.md :global(ul),
	.md :global(ol) {
		padding-left: 1.4rem;
	}

	.md :global(ul) {
		list-style: disc;
	}

	.md :global(ol) {
		list-style: decimal;
	}

	.md :global(li + li),
	.md :global(li > ul),
	.md :global(li > ol) {
		margin-top: 0.25rem;
	}

	.md :global(li::marker) {
		color: var(--muted-foreground);
	}

	.md :global(h1),
	.md :global(h2),
	.md :global(h3),
	.md :global(h4) {
		margin: 1.25rem 0 0.5rem;
		color: var(--foreground);
		font-weight: 600;
		line-height: 1.35;
	}

	.md :global(h1) {
		font-size: 1.25em;
	}

	.md :global(h2) {
		font-size: 1.12em;
	}

	.md :global(h3),
	.md :global(h4) {
		font-size: 1em;
	}

	.md :global(strong) {
		color: var(--foreground);
		font-weight: 600;
	}

	.md :global(blockquote) {
		padding-left: 0.85rem;
		border-left: 2px solid var(--input);
		color: var(--muted-foreground);
	}

	.md :global(hr) {
		margin: 1.25rem 0;
		border: 0;
		border-top: 1px solid var(--border);
	}

	.md :global(table) {
		display: block;
		overflow-x: auto;
		margin: 0.65rem 0;
		border-collapse: collapse;
		font-size: 0.92em;
	}

	.md :global(th),
	.md :global(td) {
		padding: 0.35rem 0.75rem;
		border-bottom: 1px solid var(--border);
		text-align: left;
	}

	.md :global(th) {
		color: var(--foreground);
		font-weight: 600;
	}

	.md :global(pre) {
		overflow-x: auto;
		padding: 0.75rem 0.9rem;
		border: 1px solid var(--border);
		border-radius: var(--radius);
		background: var(--card);
		line-height: 1.55;
		font-family: var(--font-mono);
		font-size: 12px;
	}

	.md :global(code) {
		font-family: var(--font-mono);
		font-size: 0.92em;
	}

	.md :global(:not(pre) > code) {
		padding: 0.1rem 0.3rem;
		border-radius: 4px;
		background: color-mix(in oklab, var(--foreground) 8%, transparent);
	}

	.md :global(.hljs) {
		background: transparent;
		color: inherit;
		padding: 0;
	}

	.md :global(a) {
		color: var(--primary);
		text-decoration-color: color-mix(in oklch, var(--primary) 40%, transparent);
		text-underline-offset: 2px;
	}
</style>
