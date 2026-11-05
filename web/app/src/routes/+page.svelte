<script lang="ts">
	import { resolve } from '$app/paths';
	import { readJSON } from '$lib/api';
	import { ago, count, dateAndTime, fileSize } from '$lib/format';
	import { healthProblems, lastImportResult } from '$lib/health';
	import { withQuery } from '$lib/links';
	import { localDayBound, search, searchFilters, sidebar, toggleSidebar } from '$lib/sidebar.svelte';
	import type { Activity } from '$lib/types';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();
	// Only what needs a look now. Failed traces in past imports stay on the Archive page.
	let problems = $derived(data.health ? healthProblems(data.health, false) : []);
	let number = new Intl.NumberFormat();
	let percent = new Intl.NumberFormat(undefined, { style: 'percent', maximumFractionDigits: 0 });
	let shortDay = new Intl.DateTimeFormat(undefined, { month: 'short', day: 'numeric' });

	const ranges = [
		{ label: '7 days', days: 7 },
		{ label: '30 days', days: 30 },
		{ label: '90 days', days: 90 },
		{ label: 'All time', days: 0 }
	];
	// Days in the range, or 0 for all time.
	let rangeDays = $state(30);
	let view = $state<'harness' | 'model'>('harness');
	let activity = $state.raw<Activity | null>(null);
	let activityError = $state('');

	/** YYYY-MM-DD of a local calendar day, the form the list's date filters take. */
	function dayKey(date: Date) {
		return `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, '0')}-${String(date.getDate()).padStart(2, '0')}`;
	}

	function parseDay(key: string) {
		const [year, month, day] = key.split('-').map(Number);
		return new Date(year, month - 1, day);
	}

	function addDays(key: string, days: number) {
		const date = parseDay(key);
		date.setDate(date.getDate() + days);
		return dayKey(date);
	}

	// Ranges are local calendar days ending today: 7 days is today and the six before it.
	let today = dayKey(new Date());
	let start = $derived(rangeDays ? addDays(today, 1 - rangeDays) : '');

	$effect(() => {
		if (!data.health?.traces) return;
		const params = new URLSearchParams({ tz: Intl.DateTimeFormat().resolvedOptions().timeZone });
		if (start) params.set('after', localDayBound(start));
		let current = true;
		readJSON<Activity>(fetch, `/api/v1/activity?${params}`)
			.then((result) => {
				if (!current) return;
				activity = result.ok ? result.data : null;
				activityError = result.ok ? '' : result.message;
			})
			.catch(() => current && (activityError = 'Could not load activity'));
		return () => (current = false);
	});

	// One bar per day of the range, with the days that had no sessions filled in.
	let bars = $derived.by(() => {
		if (!activity) return [];
		const counts = new Map(activity.days.map((day) => [day.date, day.sessions]));
		const first = start || activity.days[0]?.date;
		const days: { date: string; sessions: number }[] = [];
		if (!first) return days;
		for (let day = first; day <= today; day = addDays(day, 1)) days.push({ date: day, sessions: counts.get(day) ?? 0 });
		return days;
	});
	let peak = $derived(Math.max(1, ...bars.map((bar) => bar.sessions)));

	let rows = $derived(activity ? (view === 'harness' ? activity.harnesses : activity.models) : []);

	// A row lists its sessions in the session list: the same harness or model and range, nothing else.
	function isShown(name: string) {
		return search[view] === name && search.after === start && !search.q && searchFilters.every((filter) => filter === view || filter === 'after' || !search[filter]);
	}

	function show(name: string) {
		const shown = isShown(name);
		search.q = '';
		for (const filter of searchFilters) search[filter] = '';
		if (shown) return;
		search[view] = name;
		search.after = start;
		if (!sidebar.open) toggleSidebar();
	}
</script>

<svelte:head>
	<title>Sessions · Traicr</title>
</svelte:head>

<header class="page-head">
	<h1>Overview</h1>
	<p class="page-lede">Pick a session from the list, or search every title, prompt, reply, and tool output in it.</p>
</header>

