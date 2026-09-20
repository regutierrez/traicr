<script lang="ts">
	import { attachmentView } from '$lib/transcript/attachments';
	import type { Attachment } from '$lib/transcript/types';

	let { attachment }: { attachment: Attachment } = $props();
	let view = $derived(attachmentView(attachment));
</script>

{#if view.kind === 'archived'}
	<details class="box">
		<summary>Archived image: {view.name}</summary>
		{#if view.oversized}
			<p>Image exceeds the 10 MiB preview limit. The original is archived.</p>
		{:else if view.preview}
			<a href={view.preview} target="_blank" rel="noreferrer">
				<img src={view.preview} alt={view.name} loading="lazy" />
			</a>
		{/if}
		<a href={view.download} data-sveltekit-reload>Download image</a>
	</details>
{:else if view.kind === 'external'}
	<p class="note">
		External reference · <a href={view.href} target="_blank" rel="noreferrer">{view.name} ↗</a>
		<span> Bytes are not archived; access may require sign-in to the source service.</span>
	</p>
{:else}
	<p class="note">{view.name} · Attachment unavailable; inspect the source block.</p>
{/if}

<style>
	.box,
	.note {
		margin: 0.45rem 0;
		color: var(--muted-foreground);
		font-size: 12px;
	}

	summary {
		cursor: pointer;
		color: var(--foreground);
	}

	img {
		display: block;
		max-width: min(100%, 28rem);
		margin: 0.5rem 0;
		border: 1px solid var(--border);
	}

	a {
		color: var(--primary);
	}
</style>
