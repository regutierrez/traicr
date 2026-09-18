<script lang="ts">
	import type { Snippet } from 'svelte';
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import Book2Line from 'remixicon-svelte/icons/book-2-line';
	import GitCommitLine from 'remixicon-svelte/icons/git-commit-line';
	import { Badge } from '$lib/components/ui/badge';
	import { Button } from '$lib/components/ui/button';
	import { Kbd } from '$lib/components/ui/kbd';
	import { Separator } from '$lib/components/ui/separator';

	let {
		title = 'Ledger',
		kicker = 'Evidence layer',
		toolbar,
		children
	}: {
		title?: string;
		kicker?: string;
		toolbar?: Snippet;
		children: Snippet;
	} = $props();

	const viewerId = $derived(page.params.id ?? '');
</script>

<div class="flex h-[calc(100svh-2.5rem-1px)] min-h-0 flex-col bg-background text-foreground">
	<header class="flex shrink-0 items-center gap-3 border-b border-border px-3 py-2.5 sm:px-4">
		<div
			class="flex size-8 shrink-0 items-center justify-center rounded-sm border border-border bg-card text-primary"
			aria-hidden="true"
		>
			<Book2Line class="size-4" />
		</div>
		<div class="min-w-0">
			<p
				class="font-mono text-[10px] leading-none tracking-[0.2em] text-muted-foreground uppercase"
			>
				{kicker}
			</p>
			<div class="mt-1 flex min-w-0 items-center gap-2">
				<a href={resolve('/ledger')} class="truncate text-sm font-medium tracking-tight hover:underline">
					{title}
				</a>
				{#if viewerId}
					<Badge variant="outline" class="hidden h-5 rounded-sm font-mono text-[10px] sm:inline-flex">
						{viewerId}
					</Badge>
				{/if}
			</div>
		</div>

		<Separator orientation="vertical" class="hidden h-8 sm:block" />

		<div class="hidden items-center gap-1.5 font-mono text-[11px] text-muted-foreground sm:flex">
			<GitCommitLine class="size-3.5" />
			<span>OpenTraces × traces.com</span>
		</div>

		{#if toolbar}
			<div class="min-w-0 flex-1">{@render toolbar()}</div>
		{:else}
			<div class="flex-1"></div>
		{/if}

		<div class="hidden items-center gap-1.5 md:flex">
			{#if viewerId}
				<Button href={resolve('/ledger')} size="xs" variant="ghost" class="font-mono text-[11px]">
					All traces
				</Button>
			{:else}
				<span class="font-mono text-[11px] text-muted-foreground">Filter</span>
				<Kbd>/</Kbd>
			{/if}
		</div>
	</header>

	<div class="min-h-0 flex-1">{@render children()}</div>
</div>