{#if data.error}
	<p class="error" role="alert">{data.error}</p>
{/if}

{#if data.health}
	{@const health = data.health}
	{#if health.traces === 0}
		<div class="empty">
			<p class="empty-title">Your archive starts here</p>
			<p>Collect traces on a source machine, then upload the ZIPs. Your original records stay intact.</p>
		</div>
	{:else}
		<dl class="stats">
			<div><dt>Traces</dt><dd>{number.format(health.traces)}</dd></div>
			<div><dt>Events</dt><dd>{number.format(health.events)}</dd></div>
			<div><dt>Imports</dt><dd>{number.format(health.imports.total)}</dd></div>
			<div><dt>Stored</dt><dd>{fileSize(health.storage.database_bytes + health.storage.objects_bytes)}</dd></div>
		</dl>
	{/if}

	{#if problems.length}
		<p class="health-state" role="status">
			{#each problems as problem (problem.text)}<span class={problem.tone}>{problem.text}</span>{/each}
		</p>
	{/if}

	{#if health.traces}
		<section class="activity" aria-labelledby="activity-title">
			<div class="section-head">
				<h2 id="activity-title">Activity</h2>
				<div class="segmented" role="group" aria-label="Date range">
					{#each ranges as option (option.label)}
						<button type="button" aria-pressed={rangeDays === option.days} onclick={() => (rangeDays = option.days)}>{option.label}</button>
					{/each}
				</div>
			</div>

			{#if activityError}
				<p class="error" role="alert">{activityError}</p>
			{:else if activity}
				<p class="total">
					<span class="total-number">{number.format(activity.sessions)}</span>
					<span class="dim">
						{activity.sessions === 1 ? 'session' : 'sessions'}
						{rangeDays ? `active since ${shortDay.format(parseDay(start))}` : 'in the archive'}
					</span>
				</p>

				{#if bars.length}
					<figure class="chart">
						<figcaption class="dim">Sessions active each day. A session active on several days counts on each.</figcaption>
						<div class="plot">
							<span class="tick top">{number.format(peak)}</span>
							<span class="tick bottom">0</span>
							<svg viewBox="0 0 {bars.length} 100" preserveAspectRatio="none" role="img" aria-label="Sessions active each day">
								{#each bars as bar, index (bar.date)}
									<g class="bar">
										<title>{shortDay.format(parseDay(bar.date))}: {count(bar.sessions, 'session')}</title>
										<rect class="hit" x={index} y="0" width="1" height="100" />
										{#if bar.sessions}
											<rect x={index + (bars.length > 60 ? 0.1 : 0.18)} y={100 - (bar.sessions / peak) * 100} width={bars.length > 60 ? 0.8 : 0.64} height={(bar.sessions / peak) * 100} />
										{/if}
									</g>
								{/each}
							</svg>
						</div>
						<div class="axis dim">
							<span>{shortDay.format(parseDay(bars[0].date))}</span>
							{#if bars.length > 2}<span>{shortDay.format(parseDay(bars[Math.floor(bars.length / 2)].date))}</span>{/if}
							<span>{bars.length > 1 ? 'Today' : ''}</span>
						</div>
					</figure>
				{/if}

				<div class="section-head breakdown-head">
					<h3>Breakdown</h3>
					<div class="segmented" role="group" aria-label="Break down by">
						<button type="button" aria-pressed={view === 'harness'} onclick={() => (view = 'harness')}>Harness</button>
						<button type="button" aria-pressed={view === 'model'} onclick={() => (view = 'model')}>Model</button>
					</div>
				</div>
				{#if rows.length}
					<div class="breakdown" role="list">
						<div class="breakdown-row labels dim" aria-hidden="true">
							<span>{view === 'harness' ? 'Harness' : 'Model'}</span>
							<span class="num">Sessions</span>
							<span class="num">{view === 'harness' ? 'Share' : 'Of sessions'}</span>
						</div>
						{#each rows as row (row.name)}
							{@const share = row.sessions / activity.sessions}
							<div role="listitem">
								<button
									type="button"
									class={['breakdown-row', isShown(row.name) && 'is-shown']}
									aria-pressed={isShown(row.name)}
									title={isShown(row.name) ? 'Show every session again' : `List these sessions`}
									style={view === 'harness' ? `--ink: var(--ink-${row.name}, var(--muted-foreground))` : undefined}
									onclick={() => show(row.name)}
								>
									<span class="name" translate="no">
										{#if view === 'harness'}<span class="dot" aria-hidden="true"></span>{/if}{row.name}
									</span>
									<span class="num">{number.format(row.sessions)}</span>
									<span class="num">{percent.format(share)}</span>
									<span class="meter" aria-hidden="true"><span style:width="{share * 100}%"></span></span>
								</button>
							</div>
						{/each}
					</div>
				{:else}
					<p class="dim note">No sessions in this range.</p>
				{/if}
				<p class="dim note">
					{#if view === 'harness'}
						Each session counts once, under its harness. Subagents count with the session that started them.
					{:else}
						A session counts under every model it used, so these can add up to more than 100%.
						{#if activity.without_model}{count(activity.without_model, 'session')} recorded no model.{/if}
					{/if}
					Select a row to list its sessions.
				</p>
			{/if}
		</section>
	{/if}

	{#if health.machines.length}
		<section>
			<h2>Machines</h2>
			<ul class="machines">
				{#each health.machines as machine (machine.id)}
					{@const result = lastImportResult(health, machine.id)}
					<li>
						<span class="host" translate="no">{machine.hostname}</span>
						<span class="dim">
							last upload <time datetime={machine.last_import_at} title={dateAndTime(machine.last_import_at)}>{ago(machine.last_import_at)}</time>
							{#if result}· <a class={['result', result.tone]} href={result.href}>{result.text}</a>{/if}
						</span>
						<!-- The session list reads the machine from the address and filters by it. -->
						<a class="action" href={withQuery(resolve('/'), { machine: machine.id })}>Show its sessions</a>
					</li>
				{/each}
			</ul>
			<a class="more" href={resolve('/imports')}>Every import in Archive</a>
		</section>
	{/if}
{/if}

<section>
	<h2>Keys</h2>
	<dl class="keys">
		<div><dt><kbd>/</kbd></dt><dd>Search sessions</dd></div>
		<div><dt><kbd>↓</kbd> <kbd>↑</kbd></dt><dd>Move through the list from the search box</dd></div>
		<div><dt><kbd>Enter</kbd></dt><dd>Search now, or open the first session</dd></div>
		<div><dt><kbd>j</kbd> <kbd>k</kbd></dt><dd>Open the next or previous session</dd></div>
		<div><dt><kbd>[</kbd></dt><dd>Hide or show the session list</dd></div>
	</dl>
</section>

<style>
	section {
		margin-top: 2rem;
	}

	h2 {
		margin: 0 0 0.5rem;
		font-size: 13px;
		font-weight: 600;
	}

	.dim {
		color: var(--muted-foreground);
	}

	.error,
	.bad {
		color: var(--destructive);
	}

	.error {
		margin: 0 0 1rem;
		font-size: 13px;
	}

	.warn {
		color: var(--primary);
	}

	.stats {
		display: grid;
		grid-template-columns: repeat(auto-fit, minmax(9rem, 1fr));
		gap: 0.75rem;
		margin: 1.5rem 0 0;
	}

	.stats div {
		padding: 0.85rem 1rem;
		border: 1px solid var(--border);
		border-radius: 0.6rem;
		background: var(--card);
	}

	.stats dt {
		color: var(--muted-foreground);
		font-size: 12px;
	}

	.stats dd {
		margin: 0.2rem 0 0;
		font-size: 1.5rem;
		font-weight: 600;
		font-variant-numeric: tabular-nums;
		letter-spacing: -0.02em;
	}

	.activity {
		margin-top: 2rem;
	}

	.section-head {
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		justify-content: space-between;
		gap: 0.5rem 1rem;
		margin-bottom: 0.75rem;
	}

	.section-head h2,
	.section-head h3 {
		margin: 0;
		font-size: 13px;
		font-weight: 600;
	}

	.segmented {
		display: inline-flex;
		padding: 2px;
		border: 1px solid var(--border);
		border-radius: var(--radius);
		background: var(--card);
	}

	.segmented button {
		height: 1.6rem;
		padding: 0 0.65rem;
		border: 0;
		border-radius: calc(var(--radius) - 2px);
		background: transparent;
		color: var(--muted-foreground);
		font: inherit;
		font-size: 12px;
		cursor: pointer;
	}

	.segmented button:hover {
		color: var(--foreground);
	}

	.segmented button[aria-pressed='true'] {
		background: var(--accent);
		color: var(--foreground);
		box-shadow: inset 0 0 0 1px var(--input);
	}

	.total {
		display: flex;
		align-items: baseline;
		gap: 0.5rem;
		margin: 0;
		font-size: 13px;
	}

	.total-number {
		font-size: 1.75rem;
		font-weight: 600;
		font-variant-numeric: tabular-nums;
		letter-spacing: -0.02em;
	}

	.chart {
		margin: 1rem 0 0;
		padding: 0.85rem 1rem 0.7rem;
		border: 1px solid var(--border);
		border-radius: 0.6rem;
		background: var(--card);
	}

	.chart figcaption {
		margin-bottom: 0.6rem;
		font-size: 12px;
	}

	.plot {
		position: relative;
		height: 8.5rem;
		margin-left: 2rem;
		border-top: 1px dashed var(--border);
		border-bottom: 1px solid var(--input);
	}

	.plot svg {
		display: block;
		width: 100%;
		height: 100%;
		overflow: visible;
	}

	.tick {
		position: absolute;
		right: calc(100% + 0.5rem);
		color: var(--muted-foreground);
		font-size: 11px;
		font-variant-numeric: tabular-nums;
		line-height: 1;
	}

	.tick.top {
		top: -0.35rem;
	}

	.tick.bottom {
		bottom: -0.35rem;
	}

	.bar rect {
		fill: color-mix(in oklab, var(--foreground) 38%, transparent);
	}

	.bar .hit {
		fill: transparent;
	}

	.bar:hover rect:not(.hit) {
		fill: var(--foreground);
	}

	.bar:hover .hit {
		fill: var(--accent);
	}

	.axis {
		display: flex;
		justify-content: space-between;
		margin: 0.35rem 0 0 2rem;
		font-size: 11px;
	}

	.breakdown-head {
		margin-top: 1.5rem;
	}

	.breakdown {
		border-top: 1px solid var(--border);
		font-size: 13px;
	}

	.breakdown-row {
		display: grid;
		grid-template-columns: minmax(0, 1fr) 5.5rem 5.5rem;
		align-items: center;
		width: 100%;
		padding: 0.55rem 0.5rem 0.45rem;
		border: 0;
		border-bottom: 1px solid var(--border);
		background: transparent;
		color: inherit;
		font: inherit;
		text-align: left;
	}

	.breakdown-row.labels {
		padding-block: 0.4rem;
		font-size: 12px;
	}

	button.breakdown-row {
		border-radius: 0;
		cursor: pointer;
	}

	button.breakdown-row:hover {
		background: var(--accent);
	}

	button.breakdown-row.is-shown {
		background: color-mix(in oklch, var(--primary) 9%, transparent);
		box-shadow: inset 2px 0 0 var(--primary);
	}

	.name {
		display: flex;
		min-width: 0;
		align-items: center;
		gap: 0.5rem;
		overflow: hidden;
		font-weight: 500;
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	.dot {
		width: 7px;
		height: 7px;
		flex-shrink: 0;
		border-radius: 50%;
		background: var(--ink);
	}

	.num {
		font-variant-numeric: tabular-nums;
		text-align: right;
	}

	.meter {
		grid-column: 1 / -1;
		height: 2px;
		margin-top: 0.4rem;
		border-radius: 1px;
		background: var(--border);
	}

	.meter span {
		display: block;
		height: 100%;
		border-radius: inherit;
		background: var(--ink, color-mix(in oklab, var(--foreground) 55%, transparent));
	}

	.note {
		margin: 0.6rem 0 0;
		font-size: 12px;
	}

	.health-state {
		display: flex;
		flex-wrap: wrap;
		gap: 0.25rem 1rem;
		margin: 1rem 0 0;
		font-size: 13px;
	}

	.machines {
		margin: 0;
		padding: 0;
		list-style: none;
		border-top: 1px solid var(--border);
		font-size: 13px;
	}

	.machines li {
		display: flex;
		flex-wrap: wrap;
		align-items: baseline;
		gap: 0.25rem 1rem;
		padding: 0.6rem 0;
		border-bottom: 1px solid var(--border);
	}

	.host {
		font-weight: 500;
	}

	.result {
		text-decoration: none;
	}

	.result.ok {
		color: var(--muted-foreground);
	}

	.result.running {
		color: var(--foreground);
	}

	.result:hover {
		text-decoration: underline;
	}

	.action,
	.more {
		color: var(--muted-foreground);
		font-size: 12px;
		text-decoration: none;
	}

	.action {
		margin-left: auto;
	}

	.more {
		display: inline-block;
		margin-top: 0.6rem;
	}

	.action:hover,
	.more:hover {
		color: var(--foreground);
	}

	.keys {
		display: grid;
		gap: 0.4rem;
		margin: 0;
		font-size: 13px;
	}

	.keys div {
		display: grid;
		grid-template-columns: 6rem 1fr;
		align-items: baseline;
	}

	.keys dd {
		margin: 0;
		color: var(--muted-foreground);
	}

	kbd {
		padding: 0.05rem 0.35rem;
		border-radius: 4px;
		background: color-mix(in oklab, var(--foreground) 8%, transparent);
		font-family: var(--font-sans);
		font-size: 11px;
	}

	.empty {
		margin-top: 1.5rem;
		color: var(--muted-foreground);
		font-size: 13px;
	}

	.empty p {
		margin: 0;
	}

	.empty .empty-title {
		margin-bottom: 0.35rem;
		color: var(--foreground);
		font-size: 15px;
		font-weight: 500;
	}
</style>
