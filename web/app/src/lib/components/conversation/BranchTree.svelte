<script lang="ts">
	import { buildActivePathIds, buildTree, flattenTree, treeLabel } from '$lib/transcript/tree';
	import type { TranscriptEntry } from '$lib/transcript/types';

	let {
		entries,
		leafId,
		targetId,
		query = $bindable(''),
		onSelect
	}: {
		entries: TranscriptEntry[];
		leafId: string;
		targetId: string;
		query?: string;
		onSelect: (id: string) => void;
	} = $props();

	let roots = $derived(buildTree(entries));
	let active = $derived(buildActivePathIds(entries, leafId));
	let nodes = $derived(
		flattenTree(roots, active).filter((item) => {
			if (!query.trim()) return true;
			return treeLabel(item.node.entry).toLowerCase().includes(query.trim().toLowerCase());
		})
	);
</script>

<aside class="tree" aria-label="Conversation branches">
	<label class="sr-only" for="branch-search">Search branches</label>
	<input id="branch-search" bind:value={query} type="search" placeholder="Search branches" />
	<ul>
		{#each nodes as item (item.node.entry.id)}
			<li style:--indent={item.indent}>
				<button
					type="button"
					class={['node', active.has(item.node.entry.id) && 'in-path', item.node.entry.id === targetId && 'is-active']}
					onclick={() => onSelect(item.node.entry.id)}
				>
					<span class="pad"></span>
					<span class="label">{treeLabel(item.node.entry)}</span>
				</button>
			</li>
		{/each}
	</ul>
</aside>

<style>
	.tree {
		display: flex;
		flex-direction: column;
		min-height: 0;
		border-right: 1px solid var(--border);
		background: var(--sidebar);
	}

	input {
		height: 1.75rem;
		margin: 0.55rem 0.6rem 0.35rem;
		border: 1px solid var(--input);
		border-radius: var(--radius);
		background: var(--background);
		color: var(--foreground);
		padding: 0 0.45rem;
		font: inherit;
		font-size: 12px;
	}

	ul {
		min-height: 0;
		margin: 0;
		padding: 0 0 0.75rem;
		overflow: auto;
		list-style: none;
	}

	.node {
		display: flex;
		width: 100%;
		align-items: center;
		gap: 0.35rem;
		border: 0;
		background: transparent;
		color: var(--muted-foreground);
		padding: 0.22rem 0.6rem;
		font: inherit;
		font-size: 11px;
		text-align: left;
		cursor: pointer;
	}

	.pad {
		width: calc(var(--indent) * 0.7rem);
		flex-shrink: 0;
	}

	.label {
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	.node:hover,
	.node:focus-visible,
	.node.in-path {
		color: var(--foreground);
		background: var(--sidebar-accent);
	}

	.node.is-active {
		color: var(--primary);
		box-shadow: inset 2px 0 0 var(--primary);
	}
</style>
