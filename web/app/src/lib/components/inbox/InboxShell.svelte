<script lang="ts">
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import { Badge } from '$lib/components/ui/badge/index.js';
	import { Button } from '$lib/components/ui/button/index.js';
	import { Kbd } from '$lib/components/ui/kbd/index.js';
	import { Separator } from '$lib/components/ui/separator/index.js';
	import * as Tooltip from '$lib/components/ui/tooltip/index.js';
	import { IsMobile } from '$lib/hooks/is-mobile.svelte.js';
	import ArrowLeftLine from 'remixicon-svelte/icons/arrow-left-line';
	import ComputerLine from 'remixicon-svelte/icons/computer-line';
	import FileListLine from 'remixicon-svelte/icons/file-list-line';
	import SearchLine from 'remixicon-svelte/icons/search-line';
	import Upload2Line from 'remixicon-svelte/icons/upload-2-line';
	import type { Snippet } from 'svelte';
	import CommandSearch from './CommandSearch.svelte';

	let {
		children,
		compact,
		listIds = [],
		focusedId = $bindable('')
	}: {
		children: Snippet;
		compact?: boolean;
		listIds?: string[];
		focusedId?: string;
	} = $props();

	const mobile = new IsMobile();
	const isViewer = $derived(Boolean(page.params.id));
	const isCompact = $derived(compact ?? isViewer);
	const railNarrow = $derived(isCompact && !mobile.current);
	let searchOpen = $state(false);

	function isTypingTarget(target: EventTarget | null) {
		if (!(target instanceof HTMLElement)) return false;
		return Boolean(target.closest('input, textarea, select, [contenteditable="true"]'));
	}

	function onKeydown(event: KeyboardEvent) {
		if (event.key === 'k' && (event.metaKey || event.ctrlKey)) {
			event.preventDefault();
			searchOpen = true;
			return;
		}

		if (
			event.key === '/' &&
			!event.metaKey &&
			!event.ctrlKey &&
			!event.altKey &&
			!isTypingTarget(event.target)
		) {
			event.preventDefault();
			searchOpen = true;
			return;
		}

		if (searchOpen || event.metaKey || event.ctrlKey || event.altKey || isTypingTarget(event.target)) {
			return;
		}

		if (!listIds.length) return;

		const current = listIds.includes(focusedId) ? focusedId : listIds[0];
		const index = Math.max(listIds.indexOf(current), 0);

		if (event.key === 'j' || event.key === 'ArrowDown') {
			event.preventDefault();
			focusedId = listIds[Math.min(index + 1, listIds.length - 1)];
			return;
		}

		if (event.key === 'k' || event.key === 'ArrowUp') {
			event.preventDefault();
			focusedId = listIds[Math.max(index - 1, 0)];
			return;
		}

		if (event.key === 'Enter' && current) {
			event.preventDefault();
			goto(resolve('/inbox/[id]', { id: current }));
		}
	}
</script>

<svelte:window onkeydown={onKeydown} />

