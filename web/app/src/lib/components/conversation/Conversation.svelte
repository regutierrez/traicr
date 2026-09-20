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
				<div class="stream">
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
		max-width: 54rem;
		padding: 0 0 0 46px;
		font-size: 14px;
		line-height: 20px;
	}

	.row.is-user {
		margin: 0 0 24px;
	}

	.row.is-agent {
		margin: 24px 0;
	}

	.row.is-quiet {
		margin: 0;
	}

	.row.is-target .turn-body {
		outline: 1px solid color-mix(in oklch, var(--primary) 45%, transparent);
		outline-offset: 3px;
	}

	.avatar {
		position: absolute;
		top: 8px;
		left: 1px;
		display: flex;
		width: 18px;
		height: 18px;
		align-items: center;
		justify-content: center;
		border-radius: 999px;
		background: #1c1c1c;
		box-shadow: inset 0 0 0 1px rgb(255 255 255 / 14%);
		color: var(--muted-foreground);
	}

	.row.is-user .avatar {
		background: #2a2a2a;
		color: #d4d4d4;
	}

	.row.is-agent .avatar {
		top: 0.1rem;
		background: #202020;
		color: #9a9a9a;
	}

	.turn-body {
		min-width: 0;
	}

	.bubble {
		margin-left: -10px;
		padding: 6px 10px;
		border-radius: 10px;
		background: #242424;
		box-shadow:
			0 0 0 1px rgb(255 255 255 / 8%),
			0 1px 3px rgb(0 0 0 / 32%),
			0 1px 2px rgb(0 0 0 / 24%);
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
		background: #181818;
		font-family: var(--font-mono);
		font-size: 12px;
		white-space: pre-wrap;
	}

	@media (min-width: 900px) {
		.body {
			flex-direction: row;
			align-items: stretch;
			overflow: hidden;
		}

		.stream {
			padding: 1rem 1.75rem 2.5rem;
		}
	}
</style>
