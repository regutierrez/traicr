<script lang="ts">
	import type { ContentBlock, ConversationTurn, SessionEntry, SessionMessage } from '$lib/transcript-data';

	let {
		turn,
		showThinking,
		showTools
	}: {
		turn: ConversationTurn;
		showThinking: boolean;
		showTools: boolean;
	} = $props();

	function prose(blocks: ContentBlock[]) {
		return blocks.filter((block) => block.type === 'text');
	}

	function thinking(blocks: ContentBlock[]) {
		return blocks.filter((block) => block.type === 'thinking');
	}

	function tools(blocks: ContentBlock[]) {
		return blocks.filter((block) => block.type === 'toolCall');
	}

	function hidden(blocks: ContentBlock[]) {
		return blocks.filter((block) => block.type === 'hidden');
	}

	function attachments(blocks: ContentBlock[]) {
		return blocks.filter((block) => block.type === 'attachment');
	}

	function args(value: Record<string, unknown>) {
		return JSON.stringify(value, null, 2);
	}

	function resultText(message: SessionMessage) {
		return message.content
			.filter((block) => block.type === 'text')
			.map((block) => (block.type === 'text' ? block.text : ''))
			.join('\n');
	}
</script>

{#snippet attachment(block: ContentBlock)}
	{#if block.type === 'attachment'}
		<p class="text-muted-foreground text-xs break-all">{block.name || block.path || block.url || 'Attachment'}</p>
	{/if}
{/snippet}

{#snippet messageBlocks(message: SessionMessage)}
	{#each thinking(message.content) as block, index (`think-${index}`)}
		{#if block.type === 'thinking'}
			<details class="bg-muted/40 rounded-lg border px-3 py-2" open={showThinking}>
				<summary class="cursor-pointer text-sm font-medium">Thinking</summary>
				<p class="mt-2 whitespace-pre-wrap">{block.thinking || 'Reasoning text not included in export.'}</p>
			</details>
		{/if}
	{/each}
	{#each prose(message.content) as block, index (`text-${index}`)}
		{#if block.type === 'text' && block.text}
			<p class="whitespace-pre-wrap">{block.text}</p>
		{/if}
	{/each}
	{#each hidden(message.content) as block, index (`hidden-${index}`)}
		{#if block.type === 'hidden'}
			<details class="bg-muted/40 rounded-lg border px-3 py-2">
				<summary class="cursor-pointer text-sm font-medium">Hidden context</summary>
				<pre class="mt-2 font-mono text-xs whitespace-pre-wrap">{block.text}</pre>
			</details>
		{/if}
	{/each}
	{#each tools(message.content) as block (`tool-${block.type === 'toolCall' ? block.id : ''}`)}
		{#if block.type === 'toolCall'}
			<details class="bg-muted/40 rounded-lg border px-3 py-2" open={showTools}>
				<summary class="cursor-pointer font-mono text-sm font-medium">{block.name}</summary>
				<pre class="mt-2 font-mono text-xs whitespace-pre-wrap">{args(block.arguments)}</pre>
			</details>
		{/if}
	{/each}
	{#each attachments(message.content) as block, index (`attach-${index}`)}
		{@render attachment(block)}
	{/each}
{/snippet}

{#snippet follow(entry: SessionEntry)}
	{#if entry.type === 'message' && entry.message?.role === 'assistant'}
		<section class="flex flex-col gap-3" id="entry-{entry.id}">
			<p class="text-muted-foreground font-mono text-[11px] tracking-widest">ASSISTANT{#if entry.message.model} · {entry.message.model}{/if}</p>
			{@render messageBlocks(entry.message)}
			{#if entry.message.stopReason && !['stop', 'toolUse', 'complete'].includes(entry.message.stopReason)}
				<p class="text-destructive text-sm">Recorded response state: {entry.message.stopReason}</p>
			{/if}
		</section>
	{:else if entry.type === 'message' && entry.message?.role === 'toolResult'}
		<details class="bg-muted/40 rounded-lg border px-3 py-2" id="entry-{entry.id}" open={showTools}>
			<summary class="cursor-pointer font-mono text-sm font-medium">
				{entry.message.toolName || 'tool result'}{#if entry.message.isError} · error{/if}
			</summary>
			<pre class="mt-2 font-mono text-xs whitespace-pre-wrap">{resultText(entry.message)}</pre>
			{#each attachments(entry.message.content) as block, index (`result-attach-${index}`)}
				{@render attachment(block)}
			{/each}
		</details>
	{:else if entry.type === 'model_change'}
		<p class="text-muted-foreground text-sm" id="entry-{entry.id}">Switched to model: {[entry.provider, entry.modelId].filter(Boolean).join('/')}</p>
	{:else if entry.type === 'thinking_level_change'}
		<p class="text-muted-foreground text-sm" id="entry-{entry.id}">Thinking level: {entry.thinkingLevel}</p>
	{:else if entry.type === 'compaction' || entry.type === 'branch_summary'}
		<details class="rounded-lg border px-3 py-2" id="entry-{entry.id}">
			<summary class="cursor-pointer text-sm font-medium">{entry.type === 'compaction' ? 'Compaction summary' : 'Branch summary'}</summary>
			<p class="mt-2 whitespace-pre-wrap">{entry.summary}</p>
		</details>
	{:else if entry.type === 'custom_message'}
		<section class="rounded-lg border px-3 py-2 text-sm" id="entry-{entry.id}">
			<strong>{entry.customType}</strong>
			<p class="mt-2 whitespace-pre-wrap">{entry.content}</p>
		</section>
	{/if}
{/snippet}

<article class="flex flex-col gap-4" id="turn-{turn.id}">
	{#if turn.user?.message}
		<section class="bg-card ring-foreground/10 flex flex-col gap-3 rounded-xl p-4 ring-1" id="entry-{turn.user.id}">
			<p class="text-muted-foreground font-mono text-[11px] tracking-widest">USER</p>
			{@render messageBlocks(turn.user.message)}
		</section>
	{/if}
	{#each turn.follow as entry (entry.id)}
		{@render follow(entry)}
	{/each}
</article>
