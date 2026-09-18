<script lang="ts">
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import AlertLine from 'remixicon-svelte/icons/alert-line';
	import ArrowRightSLine from 'remixicon-svelte/icons/arrow-right-s-line';
	import CheckboxCircleLine from 'remixicon-svelte/icons/checkbox-circle-line';
	import ComputerLine from 'remixicon-svelte/icons/computer-line';
	import EyeLine from 'remixicon-svelte/icons/eye-line';
	import FileEditLine from 'remixicon-svelte/icons/file-edit-line';
	import FolderLine from 'remixicon-svelte/icons/folder-line';
	import GitBranchLine from 'remixicon-svelte/icons/git-branch-line';
	import GitCommitLine from 'remixicon-svelte/icons/git-commit-line';
	import TerminalBoxLine from 'remixicon-svelte/icons/terminal-box-line';
	import FileLedger from '$lib/components/ledger/FileLedger.svelte';
	import LedgerShell from '$lib/components/ledger/LedgerShell.svelte';
	import PatchBlock from '$lib/components/ledger/PatchBlock.svelte';
	import { Badge } from '$lib/components/ui/badge';
	import { Button } from '$lib/components/ui/button';
	import { Collapsible, CollapsibleContent, CollapsibleTrigger } from '$lib/components/ui/collapsible';
	import { Kbd } from '$lib/components/ui/kbd';
	import { ScrollArea } from '$lib/components/ui/scroll-area';
	import { Separator } from '$lib/components/ui/separator';
	import { Tabs, TabsList, TabsTrigger } from '$lib/components/ui/tabs';
	import { childTranscripts, featuredEvents, sessionById } from '$lib/data/archive';
	import type { ChildTrace, FileChange, Session, TraceEvent } from '$lib/data/types';

	let selectedPath = $state('');
	let pane = $state('said');

	const session = $derived(sessionById(page.params.id ?? ''));
	const events = $derived(session ? eventsFor(session) : []);
	const files = $derived(session ? collectFiles(session, events) : []);
	const patches = $derived(events.filter((event) => event.diff));
	const children = $derived(session?.children ?? []);

	function inventDiff(file: FileChange): string {
		const leaf = file.path.split('/').at(-1) ?? file.path;
		const plus = file.additions ?? 0;
		const minus = file.deletions ?? 0;
		const lines = [`@@ ${leaf} +${plus} −${minus}`];
		if (file.action === 'create') {
			lines.push(`+open ${leaf}`, `+keep the existing path`, `+return nil`);
		} else if (file.action === 'delete') {
			lines.push(`-drop ${leaf}`, `-return leftover`);
		} else if (minus || plus) {
			if (minus) lines.push(`-previous ${leaf} path`);
			if (plus) lines.push(`+updated ${leaf}`, `+keep existing totals`);
		} else {
			lines.push(`+touch ${leaf}`);
		}
		return lines.join('\n');
	}

	function eventsFor(current: Session): TraceEvent[] {
		if (current.id === 'billing-csv') return featuredEvents;

		const stamp = current.time;
		const next: TraceEvent[] = [
			{
				id: `${current.id}-ask`,
				role: 'user',
				kind: 'message',
				timestamp: stamp,
				content: current.preview
			}
		];

		for (const [index, file] of current.files.entries()) {
			if (file.action === 'read') {
				next.push({
					id: `${current.id}-saw-${index}`,
					role: 'assistant',
					kind: 'tool_call',
					timestamp: stamp,
					tool: 'read',
					path: file.path,
					args: `path: ${file.path}`,
					content: `Opened ${file.path.split('/').at(-1)}.`
				});
				continue;
			}

			next.push({
				id: `${current.id}-chg-${index}`,
				role: 'assistant',
				kind: 'tool_call',
				timestamp: stamp,
				tool: 'edit',
				path: file.path,
				args: `${file.action} ${file.path}`,
				content:
					file.action === 'create'
						? `Created ${file.path}.`
						: file.action === 'delete'
							? `Removed ${file.path}.`
							: `Edited ${file.path}.`,
				diff: inventDiff(file)
			});
		}

		if (current.commit) {
			next.push({
				id: `${current.id}-did`,
				role: 'assistant',
				kind: 'tool_call',
				timestamp: stamp,
				tool: 'bash',
				args: 'git log -1 --oneline',
				content: 'git log -1 --oneline',
				output: `${current.commit} ${current.title}`
			});
		}

		for (const child of current.children) {
			next.push({
				id: `${current.id}-child-${child.id}`,
				role: 'assistant',
				kind: 'delegation',
				timestamp: stamp,
				content: child.title,
				child
			});
		}

		return next;
	}

	function collectFiles(current: Session, list: TraceEvent[]): FileChange[] {
		const next = current.files.map((file) => ({ ...file }));
		for (const event of list) {
			if (!event.path || next.some((file) => file.path === event.path)) continue;
			next.push({
				path: event.path,
				action: event.tool === 'read' ? 'read' : 'edit'
			});
		}
		return next;
	}

	function verb(event: TraceEvent): 'said' | 'saw' | 'did' | 'changed' | 'reasoned' | 'delegated' {
		if (event.kind === 'delegation') return 'delegated';
		if (event.kind === 'reasoning') return 'reasoned';
		if (event.kind === 'message') return 'said';
		if (event.tool === 'read') return 'saw';
		if (event.tool === 'edit' || event.diff) return 'changed';
		if (event.tool === 'bash') return 'did';
		if (event.kind === 'tool_call' || event.kind === 'tool_result') return 'did';
		return 'said';
	}

	function related(event: TraceEvent): boolean {
		return Boolean(selectedPath && event.path === selectedPath);
	}

	function childEvents(child: ChildTrace): TraceEvent[] {
		return (
			childTranscripts[child.id] ?? [
				{
					id: `${child.id}-preview`,
					role: 'assistant',
					kind: 'message',
					timestamp: session?.time ?? '',
					content: child.preview
				}
			]
		);
	}

	function statusLabel(status: ChildTrace['status']) {
		if (status === 'finished') return 'finished';
		if (status === 'running') return 'running';
		return 'unread';
	}
