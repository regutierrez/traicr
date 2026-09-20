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
		toolChipLabel,
		toolCommand,
		toolPath,
		toolStatus
	} from '$lib/transcript/tools';
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
	let summary = $derived(toolChipLabel(call));
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
	<details
		bind:open={() => open, (value) => {
			if (value !== open) onToggle?.(value);
		}}
	>
		<summary>
			<span class="name">{summary}</span>
			{#if status !== 'unknown' && status !== 'done'}
				<span class="status">{status}</span>
			{/if}
		</summary>
		{#if isShellTool(call.name) && command}
			<pre class="command">$ {command}</pre>
			{#if typeof args.workdir === 'string' && args.workdir}
				<p class="field">Working directory · {args.workdir}</p>
			{/if}
		{/if}
		{#if isFileTool(call.name) && path}
			<p class="field">
				File · {path}
				{#if args.offset != null} · start {args.offset}{/if}
				{#if args.limit != null} · limit {args.limit}{/if}
			</p>
		{/if}
		{#if call.name === 'skill' && typeof args.name === 'string'}
			<p class="field">Skill · {args.name}</p>
		{/if}
		{#if typeof args.content === 'string' && call.name.toLowerCase() === 'write'}
			<details>
				<summary>Requested content</summary>
				<pre class="code"><code class="hljs">{@html highlightCode(args.content, languageForPath(path))}</code></pre>
			</details>
		{/if}
		{#if edit.length}
			<details>
				<summary>Requested edit</summary>
				<pre class="diff">{#each edit as line, index (`${index}:${line.text}`)}<span class={line.kind}>{line.text + '\n'}</span>{/each}</pre>
			</details>
		{/if}
		<details>
			<summary>Arguments</summary>
			<pre>{JSON.stringify(args, null, 2)}</pre>
		</details>
		{#if typeof payload?.pid === 'number'}
			<p class="field">Process ID · {payload.pid}</p>
		{/if}
		{#if typeof payload?.exitCode === 'number'}
			<p class="field">Exit code · {payload.exitCode}</p>
		{/if}
		{#if result}
			{#if output}
				<details>
					<summary>Tool output</summary>
					{#if isShellTool(call.name) || isFileTool(call.name)}
						<pre class="code"><code class="hljs">{@html highlightCode(output, highlightLanguage)}</code></pre>
					{:else}
						<Markdown text={output} />
					{/if}
				</details>
			{:else}
				<p class="field">No text output recorded.</p>
			{/if}
			{#each files as file, index (`${file.label}:${index}`)}
				<details>
					<summary>{file.label}{#if file.additions != null || file.deletions != null} · +{file.additions ?? '?'} −{file.deletions ?? '?'}{/if}</summary>
					{#if file.diff.length}
						<pre class="diff">{#each file.diff as line, lineIndex (`${lineIndex}:${line.text}`)}<span class={line.kind}>{line.text + '\n'}</span>{/each}</pre>
					{/if}
				</details>
			{/each}
			{#each attachments as attachment, index (`${attachment.path ?? attachment.url ?? index}`)}
				<AttachmentBlock {attachment} />
			{/each}
			{#if payload && typeof payload === 'object'}
				<details>
					<summary>Structured result</summary>
					<pre>{JSON.stringify(payload, null, 2)}</pre>
				</details>
			{/if}
		{/if}
	</details>
	{#each cards as child (child.id)}
		<ChildCard {child} />
	{/each}
</div>

<style>
	.tool {
		margin: 0.35rem 0;
	}

	summary {
		display: flex;
		align-items: baseline;
		justify-content: space-between;
		gap: 0.75rem;
		cursor: pointer;
		padding: 0.28rem 0;
		color: var(--muted-foreground);
		font-family: var(--font-mono);
		font-size: 12px;
		list-style: none;
	}

	summary::-webkit-details-marker {
		display: none;
	}

	.name {
		overflow: hidden;
		color: var(--foreground);
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	.status {
		flex-shrink: 0;
		font-size: 11px;
	}

	.is-error .status {
		color: var(--destructive);
	}

	.field {
		margin: 0.25rem 0;
		color: var(--muted-foreground);
		font-family: var(--font-mono);
		font-size: 11px;
	}

	pre {
		overflow-x: auto;
		margin: 0.4rem 0;
		padding: 0.55rem 0.65rem;
		border: 1px solid var(--border);
		border-radius: var(--radius);
		background: #181818;
		font-family: var(--font-mono);
		font-size: 12px;
		white-space: pre-wrap;
	}

	.command {
		color: #d4d4d4;
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
		background: transparent;
	}
</style>
