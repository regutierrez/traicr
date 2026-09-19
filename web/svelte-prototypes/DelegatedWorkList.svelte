<script lang="ts">
	import DelegatedWorkRow from './DelegatedWorkRow.svelte';

	type ChildTrace = {
		id: string;
		title: string;
		harness: string;
		status: 'unread' | 'running' | 'finished';
		duration?: string;
	};

	let {
		children,
		selectedId = $bindable<string | null>(null)
	}: {
		children: ChildTrace[];
		selectedId?: string | null;
	} = $props();
</script>

<ul class="delegated-work-list">
	{#each children as child (child.id)}
		<li>
			<DelegatedWorkRow
				title={child.title}
				harness={child.harness}
				status={child.status}
				duration={child.duration}
				selected={selectedId === child.id}
				onselect={() => (selectedId = child.id)}
			/>
		</li>
	{/each}
</ul>

<style>
	.delegated-work-list {
		list-style: none;
		margin: 0;
		padding: 0;
		display: grid;
		gap: 4px;
	}
</style>
