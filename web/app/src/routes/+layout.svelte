<script lang="ts">
	import './layout.css';
	import favicon from '$lib/assets/favicon.svg';
	import { page } from '$app/state';
	import type { ConceptId } from '$lib/data/types';
	import ConceptSwitcher from '$lib/components/ConceptSwitcher.svelte';
	import type { LayoutProps } from './$types';

	let { children }: LayoutProps = $props();

	function asConcept(segment: string | undefined): ConceptId | '' {
		if (segment === 'inbox' || segment === 'studio' || segment === 'ledger') {
			return segment;
		}

		return '';
	}

	let segments = $derived(page.url.pathname.split('/').filter(Boolean));
	let concept = $derived(asConcept(segments[0]));
	let title = $derived(
		concept === 'inbox'
			? 'Inbox · Traicr concepts'
			: concept === 'studio'
				? 'Studio · Traicr concepts'
				: concept === 'ledger'
					? 'Ledger · Traicr concepts'
					: 'Traicr concepts'
	);
</script>

<svelte:head>
	<link rel="icon" href={favicon} />
	<link rel="preconnect" href="https://fonts.googleapis.com" />
	<link
		href="https://fonts.googleapis.com/css2?family=Inter:wght@400;500;600;700&display=swap"
		rel="stylesheet"
	/>
	<title>{title}</title>
</svelte:head>

<div
	class={['min-h-svh bg-background text-foreground', concept === 'studio' ? 'dark' : '']}
	data-concept={concept}
>
	<ConceptSwitcher />
	{@render children()}
</div>
