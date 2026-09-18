<script lang="ts">
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import { Badge } from '$lib/components/ui/badge';
	import { Button } from '$lib/components/ui/button';
	import { Separator } from '$lib/components/ui/separator';
	import { concepts } from '$lib/data/concepts';
	import type { ConceptId } from '$lib/data/types';

	function asConcept(segment: string | undefined): ConceptId | '' {
		if (segment === 'inbox' || segment === 'studio' || segment === 'ledger') {
			return segment;
		}

		return '';
	}

	let segments = $derived(page.url.pathname.split('/').filter(Boolean));
	let currentConcept = $derived(asConcept(segments[0]));
	let viewerId = $derived(currentConcept && segments[1] ? segments[1] : '');
	let links = $derived(
		concepts.map((concept) => ({
			id: concept.id,
			name: concept.name,
			href: viewerId ? `/${concept.id}/${viewerId}` : concept.homeHref
		}))
	);
</script>

<header
	class="sticky top-0 z-50 h-10 border-b bg-background/80 backdrop-blur"
>
	<div class="flex h-full items-center gap-2.5 px-3">
		<a
			href={resolve('/')}
			class="shrink-0 text-xs font-medium tracking-tight text-foreground"
			aria-current={currentConcept === '' ? 'page' : undefined}
		>
			traicr concepts
		</a>
		<Badge variant="outline" class="h-4 rounded-sm px-1.5 text-[10px] font-medium leading-none">
			prototype
		</Badge>
		<Separator orientation="vertical" class="mx-0.5 h-4" />
		<nav class="ml-auto flex items-center gap-0.5" aria-label="Concept copies">
			{#each links as link (link.id)}
				<Button
					href={link.href}
					size="xs"
					variant="ghost"
					aria-current={currentConcept === link.id ? 'page' : undefined}
					class={[
						'px-2 text-xs',
						currentConcept === link.id
							? 'bg-muted text-foreground'
							: 'text-muted-foreground hover:text-foreground'
					]}
				>
					{link.name}
				</Button>
			{/each}
		</nav>
	</div>
</header>
