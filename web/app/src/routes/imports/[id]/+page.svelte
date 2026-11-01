<script lang="ts">
	import { resolve } from '$app/paths';
	import MiddleTruncate from '$lib/components/MiddleTruncate.svelte';
	import { count, dateAndTime } from '$lib/format';
	import { transcriptHref } from '$lib/links';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();
	let report = $derived(data.report);
	let totals = $derived(
		report
			? ([
					['new', report.imported, ''],
					['updated', report.updated, ''],
					['unchanged', report.unchanged, ''],
					['partially parsed', report.partially_parsed, 'warn'],
					['unsupported', report.unsupported, 'warn'],
					['failed', report.failed, 'bad']
				] as const).filter(([, value]) => value)
			: []
	);
</script>

<svelte:head><title>Import {report ? `#${report.id}` : 'report'} · Traicr</title></svelte:head>

<header class="page-head">
	<nav class="crumbs" aria-label="Breadcrumbs"><a href={resolve('/imports')}>Archive</a></nav>
	{#if report}
		<h1>Import #{report.id}</h1>
		<p class="page-lede">
			<span translate="no">{report.source_machine.hostname}</span> · <time datetime={report.created_at}>{dateAndTime(report.created_at)}</time> ·
			{count(report.traces?.length ?? 0, 'trace')}
		</p>
		{#if totals.length}
			<p class="totals">
				{#each totals as [label, value, tone] (label)}<span class={tone}>{value} {label}</span>{/each}
			</p>
		{/if}
	{:else}
		<h1>Import report</h1>
	{/if}
</header>

{#if data.error || !report}
	<p class="error" role="alert">{data.error || 'Import not found'}</p>
{:else}
	<ul class="traces">
		{#each report.traces ?? [] as trace (trace.revision_digest)}
			<li class="trace" style="--ink: var(--ink-{trace.harness}, var(--muted-foreground))">
				<div class="line">
					<span class={['status', trace.error ? 'bad' : trace.warnings?.length ? 'warn' : '']}>{trace.status}</span>
					<span class="harness" translate="no">{trace.harness}</span>
					<span class="id">
						{#if trace.trace_id}
							<a href={transcriptHref(trace.trace_id)} title={trace.native_trace_id}>{trace.title || trace.native_trace_id}</a>
						{:else}
							<span translate="no">{trace.native_trace_id}</span>
						{/if}
					</span>
					<span class="digest" translate="no"><MiddleTruncate text={trace.revision_digest} tail={12} /></span>
				</div>
				{#if trace.error}<p class="message bad">{trace.error}</p>{/if}
				{#each trace.warnings ?? [] as warning (`${warning.code}:${warning.message}`)}
					<p class="message warn"><strong>{warning.code}</strong> {warning.message}</p>
				{/each}
			</li>
		{:else}
			<li class="message">This import had no traces.</li>
		{/each}
	</ul>
{/if}

<style>
	.totals {
		display: flex;
		flex-wrap: wrap;
		gap: 0.25rem 1rem;
		margin: 0.75rem 0 0;
		font-size: 13px;
		font-variant-numeric: tabular-nums;
	}

	.error,
	.bad {
		color: var(--destructive);
	}

	.warn {
		color: var(--primary);
	}

	.traces {
		margin: 0;
		padding: 0;
		list-style: none;
		border-top: 1px solid var(--border);
	}

	.trace {
		padding: 0.55rem 0;
		border-bottom: 1px solid var(--border);
	}

	.line {
		display: grid;
		grid-template-columns: 6rem 6rem minmax(0, 1fr) 10rem;
		align-items: baseline;
		gap: 0.75rem;
		font-size: 13px;
	}

	.status {
		color: var(--muted-foreground);
		font-size: 12px;
	}

	.harness {
		display: inline-flex;
		align-items: center;
		gap: 0.4rem;
		color: var(--muted-foreground);
		font-size: 12px;
	}

	.harness::before {
		content: '';
		width: 6px;
		height: 6px;
		border-radius: 999px;
		background: var(--ink);
	}

	.id {
		overflow: hidden;
		font-size: 13px;
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	.id a {
		color: var(--foreground);
		text-decoration: none;
	}

	.id a:hover {
		color: var(--primary);
	}

	.digest {
		display: flex;
		justify-content: flex-end;
		color: var(--muted-foreground);
		font-family: var(--font-mono);
		font-size: 11px;
	}

	.message {
		margin: 0.35rem 0 0;
		font-size: 12px;
	}

	@media (max-width: 640px) {
		.line {
			grid-template-columns: auto auto minmax(0, 1fr);
		}

		.digest {
			display: none;
		}
	}
</style>
