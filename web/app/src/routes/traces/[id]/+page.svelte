<script lang="ts">
	import { resolve } from '$app/paths';
	import WorkspaceHeader from '$lib/components/conversation/WorkspaceHeader.svelte';
	import Icon from '$lib/components/conversation/Icon.svelte';
	import MiddleTruncate from '$lib/components/MiddleTruncate.svelte';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { clockTime, dateAndTime, shortRepository } from '$lib/format';
	import { submitGoForm } from '$lib/forms';
	import { revisionSourcesHref, sourceFileHref, transcriptHref, withQuery } from '$lib/links';
	import { isHousekeepingTool, toolChipParts, toolKindIcon, type ChipIcon } from '$lib/transcript/tools';
	import { getPath, graphLeaves, treeLabel } from '$lib/transcript/tree';
	import {
		activeTime,
		displayModel,
		pathSessionFacts,
		promptOutline,
		streamCounts,
		streamRows,
		timeSpan,
		toolNameCounts,
		toolResults,
		usageTotals,
		type StreamFilter
	} from '$lib/transcript/turns';
	import type { SessionNative } from '$lib/transcript/types';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();
	let trace = $derived(data.trace);
	let session = $derived(data.session);
	let title = $derived(trace?.title || trace?.native_trace_id || 'Trace');

	// Overview summarizes the path the Trace tab opens on: the newest branch.
	let path = $derived(session ? getPath(session.entries, session.leafId || session.entries.at(-1)?.id || '') : []);
	let counts = $derived(streamCounts(path, session?.entries ?? []));
	let facts = $derived(pathSessionFacts(path));
	let models = $derived([
		...new Set(
			path.flatMap((entry) =>
				entry.message?.role === 'assistant' && entry.message.model ? [displayModel(entry.message.provider, entry.message.model)] : []
			)
		)
	]);
	let span = $derived(timeSpan(session?.entries ?? []));
	let active = $derived(activeTime(path));
	let results = $derived(toolResults(path));
	let tools = $derived(toolNameCounts(path, results));
	// Each failed tool call, linked to the message that made it.
	let failures = $derived(
		path.flatMap((entry) =>
			(entry.message?.content ?? []).flatMap((block) => {
				const result = block.type === 'toolCall' ? results.get(block.id) : undefined;
				if (block.type !== 'toolCall' || !result?.message?.isError) return [];
				return [{ id: block.id, entryId: entry.id, chip: toolChipParts(block), exitCode: result.message.run?.result?.exitCode }];
			})
		)
	);
	let toolErrors = $derived(tools.reduce((total, tool) => total + tool.errors, 0));
	// Status lines, tool search, and prompt loading manage the session, not the work; Trace folds
	// them away, so here they share one muted row at the bottom instead of ranking among real tools.
	let workTools = $derived(tools.filter((tool) => !isHousekeepingTool(tool.name)));
	let housekeeping = $derived.by(() => {
		const items = tools.filter((tool) => isHousekeepingTool(tool.name));
		if (!items.length) return null;
		return {
			names: items.map((tool) => `${tool.name} ${tool.count}`).join(', '),
			count: items.reduce((total, tool) => total + tool.count, 0),
			errors: items.reduce((total, tool) => total + tool.errors, 0)
		};
	});
	let toolScale = $derived(Math.max(workTools[0]?.count ?? 0, housekeeping?.count ?? 0));
	// Every call costs tokens, including ones on branches the Trace tab does not show.
	let usage = $derived(usageTotals(session?.entries ?? []));
	let leaves = $derived(session ? graphLeaves(session.entries).map((entry) => ({ id: entry.id, label: treeLabel(entry) })) : []);
	let ampFacts = $derived(ampSessionFacts(session?.header.native ?? null));
	let outline = $derived(promptOutline(streamRows(path)));
	// The newest reply that says something; a turn can end on tool calls alone.
	let lastReply = $derived(outline.findLast((item) => item.reply));
	const PROMPTS_SHOWN = 8;
	let allPrompts = $state(false);
	let shownPrompts = $derived(allPrompts ? outline : outline.slice(0, PROMPTS_SHOWN));
	const promptDay = new Intl.DateTimeFormat(undefined, { month: 'short', day: 'numeric' });
	// A prompt shows its day ("Oct 30") only when the day changes, so a multi-day session reads as a timeline.
	function promptWhen(index: number) {
		const at = shownPrompts[index].at;
		if (!at) return '';
		const previous = shownPrompts[index - 1]?.at;
		const sameDay = previous && new Date(previous).toDateString() === new Date(at).toDateString();
		return sameDay ? clockTime(at) : `${promptDay.format(new Date(at))}, ${clockTime(at)}`;
	}

	const number = new Intl.NumberFormat();
	const parseStatus: Record<string, string> = {
		normalized: 'parsed OK',
		partially_parsed: 'partly parsed',
		unsupported: 'format not supported',
		failed: 'could not be parsed'
	};
	const compact = new Intl.NumberFormat(undefined, { notation: 'compact', maximumFractionDigits: 1 });

	function traceHref(params: Record<string, string> = {}) {
		const query = new URLSearchParams(params);
		if (data.revision && !query.has('revision')) query.set('revision', data.revision);
		return withQuery(resolve('/traces/[id]/trace', { id: String(trace?.id) }), query);
	}

	const activity: { filter: StreamFilter; label: string; icon: ChipIcon; flip?: boolean; count: () => number }[] = [
		{ filter: 'prompts', label: 'Prompts', icon: 'prompt', flip: true, count: () => counts.prompts },
		{ filter: 'responses', label: 'Replies', icon: 'response', count: () => counts.responses },
		{ filter: 'thinking', label: 'Thoughts', icon: 'brain', count: () => counts.thinking },
		{ filter: 'tools', label: 'Tool calls', icon: 'wrench', count: () => counts.tools },
		{ filter: 'compaction', label: 'Compactions', icon: 'compaction', count: () => counts.compaction }
	];

	// What the Amp export says about where and how the thread ran. Missing values are left out.
	function ampSessionFacts(native: SessionNative | null) {
		if (!native) return [];
		const record = (value: unknown) => (value && typeof value === 'object' ? (value as Record<string, unknown>) : {});
		const text = (value: unknown) => (typeof value === 'string' || typeof value === 'number' ? String(value) : '');
		const list = (value: unknown) => (Array.isArray(value) ? value.map((item) => text(item) || text(record(item).name)).filter(Boolean).join(', ') : '');
		const initial = record(record(native.env).initial);
		const platform = record(initial.platform);
		const meta = record(native.meta);
		const tree = record(Array.isArray(initial.trees) ? initial.trees[0] : undefined);
		const repository = record(tree.repository);
		const facts: [string, string][] = [
			['Agent mode', text(native.agentMode)],
			['Branch', text(repository.ref).replace(/^refs\/heads\//, '')],
			['Commit', text(repository.sha).slice(0, 12)],
			['Client', [text(platform.client), text(platform.clientVersion), text(platform.os)].filter(Boolean).join(' · ')],
			['Executor', [text(meta.executorType), text(initial.runnerID) || text(initial.hostname)].filter(Boolean).join(' · ')],
			['Skills', list(native.activatedSkills)],
			['Archived in Amp', native.archived === undefined ? '' : native.archived ? 'yes' : 'no']
		];
		return facts.filter(([, value]) => value);
	}

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

<svelte:head><title>{title} · Traicr</title></svelte:head>

{#if data.error || !trace}
	<p class="status error" role="alert">{data.error || 'Trace not found'}</p>
{:else}
	<WorkspaceHeader {trace} revision={data.revision} tab="overview" />
	<div class="scroll">
		<div class="overview">
			{#if data.loadError}
				<p class="status error" role="alert">{data.loadError}</p>
			{/if}

			{#if outline.length}
				<!-- What the session was about before any facts: the first ask and the last reply, each opening Trace at that message. -->
				<section class="story">
					<a class="story-part" href={traceHref({ targetId: outline[0].id })}>
						<span class="story-label">Ask</span>
						<span class="story-text">{outline[0].prompt || 'Prompt without text'}</span>
					</a>
					{#if lastReply}
						<a class="story-part" href={traceHref({ targetId: lastReply.replyId })}>
							<span class="story-label">Last reply</span>
							<span class="story-text">{lastReply.reply}</span>
						</a>
					{/if}
				</section>
			{/if}

			<section>
				<h2>Session</h2>
				<dl>
					<dt>Harness</dt><dd translate="no">{trace.harness}</dd>
					{#if models.length}
						<dt>{models.length > 1 ? 'Models' : 'Model'}</dt><dd translate="no">{models.join(', ')}</dd>
					{/if}
					{#if facts.thinkingLevel}<dt>Thinking level</dt><dd>{facts.thinkingLevel}</dd>{/if}
					{#if span}
						<dt>Started</dt><dd><time datetime={span.start}>{dateAndTime(span.start)}</time></dd>
						<dt>Last activity</dt><dd><time datetime={span.end}>{dateAndTime(span.end)}</time></dd>
						{#if active}<dt>Active time</dt><dd>{active} <span class="dim">· idle time between prompts left out</span></dd>{/if}
					{/if}
					{#if trace.working_directory}
						<dt>Directory</dt><dd translate="no"><MiddleTruncate text={trace.working_directory.replace(/^file:\/\//, '')} /></dd>
					{/if}
					{#if trace.repository}
						<dt>Repository</dt><dd translate="no"><MiddleTruncate text={shortRepository(trace.repository)} /></dd>
					{/if}
					{#each ampFacts as [label, value] (label)}
						<dt>{label}</dt><dd>{#if label === 'Branch'}<MiddleTruncate text={value} />{:else}{value}{/if}</dd>
					{/each}
				</dl>
			</section>

			<section>
				<h2>Activity</h2>
				{#if session}
					<ul class="stats">
						{#each activity as item (item.filter)}
							<li>
								<a href={traceHref({ filter: item.filter })} class={['stat', !item.count() && 'is-zero']}>
									<span class="stat-value">{number.format(item.count())}</span>
									<span class="stat-label"><Icon name={item.icon} size={13} flip={item.flip} />{item.label}</span>
								</a>
							</li>
						{/each}
					</ul>
				{:else}
					<p class="note">The transcript did not load, so activity is unknown.</p>
				{/if}
			</section>

			{#if outline.length > 1}
				<section>
					<h2>Prompts <span class="dim">· {outline.length}</span></h2>
					<ol class="prompts">
						{#each shownPrompts as item, index (item.id)}
							<li>
								<a href={traceHref({ targetId: item.id })}>
									<span class="prompt-number">{index + 1}</span>
									<span class="prompt-text">{item.prompt || 'Prompt without text'}</span>
									{#if item.at}<time class="prompt-time" datetime={item.at}>{promptWhen(index)}</time>{/if}
								</a>
							</li>
						{/each}
					</ol>
					{#if outline.length > PROMPTS_SHOWN}
						<button type="button" class="more" onclick={() => (allPrompts = !allPrompts)}>
							{allPrompts ? 'Show fewer' : `Show all ${outline.length} prompts`}
						</button>
					{/if}
				</section>
			{/if}

			{#if failures.length}
				<section>
					<h2>Failures <span class="dim">· {failures.length}</span></h2>
					<ul class="failures">
						{#each failures as failure (failure.id)}
							<li>
								<a href={traceHref({ targetId: failure.entryId })}>
									<Icon name={failure.chip.icon} size={13} />
									<span class="failure-verb">{failure.chip.verb}</span>
									<span class="failure-rest" translate="no">{failure.chip.rest}</span>
									{#if failure.exitCode}<span class="bad failure-exit">exit {failure.exitCode}</span>{/if}
								</a>
							</li>
						{/each}
					</ul>
				</section>
			{/if}

			{#if tools.length}
				<section>
					<h2>Tools <span class="dim">· {number.format(counts.tools)} calls</span>{#if toolErrors}<span class="dim">{' · '}</span><span class="bad">{toolErrors} failed</span>{/if}</h2>
					<ul class="tools">
						{#each workTools as tool (tool.name)}
							<li>
								<span class="tool-name" translate="no"><Icon name={toolKindIcon[tool.kind]} size={13} />{tool.name}</span>
								<span class="bar" style="--share: {tool.count / toolScale}" aria-hidden="true"></span>
								<span class="tool-count">{number.format(tool.count)}</span>
								<span class="tool-errors bad">{tool.errors ? `${tool.errors} failed` : ''}</span>
							</li>
						{/each}
						{#if housekeeping}
							<li class="is-housekeeping" title={housekeeping.names}>
								<span class="tool-name"><Icon name="wrench" size={13} />Housekeeping</span>
								<span class="bar" style="--share: {housekeeping.count / toolScale}" aria-hidden="true"></span>
								<span class="tool-count">{number.format(housekeeping.count)}</span>
								<span class="tool-errors bad">{housekeeping.errors ? `${housekeeping.errors} failed` : ''}</span>
							</li>
						{/if}
					</ul>
				</section>
			{/if}

			<section>
				<h2>Tokens</h2>
				{#if usage}
					<dl class="tokens">
						<dt>New input</dt><dd>{number.format(usage.input)}</dd>
						<dt>Cache write</dt><dd>{number.format(usage.cacheWrite)}</dd>
						<dt>Cache read</dt><dd>{number.format(usage.cacheRead)}</dd>
						<dt>Output</dt><dd>{number.format(usage.output)}</dd>
						{#if usage.peakContext}
							<dt>Peak context</dt>
							<dd>
								{compact.format(usage.peakContext)}{#if usage.contextWindow}{` of ${compact.format(usage.contextWindow)} `}<span class="dim"
										>({Math.round((usage.peakContext / usage.contextWindow) * 100)}%)</span
									>{/if}
							</dd>
						{/if}
					</dl>
					<p class="note">From {number.format(usage.messages)} model replies{leaves.length > 1 ? ', across all branches' : ''}.</p>
				{:else}
					<p class="note">This export does not record token usage.</p>
				{/if}
			</section>

			{#if leaves.length > 1}
				<section>
					<h2>Branches <span class="dim">· {leaves.length}</span></h2>
					<ul class="links">
						{#each leaves as leaf (leaf.id)}
							<li><a href={traceHref({ leafId: leaf.id })}>{leaf.label}</a></li>
						{/each}
					</ul>
				</section>
			{/if}

			{#if trace.parents?.length || trace.children?.length || trace.parent_native_trace_id}
				<section>
					<h2>Related traces</h2>
					<ul class="links">
						{#each trace.parents ?? [] as parent (parent.id)}
							<li><a href={transcriptHref(parent.id)}><span class="dim">Parent</span> {parent.title || parent.native_trace_id}</a></li>
						{:else}
							{#if trace.parent_native_trace_id}<li class="note">Parent trace was never collected.</li>{/if}
						{/each}
						{#each trace.children ?? [] as child (child.id)}
							<li><a href={transcriptHref(child.id)}><span class="dim">Subagent</span> {child.title || child.native_trace_id}</a></li>
						{/each}
					</ul>
				</section>
			{/if}

			<section>
				<h2>Archive</h2>
				<ul class="links">
					{#each trace.revisions ?? [] as revision (revision.id)}
						<li class="revision">
							<a href={revisionSourcesHref(revision.id)} title="Open the original files for revision {revision.id}">
								Collected {revision.collected_at ? dateAndTime(revision.collected_at) : `as revision ${revision.id}`}
								<span class={revision.status === 'normalized' ? 'dim' : 'bad'}>· {parseStatus[revision.status] ?? revision.status}</span>
							</a>
							{#if trace.harness === 'amp' && (trace.revisions?.length ?? 0) > 1}
								<a class="secondary" href={traceHref({ revision: String(revision.id) })}>Read this revision only</a>
							{/if}
							{#if trace.harness === 'amp'}
								<a class="secondary" href={sourceFileHref(revision.id, 'source/export.json', { download: true })} data-sveltekit-reload>Download Amp export</a>
							{/if}
							{#each revision.diagnostics ?? [] as diagnostic (`${diagnostic.code}:${diagnostic.message}`)}
								<p class="diagnostic"><strong>{diagnostic.code}</strong> {diagnostic.message}</p>
							{/each}
						</li>
					{/each}
				</ul>
				<details class="danger">
					<summary>Permanently delete trace</summary>
					<form class="mt-3 flex max-w-md flex-col gap-3" action="/traces/{trace.id}/delete" method="post" onsubmit={removeTrace} aria-busy={deleting}>
						<input type="hidden" name="csrf" value={data.csrf} />
						<label class="text-sm font-medium" for="confirm">Type <code>{trace.native_trace_id}</code> to confirm</label>
						<Input id="confirm" name="confirm" required autocomplete="off" aria-invalid={deleteError ? true : undefined} aria-describedby={deleteError ? 'delete-error' : undefined} />
						{#if deleteError}
							<p id="delete-error" class="text-destructive text-sm" role="alert">{deleteError}</p>
						{/if}
						<Button type="submit" variant="destructive" disabled={deleting}>{deleting ? 'Deleting…' : 'Delete permanently'}</Button>
					</form>
				</details>
			</section>
		</div>
	</div>
{/if}

<style>
	.scroll {
		min-height: 0;
		flex: 1;
		overflow: auto;
	}

	.overview {
		max-width: 46rem;
		margin: 0 auto;
		padding: 0.5rem 1.25rem 3rem;
		font-size: 13px;
	}

	.story {
		display: flex;
		flex-direction: column;
		gap: 0.25rem;
	}

	.story-part {
		display: flex;
		flex-direction: column;
		gap: 0.2rem;
		margin-inline: -0.75rem;
		padding: 0.55rem 0.75rem;
		border-radius: var(--radius);
		color: var(--foreground);
		text-decoration: none;
	}

	.story-part:hover,
	.story-part:focus-visible {
		background: var(--accent);
	}

	.story-label {
		color: var(--muted-foreground);
		font-size: 12px;
	}

	.story-text {
		display: -webkit-box;
		overflow: hidden;
		font-size: 14.5px;
		line-height: 1.6;
		-webkit-box-orient: vertical;
		-webkit-line-clamp: 4;
		line-clamp: 4;
	}

	section {
		padding: 1.1rem 0;
		border-bottom: 1px solid var(--border);
	}

	section:last-child {
		border-bottom: 0;
	}

	h2 {
		margin: 0 0 0.65rem;
		color: var(--foreground);
		font-size: 13px;
		font-weight: 600;
	}

	h2 .dim {
		font-weight: 400;
	}

	dl {
		display: grid;
		grid-template-columns: 8rem minmax(0, 1fr);
		gap: 0.4rem 1rem;
		margin: 0;
	}

	dt,
	.dim,
	.note {
		color: var(--muted-foreground);
	}

	dd {
		min-width: 0;
		margin: 0;
		font-variant-numeric: tabular-nums;
	}

	.tokens dd {
		text-align: left;
	}

	.note {
		margin: 0.5rem 0 0;
		font-size: 12px;
	}

	.status {
		margin: 0;
		padding: 1rem 1.25rem;
	}

	.error,
	.bad {
		color: var(--destructive);
	}

	ul {
		margin: 0;
		padding: 0;
		list-style: none;
	}

	/* Each count opens the Trace tab showing only that kind of row. */
	.stats {
		display: grid;
		grid-template-columns: repeat(auto-fill, minmax(7.5rem, 1fr));
		gap: 0.5rem;
	}

	.stat {
		display: flex;
		flex-direction: column;
		gap: 0.15rem;
		padding: 0.6rem 0.75rem;
		border: 1px solid var(--border);
		border-radius: var(--radius);
		color: var(--foreground);
		text-decoration: none;
		transition: background-color 100ms ease;
	}

	.stat:hover,
	.stat:focus-visible {
		background: var(--accent);
	}

	.stat-label {
		display: inline-flex;
		align-items: center;
		gap: 0.5rem;
		color: var(--muted-foreground);
	}

	.stat:hover .stat-label {
		color: var(--foreground);
	}

	.stat-value {
		font-size: 18px;
		font-weight: 500;
		font-variant-numeric: tabular-nums;
	}

	.stat.is-zero .stat-value {
		color: var(--muted-foreground);
		font-weight: 400;
	}

	.tools li {
		display: grid;
		grid-template-columns: minmax(8rem, 14rem) minmax(0, 1fr) 2.5rem 4.5rem;
		align-items: center;
		gap: 0.75rem;
		padding: 0.2rem 0;
	}

	.tool-name {
		display: inline-flex;
		overflow: hidden;
		align-items: center;
		gap: 0.45rem;
		font-family: var(--font-mono);
		font-size: 12px;
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	.tool-name :global(svg) {
		flex-shrink: 0;
		color: var(--muted-foreground);
	}

	/* A share bar, not a chart: how this tool compares with the most used one. */
	.bar {
		height: 4px;
		border-radius: 999px;
		background: linear-gradient(
			to right,
			color-mix(in oklab, var(--foreground) 28%, transparent) calc(var(--share) * 100%),
			color-mix(in oklab, var(--foreground) 6%, transparent) 0
		);
	}

	.is-housekeeping {
		color: var(--muted-foreground);
	}

	.is-housekeeping .tool-name {
		font-family: var(--font-sans);
		font-size: 13px;
	}

	.tool-count {
		font-variant-numeric: tabular-nums;
		text-align: right;
	}

	.tool-errors {
		font-size: 12px;
	}

	.prompts a {
		display: grid;
		grid-template-columns: 1.75rem minmax(0, 1fr) auto;
		align-items: baseline;
		gap: 0.5rem;
		margin-inline: -0.6rem;
		padding: 0.3rem 0.6rem;
		border-radius: var(--radius);
		color: var(--foreground);
		text-decoration: none;
	}

	.prompts a:hover,
	.prompts a:focus-visible {
		background: var(--accent);
	}

	.prompt-number,
	.prompt-time {
		color: var(--muted-foreground);
		font-size: 12px;
		font-variant-numeric: tabular-nums;
	}

	.prompt-text {
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	.more {
		margin-top: 0.4rem;
		border: 0;
		background: transparent;
		color: var(--muted-foreground);
		padding: 0;
		font: inherit;
		font-size: 12px;
		cursor: pointer;
	}

	.more:hover,
	.more:focus-visible {
		color: var(--foreground);
	}

	.failures a {
		display: grid;
		grid-template-columns: auto auto minmax(0, 1fr) auto;
		align-items: center;
		gap: 0.5rem;
		margin-inline: -0.6rem;
		padding: 0.3rem 0.6rem;
		border-radius: var(--radius);
		color: var(--foreground);
		text-decoration: none;
	}

	.failures a:hover,
	.failures a:focus-visible {
		background: var(--accent);
	}

	.failures :global(svg) {
		color: var(--muted-foreground);
	}

	.failure-verb {
		font-weight: 500;
	}

	.failure-rest {
		overflow: hidden;
		color: var(--muted-foreground);
		font-family: var(--font-mono);
		font-size: 12px;
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	.failure-exit {
		font-size: 12px;
		font-variant-numeric: tabular-nums;
	}

	.links li + li {
		margin-top: 0.15rem;
	}

	.links a {
		display: block;
		margin-inline: -0.6rem;
		padding: 0.35rem 0.6rem;
		border-radius: var(--radius);
		color: var(--foreground);
		text-decoration: none;
	}

	.links a:hover,
	.links a:focus-visible {
		background: var(--accent);
	}

	.revision .secondary {
		padding-top: 0;
		color: var(--muted-foreground);
		font-size: 12px;
	}

	.diagnostic {
		margin: 0.35rem 0;
		padding: 0.4rem 0.6rem;
		border-radius: var(--radius);
		background: color-mix(in oklch, var(--primary) 8%, transparent);
		font-size: 12px;
	}

	.danger {
		margin-top: 1rem;
	}

	.danger summary {
		color: color-mix(in oklch, var(--destructive) 80%, var(--muted-foreground));
		font-size: 12px;
		cursor: pointer;
	}
</style>