<Tooltip.Provider>
	<div
		data-concept="inbox"
		class="bg-background text-foreground flex h-[calc(100svh-2.5rem-1px)] min-h-0 overflow-hidden"
	>
		<aside
			class={[
				'bg-sidebar text-sidebar-foreground hidden shrink-0 flex-col md:flex',
				railNarrow ? 'w-16 items-center px-1.5' : 'w-[220px] px-3'
			]}
		>
			<div class={['flex h-12 items-center', railNarrow ? 'justify-center' : 'gap-2']}>
				<a
					href={resolve('/inbox')}
					class="text-[15px] font-medium tracking-tight"
					aria-label="traicr transcripts"
				>
					traicr
				</a>
				{#if !railNarrow}
					<Badge variant="outline" class="h-5 px-1.5 text-[10px] font-medium tracking-wide">
						PRIVATE
					</Badge>
				{/if}
			</div>

			{#if railNarrow}
				<Tooltip.Root>
					<Tooltip.Trigger>
						{#snippet child({ props })}
							<Button
								{...props}
								variant="outline"
								size="icon-sm"
								class="text-muted-foreground mb-3 shadow-none"
								onclick={() => (searchOpen = true)}
								aria-label="Search traces"
							>
								<SearchLine class="size-3.5" />
							</Button>
						{/snippet}
					</Tooltip.Trigger>
					<Tooltip.Content>
						Search traces
						<Kbd class="ml-1">/</Kbd>
					</Tooltip.Content>
				</Tooltip.Root>
			{:else}
				<Button
					variant="outline"
					class="text-muted-foreground mb-3 h-8 w-full justify-between px-2 font-normal shadow-none"
					onclick={() => (searchOpen = true)}
				>
					<span class="flex items-center gap-2">
						<SearchLine class="size-3.5" />
						Search traces
					</span>
					<Kbd>/</Kbd>
				</Button>
			{/if}

			<Separator class="mb-2" />

			<nav class={['flex flex-1 flex-col gap-0.5', railNarrow && 'items-center']} aria-label="Inbox">
				<Button
					href={resolve('/inbox')}
					variant={isViewer ? 'ghost' : 'secondary'}
					size={railNarrow ? 'icon-sm' : 'sm'}
					class={[
						'text-sidebar-foreground font-normal',
						!railNarrow && 'w-full justify-start gap-2'
					]}
					aria-current={isViewer ? undefined : 'page'}
					aria-label={isViewer ? 'Back to Transcripts' : 'Transcripts'}
				>
					{#if isViewer}
						<ArrowLeftLine class="size-3.5" />
					{:else}
						<FileListLine class="size-3.5" />
					{/if}
					{#if !railNarrow}
						<span>Transcripts</span>
					{/if}
				</Button>

				<Tooltip.Root>
					<Tooltip.Trigger>
						{#snippet child({ props })}
							<Button
								{...props}
								variant="ghost"
								size={railNarrow ? 'icon-sm' : 'sm'}
								class={[
									'text-muted-foreground font-normal',
									!railNarrow && 'w-full justify-start gap-2'
								]}
								title="Prototype"
								aria-label="Imports"
							>
								<Upload2Line class="size-3.5" />
								{#if !railNarrow}
									Imports
								{/if}
							</Button>
						{/snippet}
					</Tooltip.Trigger>
					<Tooltip.Content>Prototype</Tooltip.Content>
				</Tooltip.Root>

				<Tooltip.Root>
					<Tooltip.Trigger>
						{#snippet child({ props })}
							<Button
								{...props}
								variant="ghost"
								size={railNarrow ? 'icon-sm' : 'sm'}
								class={[
									'text-muted-foreground font-normal',
									!railNarrow && 'w-full justify-start gap-2'
								]}
								title="Prototype"
								aria-label="Machines"
							>
								<ComputerLine class="size-3.5" />
								{#if !railNarrow}
									Machines
								{/if}
							</Button>
						{/snippet}
					</Tooltip.Trigger>
					<Tooltip.Content>Prototype</Tooltip.Content>
				</Tooltip.Root>
			</nav>
		</aside>

		<div class="flex min-h-0 min-w-0 flex-1 flex-col">
			<header
				class="border-border/70 flex h-12 shrink-0 items-center gap-2 border-b px-3 md:hidden"
			>
				{#if isViewer}
					<Button
						href={resolve('/inbox')}
						variant="ghost"
						size="icon-sm"
						aria-label="Back to Transcripts"
					>
						<ArrowLeftLine class="size-4" />
					</Button>
				{/if}
				<a href={resolve('/inbox')} class="text-[15px] font-medium tracking-tight">traicr</a>
				<Badge variant="outline" class="h-5 px-1.5 text-[10px] font-medium tracking-wide">
					PRIVATE
				</Badge>
				<Button
					variant="outline"
					class="text-muted-foreground ml-auto h-8 gap-2 px-2 font-normal shadow-none"
					onclick={() => (searchOpen = true)}
				>
					<SearchLine class="size-3.5" />
					<span class="hidden sm:inline">Search traces</span>
					<Kbd>/</Kbd>
				</Button>
			</header>
			<div class="flex min-h-0 min-w-0 flex-1 flex-col">
				{@render children()}
			</div>
		</div>
	</div>
</Tooltip.Provider>

<CommandSearch bind:open={searchOpen} />
