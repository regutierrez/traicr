<script lang="ts">
	import { resolve } from '$app/paths';
	import type { ChildCard } from '$lib/transcript/children';

	let { child }: { child: ChildCard } = $props();
	let href = $derived(child.traceId ? resolve('/traces/[id]', { id: String(child.traceId) }) : child.href);
</script>

{#if child.collected && href}
	<a class="row" href={href}>
		<span class="mark">↳</span>
		<span class="title">{child.title}</span>
		<span class="meta">{child.meta}</span>
		<span class="go" aria-hidden="true">›</span>
	</a>
{:else}
	<div class="row is-missing">
		<span class="mark">↳</span>
		<span class="title">{child.title}</span>
		<span class="meta">{child.meta} · never collected</span>
		{#if child.originalHref}
			<a class="amp" href={child.originalHref} target="_blank" rel="noreferrer">Original in Amp ↗</a>
		{/if}
	</div>
{/if}

<style>
	.row {
		display: grid;
		grid-template-columns: auto minmax(0, 1fr) auto auto;
		align-items: center;
		gap: 0.55rem;
		margin: 0.2rem 0;
		padding: 0.22rem 0;
		color: inherit;
		text-decoration: none;
	}

	.row:hover,
	.row:focus-visible {
		background: var(--accent);
	}

	.mark,
	.go {
		color: var(--muted-foreground);
	}

	.title {
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	.meta,
	.amp {
		color: var(--muted-foreground);
		font-family: var(--font-mono);
		font-size: 11px;
	}

	.amp {
		color: var(--primary);
	}
</style>
