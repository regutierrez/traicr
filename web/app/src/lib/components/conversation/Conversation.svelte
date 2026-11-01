<script lang="ts">
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import { parseSkillBlock } from '$lib/transcript/skill';
	import { findNewestLeaf, getPath, graphLeaves, treeLabel } from '$lib/transcript/tree';
	import { childCards, leftoverCards } from '$lib/transcript/children';
	import {
		entryMatchesFilter,
		foldWork,
		isToolOnlyMessage,
		workSummary,
		pathSessionFacts,
		customNote,
		streamFilters,
		streamRows,
		toolResults,
		type StreamFilter
	} from '$lib/transcript/turns';
	import type { LoadedSession, TranscriptEntry } from '$lib/transcript/types';
	import type { Trace } from '$lib/types';
	import AttachmentBlock from './AttachmentBlock.svelte';
	import ChildCard from './ChildCard.svelte';
	import DebugPanel from './DebugPanel.svelte';
	import ExpandChip from './ExpandChip.svelte';
	import Icon from './Icon.svelte';
	import Markdown from './Markdown.svelte';
	import MessageDetails from './MessageDetails.svelte';
	import ToolRow from './ToolRow.svelte';
	import WorkspaceHeader from './WorkspaceHeader.svelte';

	let {
		trace,
		revision = '',
		initial,
		loadError = ''
	}: {
		trace: Trace;
		revision?: string;
		initial: LoadedSession | null;
		loadError?: string;
	} = $props();

	let selection = $state.raw<{ key: string; leaf: string; target: string } | null>(null);
	let thinkingExpanded = $state(false);
	let toolsExpanded = $state(false);
	let thinkingOpen = $state<Record<string, boolean>>({});
	let toolOpen = $state<Record<string, boolean>>({});
	let stepsExpanded = $state(false);
	let workOpen = $state<Record<string, boolean>>({});
	// Overview links here with ?filter= to show one kind of row.
	const filterParam = page.url.searchParams.get('filter') as StreamFilter | null;
	let filter = $state<StreamFilter>(filterParam && streamFilters.includes(filterParam) ? filterParam : 'all');
	const filterLabels: Record<StreamFilter, string> = {
		all: 'everything',
		prompts: 'prompts',
		responses: 'replies',
		thinking: 'thoughts',
		tools: 'tool calls',
		compaction: 'compactions'
	};
	let expandedUsers = $state<Record<string, boolean>>({});

	let sessionKey = $derived(`${trace.id}:${revision}`);
	let session = $derived(initial);
	let displayError = $derived(loadError);

	function idsFromURL(data: LoadedSession) {
		const params = page.url.searchParams;
		const urlLeaf = data.eventEntries.get(params.get('leafId') || '') || params.get('leafId') || '';
		const urlTarget =
			data.eventEntries.get(params.get('targetId') || '') ||
			params.get('targetId') ||
			data.eventEntries.get(params.get('event') || params.get('key') || '') ||
			'';
		const leaf = urlLeaf || data.leafId || data.entries.at(-1)?.id || '';
		return { leaf, target: urlTarget };
	}

	let leafId = $derived.by(() => {
		if (!session) return '';
		if (selection?.key === sessionKey) return selection.leaf;
		return idsFromURL(session).leaf;
	});
	let targetId = $derived.by(() => {
		if (!session) return '';
		if (selection?.key === sessionKey) return selection.target;
		return idsFromURL(session).target;
	});

	let path = $derived(session && leafId ? getPath(session.entries, leafId) : []);
	let facts = $derived(pathSessionFacts(path));
	let results = $derived(toolResults(path));
	let filtered = $derived(path.filter((entry) => entryMatchesFilter(entry, filter)));
	// A filtered stream already shows only what was asked for, so it is not folded.
	let rows = $derived(filter === 'all' ? foldWork(streamRows(filtered), opensChild) : streamRows(filtered));
	// Big traces have thousands of rows. Painting them all in one frame blocks the
	// page for seconds, so the first rows paint at once and the rest follow in
	// chunks on later frames.
	const FIRST_ROWS = 150;
	const MORE_ROWS = 300;
	let grown = $state.raw<{ rows: typeof rows; count: number } | null>(null);
	let shownCount = $derived(grown?.rows === rows ? grown.count : FIRST_ROWS);
	let shownRows = $derived(rows.length > shownCount ? rows.slice(0, shownCount) : rows);
	$effect(() => {
		const current = rows;
		const count = shownCount;
		if (count >= current.length) return;
		const frame = requestAnimationFrame(() => (grown = { rows: current, count: count + MORE_ROWS }));
		return () => cancelAnimationFrame(frame);
	});
	let children = $derived(trace.children ?? []);
	let leftover = $derived(leftoverCards(path, results, children));
	let leaves = $derived(
		session ? graphLeaves(session.entries).map((entry) => ({ id: entry.id, label: treeLabel(entry) })) : []
	);

	function isEditable(element: EventTarget | null) {
		if (!(element instanceof HTMLElement)) return false;
		const tag = element.tagName;
		if (tag === 'INPUT' || tag === 'TEXTAREA' || tag === 'SELECT') return true;
		return element.isContentEditable;
	}

	function onKeydown(event: KeyboardEvent) {
		if (event.defaultPrevented || event.metaKey || event.ctrlKey || event.altKey || event.isComposing) return;
		if (isEditable(event.target)) return;
		const key = event.key.toLowerCase();
		if (key === 't') {
			event.preventDefault();
			toggleThinking();
		} else if (key === 'o') {
			event.preventDefault();
			toggleTools();
		} else if (key === 'e') {
			event.preventDefault();
			toggleSteps();
		}
	}

	function toggleSteps() {
		stepsExpanded = !stepsExpanded;
		workOpen = {};
	}

	// Showing every thought or tool body also has to show the steps they sit in.
	function workIsOpen(row: { id: string; entries: TranscriptEntry[] }) {
		return (
			workOpen[row.id] ??
			(stepsExpanded || thinkingExpanded || toolsExpanded || row.entries.some((entry) => entry.id === targetId))
		);
	}

	// A step that opens a child trace stays visible, so folding never hides a link to another trace.
	function opensChild(row: { entries: TranscriptEntry[] }) {
		return row.entries.some(
			(entry) =>
				incomingCards(entry).length > 0 ||
				(entry.message?.content ?? []).some(
					(block) => block.type === 'toolCall' && childCards(entry, block, results.get(block.id), children).length > 0
				)
		);
	}

	function toggleThinking() {
		thinkingExpanded = !thinkingExpanded;
		thinkingOpen = {};
	}

	function toggleTools() {
		toolsExpanded = !toolsExpanded;
		toolOpen = {};
	}

	function thinkingIsOpen(id: string) {
		return thinkingOpen[id] ?? thinkingExpanded;
	}

	function toolIsOpen(id: string) {
		return toolOpen[id] ?? toolsExpanded;
	}

	function selectNode(entryId: string) {
		if (!session) return;
		const leaf = findNewestLeaf(session.entries, entryId);
		selection = { key: sessionKey, leaf, target: entryId };
		const params = new URLSearchParams(page.url.searchParams);
		params.set('leafId', leaf);
		params.set('targetId', entryId);
		const href = resolve('/traces/[id]/trace', { id: String(trace.id) });
		history.replaceState(history.state, '', `${href}?${params}`);
	}

	function showEverything() {
		filter = 'all';
		const params = new URLSearchParams(page.url.searchParams);
		params.delete('filter');
		const href = resolve('/traces/[id]/trace', { id: String(trace.id) });
		history.replaceState(history.state, '', params.size ? `${href}?${params}` : href);
	}

	function incomingCards(entry: TranscriptEntry) {
		return childCards(entry, undefined, undefined, children);
	}

	function userPromptLength(entries: TranscriptEntry[]) {
		return entries
			.flatMap((entry) => entry.message?.content ?? [])
			.reduce((total, block) => total + (block.type === 'text' ? block.text.length : 0), 0);
	}

	function shouldRenderResult(entry: TranscriptEntry) {
		const id = entry.message?.toolCallId;
		return !id || !path.some((item) => item.message?.content?.some((block) => block.type === 'toolCall' && block.id === id));
	}
