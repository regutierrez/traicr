<script lang="ts">
	import { resolve } from '$app/paths';
	import { Button } from '$lib/components/ui/button';
	import { dateAndTime } from '$lib/format';
	import { withQuery } from '$lib/links';
	import type { ImportReport } from '$lib/types';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();

	// One readable outcome per upload instead of six mostly-zero columns.
	function outcome(item: ImportReport) {
		const parts: { text: string; tone: '' | 'warn' | 'bad' }[] = [];
		const add = (value: number, label: string, tone: '' | 'warn' | 'bad' = '') => value && parts.push({ text: `${value} ${label}`, tone });
		add(item.imported, 'new');
		add(item.updated, 'updated');
		add(item.unchanged, 'unchanged');
		add(item.partially_parsed, 'partly parsed', 'warn');
		add(item.unsupported, 'unsupported', 'warn');
		add(item.failed, 'failed', 'bad');
		return parts.length ? parts : [{ text: 'No traces', tone: '' as const }];
	}
</script>

<svelte:head><title>Archive · Traicr</title></svelte:head>

<header class="page-head">
	<h1>Archive</h1>
	<p class="page-lede">Where your traces came from and every upload, with anything that needs your attention.</p>
</header>

{#if data.error}
	<p class="error" role="alert">{data.error}</p>
{/if}

<section>
	<h2>Machines</h2>
	<ul class="machines">
		{#each data.machines as machine (machine.id)}
			<li>
				<span class="host" translate="no">{machine.hostname}</span>
				<span class="dim" translate="no">{machine.os} / {machine.arch}</span>
				<span class="dim">last upload <time datetime={machine.last_seen}>{dateAndTime(machine.last_seen)}</time></span>
				<a href={withQuery(resolve('/'), { machine: machine.id })}>Search its sessions</a>
			</li>
		{:else}
			<li class="dim">No machines yet. A source machine appears here after its first upload.</li>
		{/each}
	</ul>
</section>

<section>
	<h2>Imports</h2>
	<div class="table-wrap">
		<table>
			<thead>
				<tr>
					<th scope="col">Import</th>
					<th scope="col">Machine</th>
					<th scope="col">Uploaded</th>
					<th scope="col">Outcome</th>
				</tr>
			</thead>
			<tbody>
				{#each data.imports as item (item.id)}
					<tr>
						<td><a href={resolve('/imports/[id]', { id: String(item.id) })}>#{item.id}</a></td>
						<td translate="no">{item.source_machine.hostname}</td>
						<td><time datetime={item.created_at}>{dateAndTime(item.created_at)}</time></td>
						<td class="outcome">
							{#each outcome(item) as part, index (part.text)}{#if index}<span class="dim">{' · '}</span>{/if}<span class={part.tone}>{part.text}</span>{/each}
						</td>
					</tr>
				{:else}
					<tr><td class="dim" colspan="4">No imports yet. Upload a Trace ZIP with the collector to get started.</td></tr>
				{/each}
			</tbody>
		</table>
	</div>
	{#if data.next}
		<Button class="mt-4" variant="outline" href={withQuery(resolve('/imports'), { cursor: data.next })}>Older imports</Button>
	{/if}
</section>

<style>
	section + section {
		margin-top: 2rem;
	}

	h2 {
		margin: 0 0 0.5rem;
		font-size: 13px;
		font-weight: 600;
	}

	.dim {
		color: var(--muted-foreground);
	}

	.error,
	.bad {
		color: var(--destructive);
	}

	.error {
		margin: 0 0 1rem;
		font-size: 13px;
	}

	.warn {
		color: var(--primary);
	}

	.machines {
		margin: 0;
		padding: 0;
		list-style: none;
		border-top: 1px solid var(--border);
		font-size: 13px;
	}

	.machines li {
		display: flex;
		flex-wrap: wrap;
		align-items: baseline;
		gap: 0.25rem 1rem;
		padding: 0.6rem 0;
		border-bottom: 1px solid var(--border);
	}

	.host {
		font-weight: 500;
	}

	.machines a {
		margin-left: auto;
		color: var(--muted-foreground);
		font-size: 12px;
		text-decoration: none;
	}

	.machines a:hover {
		color: var(--foreground);
	}

	.table-wrap {
		overflow-x: auto;
	}

	table {
		width: 100%;
		border-collapse: collapse;
		font-size: 13px;
	}

	th,
	td {
		border-bottom: 1px solid var(--border);
		padding: 0.6rem 0.6rem 0.6rem 0;
		text-align: left;
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

	.outcome {
		white-space: normal;
	}
</style>