</script>

{#if !session}
	<LedgerShell title="Missing trace" kicker="Evidence layer">
		<div class="flex h-full flex-col items-start justify-center gap-3 px-6">
			<p class="font-mono text-xs text-muted-foreground">No session for {page.params.id}.</p>
			<Button href={resolve('/ledger')} size="sm" class="rounded-sm">Back to ledger</Button>
		</div>
	</LedgerShell>
{:else}
	<LedgerShell title={session.title} kicker="What the agent saw, did, and changed">
		{#snippet toolbar()}
			<div class="hidden min-w-0 items-center gap-2 font-mono text-[11px] text-muted-foreground xl:flex">
				<span class="truncate">{session.repository}</span>
				<span aria-hidden="true">·</span>
				<span>{session.harness}</span>
				{#if session.commit}
					<span aria-hidden="true">·</span>
					<Kbd class="h-4 rounded-sm px-1">{session.commit}</Kbd>
				{/if}
			</div>
		{/snippet}

		<div class="flex h-full min-h-0 flex-col">
			<div class="flex items-start justify-between gap-3 border-b border-border px-3 py-2.5 sm:px-4">
				<div class="min-w-0">
					<h1 class="text-base font-medium tracking-tight sm:text-lg">
						What the agent saw, did, and changed
					</h1>
					<p class="mt-1 font-mono text-[11px] text-muted-foreground">
						{session.time} · {session.cwd} · {session.machine} · {session.model}
					</p>
				</div>
				<div class="flex flex-wrap items-center justify-end gap-1.5">
					{#each session.warnings as warning (warning)}
						<Badge variant="outline" class="max-w-56 truncate rounded-sm font-normal">
							<AlertLine class="size-3" />
							{warning}
						</Badge>
					{/each}
				</div>
			</div>

			<Tabs bind:value={pane} class="flex min-h-0 flex-1 flex-col gap-0">
				<TabsList variant="line" class="h-9 shrink-0 rounded-none border-b px-2 md:hidden">
					<TabsTrigger value="said" class="font-mono text-[11px]">Said</TabsTrigger>
					<TabsTrigger value="files" class="font-mono text-[11px]">Files</TabsTrigger>
					<TabsTrigger value="changed" class="font-mono text-[11px]">Changed</TabsTrigger>
				</TabsList>

				<div
					class="grid min-h-0 flex-1 grid-cols-1 md:grid-cols-2 md:grid-rows-[minmax(0,1fr)_minmax(0,1fr)] xl:grid-cols-3 xl:grid-rows-1"
				>
					<section
						class={[
							'min-h-0 flex-col border-border md:row-span-2 xl:row-span-1 xl:border-r',
							pane === 'said' ? 'flex' : 'hidden md:flex'
						]}
					>
						<p
							class="shrink-0 px-3 py-2 font-mono text-[10px] tracking-[0.16em] text-muted-foreground uppercase"
						>
							Conversation · what was said
						</p>
						<Separator />
						<ScrollArea class="min-h-0 flex-1">
							<div class="flex flex-col gap-3 px-3 py-3 sm:px-4">
								{#each events as event (event.id)}
									{@const kind = verb(event)}
									<article
										class={[
											'rounded-sm border border-transparent px-2 py-1.5',
											related(event) && 'border-primary/30 bg-accent/70'
										]}
									>
										<div class="mb-1 flex items-center gap-2 font-mono text-[10px] text-muted-foreground">
											<span class="tabular-nums">{event.timestamp}</span>
											<span class="tracking-[0.14em] uppercase">{kind}</span>
											{#if event.path}
												<span class="min-w-0 truncate">{event.path}</span>
											{/if}
											{#if event.model}
												<span class="ml-auto">{event.model}</span>
											{/if}
										</div>

										{#if event.kind === 'delegation' && event.child}
											<Collapsible>
												<CollapsibleTrigger
													class="flex w-full items-center gap-2 rounded-sm px-0 py-1 text-left hover:bg-accent/50"
												>
													<ArrowRightSLine class="size-3.5 text-muted-foreground" />
													<span class="text-[13px]">{event.child.title}</span>
													<span class="ml-auto font-mono text-[10px] text-muted-foreground">
														{event.child.harness} · {statusLabel(event.child.status)} · {event.child.duration}
													</span>
												</CollapsibleTrigger>
												<CollapsibleContent class="mt-2 ml-4 border-l border-border pl-3">
													{#each childEvents(event.child) as childEvent (childEvent.id)}
														<p class="py-1 text-[13px] leading-6 text-foreground/90">
															<span class="mr-2 font-mono text-[10px] text-muted-foreground"
																>{childEvent.timestamp}</span
															>
															{childEvent.content}
														</p>
													{/each}
												</CollapsibleContent>
											</Collapsible>
										{:else if event.kind === 'reasoning'}
											<p class="text-[13px] leading-6 text-muted-foreground italic">{event.content}</p>
										{:else if kind === 'saw'}
											<p class="flex items-start gap-2 text-[13px] leading-6">
												<EyeLine class="mt-1 size-3.5 shrink-0 text-muted-foreground" />
												<span>{event.content}</span>
											</p>
											{#if event.kind === 'tool_result'}
												<pre
													class="mt-2 overflow-x-auto rounded-sm bg-muted/70 px-2 py-1.5 font-mono text-[11px] leading-5 text-foreground/80">{event.content}</pre>
											{/if}
										{:else if kind === 'changed'}
											<p class="flex items-start gap-2 text-[13px] leading-6">
												<FileEditLine class="mt-1 size-3.5 shrink-0 text-muted-foreground" />
												<span>{event.content}</span>
											</p>
										{:else if kind === 'did'}
											<p class="flex items-start gap-2 text-[13px] leading-6">
												<TerminalBoxLine class="mt-1 size-3.5 shrink-0 text-muted-foreground" />
												<span class="font-mono text-[12px]">{event.args ?? event.content}</span>
											</p>
											{#if event.output}
												<p class="mt-1 font-mono text-[11px] text-muted-foreground">{event.output}</p>
											{/if}
										{:else}
											<p
												class={[
													'text-[13px] leading-6',
													event.role === 'user' ? 'text-foreground' : 'text-foreground/90'
												]}
											>
												{event.content}
											</p>
										{/if}
									</article>
								{/each}
							</div>
						</ScrollArea>
					</section>

					<section
						class={[
							'min-h-0 flex-col border-border md:border-l xl:border-r',
							pane === 'files' ? 'flex' : 'hidden md:flex'
						]}
					>
						<p
							class="shrink-0 px-3 py-2 font-mono text-[10px] tracking-[0.16em] text-muted-foreground uppercase"
						>
							File ledger
						</p>
						<Separator />
						<div class="min-h-0 flex-1">
							<FileLedger files={files} bind:selectedPath />
						</div>
					</section>

					<section
						class={[
							'min-h-0 flex-col border-border md:border-t md:border-l xl:border-t-0',
							pane === 'changed' ? 'flex' : 'hidden md:flex'
						]}
					>
						<p
							class="shrink-0 px-3 py-2 font-mono text-[10px] tracking-[0.16em] text-muted-foreground uppercase"
						>
							Patches / lineage · what changed
						</p>
						<Separator />
						<ScrollArea class="min-h-0 flex-1">
							<div class="flex flex-col gap-3 px-3 py-3">
								<div class="flex flex-col gap-1.5 font-mono text-[11px] text-muted-foreground">
									<p class="flex items-center gap-1.5">
										<GitCommitLine class="size-3.5" />
										{#if session.commit}
											<Kbd class="h-4 rounded-sm px-1 text-foreground">{session.commit}</Kbd>
										{:else}
											<span>unanchored</span>
										{/if}
									</p>
									<p class="flex items-center gap-1.5">
										<FolderLine class="size-3.5" />
										<span>{session.cwd}</span>
									</p>
									<p class="flex items-center gap-1.5">
										<ComputerLine class="size-3.5" />
										<span>{session.machine}</span>
									</p>
								</div>

								{#if patches.length === 0}
									<p class="font-mono text-[11px] text-muted-foreground">No patches recorded.</p>
								{:else}
									{#each patches as event (event.id)}
										<PatchBlock
											diff={event.diff ?? ''}
											path={event.path}
											commit={session.commit}
											highlighted={related(event)}
										/>
									{/each}
								{/if}

								{#if children.length}
									<Separator />
									<p
										class="flex items-center gap-1.5 font-mono text-[10px] tracking-[0.16em] text-muted-foreground uppercase"
									>
										<GitBranchLine class="size-3.5" />
										Child traces
									</p>
									{#each children as child (child.id)}
										<Collapsible>
											<CollapsibleTrigger
												class="flex w-full items-start gap-2 rounded-sm border border-border bg-card px-2.5 py-2 text-left hover:bg-accent/60"
											>
												{#if child.status === 'finished'}
													<CheckboxCircleLine class="mt-0.5 size-3.5 text-[oklch(0.42_0.1_145)]" />
												{:else}
													<span
														class={[
															'mt-1 size-2 rounded-full',
															child.status === 'running'
																? 'border border-[oklch(0.42_0.1_145)]'
																: 'bg-[oklch(0.42_0.1_145)]'
														]}
													></span>
												{/if}
												<span class="min-w-0 flex-1">
													<span class="block text-[13px] text-foreground">{child.title}</span>
													<span class="font-mono text-[10px] text-muted-foreground">
														{child.harness} · {statusLabel(child.status)} · {child.duration} · {child.model}
													</span>
												</span>
											</CollapsibleTrigger>
											<CollapsibleContent class="border-x border-b border-border bg-card px-2.5 py-2">
												{#each childEvents(child) as childEvent (childEvent.id)}
													<div class="py-1.5">
														<p class="font-mono text-[10px] text-muted-foreground">
															{childEvent.timestamp}
															{#if childEvent.path}
																· {childEvent.path}
															{/if}
														</p>
														<p class="text-[13px] leading-6">{childEvent.content}</p>
														{#if childEvent.diff}
															<div class="mt-2">
																<PatchBlock diff={childEvent.diff} path={childEvent.path} />
															</div>
														{/if}
													</div>
												{/each}
											</CollapsibleContent>
										</Collapsible>
									{/each}
								{/if}
							</div>
						</ScrollArea>
					</section>
				</div>
			</Tabs>
		</div>
	</LedgerShell>
{/if}
