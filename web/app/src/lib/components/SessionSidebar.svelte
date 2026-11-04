<script lang="ts">
	import { browser } from '$app/environment';
	import { afterNavigate, goto, replaceState } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import { readJSON } from '$lib/api';
	import SidebarToggle from '$lib/components/SidebarToggle.svelte';
	import { count, dateAndTime, dayLabel, shortAge, shortRepository } from '$lib/format';
	import { highlightParts } from '$lib/highlight';
	import { withQuery } from '$lib/links';
	import { search, searchFilters, sidebar, toggleSidebar } from '$lib/sidebar.svelte';
	import { plainMarkdown } from '$lib/transcript/turns';
	import { harnesses, type Machine, type TranscriptCard, type TranscriptCardPage } from '$lib/types';
	import { onMount, tick } from 'svelte';

	// On narrow screens the home page is this list, shown whether or not it is open beside pages.
	let { current = '', home = false }: { current?: string; home?: boolean } = $props();

	let cards = $state.raw<TranscriptCard[]>([]);
	let next = $state('');
	let partial = $state(false);
	let loading = $state(true);
	let error = $state('');
	let machines = $state.raw<Machine[]>([]);
	let filtersOpen = $state(false);
	let list = $state<HTMLElement>();
	let queryInput = $state<HTMLInputElement>();

	// The search the list asks for, as /api/v1/cards parameters.
	let wanted = $derived.by(() => {
		const params = new URLSearchParams();
		const q = search.q.trim();
		if (q) params.set('q', q);
		if (q && search.mode !== 'fulltext') params.set('mode', search.mode);
		for (const name of searchFilters) if (search[name]) params.set(name, search[name]);
		return params.toString();
	});
	// The search the shown cards answer. It trails `wanted` while a search runs.
	let shown = $state('');
	let shownQuery = $derived(new URLSearchParams(shown).get('q') ?? '');
	let shownMode = $derived(new URLSearchParams(shown).get('mode') ?? 'fulltext');
	let activeFilters = $derived(searchFilters.filter((name) => search[name]));

	let request = 0;
	async function load(params: string, cursor = '') {
		const id = ++request;
		loading = true;
		try {
			const query = new URLSearchParams(params);
			query.set('limit', '100');
			if (cursor) query.set('cursor', cursor);
			const result = await readJSON<TranscriptCardPage>(fetch, `/api/v1/cards?${query}`);
			if (id !== request) return;
			if (!result.ok) {
				error = result.message;
				return;
			}
			cards = cursor ? [...cards, ...(result.data.cards ?? [])] : (result.data.cards ?? []);
			next = result.data.next_cursor ?? '';
			partial = result.data.partial ?? false;
			shown = params;
			error = '';
		} catch {
			if (id === request) error = 'Could not load sessions';
		} finally {
			if (id === request) loading = false;
		}
	}

	// Typing waits for a pause before it searches; filters and the first load search at once.
	let timer: ReturnType<typeof setTimeout> | undefined;
	let lastQuery: string | undefined;
	$effect(() => {
		const params = wanted;
		const q = search.q.trim();
		timer = setTimeout(() => load(params), lastQuery !== undefined && q !== lastQuery ? 300 : 0);
		lastQuery = q;
		return () => clearTimeout(timer);
	});

	// Links such as /?machine=… from Archive or /?q=… from older bookmarks set the search.
	afterNavigate(({ to }) => {
		const params = to?.url.searchParams;
		if (to?.url.pathname !== '/' || !params || !['q', ...searchFilters].some((name) => params.has(name))) return;
		search.q = params.get('q') ?? '';
		search.mode = params.get('mode') ?? 'fulltext';
		for (const name of searchFilters) search[name] = params.get(name) ?? '';
	});

	// The home page's address carries the search, so reloading or sharing it keeps the list.
	$effect(() => {
		const params = wanted;
		if (!home || page.url.search.slice(1) === params) return;
		replaceState(withQuery(resolve('/'), new URLSearchParams(params)), page.state);
	});

	onMount(async () => {
		const result = await readJSON<Machine[]>(fetch, '/api/v1/machines').catch(() => null);
		if (result?.ok) machines = result.data ?? [];
	});

	// The first load scrolls the open session into view.
	let scrolled = false;
	$effect(() => {
		if (scrolled || !cards.length) return;
		scrolled = true;
		void tick().then(() => list?.querySelector('[aria-current="page"]')?.scrollIntoView({ block: 'center' }));
	});

	// Cards arrive newest first; consecutive cards on the same day share a heading.
	let days = $derived.by(() => {
		const groups: { label: string; cards: TranscriptCard[] }[] = [];
		for (const card of cards) {
			const label = dayLabel(card.updated_at);
			if (groups.at(-1)?.label === label) groups.at(-1)!.cards.push(card);
			else groups.push({ label, cards: [card] });
		}
		return groups;
	});

	// A search result opens at the message that matched, in the subagent when the match was there.
	function cardID(card: TranscriptCard) {
		return String(card.matched_child?.id ?? card.id);
	}

	function cardHref(card: TranscriptCard) {
		const id = cardID(card);
		if (!card.matched_event_id) return resolve('/traces/[id]', { id });
		// The query rides along so Trace can mark its hits and step through them.
		return withQuery(resolve('/traces/[id]/trace', { id }), { event: String(card.matched_event_id), q: shownQuery, mode: shownMode });
	}

	function filterLabel(name: (typeof searchFilters)[number]) {
		const value = search[name];
		if (name === 'machine') return machines.find((machine) => machine.id === value)?.hostname ?? value;
		if (name === 'repository') return shortRepository(value);
		if (name === 'after') return `from ${value}`;
		if (name === 'before') return `before ${value}`;
		return value;
	}

	function clearFilters() {
		for (const name of searchFilters) search[name] = '';
	}

	function isEditable(element: EventTarget | null) {
		if (!(element instanceof HTMLElement)) return false;
		return ['INPUT', 'TEXTAREA', 'SELECT'].includes(element.tagName) || element.isContentEditable;
	}

	function rows() {
		return [...(list?.querySelectorAll<HTMLAnchorElement>('a.row') ?? [])];
	}

	// / jumps to the search box, opening the list when it is hidden. j and k open the next and
	// previous session. ↑ and ↓ move between rows, and ↑ from the first row goes back to the box.
	async function onKeydown(event: KeyboardEvent) {
		if (event.defaultPrevented || event.metaKey || event.ctrlKey || event.altKey || event.isComposing) return;
		if (event.key === 'ArrowDown' || event.key === 'ArrowUp') {
			const links = rows();
			const index = links.indexOf(document.activeElement as HTMLAnchorElement);
			if (index === -1) return;
			event.preventDefault();
			const target = index + (event.key === 'ArrowDown' ? 1 : -1);
			if (target < 0) queryInput?.focus();
			else links[Math.min(target, links.length - 1)].focus();
			return;
		}
		if (isEditable(event.target)) return;
		if (event.key === '/') {
			const wide = matchMedia('(min-width: 1100px)').matches;
			// Narrow screens show the list only as the home page.
			if (!wide && !home) return;
			event.preventDefault();
			if (wide && !sidebar.open) {
				toggleSidebar();
				await tick();
			}
			queryInput?.focus();
			return;
		}
		// A hidden list takes no keys.
		if ((event.key !== 'j' && event.key !== 'k') || !list?.offsetParent) return;
		const index = cards.findIndex((card) => cardID(card) === current);
		const target = cards[index === -1 ? 0 : index + (event.key === 'j' ? 1 : -1)];
		if (!target) return;
		event.preventDefault();
		void goto(cardHref(target)).then(() => list?.querySelector('[aria-current="page"]')?.scrollIntoView({ block: 'nearest' }));
	}

	// Dragging the list's right edge sets its width, which is remembered. Double-click resets it.
	const defaultWidth = 352;
	const minWidth = 256;
	const maxWidth = 400;
	let width = $state(clampWidth(browser ? Number(localStorage.getItem('traicr.sidebar-width')) || defaultWidth : defaultWidth));

	function clampWidth(value: number) {
		return Math.round(Math.min(maxWidth, Math.max(minWidth, value)));
	}

	function saveWidth(value: number) {
		width = clampWidth(value);
		localStorage.setItem('traicr.sidebar-width', String(width));
	}

	function startResize(event: PointerEvent) {
		if (event.button !== 0) return;
		event.preventDefault();
		const handle = event.currentTarget as HTMLElement;
		const startX = event.clientX;
		const startWidth = width;
		handle.setPointerCapture(event.pointerId);
		// The page shows the resize cursor and selects no text until the pointer is released.
		document.documentElement.classList.add('is-resizing');
		const move = (moved: PointerEvent) => (width = clampWidth(startWidth + moved.clientX - startX));
		const stop = () => {
			handle.removeEventListener('pointermove', move);
			handle.removeEventListener('pointerup', stop);
			handle.removeEventListener('pointercancel', stop);
			document.documentElement.classList.remove('is-resizing');
			saveWidth(width);
		};
		handle.addEventListener('pointermove', move);
		handle.addEventListener('pointerup', stop);
		handle.addEventListener('pointercancel', stop);
	}

	function onResizeKeydown(event: KeyboardEvent) {
		const step = event.shiftKey ? 64 : 16;
		const next = { ArrowLeft: width - step, ArrowRight: width + step, Home: minWidth, End: maxWidth }[event.key];
		if (next === undefined) return;
		event.preventDefault();
		saveWidth(next);
	}

	// Enter searches now when a search is still waiting, else opens the first session.
	function onQueryKeydown(event: KeyboardEvent) {
		if (event.key === 'Escape') {
			if (search.q) search.q = '';
			else queryInput?.blur();
		} else if (event.key === 'Enter') {
			event.preventDefault();
			if (wanted !== shown) {
				clearTimeout(timer);
				void load(wanted);
			} else if (cards[0]) void goto(cardHref(cards[0]));
		} else if (event.key === 'ArrowDown' && !event.altKey && !event.metaKey && !event.ctrlKey && !event.shiftKey) {
			event.preventDefault();
			rows()[0]?.focus();
		}
	}
