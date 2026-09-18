<script lang="ts">
	import type { Session } from '$lib/data/types';

	let {
		session,
		selected = false,
		onclick,
		ondblclick
	}: {
		session: Session;
		selected?: boolean;
		onclick?: (event: MouseEvent) => void;
		ondblclick?: (event: MouseEvent) => void;
	} = $props();

	const repo = $derived(session.repository.split('/').at(-1) ?? session.repository);
</script>

<button
	type="button"
	{onclick}
	{ondblclick}
	aria-pressed={selected}
	class={[
		'grid w-full grid-cols-[4.75rem_minmax(0,5.25rem)_minmax(0,6.75rem)_2.25rem_minmax(0,1fr)] items-center gap-x-2 border-l-2 px-3 py-1.5 text-left transition-colors',
		'hover:bg-accent/70 focus-visible:bg-accent/70 focus-visible:ring-2 focus-visible:ring-ring/40 focus-visible:outline-none',
		selected ? 'border-l-primary bg-accent/80' : 'border-l-transparent'
	]}
>
	<span class="font-mono text-[11px] text-muted-foreground tabular-nums">{session.time}</span>
	<span class="truncate font-mono text-[11px] text-muted-foreground">{repo}</span>
	<span class="truncate font-mono text-[11px] text-muted-foreground">{session.harness}</span>
	<span class="text-right font-mono text-[11px] text-muted-foreground tabular-nums"
		>{session.eventCount}</span
	>
	<span class="truncate font-sans text-[13px] text-foreground">{session.title}</span>
</button>
