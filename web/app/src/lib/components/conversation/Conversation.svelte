<script lang="ts">
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import { parseSkillBlock } from '$lib/transcript/skill';
	import { findNewestLeaf, getPath, graphLeaves, treeLabel } from '$lib/transcript/tree';
	import { childCards, leftoverCards } from '$lib/transcript/children';
	import {
		entryMatchesFilter,
		isToolOnlyMessage,
		pathSessionFacts,
		customNote,
		streamCounts,
		streamRows,
		toolGroupFilter,
		toolInGroup,
		toolResults,
		type StreamFilter
	} from '$lib/transcript/turns';
	import type { LoadedSession, TranscriptEntry } from '$lib/transcript/types';
	import type { Trace } from '$lib/types';
	import AttachmentBlock from './AttachmentBlock.svelte';
	import ChildCard from './ChildCard.svelte';
	import ExpandChip from './ExpandChip.svelte';
	import Icon from './Icon.svelte';
	import Markdown from './Markdown.svelte';
	import ToolRow from './ToolRow.svelte';
	import TraceIndex from './TraceIndex.svelte';
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
	let filter = $state<StreamFilter>('all');
	let expandedUsers = $state<Record<string, boolean>>({});
	let streamEl = $state<HTMLDivElement | null>(null);
	let showJump = $state(false);

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
	let rows = $derived(streamRows(filtered));
	let children = $derived(trace.children ?? []);
	let leftover = $derived(leftoverCards(path, results, children));
	let counts = $derived(session ? streamCounts(path, session.entries) : streamCounts([]));
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
		}
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

	function onStreamScroll() {
		const el = streamEl;
		if (!el) return;
		showJump = el.scrollHeight - el.scrollTop - el.clientHeight > 180;
	}

	function jumpToRecent() {
		const el = streamEl;
		if (!el) return;
		const reduce = window.matchMedia('(prefers-reduced-motion: reduce)').matches;
		el.scrollTo({ top: el.scrollHeight, behavior: reduce ? 'auto' : 'smooth' });
	}

	$effect(() => {
		rows;
		sessionKey;
		queueMicrotask(onStreamScroll);
	});

	function selectNode(entryId: string) {
		if (!session) return;
		const leaf = findNewestLeaf(session.entries, entryId);
		selection = { key: sessionKey, leaf, target: entryId };
		const params = new URLSearchParams(page.url.searchParams);
		params.set('leafId', leaf);
		params.set('targetId', entryId);
		const href = resolve('/traces/[id]', { id: String(trace.id) });
		history.replaceState(history.state, '', `${href}?${params}`);
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

	function showTool(name: string) {
		return toolInGroup(name, filter);
	}
</script>

<svelte:window onkeydown={onKeydown} />

<div class="workspace-root">
	<WorkspaceHeader
		{trace}
		{revision}
		tab="conversation"
		recordCount={session?.events.length ?? session?.entries.length ?? 0}
		model={facts.model ?? ''}
		thinkingLevel={facts.thinkingLevel ?? ''}
		{thinkingExpanded}
		{toolsExpanded}
		onToggleThinking={toggleThinking}
		onToggleTools={toggleTools}
	/>

	{#if displayError && !session}
		<p class="status error" role="alert">{displayError}</p>
	{:else if !session}
		<p class="status" role="status">Loading transcript…</p>
	{:else}
		{#if displayError}
			<p class="status error" role="alert">{displayError}</p>
		{/if}
		{#snippet entryBlocks(entry: TranscriptEntry)}
			{@const message = entry.message}
			{#if message?.role === 'toolResult'}
				{#if shouldRenderResult(entry) && showTool(message.toolName || '')}
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
						{#if block.type === 'toolCall' && showTool(block.name)}
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
						{#if toolGroupFilter(filter)}
							{#if block.type === 'toolCall' && showTool(block.name)}
								<ToolRow
									call={block}
									result={results.get(block.id)}
									{entry}
									{children}
									open={toolIsOpen(block.id)}
									onToggle={(open) => (toolOpen[block.id] = open)}
								/>
							{/if}
						{:else if block.type === 'text'}
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
							<ExpandChip icon="compaction" verb="Hidden context">
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
						{:else if block.type === 'toolCall' && showTool(block.name)}
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
						<p class="note error">Recorded response state: {message.stopReason}</p>
					{/if}
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
			<TraceIndex
				{counts}
				{filter}
				{leaves}
				{leafId}
				onFilter={(next) => (filter = next)}
				onSelectLeaf={selectNode}
			/>
			{#key sessionKey}
				<div class="stream-wrap">
				<div class="stream" bind:this={streamEl} onscroll={onStreamScroll}>
					{#each rows as row (row.id)}
						<section class={['row', `is-${row.chrome}`, row.entries.some((entry) => entry.id === targetId) && 'is-target']}>
							{#if row.chrome !== 'quiet'}
								<div class="avatar" aria-hidden="true">
									<Icon name={row.chrome === 'user' ? 'user' : 'agent'} size={12} />
								</div>
							{/if}
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
					{/each}
					{#each leftover as child (child.native_trace_id)}
						<ChildCard child={{ id: child.native_trace_id, title: child.title || child.native_trace_id, collected: true, traceId: child.id, meta: 'Child trace' }} />
					{/each}
					{#if !rows.length}
						<p class="empty">
							{session.entries.length
								? 'Nothing in this filter.'
								: 'No normalized messages. Open Details to inspect the retained source files.'}
						</p>
					{/if}
				</div>
				{#if showJump}
					<button class="jump" type="button" onclick={jumpToRecent} aria-label="Jump to recent messages">
						<Icon name="chevron" size={16} />
					</button>
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
		width: 100%;
		max-width: 1200px;
		min-height: 0;
		flex: 1;
		flex-direction: column;
		margin: 0 auto;
	}

	.stream-wrap {
		position: relative;
		min-width: 0;
		min-height: 0;
		flex: 1;
	}

	.stream {
		height: 100%;
		min-width: 0;
		overflow: auto;
		padding: 0.5rem 1.5rem 3rem;
	}

	.jump {
		position: absolute;
		right: 17px;
		bottom: 16px;
		z-index: 3;
		display: grid;
		width: 32px;
		height: 32px;
		place-items: center;
		border: 0;
		border-radius: 8px;
		background: var(--card);
		box-shadow: var(--contour);
		color: var(--muted-foreground);
		cursor: pointer;
	}

	.jump :global(svg) {
		transform: rotate(90deg);
	}

	.jump:hover,
	.jump:focus-visible {
		color: var(--foreground);
	}

	.row {
		position: relative;
		max-width: 52rem;
		padding: 0 0 0 46px;
		font-size: 14px;
		line-height: 20px;
	}

	.row.is-user {
		margin: 8px 0 0;
	}

	.row.is-agent {
		margin: 24px 0;
	}

	.row.is-quiet {
		margin: 0;
	}

	.row.is-target .turn-body {
		outline: 1px solid rgb(0 0 0 / 18%);
		outline-offset: 3px;
	}

	.avatar {
		position: absolute;
		top: 2px;
		left: 0;
		display: flex;
		width: 20px;
		height: 20px;
		align-items: center;
		justify-content: center;
		border-radius: 999px;
		background: var(--card);
		box-shadow: var(--contour);
		color: var(--muted-foreground);
	}

	.row.is-user .avatar {
		top: 8px;
		color: var(--foreground);
	}

	.turn-body {
		min-width: 0;
	}

	.bubble {
		position: relative;
		margin-left: -10px;
		padding: 6px 10px;
		border-radius: 10px;
		background: var(--card);
		box-shadow: var(--contour);
	}

	.clamp.is-clamped {
		max-height: 11.5rem;
		overflow: hidden;
	}

	.more {
		position: absolute;
		right: 0;
		bottom: 0;
		left: 0;
		border: 0;
		border-radius: 0 0 10px 10px;
		background: var(--card);
		color: var(--muted-foreground);
		padding: 6px 10px;
		font-size: 12px;
		font-weight: 500;
		text-align: left;
		cursor: pointer;
	}

	.more::before {
		content: '';
		position: absolute;
		right: 0;
		bottom: 100%;
		left: 0;
		height: 28px;
		background: linear-gradient(to bottom, transparent, var(--card));
		pointer-events: none;
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
		overflow: auto;
		max-height: 16rem;
		padding: 6px 10px;
		border: 1px solid rgb(0 0 0 / 11%);
		border-radius: 8px;
		background: var(--muted);
		color: var(--muted-foreground);
		font-family: var(--font-mono);
		font-size: 13px;
		line-height: 20px;
		white-space: pre-wrap;
	}

	@media (min-width: 900px) {
		.body {
			flex-direction: row;
			align-items: stretch;
			overflow: hidden;
		}

		.stream {
			padding: 0.35rem 1.5rem 3rem 0.5rem;
		}
	}
</style>
