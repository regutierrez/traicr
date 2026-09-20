<script lang="ts">
	import { beforeNavigate } from '$app/navigation';
	import { page } from '$app/state';
	import { resolve } from '$app/paths';
	import { submitGoForm } from '$lib/forms';
	import type { LayoutProps } from './$types';
	import './layout.css';

	let { data, children }: LayoutProps = $props();
	let path = $derived(page.url.pathname);
	let login = $derived(path === '/login');
	let viewer = $derived(isViewer(path));
	let loggingOut = $state(false);

	type AppPath = '/' | '/imports' | '/machines';

	const navigation: { href: AppPath; label: string; icon: 'sessions' | 'archive' | 'machines' }[] = [
		{ href: '/', label: 'Sessions', icon: 'sessions' },
		{ href: '/imports', label: 'Archive', icon: 'archive' },
		{ href: '/machines', label: 'Machines', icon: 'machines' }
	];

	const viewerPath = /^\/traces\/[^/]+$/;

	function isViewer(pathname: string) {
		return viewerPath.test(pathname);
	}

	// The Pi viewer is a vendored module script that renders once per document and attaches
	// document-wide listeners. Give it a fresh document on the way in and on the way out instead
	// of letting SvelteKit swap components under it.
	beforeNavigate(({ from, to, type, cancel }) => {
		if (type === 'leave' || !from || !to) return;
		const samePage = from.url.pathname === to.url.pathname && from.url.search === to.url.search;
		if (samePage || (!isViewer(from.url.pathname) && !isViewer(to.url.pathname))) return;
		cancel();
		location.assign(to.url.href);
	});

	function current(href: string) {
		if (href === '/') return path === '/' ? 'page' : undefined;
		return path === href || path.startsWith(href + '/') ? 'page' : undefined;
	}

	async function signOut(event: SubmitEvent) {
		event.preventDefault();
		if (!(event.currentTarget instanceof HTMLFormElement)) return;
		loggingOut = true;
		try {
			const result = await submitGoForm(event.currentTarget);
			if (result.ok) {
				location.assign(result.url);
				return;
			}
		} catch {
			// Stay on the page: a failed or unreachable logout must not send them to /login.
		} finally {
			loggingOut = false;
		}
	}
</script>

<svelte:head>
	<title>Traicr</title>
</svelte:head>

