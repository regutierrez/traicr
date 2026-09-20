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
		font-size: 14px;
		line-height: 1.6;
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
		margin: 0.55rem 0;
	}

	.md :global(pre) {
		overflow-x: auto;
		padding: 0.7rem 0.8rem;
		border: 1px solid var(--border);
		border-radius: var(--radius);
		background: #181818;
		font-family: var(--font-mono);
		font-size: 12px;
	}

	.md :global(code) {
		font-family: var(--font-mono);
		font-size: 0.92em;
	}

	.md :global(:not(pre) > code) {
		padding: 0.05rem 0.25rem;
		border-radius: 3px;
		background: var(--muted);
	}

	.md :global(.hljs) {
		background: transparent;
		color: inherit;
		padding: 0;
	}

	.md :global(a) {
		color: var(--primary);
	}
</style>
