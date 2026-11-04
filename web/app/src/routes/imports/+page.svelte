<script lang="ts">
	import { resolve } from '$app/paths';
	import { Button } from '$lib/components/ui/button';
	import { ago, count, dateAndTime, fileSize } from '$lib/format';
	import { healthProblems, lastImportResult } from '$lib/health';
	import { withQuery } from '$lib/links';
	import type { ImportReport } from '$lib/types';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();

	type Tone = '' | 'warn' | 'bad';

	// One readable outcome per upload instead of six mostly-zero columns.
	function outcome(item: ImportReport) {
		if (!item.complete) {
			return item.id === data.health?.running_import_id ? [{ text: 'Running now', tone: '' as Tone }] : [{ text: 'Stopped before finishing', tone: 'warn' as Tone }];
		}
		const parts: { text: string; tone: Tone }[] = [];
		const add = (value: number, label: string, tone: Tone = '') => value && parts.push({ text: `${value} ${label}`, tone });
		add(item.imported, 'new');
		add(item.updated, 'updated');
		add(item.unchanged, 'unchanged');
		add(item.partially_parsed, 'partly parsed', 'warn');
		add(item.unsupported, 'unsupported', 'warn');
		add(item.failed, 'failed', 'bad');
		return parts.length ? parts : [{ text: 'No traces', tone: '' as Tone }];
	}

	// The worst outcome colors the row's dot, so a failed upload is visible while scanning.
	function rowTone(item: ImportReport): Tone {
		const tones = outcome(item).map((part) => part.tone);
		return tones.includes('bad') ? 'bad' : tones.includes('warn') ? 'warn' : '';
	}

	let problems = $derived(data.health ? healthProblems(data.health) : []);
</script>

<svelte:head><title>Archive · Traicr</title></svelte:head>

<header class="page-head">
	<h1>Archive</h1>
	{#if data.health}
		{@const health = data.health}
		<p class="page-lede health">
			{[
				count(health.traces, 'trace'),
				count(health.events, 'event'),
				count(health.imports.total, 'import'),
				`${fileSize(health.storage.database_bytes + health.storage.objects_bytes)} stored`
			].join(' · ')}
		</p>
		{#if problems.length}
			<p class="health-state">
				{#each problems as problem (problem.text)}<span class={problem.tone}>{problem.text}</span>{/each}
			</p>
		{:else if health.imports.total}
			<p class="health-state ok">Every import finished cleanly</p>
		{/if}
	{:else}
		<p class="page-lede">Where your traces came from and every upload, with anything that needs your attention.</p>
	{/if}
</header>

{#if data.error}
	<p class="error" role="alert">{data.error}</p>
{/if}

<section>
	<h2>Machines</h2>
	<ul class="machines">
		{#each data.machines as machine (machine.id)}
			{@const uploaded = data.health?.machines.find((item) => item.id === machine.id)?.last_import_at ?? machine.last_seen}
			{@const result = lastImportResult(data.health, machine.id)}
			<li>
				<span class="host" translate="no">{machine.hostname}</span>
				<span class="dim" translate="no">{machine.os} / {machine.arch}</span>
				<span class="dim">
					last upload <time datetime={uploaded} title={dateAndTime(uploaded)}>{ago(uploaded)}</time>
					{#if result}· <a class={['result', result.tone]} href={result.href}>{result.text}</a>{/if}
				</span>
				<a class="search" href={withQuery(resolve('/'), { machine: machine.id })}>Search its sessions</a>
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
					<tr data-tone={rowTone(item) || undefined}>
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

	.health {
		font-variant-numeric: tabular-nums;
	}

	.health-state {
		display: flex;
		flex-wrap: wrap;
		gap: 0.25rem 1rem;
		margin: 0.35rem 0 0;
		font-size: 13px;
	}

	.health-state.ok {
		color: var(--muted-foreground);
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

	.machines .search {
		margin-left: auto;
		color: var(--muted-foreground);
		font-size: 12px;
		text-decoration: none;
	}

	.machines .search:hover {
		color: var(--foreground);
	}

	.result {
		text-decoration: none;
	}

	.result.ok {
		color: var(--muted-foreground);
	}

	.result.running {
		color: var(--foreground);
	}

	.result:hover {
		text-decoration: underline;
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

	tbody tr {
		position: relative;
	}

	td a {
		display: inline-flex;
		align-items: center;
		gap: 0.5rem;
		color: var(--primary);
		font-family: var(--font-mono);
		text-decoration: none;
	}

	/* The dot keeps its space on every row so the IDs line up. */
	td a::before {
		content: '';
		width: 6px;
		height: 6px;
		border-radius: 999px;
	}

	tr[data-tone='bad'] td a::before {
		background: var(--destructive);
	}

	tr[data-tone='warn'] td a::before {
		background: var(--primary);
	}

	/* The import link covers its whole row, so any cell opens the report. */
	td a::after {
		content: '';
		position: absolute;
		inset: 0;
	}

	.outcome {
		white-space: normal;
	}
</style>
