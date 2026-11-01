<script lang="ts">
	import { count, dateAndTime, fileSize } from '$lib/format';
	import { sourceFileHref, transcriptHref } from '$lib/links';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();
	// Every path starts with "source/"; the rest is the folder worth showing.
	function parts(path: string) {
		const shown = path.replace(/^source\//, '');
		const cut = shown.lastIndexOf('/') + 1;
		const name = shown.slice(cut);
		// Attachments are named by their SHA-256; the first characters tell them apart.
		return { folder: shown.slice(0, cut), name: /^[0-9a-f]{64}$/.test(name) ? `${name.slice(0, 12)}…` : name };
	}
	// Top-level files, such as the session export, before attachments.
	let files = $derived(data.files.toSorted((a, b) => Number(parts(a.path).folder !== '') - Number(parts(b.path).folder !== '')));
	let collected = $derived(data.trace?.revisions?.find((revision) => String(revision.id) === data.id)?.collected_at);
</script>

<svelte:head><title>Original files · Traicr</title></svelte:head>
<header class="page-head">
	<nav class="crumbs" aria-label="Breadcrumbs">
		{#if data.trace}<a href={transcriptHref(data.trace.id)}>{data.trace.title || data.trace.native_trace_id}</a>{/if}
		<span>Revision {data.id}</span>
	</nav>
	<h1>Original files</h1>
	<p class="page-lede">
		{#if collected}Collected <time datetime={collected}>{dateAndTime(collected)}</time>.{/if}
		Exactly as collected, including data Traicr does not show yet.
	</p>
</header>
{#if data.error}
	<p class="text-destructive" role="alert">{data.error}</p>
{:else}
	<div class="table-wrap">
		<table>
			<thead>
				<tr>
					<th scope="col">File</th>
					<th scope="col" class="num">Size</th>
					<th scope="col"><span class="sr-only">Download</span></th>
				</tr>
			</thead>
			<tbody>
				{#each files as file (file.path)}
					{@const shown = parts(file.path)}
					<tr title={`${file.path}\nSHA-256 ${file.digest}`}>
						<td class="path">
							<a href={sourceFileHref(file.revision_id, file.path)} data-sveltekit-reload translate="no"><span class="folder">{shown.folder}</span>{shown.name}</a>
						</td>
						<td class="num" title={count(file.size, 'byte')}>{fileSize(file.size)}</td>
						<td class="num"><a class="download" href={sourceFileHref(file.revision_id, file.path)} data-sveltekit-reload>Download</a></td>
					</tr>
				{/each}
			</tbody>
		</table>
	</div>
{/if}

<style>
	.table-wrap {
		overflow-x: auto;
	}

	table {
		width: 100%;
		border-collapse: collapse;
		table-layout: fixed;
		font-size: 13px;
	}

	th,
	td {
		padding: 0.6rem 0.6rem;
		border-bottom: 1px solid var(--border);
		text-align: left;
		white-space: nowrap;
	}

	th {
		color: var(--muted-foreground);
		font-size: 12px;
		font-weight: 500;
	}

	th:nth-child(2) {
		width: 7rem;
	}

	th:nth-child(3) {
		width: 6rem;
	}

	tbody tr:hover {
		background: var(--accent);
	}

	.path {
		overflow: hidden;
		text-overflow: ellipsis;
	}

	.path a {
		color: var(--foreground);
		font-family: var(--font-mono);
		font-size: 12px;
		text-decoration: none;
	}

	.path a:hover {
		color: var(--primary);
	}

	.num {
		font-variant-numeric: tabular-nums;
		text-align: right;
	}

	.folder {
		color: var(--muted-foreground);
	}

	.download {
		color: var(--muted-foreground);
		font-size: 12px;
	}
</style>
