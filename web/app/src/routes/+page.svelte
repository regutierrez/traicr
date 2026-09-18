<script lang="ts">
	import { page } from '$app/state';
	import { resolve } from '$app/paths';
	import { Badge } from '$lib/components/ui/badge';
	import { Button } from '$lib/components/ui/button';
	import { Card, CardContent } from '$lib/components/ui/card';
	import { Input } from '$lib/components/ui/input';
	import { highlightParts } from '$lib/highlight';
	import { harnesses, type Card as TranscriptCard } from '$lib/types';

	let { data } = $props();
	let query = $derived(page.url.searchParams.get('q') ?? '');
	let mode = $derived(page.url.searchParams.get('mode') ?? 'fulltext');

	function field(name: string) {
		return page.url.searchParams.get(name) ?? '';
	}

	function nextHref(cursor: string) {
		const params = new URLSearchParams(page.url.searchParams);
		params.set('cursor', cursor);
		return resolve(`/?${params.toString()}`);
	}
</script>

<svelte:head>
	<title>Your sessions. · Traicr</title>
</svelte:head>

<section class="mb-8 flex flex-wrap items-end justify-between gap-4">
	<div>
		<p class="text-primary mb-2 font-mono text-[11px] tracking-widest">THE TRACE ARCHIVE</p>
		<h1 class="text-3xl font-semibold tracking-tight">Your sessions.</h1>
		<p class="text-muted-foreground mt-2 max-w-2xl">Whole conversations, from the first prompt to the final result.</p>
	</div>
</section>

<Card class="mb-8">
	<CardContent>
		<form class="grid gap-4" method="GET" action={resolve('/')}>
			<div class="grid items-end gap-3 md:grid-cols-[minmax(0,1fr)_12rem_auto]">
				<label class="sr-only" for="query">Search your traces</label>
				<Input id="query" name="q" type="search" value={query} placeholder="A thought, a function, a file path…" autocomplete="off" />
				<select class="border-input bg-background h-8 w-full rounded-lg border px-2.5 text-sm" name="mode" aria-label="Search mode">
					<option value="fulltext" selected={mode === 'fulltext'}>Full-text</option>
					<option value="exact" selected={mode === 'exact'}>Exact text</option>
					<option value="regex" selected={mode === 'regex'}>Regular expression</option>
				</select>
				<Button type="submit" size="lg" class="w-full md:w-auto">Search</Button>
			</div>
			<details class="border-t pt-3">
				<summary class="cursor-pointer text-sm font-medium">Narrow your search</summary>
				<div class="grid gap-4 pt-4 sm:grid-cols-2 lg:grid-cols-3">
					<label class="text-sm font-medium">Harness
						<select class="border-input bg-background mt-1.5 h-8 w-full rounded-lg border px-2.5 text-sm font-normal" name="harness">
							<option value="">All harnesses</option>
							{#each harnesses as harness (harness)}
								<option value={harness} selected={field('harness') === harness}>{harness}</option>
							{/each}
						</select>
					</label>
					<label class="text-sm font-medium">Model
						<Input class="mt-1.5 font-normal" name="model" value={field('model')} placeholder="Model ID" />
					</label>
					<label class="text-sm font-medium">Machine
						<Input class="mt-1.5 font-normal" name="machine" value={field('machine')} placeholder="Machine ID" />
					</label>
					<label class="text-sm font-medium">Repository
						<Input class="mt-1.5 font-normal" name="repository" value={field('repository')} placeholder="Remote URL or local identity" />
					</label>
					<label class="text-sm font-medium">From
						<Input class="mt-1.5 font-normal" type="date" name="after" value={field('after')} />
					</label>
					<label class="text-sm font-medium">Before
						<Input class="mt-1.5 font-normal" type="date" name="before" value={field('before')} />
					</label>
					<label class="text-sm font-medium">Role
						<Input class="mt-1.5 font-normal" name="role" value={field('role')} placeholder="user, assistant, tool…" />
					</label>
					<label class="text-sm font-medium">Event kind
						<Input class="mt-1.5 font-normal" name="kind" value={field('kind')} placeholder="message, tool_call…" />
					</label>
					<label class="text-sm font-medium">Tool
						<Input class="mt-1.5 font-normal" name="tool" value={field('tool')} placeholder="Tool name" />
					</label>
				</div>
			</details>
		</form>
	</CardContent>
</Card>

<section aria-live="polite">
	<div class="mb-4 flex items-center justify-between gap-3">
		<h2 class="text-sm font-medium">{query ? 'Matching transcripts' : 'Recent transcripts'}</h2>
		<span class="text-muted-foreground font-mono text-[11px]">{data.cards.length} sessions on this page</span>
	</div>
	{#if data.error}
		<p class="border-destructive/40 bg-destructive/10 text-destructive mb-4 rounded-lg border px-3 py-2 text-sm" role="alert">{data.error}</p>
	{/if}
	<div class="grid gap-4 md:grid-cols-2">
		{#each data.cards as card (card.id)}
			{@render session(card)}
		{:else}
			{#if !data.error}
				<div class="text-muted-foreground rounded-xl border border-dashed px-6 py-10 text-center md:col-span-2">
					<h3 class="text-foreground text-base font-medium">{query ? 'No matching transcripts' : 'Your archive starts here'}</h3>
					<p class="mx-auto mt-2 max-w-lg">
						{query ? 'Try another search mode, remove a filter, or use a shorter phrase.' : 'Collect traces on a source machine, then upload the ZIPs. Your original records stay intact.'}
					</p>
				</div>
			{/if}
		{/each}
	</div>
	{#if data.next}
		<Button class="mt-6" variant="outline" href={nextHref(data.next)}>Next results</Button>
	{/if}
</section>

{#snippet session(card: TranscriptCard)}
	<Card class="relative">
		<CardContent class="flex flex-col gap-3">
			<div class="text-muted-foreground flex flex-wrap items-center gap-2 font-mono text-[11px]">
				<Badge>{card.harness}</Badge>
				<time datetime={card.updated_at}>{card.updated_at}</time>
			</div>
			<h3 class="text-base font-medium">
				<a class="text-foreground after:absolute after:inset-0" href={resolve('/traces/[id]', { id: String(card.id) })} data-sveltekit-reload>
					{card.title || card.native_trace_id}
				</a>
			</h3>
			<p class="text-muted-foreground line-clamp-4 whitespace-pre-wrap">
				{#each highlightParts(card.snippet || 'Open the transcript to explore this session.', query, mode) as part, index (index)}
					{#if part.mark}<mark>{part.text}</mark>{:else}{part.text}{/if}
				{/each}
			</p>
			<p class="text-muted-foreground text-xs break-all">{card.repository || 'No repository recorded'}</p>
			<p class="text-muted-foreground border-t pt-3 text-xs">{card.event_count} records · {card.revision_count} revisions</p>
		</CardContent>
	</Card>
{/snippet}
