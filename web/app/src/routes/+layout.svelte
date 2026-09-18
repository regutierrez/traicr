<script lang="ts">
	import { page } from '$app/state';
	import { resolve } from '$app/paths';
	import { Button } from '$lib/components/ui/button';
	import './layout.css';

	let { data, children } = $props();
	let path = $derived(page.url.pathname);
	let login = $derived(path === '/login');

	function current(href: string) {
		if (href === '/') return path === '/' ? 'page' : undefined;
		return path === href || path.startsWith(href + '/') ? 'page' : undefined;
	}
</script>

<svelte:head>
	<title>Traicr</title>
</svelte:head>

<a class="sr-only focus:not-sr-only focus:absolute focus:top-2 focus:left-2 focus:z-50" href="#main">Skip to content</a>

{#if login}
	<main id="main" class="mx-auto flex min-h-screen w-full max-w-5xl flex-col justify-center gap-10 px-6 py-16 md:flex-row md:items-center">
		{@render children()}
	</main>
{:else}
	<div class="flex min-h-screen">
		<aside class="bg-sidebar text-sidebar-foreground sticky top-0 hidden h-screen w-60 shrink-0 flex-col border-r border-sidebar-border md:flex">
			<a class="flex items-center gap-2 px-4 py-4 text-base font-semibold tracking-tight text-sidebar-foreground no-underline" href={resolve('/')} aria-label="Traicr home">
				<span class="bg-sidebar-primary text-sidebar-primary-foreground grid size-8 place-items-center rounded-lg text-sm">t</span>
				traicr
			</a>
			<p class="text-muted-foreground px-4 pb-3 font-mono text-[10px] tracking-widest">PRIVATE ARCHIVE</p>
			<nav class="flex flex-1 flex-col gap-1 px-2" aria-label="Main navigation">
				<Button href={resolve('/')} variant={current('/') ? 'secondary' : 'ghost'} class="justify-start" aria-current={current('/')}>Transcripts</Button>
				<Button href={resolve('/imports')} variant={current('/imports') ? 'secondary' : 'ghost'} class="justify-start" aria-current={current('/imports')}>Imports</Button>
				<Button href={resolve('/machines')} variant={current('/machines') ? 'secondary' : 'ghost'} class="justify-start" aria-current={current('/machines')}>Machines</Button>
			</nav>
		</aside>
		<div class="flex min-w-0 flex-1 flex-col">
			<header class="flex flex-wrap items-center gap-2 border-b px-4 py-3">
				<a class="flex items-center gap-2 text-sm font-semibold no-underline md:hidden" href={resolve('/')} aria-label="Traicr home">
					<span class="bg-primary text-primary-foreground grid size-7 place-items-center rounded-lg text-xs">t</span>
					traicr
				</a>
				<nav class="flex w-full items-center gap-1 md:hidden" aria-label="Main navigation">
					<Button href={resolve('/')} variant={current('/') ? 'secondary' : 'ghost'} size="sm" aria-current={current('/')}>Transcripts</Button>
					<Button href={resolve('/imports')} variant={current('/imports') ? 'secondary' : 'ghost'} size="sm" aria-current={current('/imports')}>Imports</Button>
					<Button href={resolve('/machines')} variant={current('/machines') ? 'secondary' : 'ghost'} size="sm" aria-current={current('/machines')}>Machines</Button>
				</nav>
				<form class="ml-auto" action="/logout" method="post" data-sveltekit-reload>
					<input type="hidden" name="csrf" value={data.csrf} />
					<Button type="submit" variant="ghost">Log out</Button>
				</form>
			</header>
			<main id="main" class="mx-auto w-full max-w-6xl flex-1 px-4 py-8 md:px-8">
				{@render children()}
			</main>
		</div>
	</div>
{/if}
