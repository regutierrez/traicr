<script lang="ts">
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import InboxShell from '$lib/components/inbox/InboxShell.svelte';
	import { Badge } from '$lib/components/ui/badge/index.js';
	import { Button } from '$lib/components/ui/button/index.js';
	import * as Collapsible from '$lib/components/ui/collapsible/index.js';
	import { ScrollArea } from '$lib/components/ui/scroll-area/index.js';
	import { Separator } from '$lib/components/ui/separator/index.js';
	import * as Sheet from '$lib/components/ui/sheet/index.js';
	import { childTranscripts, featuredEvents, sessionById } from '$lib/data/archive';
	import { IsMobile } from '$lib/hooks/is-mobile.svelte.js';
	import type { ChildStatus, ChildTrace, Session, TraceEvent } from '$lib/data/types';
	import ArrowRightSLine from 'remixicon-svelte/icons/arrow-right-s-line';
	import CheckLine from 'remixicon-svelte/icons/check-line';
	import CloseLine from 'remixicon-svelte/icons/close-line';

	const mobile = new IsMobile();
	const id = $derived(page.params.id ?? '');
	const session = $derived(sessionById(id));
	const events = $derived(session ? sessionEvents(session) : []);
	const treeEvents = $derived(
		events.filter(
			(event) =>
				event.kind === 'message' ||
				event.kind === 'reasoning' ||
				event.kind === 'tool_call' ||
				event.kind === 'delegation'
		)
	);

	let selectedChild = $state<ChildTrace | null>(null);
	let invokingEventId = $state<string | null>(null);
	let sheetOpen = $state(false);

	const childEvents = $derived(selectedChild ? (childTranscripts[selectedChild.id] ?? []) : []);
	const paneOpenDesktop = $derived(selectedChild !== null && !mobile.current);

	function sessionEvents(current: Session): TraceEvent[] {
		if (current.id === 'billing-csv') return featuredEvents;
		return synthesizeEvents(current);
	}

	function synthesizeEvents(current: Session): TraceEvent[] {
		const repo = current.repository.split('/').slice(-1)[0] ?? current.repository;
		const synthesized: TraceEvent[] = [
			{
				id: `${current.id}-user`,
				role: 'user',
				kind: 'message',
				timestamp: current.time,
				content: current.preview
			}
		];

		if (current.files.length > 0) {
			synthesized.push({
				id: `${current.id}-plan`,
				role: 'assistant',
				kind: 'message',
				timestamp: current.time,
				model: current.model,
				content: `I'll stay in ${current.cwd} and keep the change local to ${repo}.`
			});
		}

		for (const file of current.files) {
			synthesized.push({
				id: `${current.id}-${file.path}`,
				role: 'assistant',
				kind: 'tool_call',
				timestamp: current.time,
				tool: file.action,
				path: file.path,
				durationMs: file.action === 'read' ? 36 : 180,
				content:
					file.action === 'read'
						? `Opened ${file.path}.`
						: `Updated ${file.path}${file.additions != null ? ` (+${file.additions}/−${file.deletions ?? 0})` : ''}.`,
				diff:
					file.additions != null
						? `${file.path}  +${file.additions} / −${file.deletions ?? 0}`
						: undefined
			});
		}

		for (const child of current.children) {
			synthesized.push({
				id: `${current.id}-${child.id}`,
				role: 'assistant',
				kind: 'delegation',
				timestamp: current.time,
				content: child.title,
				child
			});
		}

		synthesized.push({
			id: `${current.id}-done`,
			role: 'assistant',
			kind: 'message',
			timestamp: current.time,
			model: current.model,
			content: current.commit
				? `Landed the work against ${current.commit}. ${current.preview}`
				: current.preview
		});

		return synthesized;
	}

	function formatMs(ms?: number) {
		if (ms == null) return '';
		if (ms < 1000) return `${ms}ms`;
		return `${(ms / 1000).toFixed(ms >= 10000 ? 0 : 1)}s`;
	}

	function treeKind(event: TraceEvent) {
		if (event.kind === 'delegation') return 'agent';
		if (event.kind === 'tool_call') return 'tool';
		if (event.role === 'user') return 'user';
		return 'assistant';
	}

	function treeLabel(event: TraceEvent) {
		if (event.kind === 'delegation') return event.child?.title ?? event.content;
		if (event.kind === 'tool_call') return event.path ?? event.tool ?? event.content;
		return event.content;
	}

	function toolBody(event: TraceEvent) {
		return event.diff ?? event.output ?? event.content;
	}

	function openChild(event: TraceEvent) {
		if (!event.child) return;
		selectedChild = event.child;
		invokingEventId = event.id;
		if (mobile.current) sheetOpen = true;
	}

	function closePane() {
		const returnTo = invokingEventId;
		selectedChild = null;
		sheetOpen = false;
		invokingEventId = null;
		if (returnTo) {
			document.getElementById(`event-${returnTo}`)?.scrollIntoView({ block: 'nearest' });
		}
	}

	function focusEvent(event: TraceEvent) {
		if (event.kind === 'delegation') {
			openChild(event);
		}
		document.getElementById(`event-${event.id}`)?.scrollIntoView({ block: 'nearest' });
	}

	function onSheetOpenChange(open: boolean) {
		sheetOpen = open;
		if (!open) {
			selectedChild = null;
			invokingEventId = null;
		}
	}

	function onKeydown(event: KeyboardEvent) {
		if (event.key !== 'Escape' || event.defaultPrevented) return;
		if (selectedChild) {
			event.preventDefault();
			closePane();
		}
	}
