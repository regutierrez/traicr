<script lang="ts">
	import type { Snippet } from 'svelte';
	import HistoryRail from './HistoryRail.svelte';

	let { children }: { children: Snippet } = $props();

	let searchRef = $state<HTMLInputElement | null>(null);

	function isEditableTarget(event: KeyboardEvent) {
		const target = event.target;
		if (!(target instanceof HTMLElement)) return false;
		if (target.isContentEditable) return true;
		const tag = target.tagName;
		return tag === 'INPUT' || tag === 'TEXTAREA' || tag === 'SELECT';
	}

	function onkeydown(event: KeyboardEvent) {
		if (event.key !== '/' || event.metaKey || event.ctrlKey || event.altKey) return;
		if (isEditableTarget(event)) return;
		event.preventDefault();
		searchRef?.focus();
	}
</script>

<svelte:window {onkeydown} />

<div
	class="dark bg-background text-foreground flex h-[calc(100svh-2.5rem-1px)] min-h-0 overflow-hidden"
	data-concept="studio"
>
	<HistoryRail bind:searchRef />
	<div class="flex min-h-0 min-w-0 flex-1 flex-col">
		{@render children()}
	</div>
</div>
