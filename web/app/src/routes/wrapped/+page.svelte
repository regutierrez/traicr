<script lang="ts">
	import { resolve } from '$app/paths';
	import { readJSON } from '$lib/api';
	import { count } from '$lib/format';
	import { transcriptHref, withQuery } from '$lib/links';
	import { localDayBound } from '$lib/sidebar.svelte';
	import type { Wrapped } from '$lib/types';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();
	let wrapped = $state.raw<Wrapped | null>(null);
	let error = $state('');
	let loading = $state(true);

	// The recap reads every Event in the range, which takes a few seconds on a big archive.
	$effect(() => {
		const params = new URLSearchParams({ tz: Intl.DateTimeFormat().resolvedOptions().timeZone });
		if (data.year) {
			params.set('after', localDayBound(`${data.year}-01-01`));
			params.set('before', localDayBound(`${data.year}-12-31`, true));
		}
		let current = true;
		loading = true;
		readJSON<Wrapped>(fetch, `/api/v1/wrapped?${params}`)
			.then((result) => {
				if (!current) return;
				wrapped = result.ok ? result.data : null;
				error = result.ok ? '' : result.message;
			})
			.catch(() => current && (error = 'Could not load your recap'))
			.finally(() => current && (loading = false));
		return () => (current = false);
	});

	let label = $derived(data.year ? String(data.year) : 'All time');
	let years = $derived([data.thisYear, data.thisYear - 1, 0]);
	let number = new Intl.NumberFormat();
	let oneDecimal = new Intl.NumberFormat(undefined, { maximumFractionDigits: 1 });
	let percent = new Intl.NumberFormat(undefined, { style: 'percent', maximumFractionDigits: 0 });
	let longDay = new Intl.DateTimeFormat(undefined, { weekday: 'short', month: 'short', day: 'numeric' });
	let shortDate = new Intl.DateTimeFormat(undefined, { month: 'short', day: 'numeric' });
	let clockTime = new Intl.DateTimeFormat(undefined, { hour: 'numeric', minute: '2-digit' });
	const weekdays = ['Sunday', 'Monday', 'Tuesday', 'Wednesday', 'Thursday', 'Friday', 'Saturday'];

	/** A shade between the page background and its text, as T3 Code colors share bars. */
	const ink = (share: number) => `color-mix(in oklab, var(--foreground) ${share}%, var(--background))`;

	function hourLabel(hour: number) {
		return `${hour % 12 || 12} ${hour < 12 ? 'AM' : 'PM'}`;
	}

	function sum(values: number[]) {
		return values.reduce((total, value) => total + value, 0);
	}

	// Parts of the day, each a run of local hours. Together they cover all 24.
	const dayParts = [
		{ name: 'Morning', from: 5, to: 12 },
		{ name: 'Afternoon', from: 12, to: 17 },
		{ name: 'Evening', from: 17, to: 22 },
		{ name: 'Night', from: 22, to: 5 }
	];

	let parts = $derived.by(() => {
		if (!wrapped) return [];
		const { hours, prompts: total } = wrapped;
		return dayParts.map((part) => {
			let prompts = 0;
			for (let hour = part.from; hour !== part.to; hour = (hour + 1) % 24) prompts += hours[hour];
			return { ...part, prompts, share: total ? prompts / total : 0 };
		});
	});

	// The persona reads the whole clock, not only the peak: a quarter of prompts after
	// 10 PM makes an owl even when most work happens in the afternoon.
	let persona = $derived.by(() => {
		if (!wrapped?.prompts) return '';
		const peak = wrapped.hours.indexOf(Math.max(...wrapped.hours));
		const [morning, , , night] = parts;
		const name =
			night.share >= 0.25 ? 'Night owl' : morning.share >= 0.5 ? 'Early bird' : peak < 12 ? 'Morning shipper' : peak < 17 ? 'Afternoon grinder' : 'After-hours hacker';
		return `${name} · peaks at ${hourLabel(peak)} · favorite day ${weekdays[wrapped.weekdays.indexOf(Math.max(...wrapped.weekdays))]}`;
	});

	// Chart geometry after T3 Code's usage chart: a fixed view box stretched to the plot.
	const VIEW_WIDTH = 960;
	const VIEW_HEIGHT = 260;
	const PLOT_TOP = 8;

	function niceScale(peak: number, ticks: number) {
		if (peak <= 0) return { max: 0, ticks: [0] };
		const raw = peak / ticks;
		const magnitude = 10 ** Math.floor(Math.log10(raw));
		const normalized = raw / magnitude;
		const step = (normalized > 5 ? 10 : normalized > 2 ? 5 : normalized > 1 ? 2 : 1) * magnitude;
		const max = Math.ceil(peak / step) * step;
		const values: number[] = [];
		for (let value = 0; value <= max + step * 1e-6; value += step) values.push(value);
		return { max, ticks: values };
	}

	/** A monotone cubic through the points, so the line never swings above or below the data. */
	function smoothPath(points: { x: number; y: number }[]) {
		if (points.length < 2) return '';
		const slopes = points.slice(1).map((point, index) => {
			const dx = point.x - points[index].x;
			return dx === 0 ? 0 : (point.y - points[index].y) / dx;
		});
		const tangents = points.map((_, index) => {
			if (index === 0) return slopes[0];
			if (index === points.length - 1) return slopes[slopes.length - 1];
			const [before, after] = [slopes[index - 1], slopes[index]];
			return before * after <= 0 ? 0 : (before + after) / 2;
		});
		slopes.forEach((slope, index) => {
			if (slope === 0) {
				tangents[index] = tangents[index + 1] = 0;
				return;
			}
			const a = tangents[index] / slope;
			const b = tangents[index + 1] / slope;
			const magnitude = a * a + b * b;
			if (magnitude > 9) {
				const scale = 3 / Math.sqrt(magnitude);
				tangents[index] = scale * a * slope;
				tangents[index + 1] = scale * b * slope;
			}
		});
		let path = `M${points[0].x},${points[0].y.toFixed(2)}`;
		for (let index = 0; index < points.length - 1; index++) {
			const [from, to] = [points[index], points[index + 1]];
			const dx = to.x - from.x;
			path += ` C${from.x + dx / 3},${(from.y + (tangents[index] * dx) / 3).toFixed(2)} ${to.x - dx / 3},${(to.y - (tangents[index + 1] * dx) / 3).toFixed(2)} ${to.x},${to.y.toFixed(2)}`;
		}
		return path;
	}

	let chart = $derived.by(() => {
		if (!wrapped) return null;
		const scale = niceScale(Math.max(...wrapped.hours), 4);
		const toY = (value: number) => (scale.max === 0 ? VIEW_HEIGHT : VIEW_HEIGHT - (value / scale.max) * (VIEW_HEIGHT - PLOT_TOP));
		const step = VIEW_WIDTH / 23;
		const line = smoothPath(wrapped.hours.map((value, hour) => ({ x: hour * step, y: toY(value) })));
		return { scale, toY, step, line, area: `${line} L${VIEW_WIDTH},${VIEW_HEIGHT} L0,${VIEW_HEIGHT} Z` };
	});

	let hover = $state<{ hour: number; x: number; y: number } | null>(null);
	let plotWidth = $state(0);

	function trackHover(event: MouseEvent & { currentTarget: HTMLDivElement }) {
		const bounds = event.currentTarget.getBoundingClientRect();
		const x = Math.min(bounds.width, Math.max(0, event.clientX - bounds.left));
		hover = { hour: Math.round((x / bounds.width) * 23), x, y: Math.max(0, event.clientY - bounds.top) };
	}

	let toolSegments = $derived.by(() => {
		if (!wrapped?.tool_calls) return [];
		const top = wrapped.tools.slice(0, 4);
		const shades = [100, 72, 52, 36];
		const segments = top.map((tool, index) => ({ label: tool.name, value: tool.count, color: ink(shades[index]) }));
		const other = wrapped.tool_calls - sum(top.map((tool) => tool.count));
		if (other > 0) segments.push({ label: 'Other', value: other, color: ink(22) });
		return segments;
	});

	let politeSegments = $derived(
		wrapped?.prompts
			? [
					{ label: 'Said please or thanks', value: wrapped.polite_prompts, color: ink(100) },
					{ label: 'Got straight to it', value: wrapped.prompts - wrapped.polite_prompts, color: ink(30) }
				]
			: []
	);

	type Breakdown = 'tools' | 'files' | 'models' | 'commands';
	let breakdown = $state<Breakdown>('tools');
	const breakdowns: { key: Breakdown; label: string; column: string }[] = [
		{ key: 'tools', label: 'Tools', column: 'Calls' },
		{ key: 'files', label: 'Files', column: 'Opens' },
		{ key: 'models', label: 'Models', column: 'Sessions' },
		{ key: 'commands', label: 'Commands', column: 'Runs' }
	];
	// `of` is the total a row's share is out of, or 0 when a share means nothing, as for files.
	let rows = $derived.by(() => {
		if (!wrapped) return [];
		const { sessions, tool_calls } = wrapped;
		if (breakdown === 'models') return wrapped.models.map((model) => ({ name: model.name, value: model.sessions, of: sessions }));
		const list = breakdown === 'tools' ? wrapped.tools : breakdown === 'files' ? wrapped.files : wrapped.commands;
		return list.map((row) => ({ name: row.name, value: row.count, of: breakdown === 'tools' ? tool_calls : 0 }));
	});

	function fileName(path: string) {
		const parts = path.split('/').filter(Boolean);
		return { base: parts.at(-1) ?? path, parent: parts.length > 1 ? parts.slice(-3, -1).join('/') : '' };
	}

	function firstLine(text: string, limit = 64) {
		const line = text.split('\n')[0];
		return line.length > limit ? `${line.slice(0, limit - 1)}…` : line;
	}

	function duration(minutes: number) {
		return minutes < 60 ? `${Math.max(1, Math.round(minutes))} min` : `${oneDecimal.format(minutes / 60)} h`;
	}

	function parseDay(key: string) {
		const [year, month, day] = key.split('-').map(Number);
		return new Date(year, month - 1, day);
	}
