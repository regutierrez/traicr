<script lang="ts">
	import { renderMarkdown } from '$lib/transcript/markdown';
	import 'highlight.js/styles/github.css';

	let { text = '' }: { text?: string } = $props();
	let html = $derived(renderMarkdown(text));
</script>

<div class="md">{@html html}</div>

<style>
	.md {
		overflow-wrap: anywhere;
		font-size: 14px;
		line-height: 20px;
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
		overflow: auto;
		max-height: 24rem;
		padding: 6px 10px;
		border: 1px solid rgb(0 0 0 / 11%);
		border-radius: 8px;
		background: var(--muted);
		font-family: var(--font-mono);
		font-size: 13px;
		line-height: 20px;
	}

	.md :global(code) {
		font-family: var(--font-mono);
		font-size: 0.92em;
	}

	.md :global(:not(pre) > code) {
		padding: 1px 6px;
		border: 1px solid rgb(0 0 0 / 11%);
		border-radius: 3px;
		background: rgb(0 0 0 / 4%);
		font-size: 13px;
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
