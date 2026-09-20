<script lang="ts">
	import type { StreamCounts, StreamFilter } from '$lib/transcript/turns';
	import type { ChipIcon } from '$lib/transcript/tools';
	import { toolKindIcon } from '$lib/transcript/tools';
	import Icon from './Icon.svelte';

	let {
		counts,
		filter = 'all',
		leaves = [],
		leafId = '',
		onFilter,
		onSelectLeaf
	}: {
		counts: StreamCounts;
		filter?: StreamFilter;
		leaves?: { id: string; label: string }[];
		leafId?: string;
		onFilter: (filter: StreamFilter) => void;
		onSelectLeaf?: (id: string) => void;
	} = $props();

	const rows: { id: StreamFilter; label: string; icon: ChipIcon; flip?: boolean; count: () => number }[] = [
		{ id: 'prompts', label: 'Prompts', icon: 'prompt', flip: true, count: () => counts.prompts },
		{ id: 'responses', label: 'Agent Responses', icon: 'response', count: () => counts.responses },
		{ id: 'thinking', label: 'Thinking', icon: 'brain', count: () => counts.thinking },
		{ id: 'tools', label: 'Tool Calls', icon: 'wrench', count: () => counts.tools },
		{ id: 'compaction', label: 'Compaction', icon: 'compaction', count: () => counts.compaction },
		{ id: 'branches', label: 'Branches', icon: 'branch', count: () => counts.branches }
	];
</script>

<nav class="index" aria-label="Trace contents">
	<ul>
		{#each rows as row (row.id)}
			<li>
				<button type="button" class={['row', filter === row.id && 'is-current']} onclick={() => onFilter(filter === row.id ? 'all' : row.id)}>
					<span class="label">
						<Icon name={row.icon} size={14} flip={row.flip} />
						<span>{row.label}</span>
					</span>
					<span class="count">{row.count()}</span>
				</button>
				{#if row.id === 'tools' && counts.toolKinds.length}
					<ul class="kinds">
						{#each counts.toolKinds as kind (kind.kind)}
							<li>
								<span class="label">
									<Icon name={toolKindIcon[kind.kind]} size={14} />
									<span>{kind.label}</span>
								</span>
								<span class="count">{kind.count}</span>
							</li>
						{/each}
					</ul>
				{/if}
				{#if row.id === 'branches' && filter === 'branches' && leaves.length}
					<ul class="kinds leaves" aria-label="Branch paths">
						{#each leaves as leaf (leaf.id)}
							<li>
								<button
									type="button"
									class={['leaf', leaf.id === leafId && 'is-current']}
									onclick={() => onSelectLeaf?.(leaf.id)}
								>
									<span>{leaf.label}</span>
								</button>
							</li>
						{/each}
					</ul>
				{/if}
			</li>
		{/each}
	</ul>
</nav>

<style>
	.index {
		width: 14.5rem;
		flex-shrink: 0;
		overflow: auto;
		padding: 0.85rem 0.75rem 1.5rem;
		border-right: 1px solid var(--border);
		background: var(--sidebar);
	}

	ul {
		margin: 0;
		padding: 0;
		list-style: none;
	}

	.row,
	.leaf {
		display: flex;
		width: 100%;
		align-items: center;
		justify-content: space-between;
		gap: 0.75rem;
		border: 0;
		background: transparent;
		color: var(--muted-foreground);
		padding: 0.28rem 0.35rem;
		font: inherit;
		font-size: 12px;
		text-align: left;
		cursor: pointer;
	}

	.label {
		display: inline-flex;
		min-width: 0;
		align-items: center;
		gap: 0.45rem;
	}

	.row:hover,
	.row:focus-visible,
	.row.is-current,
	.leaf:hover,
	.leaf:focus-visible,
	.leaf.is-current {
		color: var(--foreground);
		background: var(--sidebar-accent);
	}

	.count {
		font-family: var(--font-mono);
		font-size: 11px;
		font-variant-numeric: tabular-nums;
	}

	.kinds {
		padding: 0.1rem 0 0.35rem 1.65rem;
		color: var(--muted-foreground);
		font-size: 11px;
	}

	.kinds li {
		display: flex;
		justify-content: space-between;
		gap: 0.75rem;
		padding: 0.12rem 0.35rem;
	}

	.kinds .leaf {
		padding: 0.12rem 0;
		font-size: 11px;
	}

	@media (max-width: 899px) {
		.index {
			width: auto;
			border-right: 0;
			border-bottom: 1px solid var(--border);
			padding: 0.45rem 0.75rem;
		}

		.index > ul {
			display: flex;
			flex-wrap: wrap;
			gap: 0.15rem;
		}

		.kinds:not(.leaves) {
			display: none;
		}

		.leaves {
			flex-basis: 100%;
		}
	}
</style>
