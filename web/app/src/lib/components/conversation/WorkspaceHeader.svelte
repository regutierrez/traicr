<script lang="ts">
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import type { Trace } from '$lib/types';

	let {
		trace,
		revision = '',
		tab,
		recordCount = 0,
		thinkingExpanded = false,
		toolsExpanded = false,
		onToggleThinking,
		onToggleTools
	}: {
		trace: Trace;
		revision?: string;
		tab: 'conversation' | 'details';
		recordCount?: number;
		thinkingExpanded?: boolean;
		toolsExpanded?: boolean;
		onToggleThinking?: () => void;
		onToggleTools?: () => void;
	} = $props();

	let title = $derived(trace.title || trace.native_trace_id);

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
	<div class="titles">
		<h1>{title}</h1>
		<p class="meta">
			<span>{trace.harness}</span>
			{#if recordCount}<span>{recordCount} records</span>{/if}
			{#if trace.working_directory}<span class="cwd" title={trace.working_directory}>{trace.working_directory}</span>{/if}
		</p>
	</div>
	<div class="actions">
		{#if trace.harness === 'amp' && tab === 'conversation'}
			<label class="revision">
				<span>Archive view</span>
				<select name="revision" value={revision} onchange={changeRevision}>
					<option value="">Merged archive</option>
					{#each trace.revisions ?? [] as item (item.id)}
						<option value={String(item.id)}>Revision {item.id} · {item.native_updated_at}</option>
					{/each}
				</select>
			</label>
		{/if}
		{#if tab === 'conversation'}
			<div class="toggles">
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
			</div>
		{/if}
	</div>
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
</header>

<style>
	.head {
		position: sticky;
		top: 0;
		z-index: 4;
		display: grid;
		gap: 0.55rem;
		padding: 0.75rem 1.25rem 0;
		background: var(--background);
		border-bottom: 1px solid var(--border);
	}

	.titles {
		min-width: 0;
	}

	h1 {
		margin: 0;
		overflow: hidden;
		font-size: 14px;
		font-weight: 600;
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	.meta {
		display: flex;
		flex-wrap: wrap;
		gap: 0.65rem;
		margin: 0.2rem 0 0;
		color: var(--muted-foreground);
		font-family: var(--font-mono);
		font-size: 11px;
		font-variant-numeric: tabular-nums;
	}

	.cwd {
		overflow: hidden;
		max-width: 42ch;
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	.actions {
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		gap: 0.65rem;
	}

	.revision,
	.toggles {
		display: flex;
		align-items: center;
		gap: 0.4rem;
	}

	.revision span {
		color: var(--muted-foreground);
		font-size: 11px;
	}

	select {
		height: 1.75rem;
		border: 1px solid var(--input);
		border-radius: var(--radius);
		background: var(--background);
		color: var(--foreground);
		padding: 0 0.45rem;
		font: inherit;
		font-size: 12px;
	}

	.toggle {
		height: 1.75rem;
		border: 1px solid var(--border);
		border-radius: var(--radius);
		background: transparent;
		color: var(--muted-foreground);
		padding: 0 0.55rem;
		font-size: 11px;
		cursor: pointer;
	}

	.toggle.is-on,
	.toggle:hover,
	.toggle:focus-visible {
		color: var(--foreground);
		border-color: var(--primary);
	}

	.tabs {
		display: flex;
		gap: 1rem;
	}

	.tab {
		padding: 0.35rem 0 0.55rem;
		color: var(--muted-foreground);
		font-size: 12px;
		text-decoration: none;
		border-bottom: 1px solid transparent;
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
