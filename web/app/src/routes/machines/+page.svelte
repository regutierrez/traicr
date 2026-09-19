<script lang="ts">
	import { resolve } from '$app/paths';
	import { Card, CardContent, CardHeader, CardTitle } from '$lib/components/ui/card';
	import { withQuery } from '$lib/links';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();
</script>

<svelte:head><title>Source machines · Traicr</title></svelte:head>

<p class="text-primary mb-2 font-mono text-[11px] tracking-widest">WHERE YOUR WORK HAPPENS</p>
<h1 class="text-3xl font-semibold tracking-tight">Source machines</h1>
<p class="text-muted-foreground mt-2 mb-6 max-w-2xl">Machine identity tracks provenance. The same trace stays one trace, wherever it was collected.</p>

{#if data.error}
	<p class="text-destructive mb-4 text-sm" role="alert">{data.error}</p>
{/if}
<div class="grid gap-4 md:grid-cols-2">
	{#each data.machines as machine (machine.id)}
		<Card>
			<CardHeader>
				<CardTitle>{machine.hostname}</CardTitle>
			</CardHeader>
			<CardContent class="flex flex-col gap-3 text-sm">
				<p class="text-muted-foreground">{machine.os} / {machine.arch}</p>
				<dl class="grid grid-cols-[8rem_1fr] gap-2">
					<dt class="text-muted-foreground">Machine ID</dt>
					<dd class="font-mono text-xs break-all">{machine.id}</dd>
					<dt class="text-muted-foreground">First seen</dt>
					<dd>{machine.first_seen}</dd>
					<dt class="text-muted-foreground">Last seen</dt>
					<dd>{machine.last_seen}</dd>
				</dl>
				<a href={withQuery(resolve('/'), { machine: machine.id })}>Search this machine</a>
			</CardContent>
		</Card>
	{:else}
		<p class="text-muted-foreground md:col-span-2">No machines yet. Your source machines appear here after their first upload.</p>
	{/each}
</div>
