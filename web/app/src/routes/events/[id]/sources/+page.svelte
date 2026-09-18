<script lang="ts">
	import { Card, CardContent } from '$lib/components/ui/card';
	import { resolve } from '$app/paths';

	let { data } = $props();
</script>

<svelte:head><title>Source records · Traicr</title></svelte:head>
<p class="text-primary mb-2 font-mono text-[11px] tracking-widest">EVENT {data.id} / PROVENANCE</p>
<h1 class="mb-2 text-3xl font-semibold tracking-tight">Source Records</h1>
<p class="text-muted-foreground mb-6 max-w-2xl">Native records are retained exactly as collected. Conflicting observations remain inspectable.</p>
{#if data.error}
	<p class="text-destructive" role="alert">{data.error}</p>
{:else}
	<div class="flex flex-col gap-4">
		{#each data.sources as source, index (source.observation_id ?? index)}
			<Card>
				<CardContent>
					<pre class="bg-muted overflow-x-auto rounded-lg p-3 font-mono text-xs whitespace-pre-wrap">{JSON.stringify(source, null, 2)}</pre>
					<a href={resolve(`/revisions/${source.revision_id}/sources`)}>Open revision files</a>
				</CardContent>
			</Card>
		{:else}
			<p class="text-muted-foreground">No source references for this event.</p>
		{/each}
	</div>
{/if}