</script>

<svelte:window onkeydown={onKeydown} />

<aside class={['sidebar', sidebar.open && 'is-open', home && 'is-home']} aria-label="Sessions" style:--sidebar-width="{width}px">
	<div class="head">
		<SidebarToggle />
		<div class="box">
			<svg viewBox="0 0 16 16" fill="none" aria-hidden="true">
				<circle cx="7" cy="7" r="4.5" stroke="currentColor" stroke-width="1.5" />
				<path d="m10.5 10.5 3 3" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" />
			</svg>
			<label class="sr-only" for="session-search">Search sessions: titles, prompts, replies, and tool output</label>
			<input
				id="session-search"
				type="search"
				placeholder="Search sessions"
				autocomplete="off"
				spellcheck="false"
				bind:value={search.q}
				bind:this={queryInput}
				onkeydown={onQueryKeydown}
			/>
			<kbd aria-hidden="true">/</kbd>
			<select class="mode" aria-label="Search mode" bind:value={search.mode}>
				<option value="fulltext">Words</option>
				<option value="exact">Exact</option>
				<option value="regex">Regex</option>
			</select>
		</div>
		<button
			class={['icon-button', (filtersOpen || activeFilters.length > 0) && 'is-on']}
			type="button"
			aria-expanded={filtersOpen}
			aria-controls="session-filters"
			aria-label="Filters"
			title="Filters"
			onclick={() => (filtersOpen = !filtersOpen)}
		>
			<svg viewBox="0 0 16 16" fill="none" aria-hidden="true">
				<path d="M2.5 4h11M4.5 8h7M6.5 12h3" stroke="currentColor" stroke-width="1.4" stroke-linecap="round" />
			</svg>
			{#if activeFilters.length}<span class="badge">{activeFilters.length}</span>{/if}
		</button>
	</div>

	{#if filtersOpen}
		<div class="filters" id="session-filters">
			<label>Harness
				<select bind:value={search.harness}>
					<option value="">All</option>
					{#each harnesses as harness (harness)}<option value={harness}>{harness}</option>{/each}
				</select>
			</label>
			<label>Machine
				<select bind:value={search.machine}>
					<option value="">All</option>
					{#each machines as machine (machine.id)}<option value={machine.id}>{machine.hostname}</option>{/each}
				</select>
			</label>
			<!-- Text filters apply when you leave the box or press Enter, not on every keystroke. -->
			<label>Model
				<input value={search.model} onchange={(event) => (search.model = event.currentTarget.value.trim())} placeholder="Any" title="The model ID as recorded, such as claude-opus-5" autocomplete="off" spellcheck="false" />
			</label>
			<label>Repository
				<input value={search.repository} onchange={(event) => (search.repository = event.currentTarget.value.trim())} placeholder="Any" title="A remote such as github.com/you/repo, or a local path" autocomplete="off" spellcheck="false" />
			</label>
			<label>From
				<input type="date" bind:value={search.after} />
			</label>
			<label>Before
				<input type="date" bind:value={search.before} />
			</label>
			{#if activeFilters.length}
				<button class="clear" type="button" onclick={clearFilters}>Clear filters</button>
			{/if}
		</div>
	{:else if activeFilters.length}
		<ul class="chips" aria-label="Active filters">
			{#each activeFilters as name (name)}
				<li>
					<span class="chip-name">{name === 'after' || name === 'before' ? 'date' : name}</span>
					<span class="chip-value" translate="no">{filterLabel(name)}</span>
					<button type="button" aria-label="Remove {name} filter" title="Remove" onclick={() => (search[name] = '')}>×</button>
				</li>
			{/each}
		</ul>
	{/if}

	{#if wanted}
		<!-- Each card is a session, shown once however many of its messages matched. -->
		<p class="count" role="status">
			{#if wanted !== shown}Searching…{:else}{count(cards.length, 'session')}
				{shownQuery ? (cards.length === 1 ? 'matches' : 'match') : 'shown'}{partial ? ' so far' : next ? ', more below' : ''}{/if}
		</p>
	{/if}

	<nav class="list" bind:this={list}>
		{#each days as day (day.label)}
			<h2 class="day">{day.label}</h2>
			<ul>
				{#each day.cards as card (card.id)}
					{@const selected = cardID(card) === current}
					<li>
						<a
							class={['row', selected && 'is-current']}
							href={cardHref(card)}
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
							{#if card.matched_child}
								<span class="child">In subagent: {card.matched_child.title || 'untitled'}</span>
							{/if}
							<!-- Without a query the snippet is only a preview of the session; the title says enough. -->
							{#if card.snippet && shownQuery}
								<span class="snippet">
									{#each highlightParts(plainMarkdown(card.snippet), shownQuery, shownMode) as part, index (`${index}:${part.mark}:${part.text}`)}
										{#if part.mark}<mark>{part.text}</mark>{:else}{part.text}{/if}
									{/each}
								</span>
							{/if}
						</a>
					</li>
				{/each}
			</ul>
		{:else}
			{#if error}
				<p class="note error" role="alert">{error}</p>
			{:else if !loading}
				<p class="note">
					{#if partial}No matches yet. The search stopped after 5 seconds.
					{:else if shown}No matching sessions. Try another search mode, remove a filter, or use a shorter phrase.
					{:else}No sessions yet. Collect traces on a source machine, then upload the ZIPs.{/if}
				</p>
			{/if}
		{/each}
		{#if error && cards.length}
			<p class="note error" role="alert">{error}</p>
		{/if}
		{#if loading}
			{#if !cards.length}<p class="note" role="status">Loading sessions…</p>{/if}
		{:else if next}
			<button class="older" type="button" onclick={() => load(shown, next)}>
				{partial ? 'Continue searching older sessions' : 'Load older sessions'}
			</button>
		{/if}
	</nav>
	<!-- A focusable separator is an ARIA widget (the window splitter pattern); Svelte's lint treats every separator as static. -->
	<!-- svelte-ignore a11y_no_noninteractive_tabindex, a11y_no_noninteractive_element_interactions -->
	<div
		class="resize"
		role="separator"
		aria-orientation="vertical"
		aria-label="Resize session list"
		aria-valuemin={minWidth}
		aria-valuemax={maxWidth}
		aria-valuenow={width}
		tabindex="0"
		title="Drag to resize, double-click to reset"
		onpointerdown={startResize}
		ondblclick={() => saveWidth(defaultWidth)}
		onkeydown={onResizeKeydown}
	></div>
</aside>

<style>
	.sidebar {
		position: relative;
		display: none;
		width: var(--sidebar-width, 22rem);
		flex-shrink: 0;
		flex-direction: column;
		min-height: 0;
		border-right: 1px solid var(--sidebar-border);
		background: var(--sidebar);
	}

	.head {
		display: flex;
		align-items: center;
		gap: 0.35rem;
		margin: 0.6rem 0.5rem 0.25rem;
	}

	.box {
		display: flex;
		min-width: 0;
		flex: 1;
		align-items: center;
		gap: 0.4rem;
		height: 2rem;
		padding: 0 0.15rem 0 0.6rem;
		border: 1px solid var(--border);
		border-radius: var(--radius);
		background: var(--accent);
		color: var(--muted-foreground);
	}

	.box:focus-within {
		border-color: color-mix(in oklch, var(--primary) 50%, var(--input));
	}

	.box svg {
		width: 13px;
		height: 13px;
		flex-shrink: 0;
	}

	.box input {
		min-width: 0;
		flex: 1;
		height: 100%;
		border: 0;
		background: transparent;
		color: var(--foreground);
		font: inherit;
		font-size: 13px;
	}

	.box input:focus-visible,
	.mode:focus-visible {
		outline: none;
	}

	.box:focus-within kbd {
		display: none;
	}

	kbd {
		display: inline-grid;
		min-width: 1.1rem;
		height: 1.1rem;
		flex-shrink: 0;
		place-items: center;
		border-radius: 4px;
		background: color-mix(in oklab, var(--foreground) 8%, transparent);
		font-family: var(--font-sans);
		font-size: 10px;
	}

	.mode {
		height: 1.5rem;
		flex-shrink: 0;
		padding: 0 0.3rem;
		border: 0;
		border-left: 1px solid var(--border);
		background: transparent;
		color: var(--muted-foreground);
		font: inherit;
		font-size: 12px;
		cursor: pointer;
	}

	.mode:hover,
	.mode:focus-visible {
		color: var(--foreground);
	}

	select option {
		background: var(--popover);
		color: var(--foreground);
	}

	.icon-button {
		position: relative;
		display: grid;
		width: 1.75rem;
		height: 1.75rem;
		flex-shrink: 0;
		place-items: center;
		border: 0;
		border-radius: var(--radius);
		background: transparent;
		color: var(--muted-foreground);
		cursor: pointer;
	}

	.icon-button svg {
		width: 16px;
		height: 16px;
	}

	.icon-button:hover,
	.icon-button:focus-visible {
		background: var(--accent);
		color: var(--foreground);
	}

	.icon-button.is-on {
		color: var(--primary);
	}

	.badge {
		position: absolute;
		top: 0;
		right: 0;
		display: grid;
		min-width: 0.85rem;
		height: 0.85rem;
		place-items: center;
		padding: 0 0.2rem;
		border-radius: 999px;
		background: var(--primary);
		color: var(--primary-foreground);
		font-size: 9px;
		font-weight: 600;
	}

	.filters {
		display: grid;
		grid-template-columns: 1fr 1fr;
		gap: 0.5rem;
		margin: 0.25rem 0.5rem 0.35rem;
		padding: 0.6rem;
		border: 1px solid var(--border);
		border-radius: var(--radius);
		background: var(--card);
	}

	.filters label {
		display: flex;
		min-width: 0;
		flex-direction: column;
		gap: 0.2rem;
		color: var(--muted-foreground);
		font-size: 11px;
	}

	.filters select,
	.filters input {
		width: 100%;
		min-width: 0;
		height: 1.75rem;
		padding: 0 0.4rem;
		border: 1px solid var(--input);
		border-radius: var(--radius);
		background: var(--background);
		color: var(--foreground);
		font: inherit;
		font-size: 12px;
	}

	.filters input::placeholder {
		color: var(--muted-foreground);
	}

	.clear {
		grid-column: 1 / -1;
		justify-self: start;
		padding: 0;
		border: 0;
		background: transparent;
		color: var(--muted-foreground);
		font: inherit;
		font-size: 12px;
		cursor: pointer;
	}

	.clear:hover,
	.clear:focus-visible {
		color: var(--foreground);
		text-decoration: underline;
	}

	.chips {
		display: flex;
		flex-wrap: wrap;
		gap: 0.3rem;
		margin: 0.2rem 0.5rem 0.25rem;
	}

	.chips li {
		display: inline-flex;
		min-width: 0;
		max-width: 100%;
		align-items: center;
		gap: 0.3rem;
		height: 1.5rem;
		padding: 0 0.15rem 0 0.5rem;
		border: 1px solid var(--border);
		border-radius: 999px;
		font-size: 11px;
	}

	.chip-name {
		color: var(--muted-foreground);
	}

	.chip-value {
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	.chips button {
		display: grid;
		width: 1.15rem;
		height: 1.15rem;
		place-items: center;
		border: 0;
		border-radius: 999px;
		background: transparent;
		color: var(--muted-foreground);
		font-size: 13px;
		line-height: 1;
		cursor: pointer;
	}

	.chips button:hover,
	.chips button:focus-visible {
		background: var(--accent);
		color: var(--foreground);
	}

	.count {
		margin: 0.3rem 1.1rem 0;
		color: var(--muted-foreground);
		font-size: 11px;
		font-variant-numeric: tabular-nums;
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

	.row:focus-visible {
		outline: none;
		background: var(--sidebar-accent);
		box-shadow: inset 2px 0 0 var(--primary);
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

	.meta,
	.child,
	.snippet {
		grid-column: 1 / -1;
	}

	.meta {
		display: flex;
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

	.child {
		margin-top: 0.1rem;
		color: var(--muted-foreground);
		font-size: 11px;
	}

	.snippet {
		overflow: hidden;
		display: -webkit-box;
		margin-top: 0.2rem;
		color: color-mix(in oklch, var(--foreground) 65%, var(--background));
		font-size: 12px;
		line-height: 1.1rem;
		-webkit-box-orient: vertical;
		line-clamp: 2;
		-webkit-line-clamp: 2;
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

	/* A thin strip over the list's right border; the border lights up while it is hovered or dragged. */
	.resize {
		position: absolute;
		top: 0;
		right: -4px;
		bottom: 0;
		z-index: 2;
		display: none;
		width: 7px;
		cursor: col-resize;
		touch-action: none;
	}

	.resize::after {
		content: '';
		position: absolute;
		top: 0;
		bottom: 0;
		left: 2px;
		width: 2px;
		background: transparent;
		transition: background-color 120ms ease;
	}

	.resize:hover::after,
	.resize:focus-visible::after,
	:global(html.is-resizing) .resize::after {
		background: var(--primary);
	}

	.resize:focus-visible {
		outline: none;
	}

	:global(html.is-resizing),
	:global(html.is-resizing *) {
		cursor: col-resize !important;
		user-select: none;
	}

	/* Narrow screens have no room beside a page, so there the home page is the list at full width. */
	@media (max-width: 1099px) {
		.sidebar.is-home {
			display: flex;
			width: auto;
			flex: 1;
			border-right: 0;
		}
	}

	@media (min-width: 1100px) {
		.sidebar.is-open {
			display: flex;
		}

		.resize {
			display: block;
		}
	}
</style>
