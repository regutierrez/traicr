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
						{#each counts.toolKinds as kind (kind.id)}
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
		width: 268px;
		flex-shrink: 0;
		overflow: auto;
		padding: 1.25rem 1.5rem 2rem;
		background: transparent;
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
		height: 28px;
		align-items: center;
		justify-content: space-between;
		gap: 0.75rem;
		border: 0;
		border-radius: 6px;
		background: transparent;
		color: var(--muted-foreground);
		padding: 0 0.4rem;
		font: inherit;
		font-size: 14px;
		font-weight: 500;
		line-height: 20px;
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
	.leaf:hover,
	.leaf:focus-visible {
		color: var(--foreground);
	}

	.row.is-current,
	.leaf.is-current {
		color: var(--foreground);
		background: var(--card);
		box-shadow: var(--contour);
	}

	.count {
		color: color-mix(in srgb, var(--muted-foreground) 70%, transparent);
		font-size: 12px;
		font-weight: 500;
		font-variant-numeric: tabular-nums;
	}

	.kinds {
		padding: 0.15rem 0 0.45rem 1.35rem;
		color: var(--muted-foreground);
		font-size: 13px;
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
			padding: 0.55rem 1rem;
			border-bottom: 1px solid var(--border);
		}

		.row {
			width: auto;
			height: 28px;
			border-radius: 6px;
			background: var(--muted);
			font-size: 12px;
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
