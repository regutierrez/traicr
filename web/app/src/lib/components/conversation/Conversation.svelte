<script lang="ts">
	import { tick } from 'svelte';
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
		promptOutline,
		findHits,
		toolResults,
		withoutSuggestedReplies,
		type StreamFilter
	} from '$lib/transcript/turns';
	import type { LoadedSession, TranscriptEntry } from '$lib/transcript/types';
	import { searchExpression } from '$lib/highlight';
	import { dateAndTime } from '$lib/format';
	import type { Trace } from '$lib/types';
	import AttachmentBlock from './AttachmentBlock.svelte';
	import ChildCard from './ChildCard.svelte';
	import DebugPanel from './DebugPanel.svelte';
	import ExpandChip from './ExpandChip.svelte';
	import Icon from './Icon.svelte';
	import Markdown from './Markdown.svelte';
	import PromptRail from './PromptRail.svelte';
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
	let linkedId = $derived.by(() => {
		if (!session) return '';
		if (selection?.key === sessionKey) return selection.target;
		return idsFromURL(session).target;
	});

	let path = $derived(session && leafId ? getPath(session.entries, leafId) : []);
	// A tool result shows inside its call's row, so a link to the result (such as a search match in
	// tool output) targets the message holding the call and opens that tool.
	let targetCall = $derived.by(() => {
		const message = path.find((entry) => entry.id === linkedId)?.message;
		return message?.role === 'toolResult' ? (message.toolCallId ?? '') : '';
	});
	let targetId = $derived.by(() => {
		const call = targetCall;
		if (!call) return linkedId;
		const owner = path.find((entry) => (entry.message?.content ?? []).some((block) => block.type === 'toolCall' && block.id === call));
		return owner?.id ?? linkedId;
	});
	// A message link whose message is not on the shown path would otherwise open at the top without a word.
	let missingTarget = $derived.by(() => {
		const params = page.url.searchParams;
		if (!session || !(params.get('targetId') || params.get('event') || params.get('key'))) return '';
		if (linkedId && path.some((entry) => entry.id === linkedId)) return '';
		return linkedId && session.entries.some((entry) => entry.id === linkedId)
			? 'That message is on another branch of this session, not the one shown here.'
			: 'That message is not in this session. Showing the start.';
	});
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
	let prompts = $derived(promptOutline(rows));
	let stream = $state<HTMLElement | null>(null);
	let currentPrompt = $state(-1);
	let scrollFrame = 0;

	function promptTops() {
		return stream ? [...stream.querySelectorAll('[data-prompt]')].map((element) => element.getBoundingClientRect().top) : [];
	}

	// The prompt being read is the last one that starts above a reading line a third of the way
	// down. At the end of the trace the line drops to the bottom, since the last prompts cannot
	// scroll any higher.
	function updateCurrentPrompt() {
		if (!stream) return;
		const atEnd = stream.scrollTop + stream.clientHeight >= stream.scrollHeight - 2;
		const line = stream.getBoundingClientRect().top + stream.clientHeight * (atEnd ? 1 : 1 / 3);
		currentPrompt = promptTops().findLastIndex((top) => top <= line);
	}

	function onStreamScroll() {
		cancelAnimationFrame(scrollFrame);
		scrollFrame = requestAnimationFrame(updateCurrentPrompt);
	}

	$effect(() => {
		void shownRows;
		void tick().then(updateCurrentPrompt);
	});

	// Paints rows up to `rowIndex` if needed, then scrolls to the element. Rows that have not
	// painted yet are sized by estimate, and the rows near the landing spot change size once they
	// paint, so it scrolls again until the position holds.
	async function reveal(rowIndex: number, find: (root: HTMLElement) => Element | null, block: ScrollLogicalPosition) {
		if (!stream) return;
		if (rowIndex >= shownCount) {
			grown = { rows, count: rowIndex + MORE_ROWS };
			await tick();
		}
		const element = find(stream);
		let last = -1;
		for (let attempt = 0; element && attempt < 8 && stream.scrollTop !== last; attempt += 1) {
			last = stream.scrollTop;
			element.scrollIntoView({ block });
			await new Promise((done) => requestAnimationFrame(() => requestAnimationFrame(done)));
		}
		focusHit(element);
	}

	// With a search active, the first hit inside the revealed message is marked as the current
	// one and scrolled into view when the message is taller than the screen.
	function focusHit(element: Element | null) {
		const expression = findExpression;
		if (!stream || typeof CSS === 'undefined' || !('highlights' in CSS)) return;
		CSS.highlights.delete('trace-find-current');
		if (!element || !expression) return;
		const walker = document.createTreeWalker(element, NodeFilter.SHOW_TEXT);
		for (let node = walker.nextNode(); node; node = walker.nextNode()) {
			expression.lastIndex = 0;
			const match = expression.exec(node.nodeValue ?? '');
			if (!match?.[0]) continue;
			const range = new Range();
			range.setStart(node, match.index);
			range.setEnd(node, match.index + match[0].length);
			CSS.highlights.set('trace-find-current', new Highlight(range));
			const box = range.getBoundingClientRect();
			const view = stream.getBoundingClientRect();
			if (box.bottom > view.bottom - 40) stream.scrollTop += box.top - view.top - view.height / 3;
			return;
		}
	}

	function jumpToPrompt(index: number) {
		const id = prompts[index]?.id;
		if (!id) return;
		void reveal(rows.findIndex((row) => row.id === id), (root) => root.querySelector(`[data-prompt="${CSS.escape(id)}"]`), 'start');
	}

	// A link to one message (?targetId=) opens with that message in view, once per target.
	let revealedTarget = '';
	$effect(() => {
		const target = targetId;
		const key = `${sessionKey}:${target}`;
		if (!target || !stream || revealedTarget === key) return;
		const rowIndex = rows.findIndex((row) => row.entries.some((entry) => entry.id === target));
		if (rowIndex === -1) return;
		revealedTarget = key;
		void reveal(rowIndex, (root) => root.querySelector('.row.is-target') ?? root.querySelector(`#entry-${CSS.escape(target)}`), 'start');
	});

	function messageHref(entryId: string) {
		const params = new URLSearchParams();
		if (revision) params.set('revision', revision);
		if (leaves.length > 1) params.set('leafId', leafId);
		params.set('targetId', entryId);
		return `${resolve('/traces/[id]/trace', { id: String(trace.id) })}?${params}`;
	}

	// Clicking a message's time marks it and puts its link in the address bar, without reloading.
	function linkMessage(event: MouseEvent, entryId: string) {
		if (event.button !== 0 || event.metaKey || event.ctrlKey || event.shiftKey || event.altKey) return;
		event.preventDefault();
		revealedTarget = `${sessionKey}:${entryId}`;
		selection = { key: sessionKey, leaf: leafId, target: entryId };
		history.replaceState(history.state, '', messageHref(entryId));
	}

	// A search result opens here with ?q= and ?mode=: its hits are marked in the text, and the
	// find bar steps through every message (or tool call) that matched, opening folds and tools.
	// Steps and Clear belong to one trace and query; opening another trace starts fresh.
	let findQuery = $derived(page.url.searchParams.get('q') ?? '');
	let findMode = $derived(page.url.searchParams.get('mode') ?? 'fulltext');
	let findKey = $derived(`${sessionKey}:${findMode}:${findQuery}`);
	let findCleared = $state('');
	let findExpression = $derived(findQuery && findCleared !== findKey ? searchExpression(findQuery, findMode) : null);
	let hits = $derived(session && findExpression ? findHits(filtered, findExpression) : []);
	let findStep = $state.raw<{ key: string; index: number } | null>(null);
	let findIndex = $derived(findStep?.key === findKey ? findStep.index : -1);
	// Until the reader steps, the current hit is the message the link opened.
	let hitIndex = $derived(findIndex >= 0 ? findIndex : hits.findIndex((hit) => hit.target === linkedId));
	// The current hit's tool opens, so a match in its arguments or output shows.
	let findCall = $derived(hits[hitIndex]?.call ?? '');
	async function stepFind(direction: 1 | -1) {
		const count = hits.length;
		if (!count) return;
		const index = hitIndex < 0 ? (direction === 1 ? 0 : count - 1) : (hitIndex + direction + count) % count;
		const hit = hits[index];
		findStep = { key: findKey, index };
		selection = { key: sessionKey, leaf: leafId, target: hit.target };
		const owner = targetId;
		revealedTarget = `${sessionKey}:${owner}`;
		const params = new URLSearchParams(page.url.searchParams);
		params.delete('event');
		params.set('targetId', hit.target);
		history.replaceState(history.state, '', `${resolve('/traces/[id]/trace', { id: String(trace.id) })}?${params}`);
		await tick();
		const rowIndex = rows.findIndex((row) => row.entries.some((entry) => entry.id === owner));
		void reveal(
			rowIndex,
			(root) =>
				(hit.call && root.querySelector(`#tool-call-${CSS.escape(hit.call)}`)) ||
				root.querySelector('.is-target-block') ||
				root.querySelector('.row.is-target'),
			'start'
		);
	}

	function clearFind() {
		findCleared = findKey;
		const params = new URLSearchParams(page.url.searchParams);
		params.delete('q');
		params.delete('mode');
		const href = resolve('/traces/[id]/trace', { id: String(trace.id) });
		history.replaceState(history.state, '', params.size ? `${href}?${params}` : href);
	}

	// Marks every hit in the rendered text with the CSS Custom Highlight API, which paints
	// ranges without changing the DOM. It repaints when rows grow or folds and tools open.
	$effect(() => {
		const expression = findExpression;
		const root = stream;
		if (!expression || !root || typeof CSS === 'undefined' || !('highlights' in CSS)) return;
		let frame = 0;
		const paint = () => {
			const ranges: Range[] = [];
			const walker = document.createTreeWalker(root, NodeFilter.SHOW_TEXT);
			for (let node = walker.nextNode(); node && ranges.length < 5000; node = walker.nextNode()) {
				// matchAll starts from the expression's lastIndex, which findHits may have moved.
				expression.lastIndex = 0;
				for (const match of (node.nodeValue ?? '').matchAll(expression)) {
					if (!match[0]) continue;
					const range = new Range();
					range.setStart(node, match.index);
					range.setEnd(node, match.index + match[0].length);
					ranges.push(range);
				}
			}
			CSS.highlights.set('trace-find', new Highlight(...ranges));
		};
		const schedule = () => {
			cancelAnimationFrame(frame);
			frame = requestAnimationFrame(paint);
		};
		schedule();
		const observer = new MutationObserver(schedule);
		observer.observe(root, { childList: true, subtree: true, characterData: true });
		return () => {
			observer.disconnect();
			cancelAnimationFrame(frame);
			CSS.highlights.delete('trace-find');
			CSS.highlights.delete('trace-find-current');
		};
	});

	// The next prompt starts below the top edge; the previous one starts above it.
	function stepPrompt(direction: 1 | -1) {
		if (!stream) return;
		const top = stream.getBoundingClientRect().top;
		const tops = promptTops();
		const index = direction === 1 ? tops.findIndex((y) => y > top + 24) : tops.findLastIndex((y) => y < top + 8);
		void jumpToPrompt(index === -1 && direction === 1 ? tops.length : index);
	}

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
		if (key === 'e') {
			event.preventDefault();
			toggleSteps();
		} else if (key === 'n' || key === 'p') {
			event.preventDefault();
			stepPrompt(key === 'n' ? 1 : -1);
		} else if (findExpression && (event.key === '.' || event.key === ',')) {
			event.preventDefault();
			void stepFind(event.key === '.' ? 1 : -1);
		} else if (findExpression && event.key === 'Escape') {
			clearFind();
		}
	}

	function toggleSteps() {
		stepsExpanded = !stepsExpanded;
		workOpen = {};
	}

	// A linked message opens the fold it sits in.
	function workIsOpen(row: { id: string; entries: TranscriptEntry[] }) {
		return workOpen[row.id] ?? (stepsExpanded || row.entries.some((entry) => entry.id === targetId));
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

	// While stepping through search hits, the current message's collapsed parts open so the hit shows.
	function thinkingIsOpen(id: string) {
		return thinkingOpen[id] ?? (Boolean(findExpression) && id.slice(0, id.lastIndexOf(':')) === targetId);
	}

	function toolIsOpen(id: string) {
		return toolOpen[id] ?? (id === targetCall || id === findCall);
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

	function automationText(entries: TranscriptEntry[]) {
		return entries
			.flatMap((entry) => entry.message?.content ?? [])
			.flatMap((block) => (block.type === 'text' ? [block.text] : []))
			.join('\n\n')
			.trim();
	}

	function sentAt(entries: TranscriptEntry[]) {
		return entries.find((entry) => entry.timestamp)?.timestamp ?? '';
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
		model={facts.model ?? ''}
		thinkingLevel={facts.thinkingLevel ?? ''}
		{stepsExpanded}
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
				<div class={['block', entry.id === targetId && 'is-target-block']} id="entry-{entry.id}">
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
				<article class={['block', entry.id === targetId && 'is-target-block']} id="entry-{entry.id}">
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
								<Markdown text={message.role === 'assistant' ? withoutSuggestedReplies(block.text) : block.text} />
							{/if}
						{:else if block.type === 'hidden'}
							<ExpandChip icon="compaction" verb={message.role === 'system' ? 'System message' : 'Hidden context'}>
								<pre>{block.text}</pre>
							</ExpandChip>
						{:else if block.type === 'thinking'}
							{@const thinkKey = `${entry.id}:${index}`}
							{#if !block.thinking}
								<ExpandChip icon="brain" verb="Thought" title="The export left out the reasoning text." />
							{:else}
							<ExpandChip
								icon="brain"
								verb="Thought"
								open={thinkingIsOpen(thinkKey)}
								onToggle={(next) => {
									if (next !== thinkingIsOpen(thinkKey)) thinkingOpen[thinkKey] = next;
								}}
							>
								<div class="think">{block.thinking}</div>
							</ExpandChip>
							{/if}
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
				</article>
			{:else if entry.type === 'compaction' || entry.type === 'branch_summary'}
				<div id="entry-{entry.id}">
					<ExpandChip
						icon={entry.type === 'compaction' ? 'compaction' : 'branch'}
						verb={entry.type === 'compaction' ? 'Compaction' : 'Branch summary'}
						open={thinkingIsOpen(`${entry.id}:summary`)}
						onToggle={(next) => (thinkingOpen[`${entry.id}:summary`] = next)}
					>
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
			{#if missingTarget}
				<p class="link-notice" role="status">{missingTarget}</p>
			{/if}
			{#if findExpression}
				<div class="find-bar" role="search" aria-label="Matches in this trace">
					<span class="find-query">“{findQuery}”</span>
					<span class="find-count" aria-live="polite">
						{#if !hits.length}No matching messages on this path{:else if hitIndex >= 0}{hitIndex + 1} of {hits.length}{:else}{hits.length} {hits.length === 1 ? 'message' : 'messages'}{/if}
					</span>
					{#if hits.length}
						<button type="button" title="Previous match (,)" aria-label="Previous match" onclick={() => stepFind(-1)}><span class="up"><Icon name="chevron" size={13} /></span></button>
						<button type="button" title="Next match (.)" aria-label="Next match" onclick={() => stepFind(1)}><span class="down"><Icon name="chevron" size={13} /></span></button>
					{/if}
					<button type="button" class="find-close" title="Stop marking matches (Esc)" onclick={clearFind}>Clear</button>
				</div>
			{/if}
			{#if prompts.length > 1}
				<PromptRail items={prompts} current={currentPrompt} onSelect={jumpToPrompt} onStep={stepPrompt} />
			{/if}
			{#key sessionKey}
				<div class="stream" bind:this={stream} onscroll={onStreamScroll}>
					{#if filter !== 'all' || leaves.length > 1 || revision}
						<div class="view-bar">
							{#if revision}
								<span>Showing revision {revision} only</span>
								<a href={resolve('/traces/[id]/trace', { id: String(trace.id) })}>Show merged archive</a>
							{/if}
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
						{:else if row.chrome === 'automation'}
							<!-- Amp wrote this on the user's behalf; it is a note in the flow, not a prompt. -->
							<section class={['row', 'is-automation', row.entries.some((entry) => entry.id === targetId) && 'is-target']} id="entry-{row.id}">
								<ExpandChip
									icon="agent"
									verb="Automation"
									rest={automationText(row.entries).split('\n')[0]}
									open={thinkingIsOpen(`${row.id}:automation`)}
									onToggle={(next) => (thinkingOpen[`${row.id}:automation`] = next)}
								>
									<div class="automation-body"><Markdown text={automationText(row.entries)} /></div>
								</ExpandChip>
							</section>
						{:else}
						<section class={['row', `is-${row.chrome}`, row.entries.some((entry) => entry.id === targetId) && 'is-target']} data-prompt={row.chrome === 'user' ? row.id : undefined}>
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
							{#if row.chrome !== 'quiet' && sentAt(row.entries)}
								<a class="row-time" href={messageHref(row.entries[0].id)} title="Link to this message" onclick={(event) => linkMessage(event, row.entries[0].id)}>
									<time datetime={sentAt(row.entries)}>{dateAndTime(sentAt(row.entries))}</time>
								</a>
							{/if}
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
		position: relative;
		display: flex;
		min-height: 0;
		flex: 1;
		flex-direction: column;
		/* The prompt rail shows only when this area leaves a gutter beside the column. */
		container-type: inline-size;
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
		flex-direction: column;
		align-items: flex-end;
		margin-block: 2rem 1rem;
	}

	.row,
	[id^='entry-'] {
		scroll-margin-top: 1rem;
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

	.row.is-automation {
		margin-block: 1rem;
	}

	.automation-body {
		margin: 0.25rem 0 0.5rem;
		padding-left: 0.9rem;
		border-left: 1px solid var(--border);
		color: var(--muted-foreground);
		font-size: 13px;
	}

	.row.is-work {
		margin-block: 0.35rem;
	}

	/* When a message was sent shows only while it is hovered, like T3 Code; touch screens always show it. */
	.row-time {
		display: block;
		width: fit-content;
		margin-top: 0.2rem;
		color: var(--muted-foreground);
		font-size: 11px;
		text-decoration: none;
		opacity: 0;
		transition: opacity 150ms ease;
	}

	.row-time:hover,
	.row-time:focus-visible {
		color: var(--foreground);
		text-decoration: underline;
		text-underline-offset: 2px;
	}

	.row:hover .row-time,
	.row:focus-within .row-time {
		opacity: 1;
	}

	@media (pointer: coarse) {
		.row-time {
			opacity: 1;
		}
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

	/* Inside an open work row the linked step gets the outline, since the row itself is a fold. */
	.work-steps .is-target-block {
		border-radius: 4px;
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

	.link-notice {
		width: min(46rem, calc(100% - 2.5rem));
		margin: 0.6rem auto 0;
		padding: 0.45rem 0.75rem;
		border: 1px solid var(--border);
		border-radius: var(--radius);
		background: var(--card);
		color: var(--muted-foreground);
		font-size: 12px;
	}

	/* Above the stream, so it stays in view while the reader steps through matches. */
	.find-bar {
		display: flex;
		width: min(46rem, calc(100% - 2.5rem));
		align-items: center;
		gap: 0.35rem;
		margin: 0.6rem auto 0;
		padding: 0.3rem 0.4rem 0.3rem 0.75rem;
		border: 1px solid var(--border);
		border-radius: var(--radius);
		background: var(--card);
		color: var(--muted-foreground);
		font-size: 12px;
		font-variant-numeric: tabular-nums;
	}

	.find-query {
		overflow: hidden;
		color: var(--foreground);
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	.find-count {
		flex-shrink: 0;
		margin-right: auto;
		padding-left: 0.35rem;
	}

	.find-bar button {
		display: inline-flex;
		height: 1.6rem;
		min-width: 1.6rem;
		flex-shrink: 0;
		align-items: center;
		justify-content: center;
		border: 0;
		border-radius: 6px;
		background: transparent;
		color: var(--muted-foreground);
		padding: 0 0.4rem;
		font: inherit;
		cursor: pointer;
	}

	.find-bar button:hover,
	.find-bar button:focus-visible {
		background: var(--accent);
		color: var(--foreground);
	}

	.find-bar .up {
		display: inline-flex;
		transform: rotate(-90deg);
	}

	.find-bar .down {
		display: inline-flex;
		transform: rotate(90deg);
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

	.view-bar a,
	.view-bar button {
		text-decoration: none;
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
