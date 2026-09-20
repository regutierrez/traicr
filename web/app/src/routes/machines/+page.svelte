<script lang="ts">
	import { resolve } from '$app/paths';
	import { Button } from '$lib/components/ui/button';
	import { withQuery } from '$lib/links';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();
</script>

<svelte:head><title>Machines · Traicr</title></svelte:head>

<header class="page-head">
	<div>
		<p class="meta">Archive</p>
		<h1>Source machines</h1>
	</div>
</header>
<p class="lede">Machine identity tracks provenance. The same trace stays one trace, wherever it was collected.</p>

{#if data.error}
	<p class="error" role="alert">{data.error}</p>
{/if}

<ul class="machines">
	{#each data.machines as machine (machine.id)}
		<li class="machine">
			<p class="host">{machine.hostname}</p>
			<p class="meta-row">{machine.os} / {machine.arch}</p>
			<dl>
				<div><dt>Machine ID</dt><dd>{machine.id}</dd></div>
				<div><dt>First seen</dt><dd>{machine.first_seen}</dd></div>
				<div><dt>Last seen</dt><dd>{machine.last_seen}</dd></div>
			</dl>
			<Button variant="link" class="h-auto p-0 text-xs" href={withQuery(resolve('/'), { machine: machine.id })}>Search this machine</Button>
		</li>
	{:else}
		<li class="empty">No machines yet. Your source machines appear here after their first upload.</li>
	{/each}
</ul>

<style>
	.page-head {
		margin-bottom: 0.35rem;
	}

	.meta,
	.lede,
	.empty,
	.error,
	.meta-row {
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

	.machines {
		list-style: none;
		margin: 0;
		padding: 0;
		border-top: 1px solid var(--border);
	}

	.machine,
	.empty {
		border-bottom: 1px solid var(--border);
		padding: 0.7rem 0.15rem;
	}

	.host {
		margin: 0 0 0.15rem;
		font-weight: 550;
	}

	dl {
		display: grid;
		gap: 0.2rem 1rem;
		margin: 0.45rem 0;
		grid-template-columns: 6.5rem minmax(0, 1fr);
	}

	dl div {
		display: contents;
	}

	dt {
		color: var(--muted-foreground);
		font-size: 12px;
	}

	dd {
		margin: 0;
		font-family: var(--font-mono);
		font-size: 11px;
		overflow-wrap: anywhere;
	}

</style>
