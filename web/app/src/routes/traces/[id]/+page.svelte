<script lang="ts">
	import { resolve } from '$app/paths';
	import { onMount } from 'svelte';

	let { data } = $props();
	let trace = $derived(data.trace);
	let title = $derived(trace?.title || trace?.native_trace_id || 'Trace');

	onMount(() => {
		if (!trace) return;
		let cancelled = false;
		const load = (src: string, module = false) =>
			new Promise<void>((resolve, reject) => {
				const script = document.createElement('script');
				if (module) script.type = 'module';
				script.src = src;
				script.onload = () => resolve();
				script.onerror = () => reject(new Error(src));
				document.body.appendChild(script);
			});
		void (async () => {
			await load('/static/pi-transcript/marked.min.js');
			await load('/static/pi-transcript/highlight.min.js');
			if (!cancelled) await load('/static/pi-transcript/pi-transcript.js', true);
		})();
		return () => {
			cancelled = true;
		};
	});
</script>

<svelte:head>
	<title>{title} · Traicr</title>
	<link rel="stylesheet" href="/static/pi-transcript/pi-transcript.css" />
	<link rel="stylesheet" href="/static/traicr-theme.css" />
	<link rel="stylesheet" href="/static/pi-transcript/traicr-transcript.css" />
</svelte:head>

{#if data.error || !trace}
	<p class="text-destructive px-6 py-10" role="alert">{data.error || 'Trace not found'}</p>
{:else}
	<div id="transcript-meta" data-trace-id={trace.id} data-title={title} data-harness={trace.harness} data-native-id={trace.native_trace_id} data-created-at={trace.created_at} data-cwd={trace.working_directory ?? ''}></div>
	<button id="hamburger" title="Open sidebar"><svg width="14" height="14" viewBox="0 0 24 24" fill="currentColor" stroke="none"><circle cx="6" cy="6" r="2.5"/><circle cx="6" cy="18" r="2.5"/><circle cx="18" cy="12" r="2.5"/><rect x="5" y="6" width="2" height="12"/><path d="M6 12h10c1 0 2 0 2-2V8"/></svg></button>
	<div id="sidebar-overlay"></div>
	<div id="app">
		<aside id="sidebar">
			<div class="sidebar-header">
				<div class="sidebar-controls">
					<input type="text" class="sidebar-search" id="tree-search" placeholder="Search..." />
				</div>
				<div class="sidebar-filters">
					<button class="filter-btn active" data-filter="default" title="Hide settings entries">Default</button>
					<button class="filter-btn" data-filter="no-tools" title="Default minus tool results">No-tools</button>
					<button class="filter-btn" data-filter="user-only" title="Only user messages">User</button>
					<button class="filter-btn" data-filter="labeled-only" title="Only labeled entries">Labeled</button>
					<button class="filter-btn" data-filter="all" title="Show everything">All</button>
					<button class="sidebar-close" id="sidebar-close" title="Close">✕</button>
				</div>
			</div>
			<div class="tree-container" id="tree-container"></div>
			<div class="tree-status" id="tree-status"></div>
		</aside>
		<div id="sidebar-resizer" role="separator" aria-orientation="vertical" aria-label="Resize session tree sidebar"></div>
		<main id="content">
			<nav class="transcript-navigation">
				<a class="transcript-brand" href={resolve('/')} aria-label="Traicr home" data-sveltekit-reload><span>t</span>traicr</a>
				<a href={resolve('/')} data-sveltekit-reload>← Transcripts</a>
				<a href={resolve('/traces/[id]/records', { id: String(trace.id) })}>Session details & source records</a>
			</nav>
			{#if trace.harness === 'amp'}
				<form class="amp-revisions" method="get" data-sveltekit-reload>
					<label for="revision">Archive view</label>
					<select name="revision" id="revision">
						<option value="" selected={data.revision === ''}>Merged archive</option>
						{#each trace.revisions ?? [] as revision (revision.id)}
							<option value={revision.id} selected={data.revision === String(revision.id)}>Revision {revision.id} · {revision.native_updated_at}</option>
						{/each}
					</select>
					<button type="submit" class="header-toggle-btn">Inspect</button>
				</form>
				<details class="amp-details">
					<summary>Download native Amp export</summary>
					<p>Lossless source JSON for an individual revision, not the merged viewer data.</p>
					{#each trace.revisions ?? [] as revision (revision.id)}
						<p><a href={resolve(`/revisions/${revision.id}/file?path=source/export.json&download=1`)} data-sveltekit-reload>Revision {revision.id} · {revision.native_updated_at} · Native JSON</a></p>
					{/each}
				</details>
			{/if}
			<p id="transcript-status" role="status">Loading transcript…</p>
			<noscript>Enable JavaScript to use the interactive transcript viewer. Session details remain available without JavaScript.</noscript>
			<div id="header-container"></div>
			<div id="messages"></div>
		</main>
		<div id="image-modal" class="image-modal">
			<img id="modal-image" src="" alt="" />
		</div>
	</div>
{/if}
