<script lang="ts">
	import Conversation from '$lib/components/conversation/Conversation.svelte';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();
	let trace = $derived(data.trace);
	let title = $derived(trace?.title || trace?.native_trace_id || 'Trace');
</script>

<svelte:head>
	<title>{title} · Traicr</title>
</svelte:head>

{#if data.error || !trace}
	<p class="error" role="alert">{data.error || 'Trace not found'}</p>
{:else}
	<Conversation {trace} revision={data.revision} initial={data.session} loadError={data.loadError} />
{/if}

<style>
	.error {
		margin: 0;
		padding: 1.5rem;
		color: var(--destructive);
	}
</style>
