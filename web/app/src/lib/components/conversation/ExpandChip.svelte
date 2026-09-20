<script lang="ts">
	import type { Snippet } from 'svelte';
	import type { ChipIcon } from '$lib/transcript/tools';
	import Icon from './Icon.svelte';

	let {
		icon,
		verb,
		rest = '',
		status = '',
		open = false,
		onToggle,
		children
	}: {
		icon: ChipIcon;
		verb: string;
		rest?: string;
		status?: string;
		open?: boolean;
		onToggle?: (open: boolean) => void;
		children?: Snippet;
	} = $props();
</script>

<details
	class="chip"
	bind:open={() => open, (value) => {
		if (value !== open) onToggle?.(value);
	}}
>
	<summary>
		<span class="kind"><Icon name={icon} size={14} /></span>
		<span class="verb">{verb}</span>
		{#if rest}
			<span class="rest">{rest}</span>
		{/if}
		<span class="chevron"><Icon name="chevron" size={12} /></span>
		{#if status}
			<span class="status">{status}</span>
		{/if}
	</summary>
	{#if children}
		<div class="body">
			{@render children()}
		</div>
	{/if}
</details>

<style>
	.chip {
		margin: 0;
	}

	summary {
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

	summary::-webkit-details-marker {
		display: none;
	}

	.kind {
		display: inline-flex;
		margin-right: 7px;
		transform: translateY(-0.5px);
		color: #808080;
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
