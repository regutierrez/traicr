<script lang="ts">
	import { revisionSourcesHref } from '$lib/links';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();
</script>

<svelte:head><title>Source records · Traicr</title></svelte:head>
<header class="page-head">
	<p class="crumbs"><span>Event {data.id}</span><span>Provenance</span></p>
	<h1>Source records</h1>
	<p class="page-lede">Native records are retained exactly as collected. Conflicting observations remain inspectable.</p>
</header>
{#if data.error}
	<p class="text-destructive" role="alert">{data.error}</p>
{:else}
	{#each data.sources as source, index (source.observation_id ?? index)}
		<section class="source">
			<a class="files" href={revisionSourcesHref(source.revision_id)}>Revision {source.revision_id} files</a>
			<pre>{JSON.stringify(source, null, 2)}</pre>
		</section>
	{:else}
		<p class="text-muted-foreground">No source references for this event.</p>
	{/each}
{/if}

<style>
	.source + .source {
		margin-top: 1.5rem;
	}

	.files {
		color: var(--muted-foreground);
		font-size: 12px;
		text-decoration: none;
	}

	.files:hover {
		color: var(--foreground);
	}

	pre {
		overflow: auto;
		margin: 0.4rem 0 0;
		padding: 0.75rem 0.9rem;
		border: 1px solid var(--border);
		border-radius: var(--radius);
		background: var(--card);
		font-family: var(--font-mono);
		font-size: 12px;
		line-height: 1.55;
		white-space: pre-wrap;
		overflow-wrap: anywhere;
	}
</style>
