<script lang="ts">
	import type { PromptItem } from '$lib/transcript/turns';
	import Icon from './Icon.svelte';

	// A strip of ticks beside the stream, one per prompt, after T3 Code's timeline minimap.
	// Hovering shows the prompt and its last reply; clicking jumps to it.
	let {
		items,
		current,
		onSelect,
		onStep
	}: {
		items: PromptItem[];
		/** Index of the prompt whose turn is being read, or -1. */
		current: number;
		onSelect: (index: number) => void;
		onStep: (direction: 1 | -1) => void;
	} = $props();

	const SPACING = 8;
	let active = $state<number | null>(null);
	let preview = $derived(active === null ? null : items[active]);

	function top(index: number) {
		return items.length > 1 ? (index / (items.length - 1)) * 100 : 0;
	}

	function indexAt(event: MouseEvent) {
		const rect = (event.currentTarget as HTMLElement).getBoundingClientRect();
		const progress = Math.min(1, Math.max(0, (event.clientY - rect.top) / rect.height));
		return Math.round(progress * (items.length - 1));
	}

	// Ticks near the pointer widen, so the one under it is easy to see.
	function scale(index: number) {
		if (active === null) return index === current ? 0.667 : 0.333;
		const distance = Math.abs(index - active);
		return [1, 0.667, 0.417][distance] ?? 0.333;
	}

	function onKeydown(event: KeyboardEvent) {
		const last = items.length - 1;
		const base = active ?? Math.max(current, 0);
		const next = { ArrowDown: base + 1, ArrowUp: base - 1, Home: 0, End: last }[event.key];
		if (next !== undefined) {
			event.preventDefault();
			active = Math.min(last, Math.max(0, next));
		} else if ((event.key === 'Enter' || event.key === ' ') && active !== null) {
			event.preventDefault();
			onSelect(active);
		}
	}
</script>

<nav class="rail" aria-label="Prompts">
	<button type="button" class="step" title="Previous prompt (P)" aria-label="Previous prompt" disabled={current <= 0} onclick={() => onStep(-1)}>
		<span class="up"><Icon name="chevron" size={14} /></span>
	</button>
	<button
		type="button"
		class="strip"
		style:height="min({(items.length - 1) * SPACING}px, calc(100vh - 16rem))"
		aria-label={preview ? `Jump to prompt ${active! + 1} of ${items.length}: ${preview.prompt}` : `Jump to a prompt, ${items.length} in this trace`}
		onmousemove={(event) => (active = indexAt(event))}
		onmouseleave={() => (active = null)}
		onfocus={() => (active = Math.max(current, 0))}
		onblur={() => (active = null)}
		onkeydown={onKeydown}
		onclick={(event) => {
			// A keyboard click has no pointer position; Enter is handled in onKeydown.
			if (event.detail === 0) return;
			onSelect(indexAt(event));
		}}
	>
		{#each items as item, index (item.id)}
			<span
				class={['tick', index === current && 'is-current', index === active && 'is-active']}
				style:top="{top(index)}%"
				style:transform="translateY(-50%) scaleX({scale(index)})"
			></span>
		{/each}
		{#if preview && active !== null}
			<!-- The first and last previews hang inward so they stay beside the strip. -->
			<span
				class="preview"
				style:top="{top(active)}%"
				style:transform="translateY({active === 0 ? '0%' : active === items.length - 1 ? '-100%' : '-50%'})"
				aria-hidden="true"
			>
				<span class="count">Prompt {active + 1} of {items.length}</span>
				<span class="prompt">{preview.prompt || 'Prompt without text'}</span>
				{#if preview.reply}
					<span class="reply">{preview.reply}</span>
				{/if}
			</span>
		{/if}
	</button>
	<button type="button" class="step" title="Next prompt (N)" aria-label="Next prompt" disabled={current >= items.length - 1} onclick={() => onStep(1)}>
		<span class="down"><Icon name="chevron" size={14} /></span>
	</button>
</nav>

<style>
	.rail {
		position: absolute;
		top: 50%;
		/* Right of the column, beside the scrollbar it mirrors, clear of the scrollbar itself. */
		right: 1.25rem;
		z-index: 5;
		display: none;
		flex-direction: column;
		align-items: flex-end;
		transform: translateY(-50%);
	}

	/* Only where the gutter beside the 46rem column has room, and only for a mouse. */
	@container (min-width: 54rem) {
		.rail {
			display: flex;
		}
	}

	@media (pointer: coarse) {
		.rail {
			display: none !important;
		}
	}

	.strip {
		position: relative;
		width: 2.5rem;
		min-height: 0.5rem;
		margin: 0.25rem 0;
		border: 0;
		border-radius: 4px;
		background: transparent;
		padding: 0;
		font: inherit;
		cursor: pointer;
	}

	.strip:focus-visible {
		outline: 2px solid var(--ring);
		outline-offset: 4px;
	}

	.tick {
		position: absolute;
		right: 0;
		width: 1.5rem;
		height: 2px;
		border-radius: 999px;
		background: color-mix(in oklab, var(--muted-foreground) 35%, transparent);
		transform-origin: right center;
		transition:
			transform 150ms ease,
			background-color 150ms ease;
		pointer-events: none;
	}

	.tick.is-current {
		background: var(--foreground);
	}

	.tick.is-active {
		background: var(--primary);
	}

	/* As wide as the strip, so a pointer that reaches the strip also reaches these; the
	   chevron sits over the ticks. */
	.step {
		display: inline-flex;
		width: 2.5rem;
		height: 1.5rem;
		align-items: center;
		justify-content: flex-end;
		border: 0;
		border-radius: 6px;
		background: transparent;
		color: var(--muted-foreground);
		padding: 0 0.3rem 0 0;
		cursor: pointer;
		opacity: 0;
		transition: opacity 150ms ease;
	}

	.rail:hover .step,
	.rail:focus-within .step {
		opacity: 1;
	}

	.step:hover:not(:disabled),
	.step:focus-visible {
		background: var(--accent);
		color: var(--foreground);
	}

	.step:disabled {
		cursor: default;
		opacity: 0 !important;
	}

	.up,
	.down {
		display: inline-flex;
	}

	.up {
		transform: rotate(-90deg);
	}

	.down {
		transform: rotate(90deg);
	}

	.preview {
		position: absolute;
		right: 2.75rem;
		display: flex;
		width: 21rem;
		flex-direction: column;
		gap: 0.2rem;
		padding: 0.65rem 0.8rem;
		border: 1px solid var(--border);
		border-radius: 0.75rem;
		background: var(--popover);
		box-shadow: 0 24px 64px -24px rgb(0 0 0 / 65%);
		text-align: left;
		pointer-events: none;
	}

	.count {
		color: var(--muted-foreground);
		font-size: 11px;
		font-variant-numeric: tabular-nums;
	}

	.prompt {
		overflow: hidden;
		color: var(--foreground);
		font-size: 13px;
		font-weight: 500;
		line-height: 1.4;
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	.reply {
		display: -webkit-box;
		overflow: hidden;
		color: var(--muted-foreground);
		font-size: 13px;
		line-height: 1.4;
		-webkit-box-orient: vertical;
		-webkit-line-clamp: 3;
		line-clamp: 3;
	}
</style>
