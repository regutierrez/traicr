<script lang="ts">
	import { resolve } from '$app/paths';
	import { absoluteTime, earlierTime, relativeTime, repoLabel } from '$lib/time';
	import type { Trace } from '$lib/types';

	let {
		trace,
		tab,
		recordCount = 0,
		model = '',
		thinkingLevel = ''
	}: {
		trace: Trace;
		tab: 'conversation' | 'details';
		recordCount?: number;
		model?: string;
		thinkingLevel?: string;
	} = $props();

	let title = $derived(trace.title || trace.native_trace_id);
	let started = $derived(absoluteTime(earlierTime(trace.created_at, trace.updated_at)));
	let ago = $derived(relativeTime(trace.updated_at || trace.created_at));
	let repo = $derived(repoLabel(trace.repository || ''));
	let repoHref = $derived(
		trace.repository && /^https?:\/\//.test(trace.repository) ? trace.repository : ''
	);

</script>

<header class="head">
	<div class="inner">
		<p class="byline">
			<span class="who">{trace.harness}</span>
			shared
			{#if repo}
				in
				{#if repoHref}
					<a class="where" href={repoHref} target="_blank" rel="noreferrer">{repo}</a>
				{:else}
					<span class="where" title={trace.repository}>{repo}</span>
				{/if}
			{/if}
			{#if ago}<span class="when">{ago}</span>{/if}
		</p>
		<h1>{title}</h1>
		<p class="facts">
			{#if model}<span>{model}</span>{/if}
			<span>Started {started}</span>
			{#if recordCount}<span>{recordCount} records</span>{/if}
			{#if thinkingLevel}<span>{thinkingLevel} thinking</span>{/if}
		</p>
		<nav class="tabs" aria-label="Trace views">
			<a
				class={['tab', tab === 'conversation' && 'is-current']}
				href={resolve('/traces/[id]', { id: String(trace.id) })}
				aria-current={tab === 'conversation' ? 'page' : undefined}
			>Conversation</a>
			<a
				class={['tab', tab === 'details' && 'is-current']}
				href={resolve('/traces/[id]/records', { id: String(trace.id) })}
				aria-current={tab === 'details' ? 'page' : undefined}
			>Details</a>
		</nav>
	</div>
</header>

<style>
	.head {
		z-index: 4;
		flex-shrink: 0;
		background: var(--background);
		border-bottom: 1px solid var(--border);
	}

	.inner {
		width: 100%;
		max-width: 1200px;
		margin: 0 auto;
		padding: 1rem 1.5rem 0;
	}

	.byline {
		margin: 0;
		color: var(--muted-foreground);
		font-size: 14px;
		line-height: 20px;
	}

	.who,
	.where {
		color: var(--foreground);
		font-weight: 500;
	}

	.where {
		text-decoration: none;
	}

	.where:hover,
	.where:focus-visible {
		text-decoration: underline;
	}

	.when {
		margin-left: 0.35rem;
	}

	h1 {
		margin: 0.7rem 0 0;
		max-width: 50ch;
		font-size: 30px;
		font-weight: 400;
		letter-spacing: -0.02em;
		line-height: 36px;
	}

	.facts {
		display: flex;
		flex-wrap: wrap;
		margin: 0.55rem 0 0;
		color: var(--muted-foreground);
		font-size: 13px;
		line-height: 16px;
	}

	.facts span + span::before {
		content: '·';
		margin: 0 0.45rem;
	}

	.tabs {
		display: flex;
		gap: 1.5rem;
		margin-top: 0.85rem;
	}

	.tab {
		padding: 0.45rem 0 0.7rem;
		border-bottom: 1px solid transparent;
		color: var(--muted-foreground);
		font-size: 14px;
		font-weight: 500;
		line-height: 20px;
		text-decoration: none;
	}

	.tab:hover,
	.tab:focus-visible,
	.tab.is-current {
		color: var(--foreground);
	}

	.tab.is-current {
		border-bottom-color: var(--foreground);
	}

	@media (max-width: 719px) {
		h1 {
			font-size: 24px;
			line-height: 30px;
		}
	}
</style>
