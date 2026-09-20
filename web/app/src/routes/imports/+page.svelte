<script lang="ts">
	import { resolve } from '$app/paths';
	import { Button } from '$lib/components/ui/button';
	import { withQuery } from '$lib/links';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();
</script>

<svelte:head><title>Archive · Traicr</title></svelte:head>

<header class="page-head">
	<div>
		<p class="meta">Archive</p>
		<h1>Imports</h1>
	</div>
</header>
<p class="lede">Every upload, its outcome, and anything that needs your attention.</p>

{#if data.error}
	<p class="error" role="alert">{data.error}</p>
{/if}

<div class="table-wrap">
	<table>
		<thead>
			<tr>
				<th>Import</th>
				<th>Source machine</th>
				<th>Collected</th>
				<th>New</th>
				<th>Updated</th>
				<th>Unchanged</th>
				<th>Partial / unsupported</th>
				<th>Failed</th>
			</tr>
		</thead>
		<tbody>
			{#each data.imports as item (item.id)}
				<tr>
					<td><a href={resolve('/imports/[id]', { id: String(item.id) })}>#{item.id}</a></td>
					<td>{item.source_machine.hostname}</td>
					<td>{item.created_at}</td>
					<td>{item.imported}</td>
					<td>{item.updated}</td>
					<td>{item.unchanged}</td>
					<td>{item.partially_parsed} / {item.unsupported}</td>
					<td>{item.failed}</td>
				</tr>
			{:else}
				<tr><td class="empty" colspan="8">No imports yet. Upload a Trace ZIP with the collector to get started.</td></tr>
			{/each}
		</tbody>
	</table>
</div>
{#if data.next}
	<Button class="mt-4" variant="outline" href={withQuery(resolve('/imports'), { cursor: data.next })}>Older imports</Button>
{/if}

<style>
	.page-head {
		margin-bottom: 0.35rem;
	}

	.meta,
	.lede,
	.empty,
	.error {
		margin: 0;
		color: var(--muted-foreground);
		font-size: 12px;
	}

	.meta {
		font-family: var(--font-mono);
		font-size: 11px;
		letter-spacing: 0.06em;
		text-transform: uppercase;
	}

	h1 {
		margin: 0.15rem 0 0;
		font-size: 1.05rem;
		font-weight: 600;
	}

	.lede {
		margin-bottom: 0.85rem;
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
		padding: 0.55rem 0.4rem;
		text-align: left;
		font-variant-numeric: tabular-nums;
	}

	th {
		color: var(--muted-foreground);
		font-size: 11px;
		font-weight: 500;
	}

	.empty {
		padding: 1.25rem 0.4rem;
	}
</style>
