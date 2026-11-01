<script lang="ts">
	import type { Snippet } from 'svelte';
	import type { ChipIcon } from '$lib/transcript/tools';
	import Icon from './Icon.svelte';

	let {
		icon,
		verb,
		rest = '',
		status = '',
		title,
		open = false,
		onToggle,
		children
	}: {
		icon: ChipIcon;
		verb: string;
		rest?: string;
		status?: string;
		title?: string;
		open?: boolean;
		onToggle?: (open: boolean) => void;
		children?: Snippet;
	} = $props();

	// Whether the details element is open, including when no onToggle tracks it.
	let shown = $state(false);
</script>

{#snippet line(expandable: boolean)}
	<span class="kind"><Icon name={icon} size={14} /></span>
	<span class="verb">{verb}</span>
	{#if rest}
		<span class="rest">{rest}</span>
	{/if}
	{#if expandable}
		<span class="chevron"><Icon name="chevron" size={12} /></span>
	{/if}
	{#if status}
		<span class="status">{status}</span>
	{/if}
{/snippet}

<!-- A chip with nothing to show, such as a thought whose text the export left out, is a plain line, not a control. -->
{#if !children}
	<div class="chip"><span class="summary is-static" {title}>{@render line(false)}</span></div>
{:else}
	<details
		class="chip"
		bind:open={() => open, (value) => {
			shown = value;
			if (value !== open) onToggle?.(value);
		}}
	>
		<summary class="summary">{@render line(true)}</summary>
		<!-- Render the body only once opened: highlighting every collapsed tool
		     output made large sessions take tens of seconds to display. -->
		{#if open || shown}
			<div class="body">
				{@render children()}
			</div>
		{/if}
	</details>
{/if}

<style>
	.chip {
		margin: 0;
	}

	.summary {
		display: inline-flex;
		max-width: 100%;
		align-items: center;
		gap: 5px;
		min-height: 24px;
		cursor: pointer;
		color: var(--muted-foreground);
		font-family: var(--font-sans);
		font-size: 13px;
		line-height: 20px;
		list-style: none;
	}

	.summary.is-static {
		cursor: default;
		opacity: 0.75;
	}

	summary::-webkit-details-marker {
		display: none;
	}

	.kind {
		display: inline-flex;
		margin-right: 7px;
		transform: translateY(-0.5px);
		opacity: 0.8;
	}

	.verb {
		flex-shrink: 0;
		font-weight: 500;
	}

	.rest {
		overflow: hidden;
		min-width: 0;
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	.chevron {
		display: inline-flex;
		margin: 0 -0.125rem;
		transform: translateY(-0.5px);
		opacity: 0.4;
		transition: transform 150ms ease, opacity 150ms ease;
	}

	.chip[open] .chevron {
		transform: translateY(-0.5px) rotate(90deg);
		opacity: 0.85;
	}

	summary:hover .chevron,
	summary:focus-visible .chevron {
		opacity: 1;
	}

	.status {
		flex-shrink: 0;
		margin-left: 0.35rem;
		font-size: 11px;
	}

	.body {
		margin: 0.2rem 0 0.35rem;
	}
</style>
