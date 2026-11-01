<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import { revisionSourcesHref, sourceFileHref } from '$lib/links';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();
	let revisionID = $derived(Number(data.id));
</script>

<svelte:head><title>{data.preview?.path || 'Source file'} · Traicr</title></svelte:head>

{#if data.error || !data.preview}
	<header class="page-head">
		<nav class="crumbs" aria-label="Breadcrumbs"><a href={revisionSourcesHref(revisionID)}>Revision {data.id} files</a></nav>
	</header>
	<p class="text-destructive" role="alert">{data.error || 'Choose a source file to preview.'}</p>
{:else}
	<header class="page-head head-row">
		<div class="min-w-0">
			<nav class="crumbs" aria-label="Breadcrumbs"><a href={revisionSourcesHref(revisionID)}>Revision {data.id} files</a><span>Preview</span></nav>
			<h1 class="font-mono [overflow-wrap:anywhere]">{data.preview.path}</h1>
		</div>
		<Button variant="outline" size="sm" href={sourceFileHref(revisionID, data.preview.path, { download: true })} data-sveltekit-reload>Download original</Button>
	</header>
	{#if data.preview.truncated}
		<p class="text-muted-foreground mb-3 text-xs">Preview limited to 1 MiB. Download the original for the complete file.</p>
	{/if}
	<pre class="bg-card overflow-x-auto rounded-lg border p-4 font-mono text-xs leading-relaxed whitespace-pre-wrap">{data.preview.content}</pre>
{/if}

<style>
	.head-row {
		display: flex;
		flex-wrap: wrap;
		align-items: flex-end;
		justify-content: space-between;
		gap: 1rem;
	}

	.head-row h1 {
		font-size: 1.15rem;
	}
</style>
