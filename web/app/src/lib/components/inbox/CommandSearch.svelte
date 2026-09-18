<script lang="ts">
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { Badge } from '$lib/components/ui/badge/index.js';
	import * as Command from '$lib/components/ui/command/index.js';
	import { groupSessions, sessions } from '$lib/data/archive';
	import type { Session } from '$lib/data/types';

	let { open = $bindable(false) }: { open?: boolean } = $props();

	const groups = $derived.by(() => {
		const grouped = groupSessions(sessions);
		return [
			{ id: 'today', heading: 'Today', items: grouped.today },
			{ id: 'yesterday', heading: 'Yesterday', items: grouped.yesterday },
			{ id: 'earlier', heading: 'Earlier', items: grouped.earlier }
		].filter((group) => group.items.length > 0);
	});

	function openSession(session: Session) {
		open = false;
		goto(resolve('/inbox/[id]', { id: session.id }));
	}
</script>

<Command.Dialog
	bind:open
	title="Search traces"
	description="Filter sessions by title, repository, or harness"
>
	<Command.Input placeholder="Search traces…" />
	<Command.List>
		<Command.Empty class="text-muted-foreground py-8 text-sm">No matching traces.</Command.Empty>
		{#each groups as group (group.id)}
			<Command.Group heading={group.heading}>
				{#each group.items as session (session.id)}
					<Command.Item
						value="{session.title} {session.repository} {session.harness} {session.id}"
						keywords={[session.title, session.repository, session.harness, session.preview]}
						onSelect={() => openSession(session)}
					>
						<span class="min-w-0 flex-1 truncate">{session.title}</span>
						<span class="text-muted-foreground hidden truncate text-xs sm:inline">
							{session.repository.split('/').slice(-2).join('/')}
						</span>
						<Badge variant="outline" class="h-5 px-1.5 text-[11px] font-normal">
							{session.harness}
						</Badge>
					</Command.Item>
				{/each}
			</Command.Group>
		{/each}
	</Command.List>
</Command.Dialog>
