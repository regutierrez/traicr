<script lang="ts">
	import { resolve } from '$app/paths';
	import { Button } from '$lib/components/ui/button';
	import { withQuery } from '$lib/links';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();
</script>

<svelte:head><title>Imports · Traicr</title></svelte:head>

<p class="text-primary mb-2 font-mono text-[11px] tracking-widest">COLLECTION HISTORY</p>
<h1 class="text-3xl font-semibold tracking-tight">Imports</h1>
<p class="text-muted-foreground mt-2 mb-6 max-w-2xl">Every upload, its outcome, and anything that needs your attention.</p>

{#if data.error}
	<p class="text-destructive mb-4 text-sm" role="alert">{data.error}</p>
{/if}

<div class="overflow-x-auto rounded-xl ring-1 ring-foreground/10">
	<table class="w-full text-sm">
		<thead>
			<tr class="bg-muted/60 text-muted-foreground text-left text-xs">
				<th class="px-3 py-2 font-medium">Import</th>
				<th class="px-3 py-2 font-medium">Source machine</th>
				<th class="px-3 py-2 font-medium">Collected</th>
				<th class="px-3 py-2 font-medium">New</th>
				<th class="px-3 py-2 font-medium">Updated</th>
				<th class="px-3 py-2 font-medium">Unchanged</th>
				<th class="px-3 py-2 font-medium">Partial / unsupported</th>
				<th class="px-3 py-2 font-medium">Failed</th>
			</tr>
		</thead>
		<tbody>
			{#each data.imports as item (item.id)}
				<tr class="border-t">
					<td class="px-3 py-3"><a href={resolve('/imports/[id]', { id: String(item.id) })}>#{item.id}</a></td>
					<td class="px-3 py-3">{item.source_machine.hostname}</td>
					<td class="px-3 py-3">{item.created_at}</td>
					<td class="px-3 py-3">{item.imported}</td>
					<td class="px-3 py-3">{item.updated}</td>
					<td class="px-3 py-3">{item.unchanged}</td>
					<td class="px-3 py-3">{item.partially_parsed} / {item.unsupported}</td>
					<td class="px-3 py-3">{item.failed}</td>
				</tr>
			{:else}
				<tr><td class="text-muted-foreground px-3 py-6" colspan="8">No imports yet. Upload a Trace ZIP with the collector to get started.</td></tr>
			{/each}
		</tbody>
	</table>
</div>
{#if data.next}
	<Button class="mt-6" variant="outline" href={withQuery(resolve('/imports'), { cursor: data.next })}>Older imports</Button>
{/if}
