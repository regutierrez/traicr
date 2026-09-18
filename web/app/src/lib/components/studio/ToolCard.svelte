<script lang="ts">
	import type { TraceEvent } from '$lib/data/types.js';
	import { Badge } from '$lib/components/ui/badge/index.js';
	import {
		Collapsible,
		CollapsibleContent,
		CollapsibleTrigger
	} from '$lib/components/ui/collapsible/index.js';
	import { Separator } from '$lib/components/ui/separator/index.js';
	import ArrowRightSLine from 'remixicon-svelte/icons/arrow-right-s-line.svelte';
	import TerminalBoxLine from 'remixicon-svelte/icons/terminal-box-line.svelte';

	let { event }: { event: TraceEvent } = $props();

	let open = $state(false);

	const label = $derived(event.tool ?? event.kind);
	const summary = $derived(event.path ?? event.args ?? event.content);
	const duration = $derived(
		event.durationMs != null
			? event.durationMs >= 1000
				? `${(event.durationMs / 1000).toFixed(event.durationMs >= 10000 ? 0 : 1)}s`
				: `${event.durationMs}ms`
			: null
	);
	const diffLines = $derived(event.diff ? event.diff.split('\n') : []);
</script>

<Collapsible bind:open class="w-full max-w-2xl">
	<div class="overflow-hidden rounded-lg border border-border bg-card">
		<CollapsibleTrigger
			class="flex w-full items-center gap-2 px-2.5 py-1.5 text-left text-[12px] hover:bg-muted/40"
		>
			<ArrowRightSLine
				class={['size-3.5 shrink-0 text-muted-foreground transition-transform', open && 'rotate-90']}
			/>
			<TerminalBoxLine class="size-3.5 shrink-0 text-muted-foreground" />
			<Badge variant="outline" class="h-4 rounded-sm px-1 font-mono font-normal">
				{label}
			</Badge>
			<span class="min-w-0 flex-1 truncate font-mono text-[12px] text-muted-foreground">
				{summary}
			</span>
			{#if duration}
				<span class="shrink-0 font-mono text-[11px] text-muted-foreground">{duration}</span>
			{/if}
		</CollapsibleTrigger>

		<CollapsibleContent>
			<Separator />
			<div class="space-y-2 px-3 py-2">
				{#if event.args}
					<div>
						<p class="mb-1 text-[10px] tracking-wide text-muted-foreground uppercase">Args</p>
						<pre
							class="font-mono text-[11px] leading-relaxed break-all whitespace-pre-wrap text-foreground/80">{event.args}</pre>
					</div>
				{/if}
				{#if event.content && event.content !== event.args && event.content !== event.path}
					<div>
						<p class="mb-1 text-[10px] tracking-wide text-muted-foreground uppercase">Detail</p>
						<pre
							class="font-mono text-[11px] leading-relaxed break-all whitespace-pre-wrap text-foreground/80">{event.content}</pre>
					</div>
				{/if}
				{#if event.output}
					<div>
						<p class="mb-1 text-[10px] tracking-wide text-muted-foreground uppercase">Output</p>
						<pre
							class="font-mono text-[11px] leading-relaxed break-all whitespace-pre-wrap text-foreground/80">{event.output}</pre>
					</div>
				{/if}
				{#if diffLines.length > 0}
					<div>
						<p class="mb-1 text-[10px] tracking-wide text-muted-foreground uppercase">Diff</p>
						<pre class="font-mono text-[11px] leading-relaxed whitespace-pre-wrap">
{#each diffLines as line, index (`${index}-${line}`)}<span
								class={[
									'block',
									line.startsWith('+') && 'text-emerald-400/85',
									line.startsWith('-') && 'text-red-400/80'
								]}>{line || ' '}</span>{/each}</pre>
					</div>
				{/if}
			</div>
		</CollapsibleContent>
	</div>
</Collapsible>
