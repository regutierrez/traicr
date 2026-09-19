<script lang="ts">
	import { sourceFileHref } from '$lib/links';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();
</script>

<svelte:head><title>Native source files · Traicr</title></svelte:head>
<p class="text-primary mb-2 font-mono text-[11px] tracking-widest">REVISION {data.id} / LOSSLESS SOURCE</p>
<h1 class="text-3xl font-semibold tracking-tight">Native source files</h1>
<p class="text-muted-foreground mt-2 mb-6 max-w-2xl">These files are the retained source of truth, including fields the normalizer does not yet understand.</p>
{#if data.error}
	<p class="text-destructive" role="alert">{data.error}</p>
{:else}
	<div class="overflow-x-auto rounded-xl ring-1 ring-foreground/10">
		<table class="w-full text-sm">
			<thead>
				<tr class="bg-muted/60 text-muted-foreground text-left text-xs">
					<th class="px-3 py-2 font-medium">File</th>
					<th class="px-3 py-2 font-medium">Bytes</th>
					<th class="px-3 py-2 font-medium">SHA-256</th>
					<th class="px-3 py-2"></th>
				</tr>
			</thead>
			<tbody>
				{#each data.files as file (file.path)}
					<tr class="border-t">
						<td class="px-3 py-3"><a href={sourceFileHref(file.revision_id, file.path)}>{file.path}</a></td>
						<td class="px-3 py-3">{file.size}</td>
						<td class="px-3 py-3 font-mono text-xs break-all">{file.digest}</td>
						<td class="px-3 py-3"><a href={sourceFileHref(file.revision_id, file.path, { download: true })} data-sveltekit-reload>Download</a></td>
					</tr>
				{/each}
			</tbody>
		</table>
	</div>
{/if}
