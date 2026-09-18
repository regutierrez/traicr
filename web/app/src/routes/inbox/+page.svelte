<script lang="ts">
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import InboxShell from '$lib/components/inbox/InboxShell.svelte';
	import SessionRow from '$lib/components/inbox/SessionRow.svelte';
	import { ScrollArea } from '$lib/components/ui/scroll-area/index.js';
	import * as Tabs from '$lib/components/ui/tabs/index.js';
	import { groupSessions, harnesses, sessions } from '$lib/data/archive';
	import type { Harness } from '$lib/data/types';

	type Filter = 'all' | Harness;

	let filter = $state<Filter>('all');
	let cursor = $state(0);

	const filtered = $derived.by(() => {
		if (filter === 'all') return sessions;
		return sessions.filter((session) => session.harness === filter);
	});

	const groups = $derived.by(() => {
		const grouped = groupSessions(filtered);
		return [
			{ id: 'today', label: 'Today', items: grouped.today },
			{ id: 'yesterday', label: 'Yesterday', items: grouped.yesterday },
			{ id: 'earlier', label: 'Earlier', items: grouped.earlier }
		].filter((group) => group.items.length > 0);
	});

	const focusedId = $derived(filtered[Math.min(cursor, Math.max(filtered.length - 1, 0))]?.id);

	function onFilterChange(next: string) {
		filter = next as Filter;
		cursor = 0;
	}

	function onKeydown(event: KeyboardEvent) {
		if (event.defaultPrevented || event.metaKey || event.ctrlKey || event.altKey) return;
		if (event.target instanceof HTMLElement) {
			if (
				event.target.closest(
					'input, textarea, select, [contenteditable="true"], [role="dialog"]'
				)
			) {
				return;
			}
		}

		if (event.key === 'j' || event.key === 'ArrowDown') {
			event.preventDefault();
			cursor = Math.min(cursor + 1, Math.max(filtered.length - 1, 0));
			return;
		}

		if (event.key === 'k' || event.key === 'ArrowUp') {
			event.preventDefault();
			cursor = Math.max(cursor - 1, 0);
			return;
		}

		if (event.key === 'Enter' && focusedId) {
			event.preventDefault();
			goto(resolve('/inbox/[id]', { id: focusedId }));
		}
	}
</script>

<svelte:head>
	<title>Transcripts · traicr</title>
</svelte:head>

<svelte:window onkeydown={onKeydown} />

<InboxShell>
	<div class="flex min-h-0 min-w-0 flex-1 flex-col">
		<div class="flex h-12 shrink-0 items-center px-4">
			<h1 class="text-sm font-medium">Transcripts</h1>
		</div>
		<div class="px-3 pb-2">
			<Tabs.Root value={filter} onValueChange={onFilterChange}>
				<Tabs.List
					variant="line"
					class="h-auto w-full flex-wrap justify-start gap-1 bg-transparent p-0"
				>
					<Tabs.Trigger value="all" class="px-2 text-xs">All</Tabs.Trigger>
					{#each harnesses as harness (harness)}
						<Tabs.Trigger value={harness} class="px-2 text-xs">{harness}</Tabs.Trigger>
					{/each}
				</Tabs.List>
			</Tabs.Root>
		</div>
		<ScrollArea class="min-h-0 flex-1">
			{#if filtered.length === 0}
				<p class="text-muted-foreground px-4 py-8 text-sm">No traces for this harness.</p>
			{:else}
				<div role="listbox" aria-label="Transcripts" class="pb-8">
					{#each groups as group (group.id)}
						<p
							class="text-muted-foreground px-3 pt-3 pb-1 text-[11px] font-medium tracking-wide uppercase"
						>
							{group.label}
						</p>
						{#each group.items as session (session.id)}
							<SessionRow {session} selected={session.id === focusedId} />
						{/each}
					{/each}
				</div>
			{/if}
		</ScrollArea>
	</div>
</InboxShell>
