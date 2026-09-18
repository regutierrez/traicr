<script lang="ts">
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import { sessions } from '$lib/data/archive.js';
	import { Badge } from '$lib/components/ui/badge/index.js';
	import { Button } from '$lib/components/ui/button/index.js';
	import { Input } from '$lib/components/ui/input/index.js';
	import { Kbd } from '$lib/components/ui/kbd/index.js';
	import { ScrollArea } from '$lib/components/ui/scroll-area/index.js';
	import { Separator } from '$lib/components/ui/separator/index.js';
	import HistoryLine from 'remixicon-svelte/icons/history-line';
	import SearchLine from 'remixicon-svelte/icons/search-line';

	let { searchRef = $bindable(null) }: { searchRef?: HTMLInputElement | null } = $props();

	let query = $state('');

	const activeId = $derived(page.params.id ?? '');
	const filtered = $derived.by(() => {
		const needle = query.trim().toLowerCase();
		if (!needle) return sessions;
		return sessions.filter((session) => {
			return (
				session.title.toLowerCase().includes(needle) ||
				session.harness.toLowerCase().includes(needle) ||
				session.repository.toLowerCase().includes(needle) ||
				session.model.toLowerCase().includes(needle)
			);
		});
	});

	function openSession(id: string) {
		void goto(resolve('/studio/[id]', { id }));
	}

	function onSearchKeydown(event: KeyboardEvent) {
		if (event.key !== 'Enter' || !filtered[0]) return;
		event.preventDefault();
		openSession(filtered[0].id);
	}
</script>

<aside
	class="bg-sidebar text-sidebar-foreground flex h-full w-[260px] shrink-0 flex-col border-r border-sidebar-border"
>
	<div class="flex items-center gap-2 px-3 pt-3 pb-2">
		<HistoryLine class="size-3.5 text-muted-foreground" />
		<h1 class="text-[13px] font-medium tracking-tight">Agents</h1>
	</div>

	<div class="relative px-3 pb-2">
		<SearchLine class="pointer-events-none absolute top-2.5 left-5.5 size-3.5 text-muted-foreground" />
		<Input
			bind:ref={searchRef}
			bind:value={query}
			type="search"
			placeholder="Search traces"
			aria-label="Search agents"
			class="h-8 bg-background/40 pr-8 pl-8 text-[13px]"
			onkeydown={onSearchKeydown}
		/>
		<Kbd class="absolute top-1.5 right-5">/</Kbd>
	</div>

	<Separator />

	<ScrollArea class="min-h-0 flex-1">
		<nav class="flex flex-col gap-px p-1.5" aria-label="Agent history">
			{#each filtered as session (session.id)}
				<Button
					variant="ghost"
					class={[
						'h-auto w-full flex-col items-start gap-0.5 rounded-md px-2 py-1.5 text-left font-normal',
						activeId === session.id
							? 'bg-sidebar-accent text-sidebar-accent-foreground hover:bg-sidebar-accent'
							: 'text-sidebar-foreground hover:bg-sidebar-accent/70'
					]}
					aria-current={activeId === session.id ? 'page' : undefined}
					onclick={() => openSession(session.id)}
				>
					<span class="w-full truncate text-[13px] leading-snug">{session.title}</span>
					<span class="flex w-full items-center gap-1.5 text-[11px] text-muted-foreground">
						<Badge variant="outline" class="h-4 rounded-sm px-1 font-normal">
							{session.harness}
						</Badge>
						<span class="truncate">{session.time}</span>
					</span>
				</Button>
			{:else}
				<p class="px-2 py-6 text-center text-[12px] text-muted-foreground">No matching agents</p>
			{/each}
		</nav>
	</ScrollArea>
</aside>
