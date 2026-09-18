<script lang="ts">
	import { page } from '$app/state';
	import StudioShell from '$lib/components/studio/StudioShell.svelte';
	import ToolCard from '$lib/components/studio/ToolCard.svelte';
	import Waterfall from '$lib/components/studio/Waterfall.svelte';
	import { childTranscripts, featuredEvents, sessionById } from '$lib/data/archive.js';
	import type { ChildTrace, FileChange, Session, TraceEvent } from '$lib/data/types.js';
	import { Badge } from '$lib/components/ui/badge/index.js';
	import { Bubble, BubbleContent } from '$lib/components/ui/bubble/index.js';
	import { Button } from '$lib/components/ui/button/index.js';
	import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '$lib/components/ui/card/index.js';
	import { Input } from '$lib/components/ui/input/index.js';
	import { Kbd } from '$lib/components/ui/kbd/index.js';
	import { ScrollArea } from '$lib/components/ui/scroll-area/index.js';
	import { Tabs, TabsContent, TabsList, TabsTrigger } from '$lib/components/ui/tabs/index.js';
	import ArrowLeftLine from 'remixicon-svelte/icons/arrow-left-line';
	import FileEditLine from 'remixicon-svelte/icons/file-edit-line';
	import Robot2Line from 'remixicon-svelte/icons/robot-2-line';
	import SendPlaneLine from 'remixicon-svelte/icons/send-plane-line';

	let childId = $state<string | null>(null);
	let pane = $state('timing');

	const session = $derived(sessionById(page.params.id ?? ''));
	const activeChild = $derived(session?.children.find((child) => child.id === childId) ?? null);
	const parentEvents = $derived(session ? eventsForSession(session) : []);
	const events = $derived(activeChild ? eventsForChild(activeChild) : parentEvents);
	const childCount = $derived(activeChild ? 0 : (session?.children.length ?? 0));

	function eventsForSession(current: Session): TraceEvent[] {
		if (current.id === 'billing-csv') return featuredEvents;
		return standInEvents(current.id, current.preview, current.model, current.time, current.files);
	}

	function eventsForChild(child: ChildTrace): TraceEvent[] {
		return childTranscripts[child.id] ?? standInEvents(child.id, child.preview, child.model, '', []);
	}

	function standInEvents(
		id: string,
		preview: string,
		model: string,
		time: string,
		files: FileChange[]
	): TraceEvent[] {
		const next: TraceEvent[] = [
			{
				id: `${id}-user`,
				role: 'user',
				kind: 'message',
				timestamp: time,
				content: preview
			}
		];

		for (const file of files) {
			next.push({
				id: `${id}-${file.path}`,
				role: 'assistant',
				kind: 'tool_call',
				timestamp: time,
				tool: file.action,
				path: file.path,
				args: `${file.action} ${file.path}`,
				durationMs: 80,
				content: `${file.action} ${file.path}`,
				diff:
					file.additions != null || file.deletions != null
						? [file.additions != null ? `+${file.additions}` : '', file.deletions != null ? `−${file.deletions}` : '']
								.filter(Boolean)
								.join(' ')
						: undefined
			});
		}

		next.push({
			id: `${id}-assistant`,
			role: 'assistant',
			kind: 'message',
			timestamp: time,
			model,
			content: preview
		});

		return next;
	}

	function openChild(child: ChildTrace) {
		childId = child.id;
	}

	function closeChild() {
		childId = null;
	}

	function onkeydown(event: KeyboardEvent) {
		if (event.key !== 'Escape' || !childId) return;
		event.preventDefault();
		closeChild();
	}

	function childStatusVariant(status: ChildTrace['status']) {
		if (status === 'running') return 'default' as const;
		if (status === 'unread') return 'secondary' as const;
		return 'outline' as const;
	}
</script>

<svelte:head>
	<title>{session ? `${session.title} · Studio` : 'Studio'}</title>
</svelte:head>

<svelte:window {onkeydown} />

