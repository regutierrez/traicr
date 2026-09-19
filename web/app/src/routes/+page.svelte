<script lang="ts">
	import { page } from '$app/state';
	import { resolve } from '$app/paths';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { highlightParts } from '$lib/highlight';
	import { withQuery } from '$lib/links';
	import { harnesses, type TranscriptCard } from '$lib/types';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();
	let query = $derived(page.url.searchParams.get('q') ?? '');
	let mode = $derived(page.url.searchParams.get('mode') ?? 'fulltext');

	function field(name: string) {
		return page.url.searchParams.get(name) ?? '';
	}

	function nextHref(cursor: string) {
		const params = new URLSearchParams(page.url.searchParams);
		params.set('cursor', cursor);
		return withQuery(resolve('/'), params);
	}
</script>

<svelte:head>
	<title>Sessions · Traicr</title>
</svelte:head>

<header class="page-head">
	<div>
		<p class="meta">Sessions</p>
		<h1>Your archive</h1>
	</div>
	<p class="count">{data.cards.length} on this page</p>
</header>

<form class="filters" method="GET" action={resolve('/')}>
	<label class="sr-only" for="query">Search your traces</label>
	<Input id="query" name="q" type="search" value={query} placeholder="Search sessions" autocomplete="off" />
	<select class="mode" name="mode" aria-label="Search mode">
		<option value="fulltext" selected={mode === 'fulltext'}>Full-text</option>
		<option value="exact" selected={mode === 'exact'}>Exact text</option>
		<option value="regex" selected={mode === 'regex'}>Regular expression</option>
	</select>
	<Button type="submit">Search</Button>
	<details class="more">
		<summary>Filters</summary>
		<div class="more-grid">
			<label>Harness
				<select name="harness">
					<option value="">All harnesses</option>
					{#each harnesses as harness (harness)}
						<option value={harness} selected={field('harness') === harness}>{harness}</option>
					{/each}
				</select>
			</label>
			<label>Model
				<Input name="model" value={field('model')} placeholder="Model ID" />
			</label>
			<label>Machine
				<Input name="machine" value={field('machine')} placeholder="Machine ID" />
			</label>
			<label>Repository
				<Input name="repository" value={field('repository')} placeholder="Remote URL or local identity" />
			</label>
			<label>From
				<Input type="date" name="after" value={field('after')} />
			</label>
			<label>Before
				<Input type="date" name="before" value={field('before')} />
			</label>
			<label>Role
				<Input name="role" value={field('role')} placeholder="user, assistant, tool…" />
			</label>
			<label>Event kind
				<Input name="kind" value={field('kind')} placeholder="message, tool_call…" />
			</label>
			<label>Tool
				<Input name="tool" value={field('tool')} placeholder="Tool name" />
			</label>
		</div>
	</details>
</form>

<section aria-live="polite">
	{#if data.error}
		<p class="error" role="alert">{data.error}</p>
	{/if}
	<ul class="sessions">
		{#each data.cards as card (card.id)}
			{@render session(card)}
		{:else}
			{#if !data.error}
				<li class="empty">
					<p class="empty-title">{query ? 'No matching sessions' : 'Your archive starts here'}</p>
					<p>
						{query ? 'Try another search mode, remove a filter, or use a shorter phrase.' : 'Collect traces on a source machine, then upload the ZIPs. Your original records stay intact.'}
					</p>
				</li>
			{/if}
		{/each}
	</ul>
	{#if data.next}
		<Button class="mt-4" variant="outline" href={nextHref(data.next)}>Next results</Button>
	{/if}
</section>

{#snippet session(card: TranscriptCard)}
	<li class="session">
		<a class="session-link" href={resolve('/traces/[id]', { id: String(card.id) })}>
			<span class="session-title">{card.title || card.native_trace_id}</span>
			<span class="session-meta">
				<span>{card.harness}</span>
				<time datetime={card.updated_at}>{card.updated_at}</time>
				<span>{card.event_count} records</span>
				<span>{card.revision_count} revisions</span>
			</span>
			<span class="session-repo">{card.repository || 'No repository recorded'}</span>
			{#if card.snippet || query}
				<span class="session-snippet">
					{#each highlightParts(card.snippet || 'Open the transcript to explore this session.', query, mode) as part, index (`${index}:${part.mark}:${part.text}`)}
						{#if part.mark}<mark>{part.text}</mark>{:else}{part.text}{/if}
					{/each}
				</span>
			{/if}
		</a>
	</li>
{/snippet}

<style>
	.page-head {
		display: flex;
		align-items: baseline;
		justify-content: space-between;
		gap: 1rem;
		margin-bottom: 0.85rem;
	}

	.meta,
	.count {
		margin: 0;
		color: var(--muted-foreground);
		font-family: var(--font-mono);
		font-size: 11px;
		font-variant-numeric: tabular-nums;
	}

	h1 {
		margin: 0.15rem 0 0;
		font-size: 1.05rem;
		font-weight: 600;
	}

	.filters {
		display: grid;
		grid-template-columns: minmax(0, 1fr) auto auto;
		gap: 0.45rem;
		align-items: center;
		margin-bottom: 0.75rem;
	}

	.mode,
	.more-grid select {
		height: 2rem;
		border: 1px solid var(--input);
		border-radius: var(--radius);
		background: var(--background);
		color: var(--foreground);
		padding: 0 0.55rem;
		font: inherit;
	}

	.more {
		grid-column: 1 / -1;
		border-top: 1px solid var(--border);
		padding-top: 0.45rem;
	}

	.more summary {
		cursor: pointer;
		color: var(--muted-foreground);
		font-size: 12px;
	}

	.more-grid {
		display: grid;
		gap: 0.65rem;
		grid-template-columns: repeat(auto-fill, minmax(12rem, 1fr));
		padding-top: 0.65rem;
	}

	.more-grid label {
		display: flex;
		flex-direction: column;
		gap: 0.3rem;
		font-size: 12px;
	}

	.error {
		margin: 0 0 0.75rem;
		color: var(--destructive);
		font-size: 12px;
	}

	.sessions {
		list-style: none;
		margin: 0;
		padding: 0;
		border-top: 1px solid var(--border);
	}

	.session {
		border-bottom: 1px solid var(--border);
	}

	.session-link {
		display: flex;
		flex-direction: column;
		gap: 0.2rem;
		padding: 0.65rem 0.15rem;
		color: inherit;
		text-decoration: none;
	}

	.session-link:hover,
	.session-link:focus-visible {
		background: var(--accent);
	}

	.session-title {
		overflow: hidden;
		font-weight: 550;
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	.session-meta,
	.session-repo,
	.session-snippet {
		color: var(--muted-foreground);
		font-size: 12px;
	}

	.session-meta {
		display: flex;
		flex-wrap: wrap;
		gap: 0.55rem;
		font-family: var(--font-mono);
		font-size: 11px;
		font-variant-numeric: tabular-nums;
	}

	.session-repo,
	.session-snippet {
		overflow: hidden;
		display: -webkit-box;
		-webkit-box-orient: vertical;
		-webkit-line-clamp: 2;
	}

	.empty {
		padding: 1.5rem 0.15rem;
		color: var(--muted-foreground);
		font-size: 12px;
	}

	.empty-title {
		margin: 0 0 0.35rem;
		color: var(--foreground);
		font-size: 13px;
		font-weight: 550;
	}

	@media (max-width: 639px) {
		.filters {
			grid-template-columns: 1fr;
		}
	}
</style>
