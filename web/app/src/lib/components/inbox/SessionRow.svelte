<script lang="ts">
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import { Badge } from '$lib/components/ui/badge/index.js';
	import type { Session } from '$lib/data/types';

	let {
		session,
		selected = false
	}: {
		session: Session;
		selected?: boolean;
	} = $props();

	const current = $derived(page.params.id === session.id);
	const active = $derived(selected || current);
	const repo = $derived(session.repository.split('/').slice(-2).join('/'));
</script>

<a
	href={resolve('/inbox/[id]', { id: session.id })}
	class={[
		'flex h-11 w-full min-w-0 items-center gap-2.5 px-3 text-left text-[13px] outline-none',
		'hover:bg-sidebar-accent/70 focus-visible:bg-sidebar-accent',
		active && 'bg-sidebar-accent text-sidebar-accent-foreground'
	]}
	role="option"
	aria-current={current ? 'page' : undefined}
	aria-selected={active}
>
	<span class="text-muted-foreground w-16 shrink-0 tabular-nums">{session.time}</span>
	<span class="text-muted-foreground hidden min-w-0 shrink truncate sm:block">{repo}</span>
	<Badge variant="outline" class="h-5 shrink-0 px-1.5 text-[11px] font-normal">
		{session.harness}
	</Badge>
	<span class="text-muted-foreground w-7 shrink-0 text-right tabular-nums">{session.eventCount}</span>
	<span class="min-w-0 shrink-[1.5] truncate font-medium">{session.title}</span>
	<span class="text-muted-foreground hidden min-w-0 flex-1 truncate xl:block">{session.preview}</span>
</a>
