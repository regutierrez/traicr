<script lang="ts">
	import { resolve } from '$app/paths';
	import { shortRepository } from '$lib/format';
	import { withQuery } from '$lib/links';
	import { sidebar, toggleSidebar } from '$lib/sidebar.svelte';
	import type { Trace } from '$lib/types';

	let {
		trace,
		revision = '',
		tab,
		model = '',
		thinkingLevel = '',
		stepsExpanded = false,
		onToggleSteps
	}: {
		trace: Trace;
		revision?: string;
		tab: 'overview' | 'trace';
		model?: string;
		thinkingLevel?: string;
		stepsExpanded?: boolean;
		onToggleSteps?: () => void;
	} = $props();

	let title = $derived(trace.title || trace.native_trace_id);

</script>

{#snippet tabLink(id: 'overview' | 'trace', label: string, route: '/traces/[id]' | '/traces/[id]/trace')}
	<a
		class={['tab', tab === id && 'is-current']}
		href={withQuery(resolve(route, { id: String(trace.id) }), revision ? { revision } : {})}
		aria-current={tab === id ? 'page' : undefined}>{label}</a
	>
{/snippet}

<header class="head">
	<div class="titles">
		<div class="title-row">
			<!-- Beside the panel it controls, and in the same spot whether the list is shown or hidden. -->
			<button
				class="sidebar-toggle"
				type="button"
				aria-pressed={sidebar.open}
				aria-keyshortcuts="["
				aria-label="Session list"
				title={sidebar.open ? 'Hide session list ([)' : 'Show session list ([)'}
				onclick={toggleSidebar}
			>
				<svg viewBox="0 0 16 16" fill="none" aria-hidden="true">
					<rect x="2.25" y="2.75" width="11.5" height="10.5" rx="1.75" stroke="currentColor" stroke-width="1.3" />
					<path d="M6.25 2.75v10.5" stroke="currentColor" stroke-width="1.3" />
				</svg>
			</button>
			<h1 title={title}>{title}</h1>
		</div>
		<p class="meta">
			<span class="harness" translate="no" style="--ink: var(--ink-{trace.harness}, var(--muted-foreground))">{trace.harness}</span>
			{#each trace.parents ?? [] as parent (parent.id)}
				<a class="parent" href={resolve('/traces/[id]', { id: String(parent.id) })}>Subagent of {parent.title || parent.native_trace_id}</a>
			{:else}
				{#if trace.parent_native_trace_id}<span>Subagent · parent never collected</span>{/if}
			{/each}
			{#if model}<span class="chip" translate="no">{model}{#if thinkingLevel}<span class="dim">{thinkingLevel}</span>{/if}</span>{/if}
			{#if trace.repository || trace.working_directory}
				<span class="cwd" translate="no" title={trace.working_directory}>{shortRepository(trace.repository || trace.working_directory)}</span>
			{/if}
		</p>
	</div>
	{#if tab === 'trace'}
		<div class="actions">
			<button
				type="button"
				class={['toggle', stepsExpanded && 'is-on']}
				aria-pressed={stepsExpanded}
				aria-keyshortcuts="E"
				onclick={onToggleSteps}
				title="Show every step the agent took (E)"
			>
				<span>{stepsExpanded ? 'Collapse all' : 'Expand all'}</span>
				<kbd>E</kbd>
			</button>
		</div>
	{/if}
	<nav class="tabs" aria-label="Trace views">
		{@render tabLink('overview', 'Overview', '/traces/[id]')}
		{@render tabLink('trace', 'Trace', '/traces/[id]/trace')}
	</nav>
</header>

<style>
	.head {
		position: sticky;
		top: 0;
		z-index: 4;
		display: grid;
		gap: 0.75rem;
		padding: 1rem 1.25rem 0;
		background: var(--background);
		border-bottom: 1px solid var(--border);
	}

	.titles {
		min-width: 0;
	}

	.title-row {
		display: flex;
		min-width: 0;
		align-items: center;
		gap: 0.4rem;
	}

	/* Only wide screens have room for the session list beside a trace. */
	.sidebar-toggle {
		display: none;
		width: 1.75rem;
		height: 1.75rem;
		flex-shrink: 0;
		place-items: center;
		margin-left: -0.35rem;
		border: 0;
		border-radius: var(--radius);
		background: transparent;
		color: var(--muted-foreground);
		cursor: pointer;
		transition: background-color 120ms ease, color 120ms ease;
	}

	.sidebar-toggle svg {
		width: 16px;
		height: 16px;
	}

	.sidebar-toggle:hover,
	.sidebar-toggle:focus-visible {
		background: var(--accent);
		color: var(--foreground);
	}

	@media (min-width: 1100px) {
		.sidebar-toggle {
			display: grid;
		}
	}

	h1 {
		margin: 0;
		min-width: 0;
		overflow: hidden;
		font-size: 19px;
		font-weight: 600;
		letter-spacing: -0.01em;
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	.meta {
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		gap: 0.4rem 1rem;
		margin: 0.35rem 0 0;
		color: var(--muted-foreground);
		font-size: 12px;
		font-variant-numeric: tabular-nums;
	}

	.harness {
		display: inline-flex;
		align-items: center;
		gap: 0.4rem;
		color: var(--ink);
		font-family: var(--font-mono);
	}

	.harness::before {
		content: '';
		width: 6px;
		height: 6px;
		border-radius: 999px;
		background: var(--ink);
	}

	.chip {
		display: inline-flex;
		gap: 0.4rem;
		padding: 0.1rem 0.45rem;
		border: 1px solid var(--border);
		border-radius: 999px;
		color: var(--foreground);
	}

	.dim {
		color: var(--muted-foreground);
	}

	.parent {
		overflow: hidden;
		max-width: 42ch;
		color: var(--foreground);
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	.cwd {
		overflow: hidden;
		max-width: 42ch;
		font-family: var(--font-mono);
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	.actions {
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		gap: 0.65rem;
	}

	.toggle {
		display: inline-flex;
		height: 1.75rem;
		align-items: center;
		gap: 0.4rem;
		border: 1px solid transparent;
		border-radius: var(--radius);
		background: transparent;
		color: var(--muted-foreground);
		padding: 0 0.4rem 0 0.55rem;
		font: inherit;
		font-size: 12px;
		cursor: pointer;
		transition: background-color 120ms ease, color 120ms ease;
	}

	.toggle:hover,
	.toggle:focus-visible {
		background: var(--accent);
		color: var(--foreground);
	}

	.toggle.is-on {
		background: color-mix(in oklch, var(--primary) 12%, transparent);
		border-color: color-mix(in oklch, var(--primary) 30%, transparent);
		color: var(--primary);
	}

	kbd {
		display: inline-grid;
		min-width: 1.1rem;
		height: 1.1rem;
		place-items: center;
		border-radius: 4px;
		background: color-mix(in oklab, var(--foreground) 8%, transparent);
		color: inherit;
		font-family: var(--font-sans);
		font-size: 10px;
		line-height: 1;
		opacity: 0.85;
	}

	@media (pointer: coarse) {
		kbd {
			display: none;
		}
	}

	.tabs {
		display: flex;
		gap: 1rem;
	}

	.tab {
		padding: 0.35rem 0 0.6rem;
		color: var(--muted-foreground);
		font-size: 13px;
		text-decoration: none;
		border-bottom: 2px solid transparent;
	}

	.tab:hover {
		color: var(--foreground);
	}

	.tab.is-current {
		color: var(--foreground);
		border-bottom-color: var(--primary);
	}

	@media (min-width: 768px) {
		.head {
			grid-template-columns: minmax(0, 1fr) auto;
			align-items: end;
			padding: 0.85rem 1.5rem 0;
		}

		.tabs {
			grid-column: 1 / -1;
		}
	}
</style>
