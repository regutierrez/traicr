<script lang="ts">
	import { dateAndTime } from '$lib/format';
	import type { TranscriptEntry } from '$lib/transcript/types';

	let { entry }: { entry: TranscriptEntry } = $props();

	const tokens = new Intl.NumberFormat('en-US');
	const phases: Record<string, string> = { commentary: 'Commentary', final_answer: 'Final answer' };

	let message = $derived(entry.message);
	// Shown under a written reply only; tool-only and thinking-only steps stay one quiet row.
	let phase = $derived(message?.nativeDetails?.message_meta?.openAIResponsePhase ?? '');
	let usage = $derived(message?.usage);
	// Only values the export recorded; a missing count is left out, not shown as zero.
	let rows = $derived.by(() => {
		const found: [string, string][] = [];
		const add = (label: string, value: string | undefined) => value && found.push([label, value]);
		const amount = (value?: number) => (typeof value === 'number' ? tokens.format(value) : undefined);
		add('Model', [message?.model, message?.provider].filter(Boolean).join(' · '));
		add('Sent', entry.timestamp ? dateAndTime(entry.timestamp) : undefined);
		add('Phase', phases[phase] ?? phase);
		add('Output tokens', amount(usage?.output));
		add('New input tokens', amount(usage?.input));
		add('Cache read', amount(usage?.cacheRead));
		add('Cache write', amount(usage?.cacheWrite));
		if (usage?.context && usage.contextWindow) {
			const share = Math.round((usage.context / usage.contextWindow) * 100);
			add('Context', `${tokens.format(usage.context)} of ${tokens.format(usage.contextWindow)} (${share}%)`);
		} else add('Context', amount(usage?.context));
		return found;
	});
</script>

{#if message?.role === 'assistant' && (usage || phase) && message.content.some((block) => block.type === 'text' && block.text.trim())}
	<details class="meta">
		<summary>{phases[phase] ?? 'Details'}</summary>
		<dl>
			{#each rows as [label, value] (label)}
				<dt>{label}</dt>
				<dd>{value}</dd>
			{/each}
		</dl>
	</details>
{/if}

<style>
	.meta {
		margin: 0.25rem 0 0;
		color: var(--muted-foreground);
		font-size: 11px;
	}

	summary {
		width: fit-content;
		cursor: pointer;
		list-style: none;
	}

	summary::-webkit-details-marker {
		display: none;
	}

	summary::after {
		content: ' ›';
	}

	.meta[open] summary::after {
		content: ' ⌄';
	}

	summary:hover,
	summary:focus-visible {
		color: var(--foreground);
	}

	dl {
		display: grid;
		grid-template-columns: max-content 1fr;
		gap: 0.15rem 0.75rem;
		margin: 0.35rem 0 0;
		font-family: var(--font-mono);
	}

	dd {
		margin: 0;
		color: var(--foreground);
	}
</style>
