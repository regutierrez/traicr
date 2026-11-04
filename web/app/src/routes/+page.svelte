<script lang="ts">
	import { resolve } from '$app/paths';
	import { ago, dateAndTime, fileSize } from '$lib/format';
	import { healthProblems, lastImportResult } from '$lib/health';
	import { withQuery } from '$lib/links';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();
	let problems = $derived(data.health ? healthProblems(data.health) : []);
	let number = new Intl.NumberFormat();
</script>

<svelte:head>
	<title>Sessions · Traicr</title>
</svelte:head>

<header class="page-head">
	<h1>Overview</h1>
	<p class="page-lede">Pick a session from the list, or search every title, prompt, reply, and tool output in it.</p>
</header>

{#if data.error}
	<p class="error" role="alert">{data.error}</p>
{/if}

{#if data.health}
	{@const health = data.health}
	{#if health.traces === 0}
		<div class="empty">
			<p class="empty-title">Your archive starts here</p>
			<p>Collect traces on a source machine, then upload the ZIPs. Your original records stay intact.</p>
		</div>
	{:else}
		<dl class="stats">
			<div><dt>Traces</dt><dd>{number.format(health.traces)}</dd></div>
			<div><dt>Events</dt><dd>{number.format(health.events)}</dd></div>
			<div><dt>Imports</dt><dd>{number.format(health.imports.total)}</dd></div>
			<div><dt>Stored</dt><dd>{fileSize(health.storage.database_bytes + health.storage.objects_bytes)}</dd></div>
		</dl>
	{/if}

	<section>
		<h2>Health</h2>
		{#if problems.length}
			<p class="health-state">
				{#each problems as problem (problem.text)}<span class={problem.tone}>{problem.text}</span>{/each}
			</p>
		{:else if health.imports.total}
			<p class="health-state ok">Every import finished cleanly</p>
		{:else}
			<p class="health-state ok">No imports yet</p>
		{/if}
	</section>

	{#if health.machines.length}
		<section>
			<h2>Machines</h2>
			<ul class="machines">
				{#each health.machines as machine (machine.id)}
					{@const result = lastImportResult(health, machine.id)}
					<li>
						<span class="host" translate="no">{machine.hostname}</span>
						<span class="dim">
							last upload <time datetime={machine.last_import_at} title={dateAndTime(machine.last_import_at)}>{ago(machine.last_import_at)}</time>
							{#if result}· <a class={['result', result.tone]} href={result.href}>{result.text}</a>{/if}
						</span>
						<!-- The session list reads the machine from the address and filters by it. -->
						<a class="action" href={withQuery(resolve('/'), { machine: machine.id })}>Show its sessions</a>
					</li>
				{/each}
			</ul>
			<a class="more" href={resolve('/imports')}>Every import in Archive</a>
		</section>
	{/if}
{/if}

<section>
	<h2>Keys</h2>
	<dl class="keys">
		<div><dt><kbd>/</kbd></dt><dd>Search sessions</dd></div>
		<div><dt><kbd>↓</kbd> <kbd>↑</kbd></dt><dd>Move through the list from the search box</dd></div>
		<div><dt><kbd>Enter</kbd></dt><dd>Search now, or open the first session</dd></div>
		<div><dt><kbd>j</kbd> <kbd>k</kbd></dt><dd>Open the next or previous session</dd></div>
		<div><dt><kbd>[</kbd></dt><dd>Hide or show the session list</dd></div>
	</dl>
</section>

<style>
	section {
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

	.stats {
		display: grid;
		grid-template-columns: repeat(auto-fit, minmax(9rem, 1fr));
		gap: 0.75rem;
		margin: 1.5rem 0 0;
	}

	.stats div {
		padding: 0.85rem 1rem;
		border: 1px solid var(--border);
		border-radius: 0.6rem;
		background: var(--card);
	}

	.stats dt {
		color: var(--muted-foreground);
		font-size: 12px;
	}

	.stats dd {
		margin: 0.2rem 0 0;
		font-size: 1.5rem;
		font-weight: 600;
		font-variant-numeric: tabular-nums;
		letter-spacing: -0.02em;
	}

	.health-state {
		display: flex;
		flex-wrap: wrap;
		gap: 0.25rem 1rem;
		margin: 0;
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

	.action,
	.more {
		color: var(--muted-foreground);
		font-size: 12px;
		text-decoration: none;
	}

	.action {
		margin-left: auto;
	}

	.more {
		display: inline-block;
		margin-top: 0.6rem;
	}

	.action:hover,
	.more:hover {
		color: var(--foreground);
	}

	.keys {
		display: grid;
		gap: 0.4rem;
		margin: 0;
		font-size: 13px;
	}

	.keys div {
		display: grid;
		grid-template-columns: 6rem 1fr;
		align-items: baseline;
	}

	.keys dd {
		margin: 0;
		color: var(--muted-foreground);
	}

	kbd {
		padding: 0.05rem 0.35rem;
		border-radius: 4px;
		background: color-mix(in oklab, var(--foreground) 8%, transparent);
		font-family: var(--font-sans);
		font-size: 11px;
	}

	.empty {
		margin-top: 1.5rem;
		color: var(--muted-foreground);
		font-size: 13px;
	}

	.empty p {
		margin: 0;
	}

	.empty .empty-title {
		margin-bottom: 0.35rem;
		color: var(--foreground);
		font-size: 15px;
		font-weight: 500;
	}
</style>
