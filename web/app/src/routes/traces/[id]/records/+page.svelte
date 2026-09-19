<script lang="ts">
	import { page } from '$app/state';
	import { resolve } from '$app/paths';
	import { Badge } from '$lib/components/ui/badge';
	import { Button } from '$lib/components/ui/button';
	import { Card, CardContent, CardHeader, CardTitle } from '$lib/components/ui/card';
	import { Input } from '$lib/components/ui/input';
	import { recordsHref, revisionSourcesHref, transcriptHref } from '$lib/links';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();
	let trace = $derived(data.trace);
	let cursor = $derived(page.url.searchParams.get('cursor') ?? '');
</script>

<svelte:head><title>{trace?.title || 'Trace'} · Traicr</title></svelte:head>

{#if data.error || !trace}
	<p class="text-destructive" role="alert">{data.error || 'Trace not found'}</p>
{:else}
	<Button class="mb-4" variant="ghost" href={transcriptHref(trace.id)}>Back to transcript</Button>
	<div class="mb-8 flex flex-wrap items-end justify-between gap-4">
		<div>
			<p class="text-primary mb-2 font-mono text-[11px] tracking-widest">{trace.harness} / TRACE</p>
			<h1 class="text-3xl font-semibold tracking-tight">{trace.title || 'Untitled trace'}</h1>
			<p class="text-muted-foreground mt-2 font-mono text-xs break-all">{trace.native_trace_id}</p>
		</div>
		<Button variant="outline" href="#retained">Retained revisions</Button>
	</div>
	<div class="grid items-start gap-6 lg:grid-cols-[minmax(0,1fr)_20rem]">
		<section class="flex flex-col gap-4" aria-label="Trace events">
			{#each data.events as event (event.id)}
				<Card id="event-{event.id}">
					<CardContent class="flex flex-col gap-3">
						<div class="text-muted-foreground flex flex-wrap items-center gap-2 font-mono text-[11px]">
							{#if event.role}<Badge variant="secondary">{event.role}</Badge>{/if}
							<strong class="text-foreground">{event.kind}</strong>
							{#if event.model}<span>{event.model}</span>{/if}
							{#if event.timestamp}<time>{event.timestamp}</time>{/if}
						</div>
						<div id="key-{event.key}" class="text-muted-foreground flex flex-wrap gap-3 text-xs">
							{#if event.parent_key}<a href="{recordsHref(trace.id, { key: event.parent_key })}#key-{event.parent_key}">Parent: {event.parent_key}</a>{/if}
							{#if event.branch}<span>Branch: {event.branch}</span>{/if}
							{#if event.call_id}<span>Call: {event.call_id}</span>{/if}
							{#if event.tool}<span>Tool: {event.tool}</span>{/if}
						</div>
						<pre class="bg-muted overflow-x-auto rounded-lg p-3 font-mono text-xs whitespace-pre-wrap">{event.text}</pre>
						<a class="text-sm" href={resolve('/events/[id]/sources', { id: String(event.id) })}>Inspect Source Records</a>
					</CardContent>
				</Card>
			{:else}
				<p class="text-muted-foreground rounded-xl border border-dashed px-6 py-8">No normalized events. The native source files are still retained.</p>
			{/each}
			{#if data.next}
				<Button variant="outline" href={recordsHref(trace.id, { cursor: data.next })}>Next events</Button>
			{/if}
		</section>
		<Card class="lg:sticky lg:top-6">
			<CardHeader><CardTitle>Trace details</CardTitle></CardHeader>
			<CardContent class="flex flex-col gap-4 text-sm">
				<dl class="grid grid-cols-[7rem_1fr] gap-2">
					<dt class="text-muted-foreground">Harness</dt><dd>{trace.harness}</dd>
					<dt class="text-muted-foreground">Directory</dt><dd class="break-all">{trace.working_directory}</dd>
					<dt class="text-muted-foreground">Repository</dt><dd class="break-all">{trace.repository}</dd>
				</dl>
				<div>
					<h2 class="mb-2 font-medium">Related traces</h2>
					{#each trace.parents ?? [] as parent (parent.id)}
						<a class="block" href={transcriptHref(parent.id)}>Parent: {parent.title || parent.native_trace_id}</a>
					{/each}
					{#each trace.children ?? [] as child (child.id)}
						<a class="block" href={transcriptHref(child.id)}>Child: {child.title || child.native_trace_id}</a>
					{:else}
						<p class="text-muted-foreground">No child traces collected.</p>
					{/each}
				</div>
				<div id="retained">
					<h2 class="mb-2 font-medium">Retained revisions</h2>
					<div class="flex flex-col gap-2">
						{#each trace.revisions ?? [] as revision (revision.id)}
							<a class="hover:bg-muted rounded-lg border p-3 no-underline" href={revisionSourcesHref(revision.id)}>
								<strong class="text-foreground block">Revision {revision.id}</strong>
								<span class="text-muted-foreground">{revision.status}</span>
								<small class="mt-1 block font-mono text-xs break-all">{revision.digest}</small>
							</a>
							{#each revision.diagnostics ?? [] as diagnostic (`${diagnostic.code}:${diagnostic.message}`)}
								<p class="bg-muted rounded-lg border px-3 py-2 text-sm"><strong>{diagnostic.code}</strong>: {diagnostic.message}</p>
							{/each}
						{/each}
					</div>
				</div>
				<details class="border-destructive/30 rounded-lg border p-3">
					<summary class="text-destructive cursor-pointer">Permanently delete trace</summary>
					<form class="mt-3 flex flex-col gap-3" action="/traces/{trace.id}/delete" method="post" data-sveltekit-reload>
						<input type="hidden" name="csrf" value={data.csrf} />
						<label class="text-sm font-medium" for="confirm">Type <code>{trace.native_trace_id}</code> to confirm</label>
						<Input id="confirm" name="confirm" required autocomplete="off" />
						<Button type="submit" variant="destructive">Delete permanently</Button>
					</form>
				</details>
				{#if cursor}<p class="text-muted-foreground text-xs">Showing a later page of events.</p>{/if}
			</CardContent>
		</Card>
	</div>
{/if}
