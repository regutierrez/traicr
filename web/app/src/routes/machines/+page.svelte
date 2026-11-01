<script lang="ts">
	import { resolve } from '$app/paths';
	import { Button } from '$lib/components/ui/button';
	import { dateAndTime } from '$lib/format';
	import { withQuery } from '$lib/links';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();
</script>

<svelte:head><title>Machines · Traicr</title></svelte:head>

<header class="page-head">
	<h1>Machines</h1>
	<p class="page-lede">Where your traces were collected. A session seen on two machines is still one session.</p>
</header>

{#if data.error}
	<p class="error" role="alert">{data.error}</p>
{/if}

<ul class="machines">
	{#each data.machines as machine (machine.id)}
		<li class="machine">
			<p class="host" translate="no">{machine.hostname}</p>
			<p class="meta-row" translate="no">{machine.os} / {machine.arch}</p>
			<dl>
				<div><dt>Machine ID</dt><dd>{machine.id}</dd></div>
				<div><dt>First upload</dt><dd><time datetime={machine.first_seen}>{dateAndTime(machine.first_seen)}</time></dd></div>
				<div><dt>Last upload</dt><dd><time datetime={machine.last_seen}>{dateAndTime(machine.last_seen)}</time></dd></div>
			</dl>
			<Button variant="link" class="h-auto p-0 text-xs" href={withQuery(resolve('/'), { machine: machine.id })}>Search this machine</Button>
		</li>
	{:else}
		<li class="empty">No machines yet. Your source machines appear here after their first upload.</li>
	{/each}
</ul>

<style>
	.empty,
	.error,
	.meta-row {
		margin: 0;
		color: var(--muted-foreground);
		font-size: 12px;
	}

	.error {
		margin-bottom: 0.75rem;
		color: var(--destructive);
	}

	.machines {
		display: grid;
		gap: 0.75rem;
		grid-template-columns: repeat(auto-fill, minmax(20rem, 1fr));
		list-style: none;
		margin: 0;
		padding: 0;
	}

	.machine,
	.empty {
		padding: 1rem 1.1rem;
		border: 1px solid var(--border);
		border-radius: 0.6rem;
		background: var(--card);
	}

	.host {
		margin: 0 0 0.1rem;
		font-size: 15px;
		font-weight: 600;
	}

	dl {
		display: grid;
		gap: 0.2rem 1rem;
		margin: 0.85rem 0;
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
		font-size: 12px;
		overflow-wrap: anywhere;
	}

</style>
