<script lang="ts">
	import InboxShell from '$lib/components/inbox/InboxShell.svelte';
	import SessionRow from '$lib/components/inbox/SessionRow.svelte';
	import { ScrollArea } from '$lib/components/ui/scroll-area/index.js';
	import * as Tabs from '$lib/components/ui/tabs/index.js';
	import { groupSessions, harnesses, sessions } from '$lib/data/archive';
	import type { Harness } from '$lib/data/types';

	type Filter = 'all' | Harness;

	let filter = $state<Filter>('all');
	let focusedId = $state(sessions[0]?.id ?? '');

	const filtered = $derived.by(() => {
		if (filter === 'all') return sessions;
		return sessions.filter((session) => session.harness === filter);
	});

	const listIds = $derived(filtered.map((session) => session.id));

	const groups = $derived.by(() => {
		const grouped = groupSessions(filtered);
		return [
			{ id: 'today', label: 'Today', items: grouped.today },
			{ id: 'yesterday', label: 'Yesterday', items: grouped.yesterday },
			{ id: 'earlier', label: 'Earlier', items: grouped.earlier }
		].filter((group) => group.items.length > 0);
	});

	const highlightId = $derived(listIds.includes(focusedId) ? focusedId : (listIds[0] ?? ''));
</script>

<svelte:head>
	<title>Transcripts · traicr</title>
</svelte:head>

<InboxShell listIds={listIds} bind:focusedId>
	<div class="flex min-h-0 min-w-0 flex-1 flex-col">
		<div class="flex h-12 shrink-0 items-center px-4">
			<h1 class="text-sm font-medium">Transcripts</h1>
		</div>
		<div class="px-3 pb-2">
			<Tabs.Root bind:value={filter}>
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
							<SessionRow {session} selected={session.id === highlightId} />
						{/each}
					{/each}
				</div>
			{/if}
		</ScrollArea>
	</div>
</InboxShell>
