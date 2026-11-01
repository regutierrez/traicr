<script lang="ts">
	import { page } from '$app/state';
	import { resolve } from '$app/paths';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import SidebarToggle from '$lib/components/SidebarToggle.svelte';
	import { clockTime, count, dayLabel, shortRepository } from '$lib/format';
	import { highlightParts } from '$lib/highlight';
	import { withQuery } from '$lib/links';
	import { sidebar } from '$lib/sidebar.svelte';
	import { plainMarkdown } from '$lib/transcript/turns';
	import { harnesses, type TranscriptCard } from '$lib/types';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();
	let query = $derived(page.url.searchParams.get('q') ?? '');
	let mode = $derived(page.url.searchParams.get('mode') ?? 'fulltext');

	const filterNames = ['harness', 'model', 'machine', 'repository', 'after', 'before'];
	let activeFilters = $derived(filterNames.filter((name) => field(name)).length);
	// With nothing to search for, the session list beside the page already lists every session.
	// Older links may still carry event-level filters (role, kind, tool); those are a search too.
	let browsing = $derived(!query && !activeFilters && !['role', 'kind', 'tool'].some((name) => field(name)));
	let queryInput = $state<HTMLInputElement>();

	function onKeydown(event: KeyboardEvent) {
		if (event.key !== '/' || event.metaKey || event.ctrlKey || event.altKey) return;
		const target = event.target;
		if (target instanceof HTMLElement && (['INPUT', 'TEXTAREA', 'SELECT'].includes(target.tagName) || target.isContentEditable)) return;
		event.preventDefault();
		queryInput?.focus();
	}

	// Cards arrive newest first; consecutive cards on the same day share a heading.
	let days = $derived.by(() => {
		const groups: { label: string; cards: TranscriptCard[] }[] = [];
		for (const card of data.cards) {
			const label = dayLabel(card.updated_at);
			if (groups.at(-1)?.label === label) groups.at(-1)!.cards.push(card);
			else groups.push({ label, cards: [card] });
		}
		return groups;
	});

	function field(name: string) {
		return page.url.searchParams.get(name) ?? '';
	}

	// A search result opens Trace at the message that matched, in the subagent when the match was there.
	function cardHref(card: TranscriptCard) {
		const id = String(card.matched_child?.id ?? card.id);
		if (!card.matched_event_id) return resolve('/traces/[id]', { id });
		return withQuery(resolve('/traces/[id]/trace', { id }), new URLSearchParams({ event: String(card.matched_event_id) }));
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

<svelte:window onkeydown={onKeydown} />

<header class={['page-head', browsing && sidebar.open && 'is-browse']}>
	<div class="title-row"><SidebarToggle /><h1>{browsing ? 'Sessions' : 'Search'}</h1></div>
	<p class="count">
		<!-- Each card is a session, shown once however many of its messages matched. -->
		{query
			? `${count(data.cards.length, 'session')} ${data.cards.length === 1 ? 'matches' : 'match'} “${query}”${data.partial ? ' so far' : data.next ? ' on this page' : ''}`
			: `${count(data.cards.length, 'session')} on this page`}
	</p>
</header>

<form class="search" method="GET" action={resolve('/')}>
	<div class="bar">
		<svg class="glass" viewBox="0 0 16 16" fill="none" aria-hidden="true">
			<circle cx="7" cy="7" r="4.5" stroke="currentColor" stroke-width="1.5" />
			<path d="m10.5 10.5 3 3" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" />
		</svg>
		<label class="sr-only" for="query">Search your traces</label>
		<input
			id="query"
			name="q"
			type="search"
			value={query}
			bind:this={queryInput}
			placeholder="Search prompts, answers, and tool output…"
			autocomplete="off"
			spellcheck="false"
		/>
		<select class="mode" name="mode" aria-label="Search mode">
			<option value="fulltext" selected={mode === 'fulltext'}>Words</option>
			<option value="exact" selected={mode === 'exact'}>Exact text</option>
			<option value="regex" selected={mode === 'regex'}>Regex</option>
		</select>
		<Button type="submit" class="search-button">Search</Button>
	</div>
	<details class="more" open={activeFilters > 0}>
		<summary>
			Filters{#if activeFilters}<span class="badge">{activeFilters}</span>{/if}
		</summary>
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
				<Input name="model" value={field('model')} placeholder="claude-opus-5…" autocomplete="off" spellcheck="false" />
			</label>
			<label>Machine
				<select name="machine">
					<option value="">All machines</option>
					{#each data.machines as machine (machine.id)}
						<option value={machine.id} selected={field('machine') === machine.id}>{machine.hostname}</option>
					{/each}
				</select>
			</label>
			<label>Repository
				<Input name="repository" value={field('repository')} placeholder="github.com/you/repo…" autocomplete="off" spellcheck="false" />
			</label>
			<label>From
				<Input type="date" name="after" value={field('after')} />
			</label>
			<label>Before
				<Input type="date" name="before" value={field('before')} />
			</label>
		</div>
	</details>
</form>

<section class={[browsing && sidebar.open && 'is-browse']}>
	<p class="browse-hint">Pick a session from the list, or search every prompt, reply, and tool output above. Press <kbd>/</kbd> to search.</p>
	{#if data.error}
		<p class="error" role="alert">{data.error}</p>
	{/if}
	{#each days as day (day.label)}
		<h2 class="day">{day.label}</h2>
		<ul class="sessions">
			{#each day.cards as card (card.id)}
				{@render session(card)}
			{/each}
		</ul>
	{:else}
		{#if !data.error}
			<div class="empty">
				<p class="empty-title">{data.partial ? 'No matches yet' : query ? 'No matching sessions' : 'Your archive starts here'}</p>
				<p>
					{data.partial ? 'The search stopped after 5 seconds. Older sessions are not searched yet.' : query ? 'Try another search mode, remove a filter, or use a shorter phrase.' : 'Collect traces on a source machine, then upload the ZIPs. Your original records stay intact.'}
				</p>
			</div>
		{/if}
	{/each}
	{#if data.next}
		{#if data.partial && data.cards.length}
			<p class="partial">The search stopped after 5 seconds. Older sessions are not searched yet.</p>
		{/if}
		<Button class="mt-6" variant="outline" href={nextHref(data.next)}>{data.partial ? 'Continue searching' : 'Older sessions'}</Button>
	{/if}
</section>

{#snippet session(card: TranscriptCard)}
	<li>
		<a class="session" href={cardHref(card)} style="--ink: var(--ink-{card.harness}, var(--muted-foreground))">
			<span class="when">
				<time datetime={card.updated_at}>{clockTime(card.updated_at)}</time>
				<span class="harness" translate="no">{card.harness}</span>
			</span>
			<span class="what">
				<span class="title">{card.title || card.native_trace_id}</span>
				<span class="repo" translate="no">{card.repository ? shortRepository(card.repository) : 'No repository'}</span>
				{#if card.matched_child}
					<span class="child">Matched in subagent: {card.matched_child.title || 'untitled'}</span>
				{/if}
				{#if card.snippet}
					<span class="snippet">
						{#each highlightParts(plainMarkdown(card.snippet), query, mode) as part, index (`${index}:${part.mark}:${part.text}`)}
							{#if part.mark}<mark>{part.text}</mark>{:else}{part.text}{/if}
						{/each}
					</span>
				{/if}
			</span>
		</a>
	</li>
{/snippet}

<style>
	.page-head {
		display: flex;
		align-items: baseline;
		justify-content: space-between;
		gap: 1rem;
		margin: 0.5rem 0 1.25rem;
	}

	h1 {
		margin: 0;
		font-size: 1.5rem;
		font-weight: 600;
		letter-spacing: -0.02em;
		line-height: 1.1;
	}

	.count {
		margin: 0;
		color: var(--muted-foreground);
		font-size: 13px;
		font-variant-numeric: tabular-nums;
	}

	.search {
		margin-bottom: 1.75rem;
	}

	.bar {
		display: flex;
		align-items: center;
		gap: 0.25rem;
		height: 2.75rem;
		padding: 0 0.3rem 0 0.85rem;
		border: 1px solid var(--input);
		border-radius: 0.6rem;
		background: var(--card);
	}

	.bar:focus-within {
		border-color: color-mix(in oklch, var(--primary) 60%, var(--input));
		box-shadow: 0 0 0 3px color-mix(in oklch, var(--primary) 14%, transparent);
	}

	.glass {
		width: 16px;
		height: 16px;
		flex-shrink: 0;
		color: var(--muted-foreground);
	}

	.bar input {
		min-width: 0;
		flex: 1;
		height: 100%;
		border: 0;
		background: transparent;
		color: var(--foreground);
		padding: 0 0.4rem;
		font: inherit;
		font-size: 15px;
	}

	.bar input:focus-visible {
		outline: none;
	}

	.bar input::placeholder {
		color: var(--muted-foreground);
	}

	.mode {
		height: 2rem;
		border: 0;
		border-left: 1px solid var(--border);
		background: transparent;
		color: var(--muted-foreground);
		padding: 0 0.6rem;
		font: inherit;
		font-size: 13px;
		cursor: pointer;
	}

	.mode option,
	.more-grid select option {
		background: var(--popover);
		color: var(--foreground);
	}

	.bar :global(.search-button) {
		height: 2.1rem;
		padding: 0 0.95rem;
	}

	.more {
		margin-top: 0.6rem;
	}

	.more summary {
		display: inline-flex;
		align-items: center;
		gap: 0.4rem;
		color: var(--muted-foreground);
		font-size: 13px;
		list-style: none;
		cursor: pointer;
	}

	.more summary::before {
		content: '';
		width: 6px;
		height: 6px;
		margin-right: 0.15rem;
		border-right: 1.5px solid currentColor;
		border-bottom: 1.5px solid currentColor;
		transform: rotate(-45deg);
		transition: transform 120ms ease;
	}

	.more[open] summary::before {
		transform: rotate(45deg);
	}

	.more summary::-webkit-details-marker {
		display: none;
	}

	.more summary:hover {
		color: var(--foreground);
	}

	.badge {
		display: inline-grid;
		place-items: center;
		min-width: 1.15rem;
		height: 1.15rem;
		padding: 0 0.3rem;
		border-radius: 999px;
		background: var(--primary);
		color: var(--primary-foreground);
		font-size: 11px;
		font-weight: 600;
	}

	.more-grid {
		display: grid;
		gap: 0.75rem;
		grid-template-columns: repeat(auto-fill, minmax(12rem, 1fr));
		padding: 0.85rem;
		margin-top: 0.6rem;
		border: 1px solid var(--border);
		border-radius: 0.6rem;
		background: var(--card);
	}

	.more-grid label {
		display: flex;
		flex-direction: column;
		gap: 0.3rem;
		color: var(--muted-foreground);
		font-size: 12px;
	}

	.more-grid select {
		height: 2rem;
		border: 1px solid var(--input);
		border-radius: var(--radius);
		background: var(--background);
		color: var(--foreground);
		padding: 0 0.55rem;
		font: inherit;
	}

	.error {
		margin: 0 0 1rem;
		color: var(--destructive);
		font-size: 13px;
	}

	.day {
		position: sticky;
		top: 0;
		z-index: 1;
		margin: 0;
		padding: 0.6rem 0 0.4rem;
		background: var(--background);
		border-bottom: 1px solid var(--border);
		color: var(--muted-foreground);
		font-size: 13px;
		font-weight: 500;
	}

	.sessions {
		list-style: none;
		margin: 0 0 1.5rem;
		padding: 0;
	}

	.session {
		display: grid;
		grid-template-columns: 5.5rem minmax(0, 1fr);
		gap: 0 1.25rem;
		padding: 0.85rem 0.75rem 0.9rem 0;
		border-bottom: 1px solid var(--border);
		color: inherit;
		text-decoration: none;
	}

	.session:hover {
		background: linear-gradient(to right, transparent, var(--accent) 6rem);
	}

	.session:hover .title {
		color: #fff;
	}

	.when {
		display: flex;
		flex-direction: column;
		gap: 0.2rem;
		padding-top: 0.1rem;
		font-size: 12px;
		font-variant-numeric: tabular-nums;
	}

	.when time {
		color: var(--muted-foreground);
	}

	.harness {
		display: inline-flex;
		align-items: center;
		gap: 0.4rem;
		color: var(--ink);
		font-family: var(--font-mono);
		font-size: 11px;
	}

	.harness::before {
		content: '';
		width: 6px;
		height: 6px;
		border-radius: 999px;
		background: var(--ink);
	}

	.what {
		display: flex;
		min-width: 0;
		flex-direction: column;
		gap: 0.15rem;
	}

	.title {
		overflow: hidden;
		font-size: 15px;
		font-weight: 500;
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	.repo {
		color: var(--muted-foreground);
		font-family: var(--font-mono);
		font-size: 12px;
	}

	.child {
		color: var(--muted-foreground);
		font-size: 12px;
	}

	.snippet {
		overflow: hidden;
		display: -webkit-box;
		max-width: 75ch;
		margin-top: 0.25rem;
		color: color-mix(in oklch, var(--foreground) 72%, var(--background));
		font-size: 13px;
		-webkit-box-orient: vertical;
		line-clamp: 2;
		-webkit-line-clamp: 2;
	}

	.title-row {
		display: flex;
		align-items: center;
		gap: 0.4rem;
	}

	.browse-hint {
		display: none;
		margin: 2rem 0;
		color: var(--muted-foreground);
		font-size: 13px;
	}

	.browse-hint kbd {
		padding: 0.05rem 0.35rem;
		border-radius: 4px;
		background: color-mix(in oklab, var(--foreground) 8%, transparent);
		font-family: var(--font-sans);
		font-size: 11px;
	}

	/* On wide screens the session list beside the page does the browsing; this page only searches. */
	@media (min-width: 1100px) {
		.is-browse .count {
			display: none;
		}

		section.is-browse > :not(.browse-hint) {
			display: none;
		}

		section.is-browse > .browse-hint {
			display: block;
		}
	}

	.empty {
		padding: 3rem 0;
		color: var(--muted-foreground);
		font-size: 13px;
	}

	.partial {
		margin: 1.5rem 0 0;
		color: var(--muted-foreground);
		font-size: 13px;
	}

	.empty-title {
		margin: 0 0 0.35rem;
		color: var(--foreground);
		font-size: 15px;
		font-weight: 500;
	}

	@media (max-width: 639px) {
		h1 {
			font-size: 1.4rem;
		}

		.mode {
			max-width: 5.5rem;
		}

		.session {
			grid-template-columns: minmax(0, 1fr);
			gap: 0.3rem;
			padding-right: 0;
		}

		.when {
			flex-direction: row;
			gap: 0.75rem;
		}

	}
</style>
