<script lang="ts">
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { readJSON } from '$lib/api';
	import { dateAndTime, dayLabel, shortAge, shortRepository } from '$lib/format';
	import type { TranscriptCard, TranscriptCardPage } from '$lib/types';
	import { onMount, tick } from 'svelte';

	let { current = '' }: { current?: string } = $props();

	let cards = $state.raw<TranscriptCard[]>([]);
	let next = $state('');
	let loading = $state(true);
	let error = $state('');
	let query = $state('');
	let list = $state<HTMLElement>();
	let filterInput = $state<HTMLInputElement>();

	// The layout keeps this component mounted while traces change, so the list loads once.
	async function load(cursor = '') {
		loading = true;
		try {
			const result = await readJSON<TranscriptCardPage>(fetch, `/api/v1/cards?limit=100${cursor ? `&cursor=${encodeURIComponent(cursor)}` : ''}`);
			if (!result.ok) {
				error = result.message;
				return;
			}
			cards = [...cards, ...(result.data.cards ?? [])];
			next = result.data.next_cursor ?? '';
			error = '';
		} catch {
			error = 'Could not load sessions';
		} finally {
			loading = false;
		}
	}

	onMount(async () => {
		await load();
		await tick();
		list?.querySelector('[aria-current="page"]')?.scrollIntoView({ block: 'center' });
	});

	let shown = $derived.by(() => {
		const needle = query.trim().toLowerCase();
		if (!needle) return cards;
		return cards.filter((card) =>
			`${card.title ?? ''} ${card.native_trace_id} ${card.repository ?? ''}`.toLowerCase().includes(needle)
		);
	});

	// Cards arrive newest first; consecutive cards on the same day share a heading.
	let days = $derived.by(() => {
		const groups: { label: string; cards: TranscriptCard[] }[] = [];
		for (const card of shown) {
			const label = dayLabel(card.updated_at);
			if (groups.at(-1)?.label === label) groups.at(-1)!.cards.push(card);
			else groups.push({ label, cards: [card] });
		}
		return groups;
	});

	function isEditable(element: EventTarget | null) {
		if (!(element instanceof HTMLElement)) return false;
		return ['INPUT', 'TEXTAREA', 'SELECT'].includes(element.tagName) || element.isContentEditable;
	}

	// j and k open the next and previous session; / jumps to the filter.
	function onKeydown(event: KeyboardEvent) {
		if (event.defaultPrevented || event.metaKey || event.ctrlKey || event.altKey || event.isComposing) return;
		// Narrow screens hide the sidebar with CSS; its keys go with it.
		if (isEditable(event.target) || !list?.offsetParent) return;
		if (event.key === '/') {
			event.preventDefault();
			filterInput?.focus();
			return;
		}
		if (event.key !== 'j' && event.key !== 'k') return;
		const index = shown.findIndex((card) => String(card.id) === current);
		const target = shown[index === -1 ? 0 : index + (event.key === 'j' ? 1 : -1)];
		if (!target) return;
		event.preventDefault();
		void goto(resolve('/traces/[id]', { id: String(target.id) })).then(() =>
			list?.querySelector('[aria-current="page"]')?.scrollIntoView({ block: 'nearest' })
		);
	}

	function onFilterKeydown(event: KeyboardEvent) {
		if (event.key === 'Escape') {
			query = '';
			filterInput?.blur();
		} else if (event.key === 'Enter' && shown[0]) {
			event.preventDefault();
			filterInput?.blur();
			void goto(resolve('/traces/[id]', { id: String(shown[0].id) }));
		}
	}
</script>

<svelte:window onkeydown={onKeydown} />

