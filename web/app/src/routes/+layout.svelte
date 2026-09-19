<script lang="ts">
	import { page } from '$app/state';
	import { resolve } from '$app/paths';
	import { Button } from '$lib/components/ui/button';
	import type { LayoutProps } from './$types';
	import './layout.css';

	let { data, children }: LayoutProps = $props();
	let path = $derived(page.url.pathname);
	let login = $derived(path === '/login');

	const navigation = [
		['/', 'Transcripts'],
		['/imports', 'Imports'],
		['/machines', 'Machines']
	] as const;

	function current(href: string) {
		if (href === '/') return path === '/' ? 'page' : undefined;
		return path === href || path.startsWith(href + '/') ? 'page' : undefined;
	}
</script>

<svelte:head>
	<title>Traicr</title>
</svelte:head>

{#snippet brand(size: 'sm' | 'md')}
	<a class={['flex items-center gap-2 font-semibold no-underline', size === 'sm' ? 'text-sm' : 'px-4 py-4 text-base tracking-tight text-sidebar-foreground']} href={resolve('/')} aria-label="Traicr home">
		<span class={['grid place-items-center rounded-lg', size === 'sm' ? 'bg-primary text-primary-foreground size-7 text-xs' : 'bg-sidebar-primary text-sidebar-primary-foreground size-8 text-sm']}>t</span>
		traicr
	</a>
{/snippet}

{#snippet links(size: 'sm' | 'default')}
	{#each navigation as [href, label] (href)}
		<Button href={resolve(href)} {size} variant={current(href) ? 'secondary' : 'ghost'} class={size === 'default' ? 'justify-start' : ''} aria-current={current(href)}>{label}</Button>
	{/each}
{/snippet}

<a class="sr-only focus:not-sr-only focus:absolute focus:top-2 focus:left-2 focus:z-50" href="#main">Skip to content</a>

{#if login}
	<main id="main" class="mx-auto flex min-h-screen w-full max-w-5xl flex-col justify-center gap-10 px-6 py-16 md:flex-row md:items-center">
		{@render children()}
	</main>
{:else}
	<div class="flex min-h-screen">
		<aside class="bg-sidebar text-sidebar-foreground sticky top-0 hidden h-screen w-60 shrink-0 flex-col border-r border-sidebar-border md:flex">
			{@render brand('md')}
			<p class="text-muted-foreground px-4 pb-3 font-mono text-[10px] tracking-widest">PRIVATE ARCHIVE</p>
			<nav class="flex flex-1 flex-col gap-1 px-2" aria-label="Main navigation">
				{@render links('default')}
			</nav>
		</aside>
		<div class="flex min-w-0 flex-1 flex-col">
			<header class="flex flex-wrap items-center gap-2 border-b px-4 py-3">
				<div class="md:hidden">{@render brand('sm')}</div>
				<nav class="flex w-full items-center gap-1 md:hidden" aria-label="Main navigation">
					{@render links('sm')}
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