{#snippet icon(name: 'sessions' | 'archive' | 'machines' | 'logout')}
	{#if name === 'sessions'}
		<svg viewBox="0 0 16 16" fill="none" aria-hidden="true">
			<path d="M3 4h10M3 8h10M3 12h7" stroke="currentColor" stroke-width="1.4" stroke-linecap="round" />
		</svg>
	{:else if name === 'archive'}
		<svg viewBox="0 0 16 16" fill="none" aria-hidden="true">
			<rect x="2.5" y="3.5" width="11" height="3" rx="0.6" stroke="currentColor" stroke-width="1.4" />
			<path d="M3.5 6.5h9v6.2a.8.8 0 0 1-.8.8H4.3a.8.8 0 0 1-.8-.8V6.5Z" stroke="currentColor" stroke-width="1.4" />
			<path d="M6.5 9h3" stroke="currentColor" stroke-width="1.4" stroke-linecap="round" />
		</svg>
	{:else if name === 'machines'}
		<svg viewBox="0 0 16 16" fill="none" aria-hidden="true">
			<rect x="2.5" y="3.5" width="11" height="7.5" rx="1" stroke="currentColor" stroke-width="1.4" />
			<path d="M6 13.5h4M8 11v2.5" stroke="currentColor" stroke-width="1.4" stroke-linecap="round" />
		</svg>
	{:else}
		<svg viewBox="0 0 16 16" fill="none" aria-hidden="true">
			<path d="M6 4.5H4.2A1.2 1.2 0 0 0 3 5.7v6.1A1.2 1.2 0 0 0 4.2 13h7.6A1.2 1.2 0 0 0 13 11.8V5.7A1.2 1.2 0 0 0 11.8 4.5H10" stroke="currentColor" stroke-width="1.4" stroke-linecap="round" />
			<path d="M8 2.5v6.2M6.2 7.2 8 9l1.8-1.8" stroke="currentColor" stroke-width="1.4" stroke-linecap="round" stroke-linejoin="round" />
		</svg>
	{/if}
{/snippet}

{#snippet railLink(href: AppPath, label: string, name: 'sessions' | 'archive' | 'machines')}
	<a
		class={['rail-link', current(href) && 'is-current']}
		href={resolve(href)}
		aria-current={current(href)}
		title={label}
		aria-label={label}
	>
		{@render icon(name)}
	</a>
{/snippet}

{#if !viewer}
	<a class="sr-only focus:not-sr-only focus:absolute focus:top-2 focus:left-2 focus:z-50" href="#main">Skip to content</a>
{/if}

{#if login}
	<main id="main" class="login-shell">
		{@render children()}
	</main>
{:else if viewer}
	{@render children()}
{:else}
	<div class="app-shell">
		<aside class="rail" aria-label="Main navigation">
			<a class="rail-brand" href={resolve('/')} aria-label="Traicr home" title="Traicr">t</a>
			<nav class="rail-nav">
				{#each navigation as item (item.href)}
					{@render railLink(item.href, item.label, item.icon)}
				{/each}
			</nav>
			<form class="rail-logout" action="/logout" method="post" onsubmit={signOut}>
				<input type="hidden" name="csrf" value={data.csrf} />
				<button class="rail-link" type="submit" title="Log out" aria-label="Log out" disabled={loggingOut}>
					{@render icon('logout')}
				</button>
			</form>
		</aside>
		<div class="workspace">
			<nav class="mobile-rail" aria-label="Main navigation">
				<a class="rail-brand" href={resolve('/')} aria-label="Traicr home" title="Traicr">t</a>
				{#each navigation as item (item.href)}
					{@render railLink(item.href, item.label, item.icon)}
				{/each}
				<form class="ml-auto" action="/logout" method="post" onsubmit={signOut}>
					<input type="hidden" name="csrf" value={data.csrf} />
					<button class="rail-link" type="submit" title="Log out" aria-label="Log out" disabled={loggingOut}>
						{@render icon('logout')}
					</button>
				</form>
			</nav>
			<main id="main" class="workspace-main">
				{@render children()}
			</main>
		</div>
	</div>
{/if}

<style>
	.login-shell {
		min-height: 100vh;
		display: grid;
		place-items: center;
		padding: 2rem 1.25rem;
		background: var(--background);
	}

	.app-shell {
		display: flex;
		min-height: 100vh;
		background: var(--background);
	}

	.rail {
		display: none;
		width: 48px;
		flex-shrink: 0;
		flex-direction: column;
		align-items: center;
		padding: 0.5rem 0;
		background: var(--sidebar);
		border-right: 1px solid var(--sidebar-border);
	}

	.workspace {
		display: flex;
		min-width: 0;
		flex: 1;
		flex-direction: column;
		background: var(--background);
	}

	.mobile-rail {
		display: flex;
		align-items: center;
		gap: 0.15rem;
		padding: 0.25rem 0.4rem;
		background: var(--sidebar);
		border-bottom: 1px solid var(--sidebar-border);
	}

	.workspace-main {
		flex: 1;
		width: 100%;
		max-width: 72rem;
		margin: 0 auto;
		padding: 1rem 1.25rem 2rem;
	}

	.rail-brand {
		display: grid;
		place-items: center;
		width: 28px;
		height: 28px;
		margin: 0.15rem 0 0.35rem;
		border-radius: 6px;
		background: var(--sidebar-accent);
		color: var(--sidebar-foreground);
		font-size: 12px;
		font-weight: 650;
		text-decoration: none;
	}

	.rail-nav {
		display: flex;
		flex: 1;
		flex-direction: column;
		align-items: center;
		gap: 0.15rem;
	}

	.rail-logout {
		margin-top: auto;
	}

	.rail-link {
		display: grid;
		place-items: center;
		width: 32px;
		height: 32px;
		border: 0;
		border-radius: 6px;
		background: transparent;
		color: var(--muted-foreground);
		cursor: pointer;
		text-decoration: none;
	}

	.rail-link :global(svg) {
		width: 16px;
		height: 16px;
	}

	.rail-link:hover,
	.rail-link:focus-visible {
		background: var(--sidebar-accent);
		color: var(--sidebar-foreground);
	}

	.rail-link.is-current {
		background: var(--sidebar-accent);
		color: var(--primary);
		box-shadow: inset 2px 0 0 var(--primary);
	}

	@media (min-width: 768px) {
		.rail {
			display: flex;
		}

		.mobile-rail {
			display: none;
		}

		.workspace-main {
			padding: 1.25rem 1.75rem 2.5rem;
		}
	}
</style>