<StudioShell>
	{#if !session}
		<div class="flex flex-1 items-center justify-center text-sm text-muted-foreground">
			This trace is not in the archive.
		</div>
	{:else}
		<div class="flex min-h-0 min-w-0 flex-1">
			<section class="flex min-w-0 flex-1 flex-col">
				<header class="flex items-center gap-2 border-b border-border px-4 py-2.5">
					{#if activeChild}
						<Button variant="ghost" size="icon-xs" onclick={closeChild} aria-label="Back to parent">
							<ArrowLeftLine />
						</Button>
						<nav class="min-w-0 text-[13px]" aria-label="Thread">
							<button type="button" class="text-muted-foreground hover:text-foreground" onclick={closeChild}>
								{session.title}
							</button>
							<span class="text-muted-foreground"> ▸ </span>
							<span class="text-foreground">{activeChild.title}</span>
						</nav>
						<Kbd class="ml-auto">Esc</Kbd>
					{:else}
						<div class="min-w-0">
							<div class="flex items-center gap-2">
								<Badge variant="outline">{session.harness}</Badge>
								<h2 class="truncate text-[13px] font-medium">{session.title}</h2>
							</div>
							<p class="text-[11px] text-muted-foreground">
								{session.model} · {session.eventCount} records · {session.machine}
							</p>
						</div>
					{/if}
				</header>

				<ScrollArea class="min-h-0 flex-1">
					<div class="mx-auto flex w-full max-w-3xl flex-col gap-3 px-4 py-5">
						{#each events as event (event.id)}
							{#if event.kind === 'tool_call' || event.kind === 'tool_result'}
								<ToolCard {event} />
							{:else if event.kind === 'delegation' && event.child}
								<Card size="sm" class="max-w-md ring-foreground/8">
									<CardHeader class="pb-0">
										<div class="flex items-center gap-2">
											<Robot2Line class="size-3.5 text-muted-foreground" />
											<Badge variant={childStatusVariant(event.child.status)}>
												{event.child.status}
											</Badge>
											<span class="text-[11px] text-muted-foreground">{event.child.duration}</span>
										</div>
										<CardTitle class="text-sm">{event.child.title}</CardTitle>
										<CardDescription>{event.child.preview}</CardDescription>
									</CardHeader>
									<CardContent class="flex items-center justify-between gap-2">
										<span class="font-mono text-[11px] text-muted-foreground">{event.child.model}</span>
										<Button size="sm" variant="outline" onclick={() => openChild(event.child!)}>
											Open
										</Button>
									</CardContent>
								</Card>
							{:else if event.role === 'user'}
								<div class="flex justify-end">
									<Bubble variant="muted" align="end">
										<BubbleContent class="text-[13px] leading-relaxed">
											{event.content}
										</BubbleContent>
									</Bubble>
								</div>
							{:else if event.kind === 'reasoning'}
								<p class="max-w-2xl text-[12px] leading-relaxed text-muted-foreground italic">
									{event.content}
								</p>
							{:else}
								<div class="max-w-2xl text-[13px] leading-relaxed text-foreground/90">
									{event.content}
								</div>
							{/if}
						{/each}
					</div>
				</ScrollArea>

				<footer class="border-t border-border bg-background px-4 py-3">
					<div class="mx-auto flex max-w-3xl items-center gap-2">
						<Input
							disabled
							placeholder="This is an archive — collection stays on the collector"
							aria-label="Archive composer"
						/>
						<Button disabled size="icon" aria-label="Send disabled">
							<SendPlaneLine />
						</Button>
					</div>
				</footer>
			</section>

			<div class="hidden h-full lg:flex">
				<Tabs bind:value={pane} class="flex h-full flex-col">
					<div class="flex items-center justify-between border-b border-border px-3 py-2">
						<TabsList variant="line">
							<TabsTrigger value="timing">Timing</TabsTrigger>
							<TabsTrigger value="files">Files</TabsTrigger>
						</TabsList>
					</div>
					<TabsContent value="timing" class="min-h-0 flex-1">
						<Waterfall
							{events}
							model={activeChild?.model ?? session.model}
							recordCount={events.length}
							{childCount}
						/>
					</TabsContent>
					<TabsContent value="files" class="min-h-0 w-[300px] flex-1 overflow-auto bg-sidebar">
						<ul class="flex flex-col gap-1 p-3">
							{#each session.files as file (file.path)}
								<li class="flex items-start gap-2 rounded-md px-2 py-1.5 text-[12px]">
									<FileEditLine class="mt-0.5 size-3.5 shrink-0 text-muted-foreground" />
									<div class="min-w-0">
										<p class="truncate font-mono">{file.path}</p>
										<p class="text-[11px] text-muted-foreground">
											{file.action}
											{#if file.additions != null}
												+{file.additions}
											{/if}
											{#if file.deletions != null}
												−{file.deletions}
											{/if}
										</p>
									</div>
								</li>
							{:else}
								<li class="px-2 py-6 text-center text-[12px] text-muted-foreground">No files in this trace</li>
							{/each}
						</ul>
					</TabsContent>
				</Tabs>
			</div>
		</div>
	{/if}
</StudioShell>
