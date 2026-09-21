<script lang="ts">
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import { absoluteTime, earlierTime, relativeTime, repoLabel } from '$lib/time';
	import type { Trace } from '$lib/types';

	let {
		trace,
		revision = '',
		tab,
		recordCount = 0,
		model = '',
		thinkingLevel = '',
		thinkingExpanded = false,
		toolsExpanded = false,
		onToggleThinking,
		onToggleTools
	}: {
		trace: Trace;
		revision?: string;
		tab: 'conversation' | 'details';
		recordCount?: number;
		model?: string;
		thinkingLevel?: string;
		thinkingExpanded?: boolean;
		toolsExpanded?: boolean;
		onToggleThinking?: () => void;
		onToggleTools?: () => void;
	} = $props();

	let title = $derived(trace.title || trace.native_trace_id);
	let started = $derived(absoluteTime(earlierTime(trace.created_at, trace.updated_at)));
	let ago = $derived(relativeTime(trace.updated_at || trace.created_at));
	let repo = $derived(repoLabel(trace.repository || ''));
	let repoHref = $derived(
		trace.repository && /^https?:\/\//.test(trace.repository) ? trace.repository : ''
	);

	async function changeRevision(event: Event) {
		const select = event.currentTarget;
		if (!(select instanceof HTMLSelectElement)) return;
		const params = new URLSearchParams(page.url.searchParams);
		if (select.value) params.set('revision', select.value);
		else params.delete('revision');
		const query = params.toString();
		await goto(resolve(query ? `/traces/[id]?${query}` : '/traces/[id]', { id: String(trace.id) }), {
			keepFocus: true,
			noScroll: true
		});
	}
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
		<div class="title-row">
			<h1>{title}</h1>
			<div class="actions">
				{#if trace.harness === 'amp' && tab === 'conversation'}
					<label class="revision">
						<span class="sr-only">Archive view</span>
						<select name="revision" value={revision} onchange={changeRevision} aria-label="Archive view">
							<option value="">Merged archive</option>
							{#each trace.revisions ?? [] as item (item.id)}
								<option value={String(item.id)}>Revision {item.id} · {absoluteTime(item.native_updated_at || item.collected_at)}</option>
							{/each}
						</select>
					</label>
				{/if}
				{#if tab === 'conversation'}
					<button
						type="button"
						class={['toggle', thinkingExpanded && 'is-on']}
						aria-pressed={thinkingExpanded}
						onclick={onToggleThinking}
						title="Toggle thinking (T)"
					>
						Thinking
					</button>
					<button
						type="button"
						class={['toggle', toolsExpanded && 'is-on']}
						aria-pressed={toolsExpanded}
						onclick={onToggleTools}
						title="Toggle tools (O)"
					>
						Tools
					</button>
				{/if}
			</div>
		</div>
		<p class="facts">
			{#if model}<span>{model}</span>{/if}
			<span>Started {started}</span>
			{#if recordCount}<span>{recordCount} records</span>{/if}
			{#if thinkingLevel}<span>{thinkingLevel} thinking</span>{/if}
		</p>
		<nav class="tabs" aria-label="Trace views">
			<a
				class={['tab', tab === 'conversation' && 'is-current']}
				href={resolve(
					revision ? `/traces/[id]?revision=${encodeURIComponent(revision)}` : '/traces/[id]',
					{ id: String(trace.id) }
				)}
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

	.title-row {
		display: flex;
		align-items: flex-start;
		justify-content: space-between;
		gap: 1.5rem;
		margin-top: 0.7rem;
	}

	h1 {
		margin: 0;
		max-width: 50ch;
		font-size: 30px;
		font-weight: 400;
		letter-spacing: -0.03em;
		line-height: 36px;
	}

	.actions,
	.revision {
		display: flex;
		flex-shrink: 0;
		align-items: center;
		gap: 0.4rem;
	}

	select,
	.toggle {
		height: 32px;
		border: 1px solid var(--border);
		border-radius: 5px;
		background: var(--card);
		color: var(--foreground);
		padding: 0 0.7rem;
		font: inherit;
		font-size: 13px;
		font-weight: 500;
	}

	.toggle {
		cursor: pointer;
	}

	.toggle.is-on {
		border-color: var(--primary);
		background: var(--primary);
		color: var(--primary-foreground);
	}

	.toggle:hover,
	.toggle:focus-visible,
	select:hover,
	select:focus-visible {
		border-color: #cfcfcf;
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

		.title-row {
			flex-direction: column;
			gap: 0.65rem;
		}
	}
</style>
