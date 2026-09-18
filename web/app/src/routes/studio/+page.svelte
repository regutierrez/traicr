<script lang="ts">
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import StudioShell from '$lib/components/studio/StudioShell.svelte';
	import { SvelteSet } from 'svelte/reactivity';
	import { featuredEvents, sessionById, sessions } from '$lib/data/archive.js';
	import type { Session } from '$lib/data/types.js';
	import { Badge } from '$lib/components/ui/badge/index.js';
	import { Button } from '$lib/components/ui/button/index.js';
	import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '$lib/components/ui/card/index.js';
	import { ScrollArea } from '$lib/components/ui/scroll-area/index.js';
	import { Separator } from '$lib/components/ui/separator/index.js';
	import { Tabs, TabsList, TabsTrigger } from '$lib/components/ui/tabs/index.js';
	import FileEditLine from 'remixicon-svelte/icons/file-edit-line';
	import GitRepositoryLine from 'remixicon-svelte/icons/git-repository-line';
	import StackLine from 'remixicon-svelte/icons/stack-line';
	import TerminalBoxLine from 'remixicon-svelte/icons/terminal-box-line';

	let range = $state('all');

	const featured = $derived(sessionById('billing-csv') ?? sessions[0]);
	const visible = $derived(range === 'today' ? sessions.filter((session) => session.day === 'today') : sessions);

	const usage = $derived.by(() => {
		const tools = new SvelteSet<string>();
		const edited = new SvelteSet<string>();
		for (const event of featuredEvents) {
			if (event.tool) tools.add(event.tool);
			if (event.path && (event.tool === 'edit' || event.tool === 'create')) {
				edited.add(event.path);
			}
		}
		return { tools: [...tools], edited: edited.size };
	});

	function openSession(session: Session) {
		void goto(resolve('/studio/[id]', { id: session.id }));
	}
</script>

<svelte:head>
	<title>Studio · Agents</title>
</svelte:head>

<StudioShell>
	<ScrollArea class="h-full">
		<div class="mx-auto flex w-full max-w-5xl flex-col gap-6 px-6 py-6">
			<div>
				<p class="text-[11px] tracking-wide text-muted-foreground uppercase">Cursor × AgentTrace</p>
				<h2 class="text-lg font-medium tracking-tight">Continue an agent</h2>
			</div>

			{#if featured}
				<Card class="bg-card/80 ring-foreground/8">
					<CardHeader class="border-b border-border">
						<div class="flex flex-wrap items-center gap-2">
							<Badge variant="outline">{featured.harness}</Badge>
							<Badge variant="secondary">{featured.model}</Badge>
							<span class="text-[12px] text-muted-foreground">{featured.time}</span>
						</div>
						<CardTitle class="text-xl">{featured.title}</CardTitle>
						<CardDescription class="max-w-2xl text-[13px] leading-relaxed">
							{featured.preview}
						</CardDescription>
					</CardHeader>
					<CardContent class="flex flex-wrap items-center justify-between gap-3 pt-4">
						<div class="flex flex-wrap items-center gap-3 text-[12px] text-muted-foreground">
							<span class="inline-flex items-center gap-1.5">
								<FileEditLine class="size-3.5" />
								{usage.edited} files edited
							</span>
							<span class="inline-flex items-center gap-1.5">
								<TerminalBoxLine class="size-3.5" />
								{usage.tools.length} tools used
							</span>
							<span class="flex flex-wrap gap-1">
								{#each usage.tools as tool (tool)}
									<Badge variant="outline" class="h-4 rounded-sm px-1 font-mono font-normal">
										{tool}
									</Badge>
								{/each}
							</span>
						</div>
						<Button onclick={() => openSession(featured)}>Continue</Button>
					</CardContent>
				</Card>
			{/if}

			<Separator />

			<div class="flex items-center justify-between gap-3">
				<div class="flex items-center gap-2">
					<StackLine class="size-3.5 text-muted-foreground" />
					<h3 class="text-[13px] font-medium">Traces</h3>
				</div>
				<Tabs bind:value={range}>
					<TabsList variant="line">
						<TabsTrigger value="all">All</TabsTrigger>
						<TabsTrigger value="today">Today</TabsTrigger>
					</TabsList>
				</Tabs>
			</div>

			<div class="overflow-hidden rounded-lg border border-border">
				<div
					class="text-muted-foreground grid grid-cols-[88px_minmax(0,1.4fr)_132px_64px_minmax(0,1fr)_88px] gap-3 border-b border-border bg-sidebar px-3 py-2 text-[11px] tracking-wide uppercase"
				>
					<span>Harness</span>
					<span>Title</span>
					<span>Model</span>
					<span>Events</span>
					<span>Repo</span>
					<span>Updated</span>
				</div>
				<ul>
					{#each visible as session (session.id)}
						<li class="border-b border-border last:border-b-0">
							<button
								type="button"
								class="grid w-full grid-cols-[88px_minmax(0,1.4fr)_132px_64px_minmax(0,1fr)_88px] items-center gap-3 px-3 py-2 text-left text-[13px] hover:bg-muted/40"
								onclick={() => openSession(session)}
							>
								<Badge variant="outline" class="w-fit font-normal">{session.harness}</Badge>
								<span class="truncate">{session.title}</span>
								<span class="truncate font-mono text-[11px] text-muted-foreground">{session.model}</span>
								<span class="font-mono text-[11px] text-muted-foreground">{session.eventCount}</span>
								<span class="flex min-w-0 items-center gap-1 truncate text-[12px] text-muted-foreground">
									<GitRepositoryLine class="size-3 shrink-0" />
									<span class="truncate">{session.repository.replace('github.com/', '')}</span>
								</span>
								<span class="text-[12px] text-muted-foreground">{session.time}</span>
							</button>
						</li>
					{/each}
				</ul>
			</div>
		</div>
	</ScrollArea>
</StudioShell>
