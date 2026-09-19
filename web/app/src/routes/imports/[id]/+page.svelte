<script lang="ts">
	import { resolve } from '$app/paths';
	import { Badge } from '$lib/components/ui/badge';
	import { Button } from '$lib/components/ui/button';
	import { Card, CardContent, CardHeader, CardTitle } from '$lib/components/ui/card';
	import { transcriptHref } from '$lib/links';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();
	let report = $derived(data.report);
</script>

<svelte:head><title>Import report · Traicr</title></svelte:head>

<Button class="mb-4" variant="ghost" href={resolve('/imports')}>All imports</Button>
{#if data.error || !report}
	<p class="text-destructive" role="alert">{data.error || 'Import not found'}</p>
{:else}
	<p class="text-primary mb-2 font-mono text-[11px] tracking-widest">IMPORT #{report.id} / {report.source_machine.hostname}</p>
	<h1 class="text-3xl font-semibold tracking-tight">Import report</h1>
	<p class="text-muted-foreground mt-2 mb-6">{report.created_at}</p>
	<div class="flex flex-col gap-4">
		{#each report.traces ?? [] as trace (trace.revision_digest)}
			<Card>
				<CardHeader>
					<div class="flex flex-wrap items-center gap-2">
						<Badge>{trace.harness}</Badge>
						<Badge variant="outline">{trace.status}</Badge>
					</div>
					<CardTitle>
						{#if trace.trace_id}
							<a href={transcriptHref(trace.trace_id)}>{trace.native_trace_id}</a>
						{:else}
							{trace.native_trace_id}
						{/if}
					</CardTitle>
				</CardHeader>
				<CardContent class="flex flex-col gap-2">
					{#if trace.error}<p class="text-destructive text-sm">{trace.error}</p>{/if}
					{#each trace.warnings ?? [] as warning (`${warning.code}:${warning.message}`)}
						<p class="bg-muted rounded-lg border px-3 py-2 text-sm"><strong>{warning.code}</strong>: {warning.message}</p>
					{/each}
					<p class="font-mono text-xs break-all">{trace.revision_digest}</p>
				</CardContent>
			</Card>
		{/each}
	</div>
{/if}
