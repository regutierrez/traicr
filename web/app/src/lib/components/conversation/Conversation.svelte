<script lang="ts">
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import { parseSkillBlock } from '$lib/transcript/skill';
	import { findNewestLeaf, getPath, graphHasFork } from '$lib/transcript/tree';
	import { childCards, unplacedChildren } from '$lib/transcript/children';
	import { loadMoreSession } from '$lib/transcript/load';
	import { groupTurns, pageWindow, toolResults } from '$lib/transcript/turns';
	import type { LoadedSession, TranscriptEntry } from '$lib/transcript/types';
	import type { Trace } from '$lib/types';
	import AttachmentBlock from './AttachmentBlock.svelte';
	import BranchTree from './BranchTree.svelte';
	import ChildCard from './ChildCard.svelte';
	import Markdown from './Markdown.svelte';
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

	let extra = $state<LoadedSession | null>(null);
	let extraKey = $state('');
	let error = $state('');
	let selection = $state<{ key: string; leaf: string; target: string } | null>(null);
	let pageStart = $state<number | undefined>(undefined);
	let thinkingExpanded = $state(false);
	let toolsExpanded = $state(false);
	let thinkingOpen = $state<Record<string, boolean>>({});
	let toolOpen = $state<Record<string, boolean>>({});
	let treeQuery = $state('');
	let treeOpen = $state(false);
	let loadingMore = $state(false);
	let copiedId = $state('');

	let sessionKey = $derived(`${trace.id}:${revision}`);
	let session = $derived(extraKey === sessionKey ? (extra ?? initial) : initial);
	let displayError = $derived(error || loadError);

	function idsFromURL(data: LoadedSession) {
		const params = page.url.searchParams;
		const urlLeaf = data.eventEntries.get(params.get('leafId') || '') || params.get('leafId') || '';
		const urlTarget =
			data.eventEntries.get(params.get('targetId') || '') ||
			params.get('targetId') ||
			data.eventEntries.get(params.get('event') || params.get('key') || '') ||
			'';
		const leaf = urlLeaf || data.leafId || data.entries.at(-1)?.id || '';
		return { leaf, target: urlTarget || leaf };
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

	let forked = $derived(session ? graphHasFork(session.entries) : false);
	let path = $derived(session && leafId ? getPath(session.entries, leafId) : []);
	let windowed = $derived(pageWindow(path, targetId, pageStart));
	let visible = $derived(path.slice(windowed.from, windowed.to));
	let results = $derived(toolResults(path));
	let turns = $derived(groupTurns(visible));
	let children = $derived(trace.children ?? []);
	let leftover = $derived.by(() => {
		const ids = new Set<string>();
		for (const entry of visible) {
			for (const block of entry.message?.content ?? []) {
				if (block.type !== 'toolCall') continue;
				for (const card of childCards(entry, block, results.get(block.id), children)) ids.add(card.id);
			}
			for (const card of childCards(entry, undefined, undefined, children)) ids.add(card.id);
		}
		return unplacedChildren(children, ids);
	});

	function isEditable(element: EventTarget | null) {
		if (!(element instanceof HTMLElement)) return false;
		const tag = element.tagName;
		if (tag === 'INPUT' || tag === 'TEXTAREA' || tag === 'SELECT') return true;
		return element.isContentEditable;
	}

	function onKeydown(event: KeyboardEvent) {
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
		return id in thinkingOpen ? thinkingOpen[id] : thinkingExpanded;
	}

	function toolIsOpen(id: string) {
		return id in toolOpen ? toolOpen[id] : toolsExpanded;
	}

	function selectNode(entryId: string) {
		if (!session) return;
		const leaf = findNewestLeaf(session.entries, entryId);
		selection = { key: sessionKey, leaf, target: entryId };
		pageStart = undefined;
		treeOpen = false;
		const params = new URLSearchParams(page.url.searchParams);
		params.set('leafId', leaf);
		params.set('targetId', entryId);
		const href = resolve('/traces/[id]', { id: String(trace.id) });
		history.replaceState(history.state, '', `${href}?${params}`);
	}

	async function copyLink(entryId: string) {
		const url = new URL(page.url.href);
		url.searchParams.set('leafId', leafId);
		url.searchParams.set('targetId', entryId);
		if (revision) url.searchParams.set('revision', revision);
		else url.searchParams.delete('revision');
		try {
			await navigator.clipboard.writeText(url.toString());
			copiedId = entryId;
			setTimeout(() => {
				if (copiedId === entryId) copiedId = '';
			}, 1200);
		} catch {
			// Clipboard can be unavailable over plain HTTP.
		}
	}

	async function loadMore() {
		if (!session || loadingMore) return;
		loadingMore = true;
		try {
			extra = await loadMoreSession(session);
			extraKey = sessionKey;
			error = '';
		} catch (err) {
			error = err instanceof Error ? err.message : 'Could not load more records';
		} finally {
			loadingMore = false;
		}
	}

	function originLine(entry: TranscriptEntry) {
		const origin = entry.message?.nativeDetails?.message_meta;
		if (!origin) return '';
		if (origin.openAIResponsePhase === 'final_answer') return 'Final answer';
		return origin.openAIResponsePhase || '';
	}

	function incomingCards(entry: TranscriptEntry) {
		return childCards(entry, undefined, undefined, children);
	}

	function shouldRenderResult(entry: TranscriptEntry) {
		const id = entry.message?.toolCallId;
		return !id || !visible.some((item) => item.message?.content.some((block) => block.type === 'toolCall' && block.id === id));
	}
</script>

<svelte:window onkeydown={onKeydown} />

<WorkspaceHeader
	{trace}
	{revision}
	tab="conversation"
	recordCount={session?.entries.length ?? 0}
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
	<p class="status" role="status">{session.status}</p>
	<div class={['body', forked && 'has-tree']}>
		{#if forked}
			<button class="tree-toggle" type="button" onclick={() => (treeOpen = !treeOpen)}>
				{treeOpen ? 'Hide branches' : 'Branches'}
			</button>
			<div class={['tree-slot', treeOpen && 'is-open']}>
				<BranchTree entries={session.entries} {leafId} {targetId} bind:query={treeQuery} onSelect={selectNode} />
			</div>
		{/if}
		<div class="stream">
			{#if windowed.hasEarlier}
				<button class="page" type="button" onclick={() => (pageStart = Math.max(0, windowed.from - 200))}>Earlier loaded messages</button>
			{/if}
			{#each turns as turn (turn.id)}
				<section class="turn" id="entry-{turn.id}">
					{#each turn.entries as entry (entry.id)}
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
						{:else if message}
							<article class="message" class:is-target={entry.id === targetId} id="entry-{entry.id}">
								<div class="who">
									<span>{message.role}{#if originLine(entry)} · {originLine(entry)}{/if}</span>
									{#if entry.timestamp}<time>{entry.timestamp}</time>{/if}
									<button class={['copy', copiedId === entry.id && 'is-copied']} type="button" title="Copy link to this message" onclick={() => copyLink(entry.id)}>
										{copiedId === entry.id ? 'Copied' : 'Copy link'}
									</button>
								</div>
								{#if message.nativeDetails?.message_meta?.fromAutomation}
									<p class="note">Automation message</p>
								{/if}
								{#each incomingCards(entry) as child (child.id)}
									<ChildCard {child} />
								{/each}
								{#each message.content as block, index (`${entry.id}:${index}:${block.type}`)}
									{#if block.type === 'text'}
										{#if message.role === 'user' && parseSkillBlock(block.text)}
											{@const skill = parseSkillBlock(block.text)}
											{#if skill}
												<details>
													<summary>Skill: {skill.name}</summary>
													<Markdown text={skill.content} />
												</details>
												{#if skill.userMessage}
													<Markdown text={skill.userMessage} />
												{/if}
											{/if}
										{:else}
											<Markdown text={block.text} />
										{/if}
									{:else if block.type === 'hidden'}
										<details>
											<summary>Hidden context</summary>
											<pre>{block.text}</pre>
										</details>
									{:else if block.type === 'thinking'}
										<details
											class="thinking"
											open={thinkingIsOpen(`${entry.id}:${index}`)}
											ontoggle={(event) => {
												if (event.currentTarget.open !== thinkingIsOpen(`${entry.id}:${index}`)) {
													thinkingOpen[`${entry.id}:${index}`] = event.currentTarget.open;
												}
											}}
										>
											<summary>Thinking</summary>
											<div class="think">{block.thinking || 'Reasoning text not included in export.'}</div>
										</details>
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
						{:else if entry.type === 'model_change'}
							<p class="note" id="entry-{entry.id}">Switched to model: {[entry.provider, entry.modelId].filter(Boolean).join('/')}</p>
						{:else if entry.type === 'thinking_level_change'}
							<p class="note" id="entry-{entry.id}">Thinking level: {entry.thinkingLevel}</p>
						{:else if entry.type === 'compaction' || entry.type === 'branch_summary'}
							<details class="note" id="entry-{entry.id}">
								<summary>{entry.type === 'compaction' ? 'Compaction summary' : 'Branch summary'}</summary>
								<Markdown text={entry.summary || ''} />
							</details>
						{:else if entry.type === 'custom_message'}
							<section class="note" id="entry-{entry.id}">
								<strong>{entry.customType === 'unknown' ? 'Unsupported content · ' : ''}{entry.sources?.[0]?.details?.native_type || entry.customType}</strong>
								{#if entry.customType === 'unknown'}
									<pre>{entry.content}</pre>
								{:else}
									<Markdown text={entry.content || ''} />
								{/if}
							</section>
						{/if}
					{/each}
				</section>
			{/each}
			{#each leftover as child (child.native_trace_id)}
				<ChildCard child={{ id: child.native_trace_id, title: child.title || child.native_trace_id, collected: true, traceId: child.id, meta: 'Child trace' }} />
			{/each}
			{#if windowed.hasLater}
				<button class="page" type="button" onclick={() => (pageStart = windowed.to)}>Later loaded messages</button>
			{/if}
			{#if session.hasMore}
				<button class="page" type="button" onclick={loadMore} disabled={loadingMore}>{loadingMore ? 'Loading…' : 'Load next 200 records'}</button>
			{/if}
			{#if !session.entries.length}
				<p class="empty">No normalized messages. Open Details to inspect the retained source files.</p>
			{/if}
		</div>
	</div>
{/if}

<style>
	.status,
	.empty {
		margin: 0;
		padding: 0.65rem 1.25rem;
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

	.tree-toggle {
		display: inline-flex;
		align-self: flex-start;
		margin: 0.55rem 1.25rem 0;
		border: 1px solid var(--border);
		border-radius: var(--radius);
		background: transparent;
		color: var(--muted-foreground);
		padding: 0.25rem 0.55rem;
		font-size: 11px;
		cursor: pointer;
	}

	.tree-slot {
		display: none;
		max-height: 40vh;
		overflow: hidden;
	}

	.tree-slot.is-open {
		display: block;
	}

	.stream {
		min-width: 0;
		flex: 1;
		padding: 0.75rem 1.25rem 2.5rem;
	}

	.turn {
		max-width: 76ch;
	}

	.message {
		padding: 0.85rem 0 1rem;
		border-bottom: 1px solid var(--border);
	}

	.message.is-target {
		background: color-mix(in oklch, var(--primary) 8%, transparent);
	}

	.who {
		display: flex;
		flex-wrap: wrap;
		align-items: baseline;
		gap: 0.55rem;
		margin-bottom: 0.35rem;
		color: var(--muted-foreground);
		font-family: var(--font-mono);
		font-size: 11px;
		font-variant-numeric: tabular-nums;
	}

	.copy {
		margin-left: auto;
		border: 0;
		background: transparent;
		color: var(--muted-foreground);
		font: inherit;
		cursor: pointer;
	}

	.copy:hover,
	.copy:focus-visible,
	.copy.is-copied {
		color: var(--primary);
	}

	.note,
	.thinking,
	details {
		margin: 0.4rem 0;
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

	.page {
		display: inline-flex;
		margin: 0.75rem 0;
		border: 1px solid var(--border);
		border-radius: var(--radius);
		background: transparent;
		color: var(--muted-foreground);
		padding: 0.3rem 0.6rem;
		font-size: 12px;
		cursor: pointer;
	}

	@media (min-width: 900px) {
		.body.has-tree {
			flex-direction: row;
			align-items: stretch;
		}

		.tree-toggle {
			display: none;
		}

		.has-tree .tree-slot {
			display: block;
			width: 16.5rem;
			max-height: none;
			flex-shrink: 0;
		}

		.stream {
			padding: 0.9rem 1.75rem 2.75rem;
		}
	}
</style>
