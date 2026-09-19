<script lang="ts">
	import { resolve } from '$app/paths';
	import { Badge } from '$lib/components/ui/badge';
	import { Button } from '$lib/components/ui/button';
	import { Card, CardContent, CardHeader, CardTitle } from '$lib/components/ui/card';
	import Turn from '$lib/components/turn.svelte';
	import {
		childrenOf,
		entryLabel,
		graphHasFork,
		pathToEntry,
		rootsOf,
		turnsFromEntries,
		type SessionEntry,
		type TranscriptSession
	} from '$lib/transcript-data';
	import type { Trace } from '$lib/types';

	let {
		trace,
		session,
		revision,
		eventCount,
		loadError,
		reload
	}: {
		trace: Trace;
		session: TranscriptSession | null;
		revision: string;
		eventCount: number;
		loadError: string;
		reload: boolean;
	} = $props();

	let chosenId = $state<string | undefined>();
	let showThinking = $state(false);
	let showTools = $state(false);

	let title = $derived(session?.header.title || trace.title || trace.native_trace_id || 'Trace');
	let entries = $derived(session?.entries ?? []);
	let forked = $derived(graphHasFork(entries));
	let ids = $derived(new Set(entries.map((entry) => entry.id)));
	let activeId = $derived(chosenId && ids.has(chosenId) ? chosenId : (session?.leafId ?? ''));
	let visible = $derived(forked ? pathToEntry(entries, activeId) : entries);
	let turns = $derived(turnsFromEntries(visible));
	let roots = $derived(rootsOf(entries));

	function onKeydown(event: KeyboardEvent) {
		if (event.metaKey || event.ctrlKey || event.altKey) return;
		const target = event.target;
		if (target instanceof HTMLElement && target.closest('input, textarea, select, [contenteditable=true]')) return;
		if (event.key === 't' || event.key === 'T') {
			showThinking = !showThinking;
			event.preventDefault();
		}
		if (event.key === 'o' || event.key === 'O') {
			showTools = !showTools;
			event.preventDefault();
		}
	}

	function select(id: string) {
		chosenId = id;
	}

	function nodeClass(id: string) {
		return [
			'hover:bg-muted w-full rounded-md px-2 py-1 text-left text-sm',
			id === activeId ? 'bg-muted text-foreground' : 'text-muted-foreground'
		];
	}
</script>

<svelte:window onkeydown={onKeydown} />

{#snippet tree(nodes: SessionEntry[])}
	<ul class="flex flex-col gap-1">
		{#each nodes as node (node.id)}
			<li>
				<button type="button" class={nodeClass(node.id)} onclick={() => select(node.id)}>{entryLabel(node)}</button>
				{#if childrenOf(entries, node.id).length}
					<div class="border-border mt-1 ml-3 border-l pl-2">
						{@render tree(childrenOf(entries, node.id))}
					</div>
				{/if}
			</li>
		{/each}
	</ul>
{/snippet}

<header class="bg-background/95 sticky top-0 z-10 -mx-4 mb-6 flex flex-col gap-3 border-b px-4 py-4 backdrop-blur md:-mx-8 md:px-8">
	<div class="flex flex-wrap items-end justify-between gap-3">
		<div class="min-w-0">
			<div class="mb-2 flex flex-wrap items-center gap-2">
				<Badge>{trace.harness}</Badge>
				<span class="text-muted-foreground font-mono text-[11px]">{eventCount} records · {turns.length} turns</span>
			</div>
			<h1 class="text-2xl font-semibold tracking-tight">{title}</h1>
		</div>
		<div class="flex flex-wrap items-center gap-2">
			<p class="text-muted-foreground font-mono text-[11px]" title="Toggle thinking and tools">
				T thinking {showThinking ? 'on' : 'off'} · O tools {showTools ? 'on' : 'off'}
			</p>
			<Button variant="outline" href={resolve('/traces/[id]/records', { id: String(trace.id) })}>Session details</Button>
		</div>
	</div>
	{#if trace.harness === 'amp'}
		<form class="flex flex-wrap items-end gap-3" method="get">
			<label class="text-sm font-medium" for="revision">
				Archive view
				<select class="border-input bg-background mt-1.5 h-8 w-full min-w-56 rounded-lg border px-2.5 text-sm font-normal" name="revision" id="revision">
					<option value="" selected={revision === ''}>Merged archive</option>
					{#each trace.revisions ?? [] as item (item.id)}
						<option value={item.id} selected={revision === String(item.id)}>Revision {item.id} · {item.native_updated_at}</option>
					{/each}
				</select>
			</label>
			<Button type="submit">Inspect</Button>
		</form>
		<details>
			<summary class="cursor-pointer text-sm font-medium">Download native Amp export</summary>
			<p class="text-muted-foreground mt-2 text-sm">Lossless source JSON for an individual revision, not the merged viewer data.</p>
			{#each trace.revisions ?? [] as item (item.id)}
				<p class="mt-2 text-sm">
					<a href="{resolve('/revisions/[id]/file', { id: String(item.id) })}?path=source/export.json&download=1" data-sveltekit-reload>Revision {item.id} · {item.native_updated_at} · Native JSON</a>
				</p>
			{/each}
		</details>
	{/if}
</header>

{#if loadError}
	<p class="border-destructive/40 bg-destructive/10 text-destructive mb-6 rounded-lg border px-3 py-2 text-sm" role="alert">{loadError}</p>
	{#if reload}
		<Button class="mb-6" onclick={() => location.reload()}>Reload</Button>
	{/if}
{/if}

{#if !session || !entries.length}
	<p class="text-muted-foreground rounded-xl border border-dashed px-6 py-10 text-center">
		No normalized messages. Open <a href={resolve('/traces/[id]/records', { id: String(trace.id) })}>Session details</a> to inspect the retained source files.
	</p>
{:else}
	<div class={forked ? 'grid items-start gap-6 lg:grid-cols-[18rem_minmax(0,1fr)]' : 'flex flex-col gap-6'}>
		{#if forked}
			<nav class="bg-card ring-foreground/10 sticky top-28 rounded-xl p-4 ring-1" aria-label="Conversation branches">
				<p class="mb-3 text-sm font-medium">Branches</p>
				{@render tree(roots)}
			</nav>
		{/if}
		<div class="flex min-w-0 flex-col gap-6">
			{#each turns as turn (turn.id)}
				<Turn {turn} {showThinking} {showTools} />
			{/each}
		</div>
	</div>
{/if}

{#if (trace.children ?? []).length}
	<section class="mt-10" aria-label="Child traces">
		<h2 class="mb-4 text-sm font-medium">Child traces</h2>
		<div class="grid gap-4 md:grid-cols-2">
			{#each trace.children ?? [] as child (child.id)}
				<Card class="relative">
					<CardHeader>
						<CardTitle>
							<a class="text-foreground after:absolute after:inset-0" href={resolve('/traces/[id]', { id: String(child.id) })}>{child.title || child.native_trace_id}</a>
						</CardTitle>
					</CardHeader>
					<CardContent>
						<p class="text-muted-foreground font-mono text-xs break-all">{child.native_trace_id}</p>
					</CardContent>
				</Card>
			{/each}
		</div>
	</section>
{/if}
