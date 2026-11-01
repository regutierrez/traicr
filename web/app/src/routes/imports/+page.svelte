<script lang="ts">
	import { resolve } from '$app/paths';
	import { Button } from '$lib/components/ui/button';
	import { count, dateAndTime, shortAge } from '$lib/format';
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

	// "18h ago", "just now", or "on Oct 12" once shortAge switches to a date.
	function ago(value: string) {
		const age = shortAge(value);
		return age === 'now' ? 'just now' : /^\d+[mhdw]$/.test(age) ? `${age} ago` : `on ${age}`;
	}

	// Each machine's newest import, from /api/v1/status.
	let latest = $derived(new Map((data.health?.machines ?? []).map((machine) => [machine.id, machine])));

	function lastResult(id: string) {
		const machine = latest.get(id);
		if (!machine) return null;
		const href = resolve('/imports/[id]', { id: String(machine.last_import_id) });
		if (machine.last_import_id === data.health?.running_import_id) return { href, text: 'running', tone: 'running' };
		if (!machine.last_import_complete) return { href, text: 'stopped before finishing', tone: 'warn' };
		if (machine.last_import_failed_traces) return { href, text: `${machine.last_import_failed_traces} failed`, tone: 'bad' };
		return { href, text: 'OK', tone: 'ok' };
	}

	const sizeUnits = ['byte', 'kilobyte', 'megabyte', 'gigabyte', 'terabyte'];
	function size(bytes: number) {
		let unit = 0;
		while (bytes >= 1000 && unit < sizeUnits.length - 1) {
			bytes /= 1000;
			unit++;
		}
		return new Intl.NumberFormat(undefined, { style: 'unit', unit: sizeUnits[unit], unitDisplay: 'short', maximumFractionDigits: bytes < 10 ? 1 : 0 }).format(bytes);
	}

	// Problems first, so a glance at the top says whether anything needs a look.
	let problems = $derived.by(() => {
		const health = data.health;
		if (!health) return [];
		const lines: { text: string; tone: 'warn' | 'bad' | '' }[] = [];
		if (health.running_import_id) lines.push({ text: `Import #${health.running_import_id} is running`, tone: '' });
		if (health.imports.with_failed_traces) lines.push({ text: `${count(health.imports.with_failed_traces, 'import')} had failed traces`, tone: 'bad' });
		if (health.imports.incomplete) lines.push({ text: `${count(health.imports.incomplete, 'import')} stopped before finishing`, tone: 'warn' });
		if (health.titles.enabled && health.titles.pending) lines.push({ text: `${count(health.titles.pending, 'title')} waiting to be named`, tone: '' });
		return lines;
	});
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
				`${size(health.storage.database_bytes + health.storage.objects_bytes)} stored`
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
			{@const uploaded = latest.get(machine.id)?.last_import_at ?? machine.last_seen}
			{@const result = lastResult(machine.id)}
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

	td a {
		color: var(--primary);
		font-family: var(--font-mono);
		text-decoration: none;
	}

	.outcome {
		white-space: normal;
	}
</style>