</script>

{#snippet shareBar(title: string, segments: { label: string; value: number; color: string }[])}
	{@const total = sum(segments.map((segment) => segment.value))}
	<div class="share">
		<h3>{title}</h3>
		<div class="share-bar" role="img" aria-label="{title}: {segments.map((segment) => `${segment.label} ${number.format(segment.value)}`).join(', ')}">
			{#each segments.filter((segment) => segment.value > 0) as segment (segment.label)}
				<span style:flex="{segment.value} 1 0" style:background={segment.color} title="{segment.label} · {number.format(segment.value)} · {percent.format(segment.value / total)}"></span>
			{/each}
		</div>
		<div class="legend">
			{#each segments as segment (segment.label)}
				<span><i style:background={segment.color} aria-hidden="true"></i><span class="dim" translate="no">{segment.label}</span> <span class="num">{number.format(segment.value)}</span></span>
			{/each}
		</div>
	</div>
{/snippet}

<svelte:head>
	<title>Wrapped {label} · Traicr</title>
</svelte:head>

<header class="page-head head">
	<div>
		<h1>Wrapped</h1>
		<p class="page-lede">Your {data.year ? data.year : 'all-time'} in code: when you prompt, what your agents reach for, and the sessions that got away.</p>
	</div>
	<nav class="segmented" aria-label="Year">
		{#each years as year (year)}
			<a href={withQuery(resolve('/wrapped'), { year: year ? String(year) : 'all' })} aria-current={data.year === year ? 'page' : undefined}>{year || 'All time'}</a>
		{/each}
	</nav>
</header>

{#if loading}
	<p class="state dim" role="status">Counting every prompt and tool call…</p>
{:else if error}
	<p class="error" role="alert">{error}</p>
{:else if !wrapped || !wrapped.sessions}
	<p class="state dim">No sessions in {label === 'All time' ? 'the archive' : label} yet.</p>
{:else}
	<div class="page">
		<section class="overview">
			<div class="summary">
				<div>
					<p class="headline">{number.format(wrapped.prompts)}</p>
					<p class="caption dim">prompts you typed · {count(wrapped.sessions, 'session')}</p>
				</div>
				{#each parts as part (part.name)}
					<div class="part">
						<div class="part-line">
							<span>{part.name}</span>
							<span class="num strong">{number.format(part.prompts)}</span>
						</div>
						<span class="caption dim">{percent.format(part.share)} of prompts · {hourLabel(part.from)} to {hourLabel(part.to)}</span>
					</div>
				{/each}
			</div>

			{#if chart}
				<div class="trend">
					<div class="trend-head">
						<h2>Prompts by hour</h2>
						<span class="caption dim">{persona}</span>
					</div>
					<div class="chart">
						<div class="y-axis" aria-hidden="true">
							{#each chart.scale.ticks as tick (tick)}
								<span style:top="{(chart.toY(tick) / VIEW_HEIGHT) * 100}%">{number.format(tick)}</span>
							{/each}
						</div>
						<div class="plot" role="presentation" bind:clientWidth={plotWidth} onmousemove={trackHover} onmouseleave={() => (hover = null)}>
							<svg viewBox="0 0 {VIEW_WIDTH} {VIEW_HEIGHT}" preserveAspectRatio="none" role="img" aria-label="Prompts by local hour of day">
								{#each chart.scale.ticks as tick (tick)}
									<line class="grid" x1="0" x2={VIEW_WIDTH} y1={chart.toY(tick)} y2={chart.toY(tick)} vector-effect="non-scaling-stroke" />
								{/each}
								<path class="area" d={chart.area} />
								<path class="line" d={chart.line} vector-effect="non-scaling-stroke" />
								{#if hover}
									<line class="cursor" x1={hover.hour * chart.step} x2={hover.hour * chart.step} y1={PLOT_TOP} y2={VIEW_HEIGHT} vector-effect="non-scaling-stroke" />
								{/if}
							</svg>
							{#if hover}
								<div class="tooltip" style:left="{hover.x + 140 > plotWidth ? hover.x - 132 : hover.x + 12}px" style:top="{Math.min(hover.y + 12, 150)}px">
									<div class="dim">{hourLabel(hover.hour)}</div>
									<div class="tooltip-row"><span class="dim">Prompts</span><span class="num">{number.format(wrapped.hours[hover.hour])}</span></div>
								</div>
							{/if}
						</div>
					</div>
					<div class="x-axis dim"><span>12 AM</span><span>12 PM</span><span>11 PM</span></div>
				</div>
			{/if}
		</section>

		<section>
			<h2>Totals</h2>
			<dl class="totals">
				<div><dt>Tool calls</dt><dd>{number.format(wrapped.tool_calls)}</dd></div>
				<div><dt>Calls per prompt</dt><dd>{oneDecimal.format(wrapped.prompts ? wrapped.tool_calls / wrapped.prompts : 0)}</dd></div>
				<div><dt>Agent hours</dt><dd>{oneDecimal.format(wrapped.active_hours)}</dd></div>
				<div><dt>Days with a prompt</dt><dd>{number.format(wrapped.active_days)}</dd></div>
				<div><dt>Longest streak</dt><dd>{count(wrapped.longest_streak, 'day')}</dd></div>
			</dl>
		</section>

		<section class="shares">
			{#if toolSegments.length}{@render shareBar('Tool calls by tool', toolSegments)}{/if}
			{#if politeSegments.length}{@render shareBar('Manners', politeSegments)}{/if}
		</section>

		<section>
			<div class="section-head">
				<h2>Breakdown</h2>
				<div class="segmented" role="group" aria-label="Break down by">
					{#each breakdowns as option (option.key)}
						<button type="button" aria-pressed={breakdown === option.key} onclick={() => (breakdown = option.key)}>{option.label}</button>
					{/each}
				</div>
			</div>
			<table class="table">
				<thead>
					<tr>
						<th class="rank">#</th>
						<th class="name">{breakdowns.find((option) => option.key === breakdown)?.label.slice(0, -1)}</th>
						<th>{breakdowns.find((option) => option.key === breakdown)?.column}</th>
						{#if rows[0]?.of}<th class="share-col">Share</th>{/if}
					</tr>
				</thead>
				<tbody>
					{#each rows as row, index (row.name)}
						<tr>
							<td class="rank">{index + 1}</td>
							<td class="name" title={row.name}>
								{#if breakdown === 'files'}
									{@const file = fileName(row.name)}
									<span class="mono" translate="no">{file.base}</span>{#if file.parent}<span class="dim mono parent">{file.parent}</span>{/if}
								{:else}
									<span class={breakdown === 'commands' ? 'mono' : ''} translate="no">{breakdown === 'commands' ? firstLine(row.name, 90) : row.name}</span>
								{/if}
								<div class="row-bar" aria-hidden="true"><span style:width="max(0.5rem, {(row.value / rows[0].value) * 100}%)"></span></div>
							</td>
							<td class="strong">{number.format(row.value)}</td>
							{#if row.of}<td class="share-col">{percent.format(row.value / row.of)}</td>{/if}
						</tr>
					{:else}
						<tr><td colspan="3" class="empty-row dim">Nothing recorded.</td></tr>
					{/each}
				</tbody>
			</table>
			{#if breakdown === 'models'}<p class="caption dim note">A session counts under every model it used, so shares can add up to more than 100%.</p>{/if}
		</section>

		<section>
			<h2>Highlights</h2>
			<table class="table highlights">
				<tbody>
					{#if wrapped.rabbit_hole}
						{@const hole = wrapped.rabbit_hole}
						<tr>
							<th>Longest rabbit hole</th>
							<td class="strong">{number.format(hole.tool_calls)} calls · {duration(hole.minutes)}</td>
							<td class="detail">One prompt, then the agent kept digging: <a href={transcriptHref(hole.trace_id)}>"{firstLine(hole.prompt)}"</a></td>
						</tr>
					{/if}
					{#if wrapped.marathon}
						<tr>
							<th>Marathon session</th>
							<td class="strong">{oneDecimal.format(wrapped.marathon.active_hours)} h</td>
							<td class="detail"><a href={transcriptHref(wrapped.marathon.trace_id)}>{wrapped.marathon.title || 'Untitled session'}</a>. Hydrate.</td>
						</tr>
					{/if}
					{#if wrapped.busiest_day}
						<tr>
							<th>Busiest day</th>
							<td class="strong">{number.format(wrapped.busiest_day.prompts)} prompts</td>
							<td class="detail">{longDay.format(parseDay(wrapped.busiest_day.date))}. What happened there?</td>
						</tr>
					{/if}
					{#if wrapped.latest_prompt}
						<tr>
							<th>Up past bedtime</th>
							<td class="strong">{clockTime.format(new Date(wrapped.latest_prompt.at))}</td>
							<td class="detail">{shortDate.format(new Date(wrapped.latest_prompt.at))}: <a href={transcriptHref(wrapped.latest_prompt.trace_id)}>"{firstLine(wrapped.latest_prompt.text)}"</a></td>
						</tr>
					{/if}
					{#if wrapped.shortest_prompt}
						<tr>
							<th>Fewest words</th>
							<td class="strong">{count(wrapped.shortest_prompt.chars, 'character')}</td>
							<td class="detail"><a class="mono" href={transcriptHref(wrapped.shortest_prompt.trace_id)}>"{firstLine(wrapped.shortest_prompt.text)}"</a>. Truly a person of few words.</td>
						</tr>
					{/if}
					{#if wrapped.longest_prompt}
						<tr>
							<th>The novelist</th>
							<td class="strong">{count(wrapped.longest_prompt.chars, 'character')}</td>
							<td class="detail">Your longest prompt. <a href={transcriptHref(wrapped.longest_prompt.trace_id)}>Read the saga</a></td>
						</tr>
					{/if}
					{#if wrapped.commands[0]}
						<tr>
							<th>On repeat</th>
							<td class="strong">{number.format(wrapped.commands[0].count)} runs</td>
							<td class="detail"><code title={wrapped.commands[0].name}>{firstLine(wrapped.commands[0].name, 56)}</code></td>
						</tr>
					{/if}
				</tbody>
			</table>
		</section>

		<p class="caption dim">
			Times are in your time zone. Each event counts once, however many times it was collected, and subagents count with the session that started them.
			Prompts are messages you typed; messages from other threads, automations, and plugins are left out.
		</p>
	</div>
{/if}

<style>
	.head {
		display: flex;
		flex-wrap: wrap;
		align-items: flex-end;
		justify-content: space-between;
		gap: 1rem;
	}

	.page {
		display: flex;
		flex-direction: column;
		gap: 2rem;
	}

	h2,
	h3 {
		margin: 0;
		color: var(--foreground);
		font-size: 13px;
		font-weight: 500;
	}

	section > h2 {
		margin-bottom: 0.6rem;
	}

	.dim {
		color: var(--muted-foreground);
	}

	.num,
	.strong,
	dd,
	td {
		font-variant-numeric: tabular-nums;
	}

	.strong {
		color: var(--foreground);
		font-weight: 500;
	}

	.mono,
	code {
		font-family: var(--font-mono);
		font-size: 12px;
	}

	.caption {
		margin: 0;
		font-size: 12px;
	}

	.note {
		margin-top: 0.5rem;
	}

	.state {
		padding: 3rem 0;
		font-size: 13px;
		text-align: center;
	}

	.error {
		color: var(--destructive);
		font-size: 13px;
	}

	.segmented {
		display: inline-flex;
		padding: 2px;
		border: 1px solid var(--border);
		border-radius: var(--radius);
		background: var(--card);
	}

	.segmented a,
	.segmented button {
		display: inline-flex;
		align-items: center;
		height: 1.6rem;
		padding: 0 0.65rem;
		border: 0;
		border-radius: calc(var(--radius) - 2px);
		background: transparent;
		color: var(--muted-foreground);
		font: inherit;
		font-size: 12px;
		text-decoration: none;
		cursor: pointer;
	}

	.segmented a:hover,
	.segmented button:hover {
		color: var(--foreground);
	}

	.segmented a[aria-current='page'],
	.segmented button[aria-pressed='true'] {
		background: var(--accent);
		color: var(--foreground);
		box-shadow: inset 0 0 0 1px var(--input);
	}

	/* Headline and its parts on the left, the hourly trend on the right, as T3 Code's usage overview. */
	.overview {
		display: grid;
		gap: 1.5rem;
	}

	@media (min-width: 1024px) {
		.overview {
			grid-template-columns: minmax(0, 18rem) minmax(0, 1fr);
		}
	}

	.summary {
		display: flex;
		flex-direction: column;
		gap: 1.1rem;
		min-width: 0;
	}

	.headline {
		margin: 0 0 0.25rem;
		color: var(--foreground);
		font-size: 2.25rem;
		font-weight: 600;
		line-height: 1.1;
		font-variant-numeric: tabular-nums;
	}

	.part {
		display: flex;
		flex-direction: column;
		gap: 0.2rem;
	}

	.part-line {
		display: flex;
		align-items: baseline;
		justify-content: space-between;
		gap: 1rem;
		font-size: 14px;
	}

	.trend {
		display: flex;
		flex-direction: column;
		gap: 0.75rem;
		min-width: 0;
	}

	.trend-head {
		display: flex;
		flex-wrap: wrap;
		align-items: baseline;
		justify-content: space-between;
		gap: 0.25rem 1rem;
	}

	.chart {
		display: flex;
		gap: 0.5rem;
	}

	.y-axis {
		position: relative;
		flex: 0 0 3.5rem;
		height: 14rem;
	}

	.y-axis span {
		position: absolute;
		right: 0;
		transform: translateY(-50%);
		color: var(--muted-foreground);
		font-size: 10px;
		font-variant-numeric: tabular-nums;
	}

	.plot {
		position: relative;
		flex: 1;
		height: 14rem;
	}

	.plot svg {
		display: block;
		width: 100%;
		height: 100%;
		overflow: visible;
	}

	.grid {
		stroke: var(--border);
		stroke-width: 1;
	}

	.area {
		fill: var(--foreground);
		fill-opacity: 0.12;
	}

	.line {
		fill: none;
		stroke: var(--foreground);
		stroke-width: 2;
	}

	.cursor {
		stroke: var(--muted-foreground);
		stroke-width: 1;
	}

	.tooltip {
		position: absolute;
		z-index: 1;
		min-width: 8rem;
		padding: 0.45rem 0.6rem;
		border: 1px solid var(--border);
		border-radius: 0.75rem;
		background: color-mix(in srgb, var(--popover) 88%, transparent);
		box-shadow: 0 8px 24px rgb(0 0 0 / 35%);
		backdrop-filter: blur(12px);
		font-size: 12px;
		pointer-events: none;
	}

	.tooltip-row {
		display: flex;
		justify-content: space-between;
		gap: 0.75rem;
		margin-top: 0.2rem;
		font-variant-numeric: tabular-nums;
	}

	.x-axis {
		display: flex;
		justify-content: space-between;
		padding-left: 4rem;
		font-size: 10px;
		text-transform: uppercase;
	}

	.totals {
		display: grid;
		grid-template-columns: repeat(2, minmax(0, 1fr));
		gap: 1rem 1.5rem;
		margin: 0;
	}

	@media (min-width: 768px) {
		.totals {
			grid-template-columns: repeat(5, minmax(0, 1fr));
		}
	}

	.totals dt {
		color: var(--muted-foreground);
		font-size: 12px;
	}

	.totals dd {
		margin: 0.15rem 0 0;
		color: var(--foreground);
		font-size: 1rem;
		font-weight: 500;
	}

	.shares {
		display: grid;
		gap: 2rem 3rem;
	}

	@media (min-width: 1024px) {
		.shares {
			grid-template-columns: repeat(2, minmax(0, 1fr));
		}
	}

	.share {
		display: flex;
		flex-direction: column;
		gap: 0.6rem;
		min-width: 0;
	}

	.share-bar {
		display: flex;
		gap: 2px;
		height: 0.5rem;
	}

	.share-bar span {
		min-width: 0.25rem;
		border-radius: 2px;
	}

	.share-bar span:first-child {
		border-radius: 999px 2px 2px 999px;
	}

	.share-bar span:last-child {
		border-radius: 2px 999px 999px 2px;
	}

	.legend {
		display: flex;
		flex-wrap: wrap;
		gap: 0.25rem 1rem;
		font-size: 12px;
	}

	.legend > span {
		display: inline-flex;
		align-items: center;
		gap: 0.35rem;
	}

	.legend i {
		width: 0.5rem;
		height: 0.5rem;
		border-radius: 2px;
	}

	.section-head {
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		justify-content: space-between;
		gap: 0.5rem 1rem;
		margin-bottom: 0.5rem;
	}

	.table {
		width: 100%;
		border-collapse: collapse;
		font-size: 13px;
	}

	.table thead tr {
		border-bottom: 1px solid var(--border);
	}

	.table th {
		padding: 0.5rem 0 0.5rem 1.5rem;
		color: var(--muted-foreground);
		font-size: 12px;
		font-weight: 400;
		text-align: right;
		white-space: nowrap;
	}

	.table tbody tr {
		border-bottom: 1px solid color-mix(in srgb, var(--border) 50%, transparent);
		transition: background-color 120ms;
	}

	.table tbody tr:hover {
		background: color-mix(in srgb, var(--muted) 50%, transparent);
	}

	.table td {
		padding: 0.6rem 0 0.6rem 1.5rem;
		color: var(--muted-foreground);
		text-align: right;
		white-space: nowrap;
	}

	.table .rank {
		width: 1%;
		padding-right: 0.75rem;
		padding-left: 0.5rem;
		font-size: 12px;
		text-align: left;
	}

	.table .name {
		width: 100%;
		padding-left: 0;
		color: var(--foreground);
		text-align: left;
		white-space: normal;
		overflow-wrap: anywhere;
	}

	.parent {
		margin-left: 0.5rem;
		font-size: 11px;
	}

	.row-bar {
		max-width: 12rem;
		height: 2px;
		margin-top: 0.4rem;
	}

	.row-bar span {
		display: block;
		height: 100%;
		border-radius: 999px;
		background: var(--foreground);
		opacity: 0.55;
	}

	.share-col {
		display: none;
	}

	@media (min-width: 640px) {
		.share-col {
			display: table-cell;
		}
	}

	.empty-row {
		padding: 1.5rem 0;
		text-align: center;
	}

	.highlights th {
		width: 1%;
		padding: 0.6rem 1.5rem 0.6rem 0.5rem;
		font-size: 13px;
		text-align: left;
	}

	.highlights td.strong {
		width: 1%;
		padding-left: 0;
		text-align: left;
	}

	.highlights .detail {
		text-align: left;
		white-space: normal;
		overflow-wrap: anywhere;
	}

	.highlights a {
		color: var(--foreground);
		text-decoration: underline;
		text-decoration-color: var(--input);
		text-underline-offset: 3px;
	}

	.highlights a:hover {
		text-decoration-color: currentColor;
	}

	@media (max-width: 639px) {
		.highlights tr {
			display: grid;
			grid-template-columns: 1fr auto;
			padding: 0.5rem 0;
		}

		.highlights th,
		.highlights td.strong,
		.highlights .detail {
			width: auto;
			padding: 0.15rem 0.5rem;
		}

		.highlights td.strong {
			text-align: right;
		}

		.highlights .detail {
			grid-column: 1 / -1;
		}
	}
</style>
