<script lang="ts">
	import type { TraceEvent } from '$lib/data/types.js';
	import { Badge } from '$lib/components/ui/badge/index.js';
	import { ScrollArea } from '$lib/components/ui/scroll-area/index.js';
	import { Separator } from '$lib/components/ui/separator/index.js';
	import TimeLine from 'remixicon-svelte/icons/time-line';

	const DEFAULT_MS = 80;

	let {
		events,
		model = '',
		recordCount,
		childCount
	}: {
		events: TraceEvent[];
		model?: string;
		recordCount?: number;
		childCount?: number;
	} = $props();

	const spans = $derived.by(() => {
		const timed = events.filter(
			(event) =>
				event.kind === 'tool_call' || event.kind === 'delegation' || event.durationMs != null
		);
		const source = timed.length > 0 ? timed : events;
		return source.map((event) => ({
			id: event.id,
			name: event.tool ?? (event.kind === 'delegation' ? (event.child?.title ?? 'delegate') : event.kind),
			path: event.path ?? event.args ?? event.child?.title ?? '',
			duration: event.durationMs ?? DEFAULT_MS
		}));
	});

	const maxMs = $derived(Math.max(1, ...spans.map((span) => span.duration)));
	const records = $derived(recordCount ?? events.length);
	const children = $derived(childCount ?? events.filter((event) => event.kind === 'delegation').length);

	function formatDuration(ms: number) {
		if (ms >= 1000) return `${(ms / 1000).toFixed(ms >= 10000 ? 0 : 1)}s`;
		return `${Math.round(ms)}ms`;
	}
</script>

<aside class="flex h-full w-[300px] shrink-0 flex-col border-l border-border bg-sidebar">
	<div class="px-3 py-3">
		<div class="mb-1 flex items-center gap-1.5 text-[11px] text-muted-foreground">
			<TimeLine class="size-3.5" />
			<span>Timing</span>
		</div>
		{#if model}
			<p class="truncate font-mono text-[12px]">{model}</p>
		{/if}
		<p class="text-[12px] text-muted-foreground">
			{records} records · {children} child traces
		</p>
	</div>
	<Separator />
	<ScrollArea class="min-h-0 flex-1">
		<ol class="flex flex-col gap-2 px-3 py-3">
			{#each spans as span (span.id)}
				<li class="flex flex-col gap-1">
					<div class="flex items-baseline justify-between gap-2">
						<div class="min-w-0">
							<div class="flex items-center gap-1.5">
								<Badge variant="outline" class="h-4 rounded-sm px-1 font-mono font-normal">
									{span.name}
								</Badge>
							</div>
							{#if span.path}
								<p class="mt-0.5 truncate font-mono text-[10px] text-muted-foreground">{span.path}</p>
							{/if}
						</div>
						<span class="shrink-0 font-mono text-[10px] text-muted-foreground">
							{formatDuration(span.duration)}
						</span>
					</div>
					<div class="bg-muted h-1.5 overflow-hidden rounded-sm">
						<div
							class="h-full rounded-sm bg-foreground/30"
							style:width={`${Math.max(6, (span.duration / maxMs) * 100)}%`}
						></div>
					</div>
				</li>
			{/each}
		</ol>
	</ScrollArea>
</aside>
