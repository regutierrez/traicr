<script lang="ts">
	import { page } from '$app/state';
	import { resolve } from '$app/paths';
	import { Badge } from '$lib/components/ui/badge';
	import { Button } from '$lib/components/ui/button';
	import { Card, CardContent, CardHeader, CardTitle } from '$lib/components/ui/card';
	import { Input } from '$lib/components/ui/input';
	import { submitGoForm } from '$lib/forms';
	import WorkspaceHeader from '$lib/components/conversation/WorkspaceHeader.svelte';
	import { recordsHref, revisionSourcesHref, sourceFileHref, transcriptHref } from '$lib/links';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();
	let trace = $derived(data.trace);
	let cursor = $derived(page.url.searchParams.get('cursor') ?? '');
	let deleteError = $state('');
	let deleting = $state(false);

	async function removeTrace(event: SubmitEvent) {
		event.preventDefault();
		const form = event.currentTarget;
		if (!(form instanceof HTMLFormElement)) return;
		deleting = true;
		deleteError = '';
		try {
			const result = await submitGoForm(form);
			if (result.ok) {
				location.assign(result.url);
				return;
			}
			deleteError = result.message;
		} catch {
			deleteError = 'Could not reach the server';
		} finally {
			deleting = false;
		}
	}
</script>

<svelte:head><title>{trace?.title || 'Trace'} · Traicr</title></svelte:head>

{#if data.error || !trace}
	<p class="text-destructive p-6" role="alert">{data.error || 'Trace not found'}</p>
{:else}
	<WorkspaceHeader {trace} tab="details" />
	<div class="details-grid mx-auto grid w-full max-w-[1200px] items-start gap-6 px-6 py-5 lg:grid-cols-[minmax(0,1fr)_20rem]">
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
				{#if trace.harness === 'amp'}
					<div>
						<h2 class="mb-2 font-medium">Native Amp export</h2>
						<p class="text-muted-foreground mb-2 text-xs">Lossless source JSON for an individual revision, not the merged viewer data.</p>
						{#each trace.revisions ?? [] as revision (revision.id)}
							<p class="mb-1 text-sm">
								<a href={sourceFileHref(revision.id, 'source/export.json', { download: true })} data-sveltekit-reload>
									Revision {revision.id} · {revision.native_updated_at} · Native JSON
								</a>
							</p>
						{/each}
					</div>
				{/if}
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
					<form class="mt-3 flex flex-col gap-3" action="/traces/{trace.id}/delete" method="post" onsubmit={removeTrace} aria-busy={deleting}>
						<input type="hidden" name="csrf" value={data.csrf} />
						<label class="text-sm font-medium" for="confirm">Type <code>{trace.native_trace_id}</code> to confirm</label>
						<Input id="confirm" name="confirm" required autocomplete="off" aria-invalid={deleteError ? true : undefined} aria-describedby={deleteError ? 'delete-error' : undefined} />
						{#if deleteError}
							<p id="delete-error" class="text-destructive text-sm" role="alert">{deleteError}</p>
						{/if}
						<Button type="submit" variant="destructive" disabled={deleting}>{deleting ? 'Deleting…' : 'Delete permanently'}</Button>
					</form>
				</details>
				{#if cursor}<p class="text-muted-foreground text-xs">Showing a later page of events.</p>{/if}
			</CardContent>
		</Card>
	</div>
{/if}
