<script lang="ts">
	import { page } from '$app/state';
	import { resolve } from '$app/paths';
	import { Button } from '$lib/components/ui/button';

	// SvelteKit says "Not Found" for a URL no route matches; loads that call error() pass their own words.
	let copy = $derived.by(() => {
		const message = page.error?.message ?? '';
		if (page.status === 404) return { title: 'Not found', text: message && message !== 'Not Found' ? message : 'This page does not exist.', reload: false };
		if (page.status === 503) return { title: 'Server not answering', text: message, reload: true };
		return { title: 'Something went wrong', text: 'The page could not load. Reload to try again.', reload: true };
	});
</script>

<svelte:head><title>{copy.title} · Traicr</title></svelte:head>

<header class="page-head">
	<h1>{copy.title}</h1>
	<p class="page-lede">{copy.text}</p>
</header>
<div class="actions">
	{#if copy.reload}<Button size="sm" onclick={() => location.reload()}>Reload</Button>{/if}
	<Button size="sm" variant={copy.reload ? 'outline' : 'default'} href={resolve('/')}>Search sessions</Button>
	<Button size="sm" variant="outline" href={resolve('/imports')}>Archive</Button>
</div>

<style>
	.actions {
		display: flex;
		flex-wrap: wrap;
		gap: 0.5rem;
	}
</style>