<aside class="sidebar" aria-label="Sessions">
	<div class="filter">
		<svg viewBox="0 0 16 16" fill="none" aria-hidden="true">
			<circle cx="7" cy="7" r="4.5" stroke="currentColor" stroke-width="1.5" />
			<path d="m10.5 10.5 3 3" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" />
		</svg>
		<label class="sr-only" for="session-filter">Filter sessions by title or repository</label>
		<input
			id="session-filter"
			type="search"
			placeholder="Filter sessions"
			autocomplete="off"
			spellcheck="false"
			bind:value={query}
			bind:this={filterInput}
			onkeydown={onFilterKeydown}
		/>
		<kbd aria-hidden="true">/</kbd>
	</div>
	<nav class="list" bind:this={list}>
		{#each days as day (day.label)}
			<h2 class="day">{day.label}</h2>
			<ul>
				{#each day.cards as card (card.id)}
					{@const selected = String(card.id) === current}
					<li>
						<a
							class={['row', selected && 'is-current']}
							href={resolve('/traces/[id]', { id: String(card.id) })}
							aria-current={selected ? 'page' : undefined}
							style="--ink: var(--ink-{card.harness}, var(--muted-foreground))"
						>
							<span class="title">{card.title || card.native_trace_id}</span>
							<time class="age" datetime={card.updated_at} title={dateAndTime(card.updated_at)}>{shortAge(card.updated_at)}</time>
							<span class="meta" translate="no">
								<span class="dot" aria-hidden="true"></span>
								<span class="harness">{card.harness}</span>
								{#if card.repository}<span class="repo">{shortRepository(card.repository)}</span>{/if}
							</span>
						</a>
					</li>
				{/each}
			</ul>
		{:else}
			{#if error}
				<p class="note error" role="alert">{error}</p>
			{:else if !loading}
				<p class="note">{query ? 'No loaded session matches. Search finds older ones.' : 'No sessions yet.'}</p>
			{/if}
		{/each}
		{#if loading}
			<p class="note" role="status">Loading sessions…</p>
		{:else if next}
			<button class="older" type="button" onclick={() => load(next)}>Load older sessions</button>
		{/if}
	</nav>
</aside>

<style>
	.sidebar {
		display: none;
		width: 17rem;
		flex-shrink: 0;
		flex-direction: column;
		min-height: 0;
		border-right: 1px solid var(--sidebar-border);
		background: var(--sidebar);
	}

	.filter {
		display: flex;
		align-items: center;
		gap: 0.4rem;
		height: 2rem;
		margin: 0.6rem 0.5rem 0.25rem;
		padding: 0 0.4rem 0 0.6rem;
		border: 1px solid var(--border);
		border-radius: var(--radius);
		background: var(--accent);
		color: var(--muted-foreground);
	}

	.filter:focus-within {
		border-color: color-mix(in oklch, var(--primary) 50%, var(--input));
	}

	.filter svg {
		width: 13px;
		height: 13px;
		flex-shrink: 0;
	}

	.filter input {
		min-width: 0;
		flex: 1;
		height: 100%;
		border: 0;
		background: transparent;
		color: var(--foreground);
		font: inherit;
		font-size: 13px;
	}

	.filter input:focus-visible {
		outline: none;
	}

	.filter:focus-within kbd {
		display: none;
	}

	kbd {
		display: inline-grid;
		min-width: 1.1rem;
		height: 1.1rem;
		place-items: center;
		border-radius: 4px;
		background: color-mix(in oklab, var(--foreground) 8%, transparent);
		font-family: var(--font-sans);
		font-size: 10px;
	}

	.list {
		min-height: 0;
		flex: 1;
		overflow-y: auto;
		padding: 0 0.5rem 1rem;
	}

	.day {
		position: sticky;
		top: 0;
		z-index: 1;
		margin: 0;
		padding: 0.85rem 0.6rem 0.3rem;
		background: var(--sidebar);
		color: var(--muted-foreground);
		font-size: 11px;
		font-weight: 500;
	}

	ul {
		margin: 0;
		padding: 0;
		list-style: none;
	}

	.row {
		display: grid;
		grid-template-columns: minmax(0, 1fr) auto;
		column-gap: 0.5rem;
		padding: 0.4rem 0.6rem;
		border-radius: var(--radius);
		color: color-mix(in oklab, var(--foreground) 80%, transparent);
		text-decoration: none;
		transition: background-color 100ms ease;
	}

	.row:hover {
		background: var(--sidebar-accent);
		color: var(--foreground);
	}

	.row.is-current {
		background: color-mix(in oklab, var(--foreground) 10%, transparent);
		color: var(--foreground);
	}

	.title {
		overflow: hidden;
		font-size: 13px;
		line-height: 1.35rem;
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	.row.is-current .title {
		font-weight: 500;
	}

	.age {
		color: var(--muted-foreground);
		font-size: 11px;
		line-height: 1.35rem;
		opacity: 0.75;
	}

	.meta {
		display: flex;
		grid-column: 1 / -1;
		align-items: center;
		gap: 0.4rem;
		min-width: 0;
		color: var(--muted-foreground);
		font-size: 11px;
		line-height: 1rem;
	}

	.dot {
		width: 6px;
		height: 6px;
		flex-shrink: 0;
		border-radius: 999px;
		background: var(--ink);
	}

	.harness {
		flex-shrink: 0;
	}

	.repo {
		overflow: hidden;
		font-family: var(--font-mono);
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	.repo::before {
		content: '·';
		margin-right: 0.4rem;
		font-family: var(--font-sans);
	}

	.note {
		margin: 0.75rem 0.6rem;
		color: var(--muted-foreground);
		font-size: 12px;
	}

	.note.error {
		color: var(--destructive);
	}

	.older {
		width: 100%;
		margin-top: 0.5rem;
		padding: 0.45rem;
		border: 0;
		border-radius: var(--radius);
		background: transparent;
		color: var(--muted-foreground);
		font: inherit;
		font-size: 12px;
		cursor: pointer;
	}

	.older:hover,
	.older:focus-visible {
		background: var(--sidebar-accent);
		color: var(--foreground);
	}

	/* Narrow screens keep the full-page session list from the rail. */
	@media (min-width: 1100px) {
		.sidebar {
			display: flex;
		}
	}
</style>
