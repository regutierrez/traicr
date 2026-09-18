<script lang="ts">
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import AlertLine from 'remixicon-svelte/icons/alert-line';
	import ComputerLine from 'remixicon-svelte/icons/computer-line';
	import FolderLine from 'remixicon-svelte/icons/folder-line';
	import GitCommitLine from 'remixicon-svelte/icons/git-commit-line';
	import SearchLine from 'remixicon-svelte/icons/search-line';
	import EvidenceRow from '$lib/components/ledger/EvidenceRow.svelte';
	import LedgerShell from '$lib/components/ledger/LedgerShell.svelte';
	import { Badge } from '$lib/components/ui/badge';
	import { Button } from '$lib/components/ui/button';
	import {
		Card,
		CardContent,
		CardDescription,
		CardFooter,
		CardHeader,
		CardTitle
	} from '$lib/components/ui/card';
	import { Input } from '$lib/components/ui/input';
	import { Kbd } from '$lib/components/ui/kbd';
	import { ScrollArea } from '$lib/components/ui/scroll-area';
	import { Separator } from '$lib/components/ui/separator';
	import { Tabs, TabsList, TabsTrigger } from '$lib/components/ui/tabs';
	import { groupSessions, sessions } from '$lib/data/archive';
	import type { Session } from '$lib/data/types';

	const dayTabs = [
		{ id: 'all', label: 'All' },
		{ id: 'today', label: 'Today' },
		{ id: 'yesterday', label: 'Yesterday' },
		{ id: 'earlier', label: 'Earlier' }
	] as const;

	const dayOrder = ['today', 'yesterday', 'earlier'] as const;
	const dayLabel: Record<(typeof dayOrder)[number], string> = {
		today: 'Today',
		yesterday: 'Yesterday',
		earlier: 'Earlier'
	};

	let query = $state('');
	let dayTab = $state('all');
	let selectedId = $state(sessions[0]?.id ?? '');
	let searchEl = $state<HTMLInputElement | null>(null);

	const filtered = $derived.by(() => {
		const needle = query.trim().toLowerCase();
		return sessions.filter((session) => {
			if (dayTab !== 'all' && session.day !== dayTab) return false;
			if (!needle) return true;
			return [
				session.title,
				session.repository,
				session.harness,
				session.preview,
				session.cwd,
				session.machine,
				session.model,
				session.commit ?? ''
			]
				.join(' ')
				.toLowerCase()
				.includes(needle);
		});
	});

	const grouped = $derived(groupSessions(filtered));
	const selected = $derived(filtered.find((session) => session.id === selectedId) ?? filtered[0]);
	const activeId = $derived(selected?.id ?? '');
	const repo = $derived(selected ? (selected.repository.split('/').at(-1) ?? selected.repository) : '');

	function select(session: Session) {
		selectedId = session.id;
	}

	function openTrace(session: Session) {
		void goto(resolve('/ledger/[id]', { id: session.id }));
	}

	function move(delta: number) {
		if (filtered.length === 0) return;
		const index = filtered.findIndex((session) => session.id === activeId);
		const next = filtered[Math.min(filtered.length - 1, Math.max(0, index + delta))];
		if (next) selectedId = next.id;
	}

	function onkeydown(event: KeyboardEvent) {
		const tag = (event.target as HTMLElement | null)?.tagName;
		if (event.key === '/' && tag !== 'INPUT' && tag !== 'TEXTAREA') {
			event.preventDefault();
			searchEl?.focus();
			return;
		}
		if (event.key === 'ArrowDown') {
			event.preventDefault();
			move(1);
			return;
		}
		if (event.key === 'ArrowUp') {
			event.preventDefault();
			move(-1);
			return;
		}
		if (event.key === 'Enter' && selected && tag !== 'TEXTAREA') {
			if (tag === 'INPUT' && query && !filtered.length) return;
			event.preventDefault();
			openTrace(selected);
		}
	}
</script>

<svelte:window {onkeydown} />

