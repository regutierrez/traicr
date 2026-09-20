<script lang="ts">
	import type { ChipIcon } from '$lib/transcript/tools';

	let {
		name,
		size = 14,
		flip = false,
		label = ''
	}: {
		name: ChipIcon;
		size?: number;
		flip?: boolean;
		label?: string;
	} = $props();
</script>

<svg
	class={{ flip }}
	width={size}
	height={size}
	viewBox="0 0 24 24"
	fill="none"
	stroke="currentColor"
	stroke-width="2.25"
	stroke-linecap="round"
	stroke-linejoin="round"
	aria-hidden={label ? undefined : true}
	aria-label={label || undefined}
>
	{#if name === 'terminal'}
		<rect width="18" height="18" x="3" y="3" rx="2"></rect>
		<path d="m10 8 4 4-4 4"></path>
	{:else if name === 'book'}
		<path d="M12 5v16"></path>
		<path
			d="M20.001 19A2 2 0 0022 17V5a2 2 0 00-1.999-2L16 3.002A5 5 0 0012 5a5 5 0 00-4-2H4a2 2 0 00-2 2v12a2 2 0 001.999 2H8a5 5 0 014 2 5 5 0 014-2z"
		></path>
	{:else if name === 'pen'}
		<path
			d="M21.174 6.812a1 1 0 0 0-3.986-3.987L3.842 16.174a2 2 0 0 0-.5.83l-1.321 4.352a.5.5 0 0 0 .623.622l4.353-1.32a2 2 0 0 0 .83-.497z"
		></path>
	{:else if name === 'file-plus'}
		<path d="M6 22a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h8a2.4 2.4 0 0 1 1.704.706l3.588 3.588A2.4 2.4 0 0 1 20 8v12a2 2 0 0 1-2 2z"></path>
		<path d="M14 2v5a1 1 0 0 0 1 1h5"></path>
		<path d="M9 15h6"></path>
		<path d="M12 18v-6"></path>
	{:else if name === 'globe'}
		<circle cx="12" cy="12" r="10"></circle>
		<path d="M12 2a14.5 14.5 0 0 0 0 20 14.5 14.5 0 0 0 0-20"></path>
		<path d="M2 12h20"></path>
	{:else if name === 'wrench'}
		<path
			d="M14.7 6.3a1 1 0 0 0 0 1.4l1.6 1.6a1 1 0 0 0 1.4 0l3.106-3.105c.32-.322.863-.22.983.218a6 6 0 0 1-8.259 7.057l-7.91 7.91a1 1 0 0 1-2.999-3l7.91-7.91a6 6 0 0 1 7.057-8.259c.438.12.54.662.219.984z"
		></path>
	{:else if name === 'brain'}
		<path d="M12 18V5"></path>
		<path d="M15 13a4.17 4.17 0 0 1-3-4 4.17 4.17 0 0 1-3 4"></path>
		<path d="M17.598 6.5A3 3 0 1 0 12 5a3 3 0 1 0-5.598 1.5"></path>
		<path d="M17.997 5.125a4 4 0 0 1 2.526 5.77"></path>
		<path d="M18 18a4 4 0 0 0 2-7.464"></path>
		<path d="M19.967 17.483A4 4 0 1 1 12 18a4 4 0 1 1-7.967-.517"></path>
		<path d="M6 18a4 4 0 0 1-2-7.464"></path>
		<path d="M6.003 5.125a4 4 0 0 0-2.526 5.77"></path>
	{:else if name === 'sparkles'}
		<path
			d="M9.937 15.5A2 2 0 0 0 8.5 14.063l-6.135-1.582a.5.5 0 0 1 0-.962L8.5 9.936A2 2 0 0 0 9.937 8.5l1.582-6.135a.5.5 0 0 1 .963 0L14.063 8.5A2 2 0 0 0 15.5 9.937l6.135 1.581a.5.5 0 0 1 0 .964L15.5 14.063a2 2 0 0 0-1.437 1.437l-1.582 6.135a.5.5 0 0 1-.963 0z"
		></path>
	{:else if name === 'thread'}
		<path d="M16 10a2 2 0 0 1-2 2H6.828a2 2 0 0 0-1.414.586l-2.202 2.202A.71.71 0 0 1 2 14.286V4a2 2 0 0 1 2-2h10a2 2 0 0 1 2 2z"></path>
		<path
			d="M20 9a2 2 0 0 1 2 2v10.286a.71.71 0 0 1-1.212.564l-2.202-2.202A2 2 0 0 0 17.172 19H10a2 2 0 0 1-2-2v-1"
		></path>
	{:else if name === 'user'}
		<path d="M19 21v-2a4 4 0 0 0-4-4H9a4 4 0 0 0-4 4v2"></path>
		<circle cx="12" cy="7" r="4"></circle>
	{:else if name === 'agent'}
		<path d="M12 8V4H8"></path>
		<rect width="16" height="12" x="4" y="8" rx="2"></rect>
		<path d="M2 14h2"></path>
		<path d="M20 14h2"></path>
		<path d="M15 13v2"></path>
		<path d="M9 13v2"></path>
	{:else if name === 'prompt'}
		<path d="M22 17a2 2 0 0 1-2 2H6.828a2 2 0 0 0-1.414.586l-2.202 2.202A.71.71 0 0 1 2 21.286V5a2 2 0 0 1 2-2h16a2 2 0 0 1 2 2z"></path>
	{:else if name === 'response'}
		<path d="M22 17a2 2 0 0 1-2 2H6.828a2 2 0 0 0-1.414.586l-2.202 2.202A.71.71 0 0 1 2 21.286V5a2 2 0 0 1 2-2h16a2 2 0 0 1 2 2z"></path>
	{:else if name === 'compaction'}
		<path d="M3 5h8"></path>
		<path d="M3 12h8"></path>
		<path d="M3 19h8"></path>
		<path d="m15 5 3 3 3-3"></path>
		<path d="m15 19 3-3 3 3"></path>
	{:else if name === 'branch'}
		<line x1="6" x2="6" y1="3" y2="15"></line>
		<circle cx="18" cy="6" r="3"></circle>
		<circle cx="6" cy="18" r="3"></circle>
		<path d="M18 9a9 9 0 0 1-9 9"></path>
	{:else}
		<path d="m9 18 6-6-6-6"></path>
	{/if}
</svg>

<style>
	svg {
		display: block;
		flex-shrink: 0;
	}

	.flip {
		transform: scaleX(-1);
	}
</style>
