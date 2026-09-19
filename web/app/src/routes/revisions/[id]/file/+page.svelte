<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import { revisionSourcesHref, sourceFileHref } from '$lib/links';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();
	let revisionID = $derived(Number(data.id));
</script>

<svelte:head><title>{data.preview?.path || 'Source file'} · Traicr</title></svelte:head>

<Button class="mb-4" variant="ghost" href={revisionSourcesHref(revisionID)}>Revision files</Button>
{#if data.error || !data.preview}
	<p class="text-destructive" role="alert">{data.error || 'Choose a source file to preview.'}</p>
{:else}
	<div class="mb-6 flex flex-wrap items-end justify-between gap-4">
		<div>
			<p class="text-primary mb-2 font-mono text-[11px] tracking-widest">NATIVE SOURCE / PREVIEW</p>
			<h1 class="text-3xl font-semibold tracking-tight break-all">{data.preview.path}</h1>
		</div>
		<Button variant="outline" href={sourceFileHref(revisionID, data.preview.path, { download: true })} data-sveltekit-reload>Download original</Button>
	</div>
	{#if data.preview.truncated}
		<p class="bg-muted mb-4 rounded-lg border px-3 py-2 text-sm">Preview limited to 1 MiB. Download the original for the complete file.</p>
	{/if}
	<pre class="bg-muted overflow-x-auto rounded-lg p-4 font-mono text-xs whitespace-pre-wrap">{data.preview.content}</pre>
{/if}
