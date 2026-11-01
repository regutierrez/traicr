<script lang="ts">
	import AttachmentBlock from './AttachmentBlock.svelte';
	import ChildCard from './ChildCard.svelte';
	import Markdown from './Markdown.svelte';
	import { childCards } from '$lib/transcript/children';
	import { highlightCode, languageForPath } from '$lib/transcript/markdown';
	import {
		isFileTool,
		isShellTool,
		isStructuredOutput,
		requestedEdit,
		resultFiles,
		resultText,
		toolChipParts,
		toolCommand,
		toolPath,
		toolKind,
		toolStatus
	} from '$lib/transcript/tools';
	import ExpandChip from './ExpandChip.svelte';
	import type { ToolCallBlock, TranscriptEntry } from '$lib/transcript/types';
	import type { TraceRef } from '$lib/types';

	let {
		call,
		result,
		entry,
		children = [],
		open = false,
		onToggle
	}: {
		call: ToolCallBlock;
		result?: TranscriptEntry;
		entry: TranscriptEntry;
		children?: TraceRef[];
		open?: boolean;
		onToggle?: (open: boolean) => void;
	} = $props();

	let status = $derived(toolStatus(call, result));
	let chip = $derived(toolChipParts(call));
	// An export is a snapshot: a process running then may have finished long ago.
	let statusLabel = $derived(status === 'running' ? 'still running when recorded' : status !== 'unknown' && status !== 'done' ? status : '');
	let args = $derived(call.arguments || {});
	let command = $derived(toolCommand(args));
	let path = $derived(toolPath(args));
	let output = $derived(resultText(result));
	let structured = $derived(isStructuredOutput(output));
	let files = $derived(resultFiles(result?.message?.run?.result));
	let edit = $derived(requestedEdit(args));
	let cards = $derived(childCards(entry, call, result, children));
	let payload = $derived(result?.message?.run?.result);
	let attachments = $derived((result?.message?.content ?? []).filter((block) => block.type === 'attachment'));
	let highlightLanguage = $derived(
		isFileTool(call.name) && call.name.toLowerCase() === 'read' ? languageForPath(path) : ''
	);
</script>

<div class={['tool', result?.message?.isError && 'is-error']} id="tool-call-{call.id}">
	<ExpandChip
		icon={chip.icon}
		verb={chip.verb}
		rest={chip.rest}
		status={statusLabel}
		{open}
		{onToggle}
	>
		{#if isShellTool(call.name) && command}
			<pre class="command">$ {command}</pre>
		{:else if isFileTool(call.name) && path}
			<p class="field" translate="no">
				{path}{#if args.offset != null} · from line {args.offset}{/if}{#if args.limit != null} · {args.limit} lines{/if}
			</p>
		{:else if Object.keys(args).length}
			<!-- An unknown tool has nothing more readable than what it was asked to do. -->
			<pre>{JSON.stringify(args, null, 2)}</pre>
		{/if}
		{#if typeof args.content === 'string' && toolKind(call.name) === 'write'}
			<pre class="code"><code class="hljs">{@html highlightCode(args.content, languageForPath(path))}</code></pre>
		{/if}
		{#if edit.length && !files.some((file) => file.diff.length)}
			<pre class="diff">{#each edit as line, index (`${index}:${line.text}`)}<span class={line.kind}>{line.text + '\n'}</span>{/each}</pre>
		{/if}
		{#each files as file, index (`${file.label}:${index}`)}
			{#if file.label}<p class="field" translate="no">
				{file.label}{#if file.additions != null || file.deletions != null}<span class="added-count"> +{file.additions ?? '?'}</span><span class="removed-count"> −{file.deletions ?? '?'}</span>{/if}
			</p>{/if}
			{#if file.diff.length}
				<pre class="diff">{#each file.diff as line, lineIndex (`${lineIndex}:${line.text}`)}<span class={line.kind}>{line.text + '\n'}</span>{/each}</pre>
			{/if}
		{/each}
		{#if result}
			{#if typeof payload?.exitCode === 'number' && payload.exitCode !== 0}
				<p class="field failed">Exited with code {payload.exitCode}</p>
			{/if}
			{#if output && !files.some((file) => file.diff.length)}
				{#if isShellTool(call.name) || isFileTool(call.name) || structured}
					<pre class="code"><code class="hljs">{@html highlightCode(output, structured ? 'json' : highlightLanguage)}</code></pre>
				{:else}
					<div class="output"><Markdown text={output} /></div>
				{/if}
			{:else if payload && typeof payload === 'object' && !files.length}
				<pre>{JSON.stringify(payload, null, 2)}</pre>
			{:else if !files.length && !attachments.length}
				<p class="field">No output recorded.</p>
			{/if}
			{#each attachments as attachment, index (`${attachment.path ?? attachment.url ?? index}`)}
				<AttachmentBlock {attachment} />
			{/each}
		{/if}
	</ExpandChip>
	{#each cards as child (child.id)}
		<ChildCard {child} />
	{/each}
</div>

<style>
	.tool {
		margin: 0;
	}

	.tool :global(.status) {
		font-size: 11px;
	}

	.is-error :global(.status) {
		color: var(--destructive);
	}

	.field {
		margin: 0.25rem 0;
		color: var(--muted-foreground);
		font-family: var(--font-mono);
		font-size: 11px;
	}

	pre {
		overflow: auto;
		max-height: 28rem;
		margin: 0.3rem 0 0.5rem;
		padding: 0.6rem 0.75rem;
		border: 1px solid var(--border);
		border-radius: var(--radius);
		background: var(--card);
		font-family: var(--font-mono);
		font-size: 12px;
		white-space: pre-wrap;
		overflow-wrap: anywhere;
	}

	.command {
		color: var(--foreground);
	}

	.failed,
	.removed-count {
		color: var(--destructive);
	}

	.added-count {
		color: #89d185;
	}

	.output {
		margin: 0.3rem 0 0.5rem;
		padding: 0.6rem 0.75rem;
		border-left: 2px solid var(--border);
		color: var(--muted-foreground);
		font-size: 13px;
	}

	.diff {
		display: grid;
		padding: 0.4rem 0;
	}

	.diff span {
		padding: 0 0.65rem;
	}

	.added {
		background: rgb(80 200 120 / 12%);
		color: #89d185;
	}

	.removed {
		background: rgb(241 76 76 / 12%);
		color: #f14c4c;
	}

	.hljs {
		padding: 0;
		background: transparent;
	}
</style>
