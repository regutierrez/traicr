<script lang="ts">
	import SearchLine from 'remixicon-svelte/icons/search-line';
	import { Badge } from '$lib/components/ui/badge';
	import { Button } from '$lib/components/ui/button';
	import {
		Card,
		CardContent,
		CardDescription,
		CardFooter,
		CardHeader,
		CardTitle
	} from '$lib/components/ui/card';
	import { Kbd } from '$lib/components/ui/kbd';
	import { Separator } from '$lib/components/ui/separator';
	import { concepts } from '$lib/data/concepts';

	const vocabulary = ['Trace', 'Harness', 'Event', 'Child Trace'] as const;
	const featuredSession = 'Add CSV export to billing';
</script>

<main class="mx-auto w-full max-w-5xl px-4 py-10 sm:px-6">
	<p class="text-xs font-medium tracking-wide text-muted-foreground uppercase">Issue #9</p>
	<h1 class="mt-2 text-2xl font-semibold tracking-tight">Three copies of Traicr</h1>
	<p class="mt-2 max-w-2xl text-sm leading-6 text-muted-foreground">
		Private archive for AI coding traces. These are interactive Svelte + shadcn prototypes for issue
		#9. Existing Go/HTMX UI is unchanged.
	</p>

	<div class="mt-5 flex flex-wrap items-center gap-2">
		{#each vocabulary as term (term)}
			<Badge variant="outline">{term}</Badge>
		{/each}
	</div>

	<p class="mt-4 flex items-center gap-1.5 text-xs text-muted-foreground">
		<SearchLine class="size-3.5" />
		<span>
			Command search with <Kbd>/</Kbd> is documented inside each concept, not on this page.
		</span>
	</p>

	<Separator class="my-8" />

	<section class="grid grid-cols-1 gap-4 md:grid-cols-3" aria-label="Concept copies">
		{#each concepts as concept (concept.id)}
			<Card class="h-full">
				<CardHeader>
					<p class="text-xs text-muted-foreground">{concept.eyebrow}</p>
					<CardTitle class="text-lg">{concept.name}</CardTitle>
					<CardDescription class="leading-6">{concept.summary}</CardDescription>
				</CardHeader>
				<CardContent>
					<p class="text-xs leading-5 text-muted-foreground">Inspired by {concept.inspiredBy}</p>
				</CardContent>
				<CardFooter class="gap-2">
					<Button href={concept.homeHref} size="sm">Open homepage</Button>
					<Button href={concept.viewerHref} size="sm" variant="outline">Open viewer</Button>
				</CardFooter>
			</Card>
		{/each}
	</section>

	<p class="mt-8 text-xs text-muted-foreground">
		Synthetic data. Featured session is “{featuredSession}”.
	</p>
</main>