</script>

<svelte:window onkeydown={onKeydown} />

<div class="workspace-root">
	<WorkspaceHeader
		{trace}
		{revision}
		tab="trace"
		recordCount={session?.events.length ?? session?.entries.length ?? 0}
		model={facts.model ?? ''}
		thinkingLevel={facts.thinkingLevel ?? ''}
		{thinkingExpanded}
		{toolsExpanded}
		{stepsExpanded}
		onToggleThinking={toggleThinking}
		onToggleTools={toggleTools}
		onToggleSteps={toggleSteps}
	/>

	{#if displayError && !session}
		<p class="status error" role="alert">{displayError}</p>
	{:else if !session}
		<p class="status" role="status">Loading transcript…</p>
	{:else}
		{#if displayError}
			<p class="status error" role="alert">{displayError}</p>
		{/if}
		{#if page.url.searchParams.get('debug') === '1'}
			<DebugPanel {session} />
		{/if}
		{#snippet entryBlocks(entry: TranscriptEntry)}
			{@const message = entry.message}
			{#if message?.role === 'toolResult'}
				{#if shouldRenderResult(entry)}
					<ToolRow
						call={{ type: 'toolCall', id: message.toolCallId || entry.id, name: message.toolName || 'Unpaired tool result', arguments: {} }}
						result={entry}
						{entry}
						{children}
						open={toolIsOpen(message.toolCallId || entry.id)}
						onToggle={(open) => (toolOpen[message.toolCallId || entry.id] = open)}
					/>
				{/if}
			{:else if message && isToolOnlyMessage(entry)}
				<div class="block" id="entry-{entry.id}">
					{#each incomingCards(entry) as child (child.id)}
						<ChildCard {child} />
					{/each}
					{#each message.content as block, index (`${entry.id}:${index}:${block.type}`)}
						{#if block.type === 'toolCall'}
							<ToolRow
								call={block}
								result={results.get(block.id)}
								{entry}
								{children}
								open={toolIsOpen(block.id)}
								onToggle={(open) => (toolOpen[block.id] = open)}
							/>
						{/if}
					{/each}
				</div>
			{:else if message}
				<article class="block" id="entry-{entry.id}">
					{#if message.nativeDetails?.message_meta?.fromAutomation}
						<p class="note">Automation message</p>
					{/if}
					{#each incomingCards(entry) as child (child.id)}
						<ChildCard {child} />
					{/each}
					{#each message.content as block, index (`${entry.id}:${index}:${block.type}`)}
						{#if block.type === 'text'}
							{@const skill = message.role === 'user' ? parseSkillBlock(block.text) : null}
							{#if skill}
								<ExpandChip icon="sparkles" verb="Skill" rest={skill.name}>
									<Markdown text={skill.content} />
								</ExpandChip>
								{#if skill.userMessage}
									<Markdown text={skill.userMessage} />
								{/if}
							{:else}
								<Markdown text={block.text} />
							{/if}
						{:else if block.type === 'hidden'}
							<ExpandChip icon="compaction" verb={message.role === 'system' ? 'System message' : 'Hidden context'}>
								<pre>{block.text}</pre>
							</ExpandChip>
						{:else if block.type === 'thinking'}
							{@const thinkKey = `${entry.id}:${index}`}
							<ExpandChip
								icon="brain"
								verb="Thought"
								open={thinkingIsOpen(thinkKey)}
								onToggle={(next) => {
									if (next !== thinkingIsOpen(thinkKey)) thinkingOpen[thinkKey] = next;
								}}
							>
								<div class="think">{block.thinking || 'Reasoning text not included in export.'}</div>
							</ExpandChip>
						{:else if block.type === 'attachment'}
							<AttachmentBlock attachment={block} />
						{:else if block.type === 'toolCall'}
							<ToolRow
								call={block}
								result={results.get(block.id)}
								{entry}
								{children}
								open={toolIsOpen(block.id)}
								onToggle={(open) => (toolOpen[block.id] = open)}
							/>
						{/if}
					{/each}
					{#if message.stopReason && !['stop', 'toolUse', 'complete'].includes(message.stopReason)}
						{@const error = message.nativeDetails?.state?.errorMessage}
						<p class="note error">Recorded response state: {message.stopReason}{error ? ` · ${error}` : ''}</p>
					{/if}
					<MessageDetails {entry} />
				</article>
			{:else if entry.type === 'compaction' || entry.type === 'branch_summary'}
				<div id="entry-{entry.id}">
					<ExpandChip icon={entry.type === 'compaction' ? 'compaction' : 'branch'} verb={entry.type === 'compaction' ? 'Compaction' : 'Branch summary'}>
						<Markdown text={entry.summary || ''} />
					</ExpandChip>
				</div>
			{:else}
				{const note = customNote(entry)}
				{#if note}
					<section class="note" id="entry-{entry.id}">
						{note.title}
						{#if note.body}
							<pre>{note.body}</pre>
						{/if}
					</section>
				{/if}
			{/if}
		{/snippet}
		<div class="body">
			{#key sessionKey}
				<div class="stream">
					{#if filter !== 'all' || leaves.length > 1}
						<div class="view-bar">
							{#if filter !== 'all'}
								<span>Showing only {filterLabels[filter]}</span>
								<button type="button" onclick={showEverything}>Show everything</button>
							{/if}
							{#if leaves.length > 1}
								<label class="branch">
									<span>Branch</span>
									<select value={leafId} onchange={(event) => selectNode(event.currentTarget.value)}>
										{#each leaves as leaf, index (leaf.id)}
											<option value={leaf.id}>{index + 1} of {leaves.length} · {leaf.label}</option>
										{/each}
									</select>
								</label>
							{/if}
						</div>
					{/if}
					{#each shownRows as row (row.id)}
						{#if row.chrome === 'work'}
							{@const summary = workSummary(row.entries, results)}
							{@const open = workIsOpen(row)}
							<section class={['row', 'is-work', open && 'is-open']}>
								<button
									type="button"
									class="work-toggle"
									aria-expanded={open}
									onclick={() => (workOpen[row.id] = !open)}
								>
									{#if summary.duration}<span class="work-duration">{summary.duration}</span>{/if}
									<span class="work-sentence">{summary.sentence}</span>
									<span class="work-chevron"><Icon name="chevron" size={12} /></span>
								</button>
								{#if open}
									<div class="work-steps">
										{#each row.steps as step (step.id)}
											{#each step.entries as entry (entry.id)}
												{@render entryBlocks(entry)}
											{/each}
										{/each}
									</div>
								{/if}
							</section>
						{:else}
						<section class={['row', `is-${row.chrome}`, row.entries.some((entry) => entry.id === targetId) && 'is-target']}>
							{#if row.chrome === 'user'}<h2 class="sr-only">You</h2>{:else if row.chrome === 'agent'}<h2 class="sr-only">Agent</h2>{/if}
							<div class={['turn-body', row.chrome === 'user' && 'bubble']}>
								<div class={['clamp', row.chrome === 'user' && userPromptLength(row.entries) > 700 && !expandedUsers[row.id] && 'is-clamped']}>
									{#each row.entries as entry (entry.id)}
										{@render entryBlocks(entry)}
									{/each}
								</div>
								{#if row.chrome === 'user' && userPromptLength(row.entries) > 700 && !expandedUsers[row.id]}
									<button class="more" type="button" onclick={() => (expandedUsers[row.id] = true)}>Show more</button>
								{/if}
							</div>
						</section>
						{/if}
					{/each}
					{#each leftover as child (child.native_trace_id)}
						<ChildCard child={{ id: child.native_trace_id, title: child.title || child.native_trace_id, collected: true, traceId: child.id, meta: 'Child trace' }} />
					{/each}
					{#if !session.entries.length}
						<p class="empty">No normalized messages. Open Details to inspect the retained source files.</p>
					{/if}
				</div>
			{/key}
		</div>
	{/if}
</div>

<style>
	.workspace-root {
		display: flex;
		min-height: 0;
		flex: 1;
		flex-direction: column;
	}

	.status,
	.empty {
		margin: 0;
		padding: 0.45rem 1.25rem;
		color: var(--muted-foreground);
		font-size: 12px;
	}

	.status.error,
	.note.error {
		color: var(--destructive);
	}

	.body {
		display: flex;
		min-height: 0;
		flex: 1;
		flex-direction: column;
	}

	.stream {
		min-width: 0;
		flex: 1;
		overflow: auto;
		padding: 0.85rem 1.25rem 2.25rem;
	}

	.row {
		position: relative;
		max-width: 46rem;
		margin-inline: auto;
		font-size: 14.5px;
		line-height: 1.65;
		/* Off-screen rows skip layout and paint until they scroll near view. */
		content-visibility: auto;
		contain-intrinsic-size: auto 3rem;
	}

	/* The prompt is the landmark of each turn: a compact bubble on the right. */
	.row.is-user {
		display: flex;
		justify-content: flex-end;
		margin-block: 2rem 1rem;
	}

	.row.is-user:first-child {
		margin-top: 0.5rem;
	}

	/* Replies are plain prose, a step quieter than full white for long reading. */
	.row.is-agent {
		margin-block: 0.75rem;
		color: color-mix(in oklab, var(--foreground) 90%, transparent);
	}

	.row.is-quiet {
		margin-block: 0;
	}

	.row.is-work {
		margin-block: 0.35rem;
	}

	.work-toggle {
		display: inline-flex;
		max-width: 100%;
		align-items: center;
		gap: 0.5rem;
		min-height: 28px;
		border: 0;
		background: transparent;
		color: var(--muted-foreground);
		padding: 0;
		font: inherit;
		font-size: 13px;
		font-variant-numeric: tabular-nums;
		text-align: left;
		cursor: pointer;
	}

	.work-toggle:hover,
	.work-toggle:focus-visible {
		color: var(--foreground);
	}

	.work-duration {
		flex-shrink: 0;
		color: var(--foreground);
		font-weight: 500;
		opacity: 0.8;
	}

	.work-duration + .work-sentence::before {
		content: '·';
		margin-right: 0.5rem;
		opacity: 0.6;
	}

	.work-sentence {
		overflow: hidden;
		min-width: 0;
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	.work-chevron {
		display: inline-flex;
		flex-shrink: 0;
		opacity: 0.5;
		transition: transform 150ms ease, opacity 150ms ease;
	}

	.is-open .work-chevron {
		transform: rotate(90deg);
	}

	.work-toggle:hover .work-chevron {
		opacity: 1;
	}

	/* Expanded steps hang off a hairline so they read as one unit of work. */
	.work-steps {
		margin: 0.15rem 0 0.5rem 0.3rem;
		padding-left: 0.9rem;
		border-left: 1px solid var(--border);
	}

	/* Containment clips paint at the row edge; the one target row shows its full outline. */
	.row.is-target {
		content-visibility: visible;
	}

	.row.is-target .turn-body {
		outline: 1px solid color-mix(in oklch, var(--primary) 45%, transparent);
		outline-offset: 3px;
	}

	.turn-body {
		min-width: 0;
	}

	.bubble {
		max-width: min(80%, 38rem);
		padding: 0.6rem 0.9rem;
		border-radius: 1rem;
		background: var(--message);
		color: var(--foreground);
	}

	.clamp.is-clamped {
		max-height: 15rem;
		overflow: hidden;
		mask-image: linear-gradient(to bottom, #000 70%, transparent);
	}

	.more {
		margin: 0.35rem 0 0.1rem;
		border: 0;
		background: transparent;
		color: var(--muted-foreground);
		padding: 0;
		font-size: 12px;
		cursor: pointer;
	}

	.more:hover,
	.more:focus-visible {
		color: var(--foreground);
	}

	.block {
		padding: 0;
	}

	.row.is-quiet .block {
		padding: 0;
	}

	.note {
		margin: 0.3rem 0;
		color: var(--muted-foreground);
		font-size: 12px;
	}

	.think {
		white-space: pre-wrap;
		color: var(--muted-foreground);
		font-size: 12px;
		line-height: 1.5;
	}

	pre {
		overflow-x: auto;
		padding: 0.55rem 0.65rem;
		border: 1px solid var(--border);
		border-radius: var(--radius);
		background: var(--sidebar);
		font-family: var(--font-mono);
		font-size: 12px;
		white-space: pre-wrap;
	}

	.view-bar {
		display: flex;
		max-width: 46rem;
		flex-wrap: wrap;
		align-items: center;
		gap: 0.5rem 0.75rem;
		margin: 0 auto 1rem;
		padding: 0.45rem 0.75rem;
		border: 1px solid var(--border);
		border-radius: var(--radius);
		background: var(--card);
		color: var(--muted-foreground);
		font-size: 12px;
	}

	.view-bar button {
		border: 0;
		background: transparent;
		color: var(--primary);
		padding: 0;
		font: inherit;
		cursor: pointer;
	}

	.branch {
		display: inline-flex;
		min-width: 0;
		align-items: center;
		gap: 0.4rem;
		margin-left: auto;
	}

	.branch select {
		max-width: 22rem;
		height: 1.6rem;
		border: 1px solid var(--input);
		border-radius: var(--radius);
		background: var(--background);
		color: var(--foreground);
		padding: 0 0.4rem;
		font: inherit;
	}

	@media (min-width: 900px) {
		.stream {
			padding: 1rem 1.75rem 2.5rem;
		}
	}
</style>
