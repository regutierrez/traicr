<script lang="ts">
	import AttachmentBlock from './AttachmentBlock.svelte';
	import ChildCard from './ChildCard.svelte';
	import Markdown from './Markdown.svelte';
	import { childCards } from '$lib/transcript/children';
	import { highlightCode, languageForPath } from '$lib/transcript/markdown';
	import {
		isFileTool,
		isShellTool,
		requestedEdit,
		resultFiles,
		resultText,
		toolChipParts,
		toolCommand,
		toolPath,
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
	let statusLabel = $derived(status !== 'unknown' && status !== 'done' ? status : '');
	let args = $derived(call.arguments || {});
	let command = $derived(toolCommand(args));
	let path = $derived(toolPath(args));
	let output = $derived(resultText(result));
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
			<pre class="block command"><span class="sig">&gt;</span> {command}</pre>
			{#if typeof args.workdir === 'string' && args.workdir}
				<p class="field">Working directory · {args.workdir}</p>
			{/if}
		{/if}
		{#if isFileTool(call.name) && (args.offset != null || args.limit != null)}
			<p class="field">
				{#if args.offset != null}start {args.offset}{/if}
				{#if args.offset != null && args.limit != null} · {/if}
				{#if args.limit != null}limit {args.limit}{/if}
			</p>
		{/if}
		{#if typeof args.content === 'string' && call.name.toLowerCase() === 'write'}
			<pre class="block code"><code class="hljs">{@html highlightCode(args.content, languageForPath(path))}</code></pre>
		{/if}
		{#if edit.length}
			<pre class="block diff">{#each edit as line, index (`${index}:${line.text}`)}<span class={line.kind}>{line.text + '\n'}</span>{/each}</pre>
		{/if}
		{#if typeof payload?.pid === 'number'}
			<p class="field">Process ID · {payload.pid}</p>
		{/if}
		{#if typeof payload?.exitCode === 'number'}
			<p class="field">Exit code · {payload.exitCode}</p>
		{/if}
		{#if result}
			{#if output}
				{#if isShellTool(call.name)}
					<pre class="block terminal">{output}</pre>
				{:else if isFileTool(call.name)}
					<pre class="block terminal"><code class="hljs">{@html highlightCode(output, highlightLanguage)}</code></pre>
				{:else}
					<div class="prose"><Markdown text={output} /></div>
				{/if}
			{:else}
				<p class="field">No text output recorded.</p>
			{/if}
			{#each files as file, index (`${file.label}:${index}`)}
				<p class="field">{file.label}{#if file.additions != null || file.deletions != null} · +{file.additions ?? '?'} −{file.deletions ?? '?'}{/if}</p>
				{#if file.diff.length}
					<pre class="block diff">{#each file.diff as line, lineIndex (`${lineIndex}:${line.text}`)}<span class={line.kind}>{line.text + '\n'}</span>{/each}</pre>
				{/if}
			{/each}
			{#each attachments as attachment, index (`${attachment.path ?? attachment.url ?? index}`)}
				<AttachmentBlock {attachment} />
			{/each}
		{/if}
		<details class="extra">
			<summary>Arguments</summary>
			<pre class="block">{JSON.stringify(args, null, 2)}</pre>
		</details>
		{#if payload && typeof payload === 'object'}
			<details class="extra">
				<summary>Structured result</summary>
				<pre class="block">{JSON.stringify(payload, null, 2)}</pre>
			</details>
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

	.block {
		overflow: auto;
		max-height: 16rem;
		margin: 0.35rem 0;
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

	.command {
		white-space: pre-wrap;
	}

	.sig {
		margin-right: 0.45rem;
	}

	.extra {
		margin: 0.15rem 0 0.35rem;
	}

	.extra summary {
		width: fit-content;
		color: var(--muted-foreground);
		font-size: 12px;
		cursor: pointer;
	}

	.prose {
		margin: 0.35rem 0;
		color: var(--muted-foreground);
		font-size: 13px;
	}

	.diff {
		display: grid;
		padding: 4px 0;
	}

	.diff span {
		padding: 0 0.65rem;
	}

	.added {
		background: #e6f4ea;
		color: #137333;
	}

	.removed {
		background: #fce8e6;
		color: #c5221f;
	}

	.hljs {
		background: transparent;
	}
</style>
