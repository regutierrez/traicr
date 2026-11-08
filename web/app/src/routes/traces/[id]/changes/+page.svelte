<script lang="ts">
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import WorkspaceHeader from '$lib/components/conversation/WorkspaceHeader.svelte';
	import MiddleTruncate from '$lib/components/MiddleTruncate.svelte';
	import { clockTime } from '$lib/format';
	import { withQuery } from '$lib/links';
	import { diffLines } from '$lib/transcript/tools';
	import type { Patch } from '$lib/types';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();
	let trace = $derived(data.trace);
	let title = $derived(trace?.title || trace?.native_trace_id || 'Trace');

	// Paths show relative to the session's directory when they are inside it.
	let root = $derived((trace?.working_directory ?? '').replace(/^file:\/\//, '').replace(/\/+$/, ''));
	function shortPath(path: string) {
		return root && path.startsWith(`${root}/`) ? path.slice(root.length + 1) : path;
	}

	// One row per file, in the order the session first touched it. Each row keeps every edit to that file.
	let files = $derived.by(() => {
		const byPath = new Map<string, { path: string; patches: Patch[]; additions: number; deletions: number; tools: Set<string> }>();
		for (const patch of data.patches) {
			let file = byPath.get(patch.path);
			if (!file) byPath.set(patch.path, (file = { path: patch.path, patches: [], additions: 0, deletions: 0, tools: new Set() }));
			file.patches.push(patch);
			file.tools.add(patch.tool);
			if (!patch.failed) {
				file.additions += patch.additions;
				file.deletions += patch.deletions;
			}
		}
		return [...byPath.values()];
	});

	let selectedPath = $derived(page.url.searchParams.get('file') ?? files[0]?.path ?? '');
	let selected = $derived(files.find((file) => file.path === selectedPath) ?? files[0]);

	function fileHref(path: string) {
		const query = new URLSearchParams(page.url.searchParams);
		query.set('file', path);
		return withQuery(page.url.pathname, query);
	}

	function callHref(patch: Patch) {
		const query: Record<string, string> = { event: String(patch.event_id) };
		if (data.revision) query.revision = data.revision;
		return withQuery(resolve('/traces/[id]/trace', { id: String(trace?.id) }), query);
	}

	const operationLabel: Record<Patch['operation'], string> = { edit: 'Edited', write: 'Wrote', delete: 'Deleted' };
	const number = new Intl.NumberFormat();

	// j and k move through the files, like the session list.
	function onkeydown(event: KeyboardEvent) {
		if ((event.key !== 'j' && event.key !== 'k') || event.metaKey || event.ctrlKey || event.altKey || !selected) return;
		const target = event.target as HTMLElement | null;
		if (target?.closest('input, textarea, select, [contenteditable]')) return;
		const index = files.indexOf(selected) + (event.key === 'j' ? 1 : -1);
		const next = files[index];
		if (!next) return;
		event.preventDefault();
		document.getElementById(`file-${index}`)?.click();
		document.getElementById(`file-${index}`)?.focus();
	}
</script>

<svelte:head><title>Changes · {title} · Traicr</title></svelte:head>
<svelte:window {onkeydown} />

{#if data.error || !trace}
	<p class="status error" role="alert">{data.error || 'Trace not found'}</p>
{:else}
	<WorkspaceHeader {trace} revision={data.revision} tab="changes" />
	{#if data.patchError}
		<p class="status error" role="alert">Changes did not load: {data.patchError}</p>
	{:else if !files.length}
		<p class="status empty">This session changed no files with an edit tool. Edits made through shell commands are not listed here.</p>
	{:else}
		<div class="changes">
			<nav class="files" aria-label="Changed files">
				<p class="summary">
					{files.length === 1 ? '1 file' : `${number.format(files.length)} files`} · {number.format(data.patches.length)}
					{data.patches.length === 1 ? 'edit' : 'edits'}
				</p>
				<ul>
					{#each files as file, index (file.path)}
						<li>
							<a
								id="file-{index}"
								class={['file', file === selected && 'is-current']}
								href={fileHref(file.path)}
								aria-current={file === selected ? 'true' : undefined}
								data-sveltekit-replacestate
								data-sveltekit-noscroll
								data-sveltekit-keepfocus
							>
								<span class="file-path" translate="no"><MiddleTruncate text={shortPath(file.path)} /></span>
								<span class="file-meta">
									<span class="tool" translate="no">{[...file.tools].join(', ')}</span>
									{#if file.patches.length > 1}<span>×{file.patches.length}</span>{/if}
									<span class="counts">
										{#if file.additions}<span class="plus">+{number.format(file.additions)}</span>{/if}
										{#if file.deletions}<span class="minus">−{number.format(file.deletions)}</span>{/if}
									</span>
								</span>
							</a>
						</li>
					{/each}
				</ul>
			</nav>

			{#if selected}
				<section class="file-diff" aria-label="Changes to {shortPath(selected.path)}">
					<h2 translate="no">{selected.path}</h2>
					{#each selected.patches as patch, index (`${patch.event_id}:${index}`)}
						<article class={['patch', patch.failed && 'is-failed']}>
							<header>
								<span>{operationLabel[patch.operation]}</span>
								<span class="tool" translate="no">{patch.tool}</span>
								{#if patch.timestamp}<time datetime={patch.timestamp}>{clockTime(patch.timestamp)}</time>{/if}
								{#if patch.failed}<span class="bad">did not apply</span>{/if}
								<span class="counts">
									{#if patch.additions}<span class="plus">+{patch.additions}</span>{/if}
									{#if patch.deletions}<span class="minus">−{patch.deletions}</span>{/if}
								</span>
								<a class="open" href={callHref(patch)}>Open in Trace</a>
							</header>
							{#if patch.diff}
								<pre class="diff">{#each diffLines(patch.diff) as line, lineIndex (lineIndex)}<span
											class={[line.kind, line.text.startsWith('@@') && 'hunk']}>{line.text + '\n'}</span
										>{/each}</pre>
							{:else if patch.operation === 'delete'}
								<p class="note">The file was deleted.</p>
							{:else}
								<p class="note">The call recorded no diff. Its arguments and result follow.</p>
								{#if patch.arguments}<pre class="raw">{patch.arguments}</pre>{/if}
								{#if patch.result}<pre class="raw">{patch.result}</pre>{/if}
							{/if}
						</article>
					{/each}
				</section>
			{/if}
		</div>
	{/if}
{/if}

<style>
	.status {
		margin: 0;
		padding: 1.5rem;
		font-size: 13px;
	}

	.empty {
		color: var(--muted-foreground);
	}

	.error,
	.bad {
		color: var(--destructive);
	}

	.changes {
		display: grid;
		min-height: 0;
		flex: 1;
		grid-template-rows: auto minmax(0, 1fr);
		font-size: 13px;
	}

	.files {
		max-height: 40vh;
		overflow: auto;
		border-bottom: 1px solid var(--border);
	}

	.summary {
		margin: 0;
		padding: 0.75rem 1rem 0.4rem;
		color: var(--muted-foreground);
		font-size: 12px;
		font-variant-numeric: tabular-nums;
	}

	ul {
		margin: 0;
		padding: 0 0 0.5rem;
		list-style: none;
	}

	.file {
		display: grid;
		gap: 0.15rem;
		padding: 0.4rem 1rem;
		color: var(--foreground);
		text-decoration: none;
		border-left: 2px solid transparent;
	}

	.file:hover,
	.file:focus-visible {
		background: var(--accent);
	}

	.file.is-current {
		background: var(--accent);
		border-left-color: var(--primary);
	}

	.file-path {
		display: flex;
		min-width: 0;
		font-family: var(--font-mono);
		font-size: 12.5px;
	}

	.file-meta {
		display: flex;
		gap: 0.6rem;
		color: var(--muted-foreground);
		font-size: 11.5px;
		font-variant-numeric: tabular-nums;
	}

	.tool {
		font-family: var(--font-mono);
	}

	.counts {
		display: inline-flex;
		gap: 0.4rem;
		margin-left: auto;
		font-family: var(--font-mono);
		font-variant-numeric: tabular-nums;
	}

	.plus {
		color: #89d185;
	}

	.minus {
		color: #f14c4c;
	}

	.file-diff {
		min-width: 0;
		overflow: auto;
		padding: 0.75rem 1rem 3rem;
	}

	h2 {
		margin: 0 0 0.75rem;
		overflow-wrap: anywhere;
		font-family: var(--font-mono);
		font-size: 12.5px;
		font-weight: 500;
	}

	.patch {
		margin-bottom: 1rem;
		border-top: 1px solid var(--border);
		/* A file edited many times can hold thousands of diff lines; lay out only the edits in view. */
		content-visibility: auto;
		contain-intrinsic-size: auto 12rem;
	}

	.patch header {
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		gap: 0.3rem 0.75rem;
		padding: 0.5rem 0;
		color: var(--muted-foreground);
		font-size: 12px;
		font-variant-numeric: tabular-nums;
	}

	.patch header > span:first-child {
		color: var(--foreground);
	}

	.patch header .counts {
		margin-left: 0;
	}

	.open {
		margin-left: auto;
		color: var(--muted-foreground);
	}

	.open:hover {
		color: var(--foreground);
	}

	.is-failed .diff {
		opacity: 0.55;
	}

	pre {
		margin: 0;
		overflow-x: auto;
		font-family: var(--font-mono);
		font-size: 12px;
		line-height: 1.55;
	}

	.diff {
		display: grid;
		padding: 0.25rem 0;
		background: var(--card);
		border-radius: var(--radius);
	}

	.diff span {
		padding: 0 0.75rem;
		white-space: pre;
	}

	.diff .added {
		background: rgb(80 200 120 / 10%);
		color: #89d185;
	}

	.diff .removed {
		background: rgb(241 76 76 / 10%);
		color: #f14c4c;
	}

	.diff .context {
		color: var(--foreground);
	}

	.diff .hunk {
		color: var(--muted-foreground);
	}

	.raw {
		margin-top: 0.4rem;
		padding: 0.5rem 0.75rem;
		background: var(--card);
		border-radius: var(--radius);
		color: var(--muted-foreground);
		white-space: pre-wrap;
		overflow-wrap: anywhere;
	}

	.note {
		margin: 0 0 0.3rem;
		color: var(--muted-foreground);
		font-size: 12px;
	}

	@media (min-width: 900px) {
		.changes {
			grid-template-columns: minmax(16rem, 22rem) minmax(0, 1fr);
			grid-template-rows: minmax(0, 1fr);
		}

		.files {
			max-height: none;
			border-right: 1px solid var(--border);
			border-bottom: 0;
		}

		.file-diff {
			padding: 0.75rem 1.5rem 3rem;
		}
	}
</style>