</script>

<svelte:head>
	<title>{session ? `${session.title} · traicr` : 'Not found · traicr'}</title>
</svelte:head>

<svelte:window onkeydown={onKeydown} />

{#snippet statusMark(status: ChildStatus)}
	{#if status === 'unread'}
		<span class="bg-foreground size-1.5 rounded-full" aria-hidden="true"></span>
	{:else if status === 'running'}
		<span class="border-foreground size-1.5 rounded-full border" aria-hidden="true"></span>
	{:else}
		<CheckLine class="size-3" />
	{/if}
	<span class="sr-only">{status}</span>
{/snippet}

{#snippet userEvent(event: TraceEvent)}
	<div id="event-{event.id}" class="px-4 py-2">
		<div class="bg-sidebar/90 rounded-md px-3 py-2 text-[13px] leading-relaxed">
			{event.content}
		</div>
	</div>
{/snippet}

{#snippet assistantEvent(event: TraceEvent)}
	<div
		id="event-{event.id}"
		class={['px-4 py-2 text-[13px] leading-7', event.kind === 'reasoning' && 'text-muted-foreground italic']}
	>
		<p>{event.content}</p>
	</div>
{/snippet}

{#snippet toolEvent(event: TraceEvent)}
	<div id="event-{event.id}" class="px-3 py-0.5">
		<Collapsible.Root>
			<Collapsible.Trigger
				class="text-muted-foreground hover:text-foreground flex h-8 w-full items-center gap-2 rounded-md px-1 text-left text-xs"
			>
				<span class="font-mono">{event.tool ?? 'tool'}</span>
				<span class="min-w-0 truncate font-mono">{event.path ?? event.args ?? ''}</span>
				{#if event.durationMs != null}
					<span class="ml-auto shrink-0 tabular-nums">{formatMs(event.durationMs)}</span>
				{/if}
			</Collapsible.Trigger>
			<Collapsible.Content>
				<pre
					class="bg-muted/50 text-muted-foreground mb-1 overflow-x-auto rounded-md px-2 py-1.5 font-mono text-[11px] leading-5 whitespace-pre-wrap">{toolBody(event)}</pre>
			</Collapsible.Content>
		</Collapsible.Root>
	</div>
{/snippet}

{#snippet delegationEvent(event: TraceEvent)}
	{#if event.child}
		<div id="event-{event.id}" class="px-3 py-0.5">
			<button
				type="button"
				class={[
					'flex h-9 w-full items-center gap-2 rounded-md px-2 text-left text-[13px]',
					'hover:bg-sidebar-accent/70',
					selectedChild?.id === event.child.id && 'bg-sidebar-accent'
				]}
				onclick={() => openChild(event)}
			>
				<span class="min-w-0 truncate">↳ {event.child.title}</span>
				<span class="text-muted-foreground ml-auto flex shrink-0 items-center gap-1.5 text-xs">
					{@render statusMark(event.child.status)}
					<span>{event.child.harness}</span>
					<span>·</span>
					<span>{event.child.duration}</span>
				</span>
				<ArrowRightSLine class="text-muted-foreground size-3.5 shrink-0" />
			</button>
		</div>
	{/if}
{/snippet}

{#snippet stream(items: TraceEvent[])}
	{#each items.filter((event) => event.kind !== 'tool_result') as event (event.id)}
		{#if event.kind === 'tool_call'}
			{@render toolEvent(event)}
		{:else if event.kind === 'delegation'}
			{@render delegationEvent(event)}
		{:else if event.role === 'user'}
			{@render userEvent(event)}
		{:else}
			{@render assistantEvent(event)}
		{/if}
	{/each}
{/snippet}

{#snippet pane(child: ChildTrace)}
	<div class="flex h-full min-h-0 flex-col">
		<div class="flex items-start justify-between gap-3 px-4 pt-3 pb-2">
			<div class="min-w-0">
				<p class="text-muted-foreground text-[10px] font-medium tracking-[0.14em]">DELEGATED WORK</p>
				<h2 class="mt-1 truncate text-sm font-medium">{child.title}</h2>
				<p class="text-muted-foreground mt-1 flex items-center gap-1.5 text-xs">
					{@render statusMark(child.status)}
					<span>{child.harness}</span>
					<span>·</span>
					<span>{child.duration}</span>
					<span>·</span>
					<span>{child.model}</span>
				</p>
			</div>
			<Button variant="ghost" size="icon-sm" onclick={closePane} aria-label="Close delegated work">
				<CloseLine class="size-4" />
			</Button>
		</div>
		<Separator />
		<ScrollArea class="min-h-0 flex-1">
			{#if childEvents.length > 0}
				<div class="py-2">
					{@render stream(childEvents)}
				</div>
			{:else}
				<p class="text-muted-foreground px-4 py-6 text-[13px] leading-relaxed">{child.preview}</p>
			{/if}
		</ScrollArea>
		<Separator />
		<p class="text-muted-foreground px-4 py-3 text-xs">Close returns to the invoking row</p>
	</div>
{/snippet}

<InboxShell compact>
	{#if session}
		<div class="flex min-h-0 min-w-0 flex-1">
			<aside class="border-border/60 hidden w-[200px] shrink-0 flex-col border-r lg:flex">
				<p class="text-muted-foreground px-3 pt-3 pb-1 font-mono text-[10px] tracking-wide uppercase">
					Session
				</p>
				<ScrollArea class="min-h-0 flex-1">
					<nav class="px-1 pb-4" aria-label="Session tree">
						{#each treeEvents as event (event.id)}
							<button
								type="button"
								class={[
									'text-muted-foreground hover:text-foreground flex w-full items-center gap-2 rounded-sm px-2 py-1 text-left font-mono text-[12px]',
									(invokingEventId === event.id ||
										(event.child && selectedChild?.id === event.child.id)) &&
										'bg-sidebar-accent text-sidebar-accent-foreground'
								]}
								onclick={() => focusEvent(event)}
							>
								{#if treeKind(event) === 'agent'}
									<span class="w-10 shrink-0">◆</span>
								{:else}
									<span class="w-10 shrink-0">{treeKind(event)}</span>
								{/if}
								<span class="min-w-0 truncate">{treeLabel(event)}</span>
							</button>
						{/each}
					</nav>
				</ScrollArea>
			</aside>

			<div class="flex min-h-0 min-w-0 flex-1 flex-col">
				<header class="shrink-0 px-4 py-3">
					<div class="flex items-center gap-2">
						<h1 class="truncate text-sm font-medium">{session.title}</h1>
						<Badge variant="outline" class="h-5 px-1.5 text-[11px] font-normal">
							{session.harness}
						</Badge>
					</div>
					<p class="text-muted-foreground mt-1 truncate text-xs">
						{session.model}
						<span class="px-1">·</span>
						{session.repository}
						<span class="px-1">·</span>
						{session.machine}
						{#if session.commit}
							<span class="px-1">·</span>
							<span class="font-mono">{session.commit}</span>
						{/if}
					</p>
					{#if session.warnings.length > 0}
						<p class="text-destructive/80 mt-1 text-xs">{session.warnings.join(' · ')}</p>
					{/if}
				</header>
				<Separator />
				<ScrollArea class="min-h-0 flex-1">
					<div class="mx-auto max-w-3xl py-3">
						{@render stream(events)}
					</div>
				</ScrollArea>
			</div>

			{#if paneOpenDesktop && selectedChild}
				<aside class="border-border/60 flex w-[360px] shrink-0 flex-col border-l">
					{@render pane(selectedChild)}
				</aside>
			{/if}
		</div>

		<Sheet.Root bind:open={sheetOpen} onOpenChange={onSheetOpenChange}>
			<Sheet.Content class="w-full p-0 sm:max-w-[360px]" showCloseButton={false}>
				<Sheet.Header class="sr-only">
					<Sheet.Title>{selectedChild?.title ?? 'Delegated work'}</Sheet.Title>
					<Sheet.Description>Child transcript</Sheet.Description>
				</Sheet.Header>
				{#if selectedChild}
					{@render pane(selectedChild)}
				{/if}
			</Sheet.Content>
		</Sheet.Root>
	{:else}
		<div class="px-6 py-10">
			<h1 class="text-sm font-medium">Session not found</h1>
			<p class="text-muted-foreground mt-1 text-sm">This trace is not in the archive.</p>
			<Button href={resolve('/inbox')} variant="ghost" class="mt-4 px-0">Back to Transcripts</Button>
		</div>
	{/if}
</InboxShell>
