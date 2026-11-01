<script lang="ts">
	import MiddleTruncate from '$lib/components/MiddleTruncate.svelte';
	import { count } from '$lib/format';
	import { sourceFileHref, transcriptHref } from '$lib/links';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();
</script>

<svelte:head><title>Native source files · Traicr</title></svelte:head>
<header class="page-head">
	<nav class="crumbs" aria-label="Breadcrumbs">
		{#if data.trace}<a href={transcriptHref(data.trace.id)}>{data.trace.title || data.trace.native_trace_id}</a>{/if}
		<span>Revision {data.id}</span>
	</nav>
	<h1>Native source files</h1>
	<p class="page-lede">These files are the retained source of truth, including fields the normalizer does not yet understand.</p>
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
					<th scope="col">SHA-256</th>
					<th scope="col"><span class="sr-only">Download</span></th>
				</tr>
			</thead>
			<tbody>
				{#each data.files as file (file.path)}
					<tr>
						<td class="path"><a href={sourceFileHref(file.revision_id, file.path)} data-sveltekit-reload><MiddleTruncate text={file.path} /></a></td>
						<td class="num">{count(file.size, 'byte')}</td>
						<td class="digest" translate="no"><MiddleTruncate text={file.digest} tail={12} /></td>
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
		width: 9rem;
	}

	th:nth-child(3) {
		width: 12rem;
	}

	th:nth-child(4) {
		width: 6rem;
	}

	tbody tr:hover {
		background: var(--accent);
	}

	.path a {
		display: flex;
		color: var(--foreground);
		font-family: var(--font-mono);
		font-size: 12px;
		text-decoration: none;
	}

	.path a:hover {
		color: var(--primary);
	}

	.num {
		text-align: right;
	}

	.digest {
		color: var(--muted-foreground);
		font-family: var(--font-mono);
		font-size: 11px;
	}

	.download {
		color: var(--muted-foreground);
		font-size: 12px;
	}
</style>
