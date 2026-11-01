<script lang="ts">
	import { resolve } from '$app/paths';
	import { Button } from '$lib/components/ui/button';
	import { dateAndTime } from '$lib/format';
	import { withQuery } from '$lib/links';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();
</script>

<svelte:head><title>Archive · Traicr</title></svelte:head>

<header class="page-head">
	<h1>Imports</h1>
	<p class="page-lede">Every upload, its outcome, and anything that needs your attention.</p>
</header>

{#if data.error}
	<p class="error" role="alert">{data.error}</p>
{/if}

<div class="table-wrap">
	<table>
		<thead>
			<tr>
				<th scope="col">Import</th>
				<th scope="col">Source machine</th>
				<th scope="col">Uploaded</th>
				<th scope="col" class="num">New</th>
				<th scope="col" class="num">Updated</th>
				<th scope="col" class="num">Unchanged</th>
				<th scope="col" class="num">Partial</th>
				<th scope="col" class="num">Unsupported</th>
				<th scope="col" class="num">Failed</th>
			</tr>
		</thead>
		<tbody>
			{#each data.imports as item (item.id)}
				<tr>
					<td><a href={resolve('/imports/[id]', { id: String(item.id) })}>#{item.id}</a></td>
					<td translate="no">{item.source_machine.hostname}</td>
					<td><time datetime={item.created_at}>{dateAndTime(item.created_at)}</time></td>
					<td class={['num', !item.imported && 'zero']}>{item.imported}</td>
					<td class={['num', !item.updated && 'zero']}>{item.updated}</td>
					<td class={['num', !item.unchanged && 'zero']}>{item.unchanged}</td>
					<td class={['num', item.partially_parsed ? 'warn' : 'zero']}>{item.partially_parsed}</td>
					<td class={['num', item.unsupported ? 'warn' : 'zero']}>{item.unsupported}</td>
					<td class={['num', item.failed ? 'bad' : 'zero']}>{item.failed}</td>
				</tr>
			{:else}
				<tr><td class="empty" colspan="9">No imports yet. Upload a Trace ZIP with the collector to get started.</td></tr>
			{/each}
		</tbody>
	</table>
</div>
{#if data.next}
	<Button class="mt-4" variant="outline" href={withQuery(resolve('/imports'), { cursor: data.next })}>Older imports</Button>
{/if}

<style>
	.empty,
	.error {
		margin: 0;
		color: var(--muted-foreground);
		font-size: 12px;
	}

	.error {
		margin-bottom: 0.75rem;
		color: var(--destructive);
	}

	.table-wrap {
		overflow-x: auto;
	}

	table {
		width: 100%;
		border-collapse: collapse;
	}

	th,
	td {
		border-bottom: 1px solid var(--border);
		padding: 0.7rem 0.6rem;
		text-align: left;
		font-variant-numeric: tabular-nums;
		white-space: nowrap;
	}

	th {
		color: var(--muted-foreground);
		font-size: 12px;
		font-weight: 500;
	}

	tbody tr:hover {
		background: var(--accent);
	}

	td a {
		color: var(--primary);
		font-family: var(--font-mono);
		text-decoration: none;
	}

	.num {
		text-align: right;
	}

	.zero {
		color: color-mix(in oklch, var(--muted-foreground) 55%, var(--background));
	}

	.warn {
		color: var(--primary);
	}

	.bad {
		color: var(--destructive);
		font-weight: 600;
	}

	.empty {
		padding: 1.25rem 0.4rem;
	}
</style>
