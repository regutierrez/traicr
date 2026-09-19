<script lang="ts">
	import Conversation from '$lib/components/conversation.svelte';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();
	let trace = $derived(data.trace);
	let title = $derived(trace?.title || trace?.native_trace_id || 'Trace');
</script>

<svelte:head>
	<title>{title} · Traicr</title>
</svelte:head>

{#if data.error && !trace}
	<p class="text-destructive px-6 py-10" role="alert">{data.error}</p>
{:else if !trace}
	<p class="text-destructive px-6 py-10" role="alert">Trace not found</p>
{:else}
	<Conversation
		{trace}
		session={data.session}
		revision={data.revision}
		eventCount={data.eventCount}
		loadError={data.sessionError}
		reload={data.reload}
	/>
{/if}
