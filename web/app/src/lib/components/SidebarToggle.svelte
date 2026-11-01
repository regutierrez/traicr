<script lang="ts">
	import { sidebar, toggleSidebar } from '$lib/sidebar.svelte';
</script>

<!-- Beside the page title, in the same spot whether the list is shown or hidden. -->
<button
	class="sidebar-toggle"
	type="button"
	aria-pressed={sidebar.open}
	aria-keyshortcuts="["
	aria-label="Session list"
	title={sidebar.open ? 'Hide session list ([)' : 'Show session list ([)'}
	onclick={toggleSidebar}
>
	<svg viewBox="0 0 16 16" fill="none" aria-hidden="true">
		<rect x="2.25" y="2.75" width="11.5" height="10.5" rx="1.75" stroke="currentColor" stroke-width="1.3" />
		<path d="M6.25 2.75v10.5" stroke="currentColor" stroke-width="1.3" />
	</svg>
</button>

<style>
	/* Only wide screens have room for the session list beside the page. */
	.sidebar-toggle {
		display: none;
		width: 1.75rem;
		height: 1.75rem;
		flex-shrink: 0;
		place-items: center;
		margin-left: -0.35rem;
		border: 0;
		border-radius: var(--radius);
		background: transparent;
		color: var(--muted-foreground);
		cursor: pointer;
		transition: background-color 120ms ease, color 120ms ease;
	}

	.sidebar-toggle svg {
		width: 16px;
		height: 16px;
	}

	.sidebar-toggle:hover,
	.sidebar-toggle:focus-visible {
		background: var(--accent);
		color: var(--foreground);
	}

	@media (min-width: 1100px) {
		.sidebar-toggle {
			display: grid;
		}
	}
</style>