<LedgerShell title="Ledger" kicker="Evidence layer">
	{#snippet toolbar()}
		<div class="relative max-w-md">
			<SearchLine class="pointer-events-none absolute top-1/2 left-2 size-3.5 -translate-y-1/2 text-muted-foreground" />
			<Input
				bind:ref={searchEl}
				bind:value={query}
				type="search"
				placeholder="Filter time, repo, harness, title…"
				aria-label="Filter traces"
				class="h-7 rounded-sm border-border bg-card pl-7 font-mono text-xs"
			/>
		</div>
	{/snippet}

	<div
		class="grid h-full min-h-0 grid-rows-[minmax(16rem,1fr)_auto] lg:grid-cols-[42%_minmax(0,1fr)] lg:grid-rows-1"
	>
		<section class="flex min-h-0 flex-col border-b border-border lg:border-r lg:border-b-0">
			<div class="flex items-center justify-between gap-2 px-3 py-2">
				<p class="font-mono text-[10px] tracking-[0.16em] text-muted-foreground uppercase">
					Chronological evidence
				</p>
				<Tabs bind:value={dayTab}>
					<TabsList variant="line" class="h-7 bg-transparent">
						{#each dayTabs as tab (tab.id)}
							<TabsTrigger value={tab.id} class="px-2 font-mono text-[11px]">{tab.label}</TabsTrigger>
						{/each}
					</TabsList>
				</Tabs>
			</div>
			<Separator />
			<ScrollArea class="min-h-0 flex-1">
				{#if filtered.length === 0}
					<p class="px-4 py-8 font-mono text-xs text-muted-foreground">No traces match.</p>
				{:else}
					{#each dayOrder as day (day)}
						{#if grouped[day].length}
							<div class="border-b border-border/70">
								<p
									class="sticky top-0 z-10 bg-sidebar/95 px-3 py-1.5 font-mono text-[10px] tracking-[0.18em] text-muted-foreground uppercase backdrop-blur"
								>
									{dayLabel[day]}
								</p>
								{#each grouped[day] as session (session.id)}
									<EvidenceRow
										{session}
										selected={session.id === activeId}
										onclick={() => select(session)}
										ondblclick={() => openTrace(session)}
									/>
								{/each}
							</div>
						{/if}
					{/each}
				{/if}
			</ScrollArea>
		</section>

		<aside class="min-h-0 bg-sidebar/40 p-3 sm:p-4">
			{#if selected}
				<Card size="sm" class="h-full rounded-sm bg-card/90 py-0 ring-border">
					<CardHeader class="border-b border-border px-4 py-3">
						<p class="font-mono text-[10px] tracking-[0.16em] text-muted-foreground uppercase">
							Evidence preview
						</p>
						<CardTitle class="text-base font-medium tracking-tight">{selected.title}</CardTitle>
						<CardDescription class="font-mono text-[11px]">
							{selected.time} · {repo} · {selected.harness} · {selected.eventCount}
						</CardDescription>
					</CardHeader>
					<CardContent class="flex min-h-0 flex-1 flex-col gap-4 overflow-y-auto px-4 py-4">
						<dl class="grid grid-cols-1 gap-2 font-mono text-[11px] sm:grid-cols-2">
							<div class="flex items-start gap-1.5">
								<GitCommitLine class="mt-0.5 size-3.5 text-muted-foreground" />
								<div>
									<dt class="text-muted-foreground">commit</dt>
									<dd>
										{#if selected.commit}
											<Kbd class="h-4 rounded-sm px-1">{selected.commit}</Kbd>
										{:else}
											<span class="text-muted-foreground">unanchored</span>
										{/if}
									</dd>
								</div>
							</div>
							<div class="flex items-start gap-1.5">
								<FolderLine class="mt-0.5 size-3.5 text-muted-foreground" />
								<div>
									<dt class="text-muted-foreground">cwd</dt>
									<dd class="text-foreground">{selected.cwd}</dd>
								</div>
							</div>
							<div class="flex items-start gap-1.5">
								<ComputerLine class="mt-0.5 size-3.5 text-muted-foreground" />
								<div>
									<dt class="text-muted-foreground">machine</dt>
									<dd class="text-foreground">{selected.machine}</dd>
								</div>
							</div>
							<div>
								<dt class="text-muted-foreground">model</dt>
								<dd class="text-foreground">{selected.model}</dd>
							</div>
						</dl>

						<div>
							<p class="mb-1.5 font-mono text-[10px] tracking-[0.16em] text-muted-foreground uppercase">
								Files touched
							</p>
							{#if selected.files.length === 0}
								<p class="font-mono text-[11px] text-muted-foreground">No files recorded.</p>
							{:else}
								<ul class="flex flex-col gap-1">
									{#each selected.files as file (file.path)}
										<li
											class="flex items-baseline justify-between gap-3 font-mono text-[11px] text-foreground"
										>
											<span class="min-w-0 truncate">{file.path}</span>
											<span class="shrink-0 tabular-nums">
												{#if file.additions}
													<span class="text-[oklch(0.42_0.1_145)]">+{file.additions}</span>
												{/if}
												{#if file.deletions}
													<span class="ml-1 text-[oklch(0.5_0.14_25)]">−{file.deletions}</span>
												{/if}
												{#if !file.additions && !file.deletions}
													<span class="text-muted-foreground">{file.action}</span>
												{/if}
											</span>
										</li>
									{/each}
								</ul>
							{/if}
						</div>

						{#if selected.warnings.length}
							<div class="flex flex-col gap-1.5">
								{#each selected.warnings as warning (warning)}
									<p class="flex items-start gap-1.5 text-[12px] text-[oklch(0.5_0.12_55)]">
										<AlertLine class="mt-0.5 size-3.5 shrink-0" />
										<span>{warning}</span>
									</p>
								{/each}
							</div>
						{/if}

						<Separator />

						<p class="text-[13px] leading-6 text-foreground/90">{selected.preview}</p>
					</CardContent>
					<CardFooter class="justify-between gap-2 border-border bg-transparent px-4 py-3">
						<Badge variant="outline" class="rounded-sm font-mono text-[10px] font-normal">
							{selected.revisionCount} revisions
						</Badge>
						<Button size="sm" class="rounded-sm" onclick={() => openTrace(selected)}>
							Open trace
						</Button>
					</CardFooter>
				</Card>
			{:else}
				<p class="px-2 py-8 font-mono text-xs text-muted-foreground">Select a session to preview.</p>
			{/if}
		</aside>
	</div>
</LedgerShell>
