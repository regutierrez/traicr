<script lang="ts">
	import EyeLine from 'remixicon-svelte/icons/eye-line';
	import FileEditLine from 'remixicon-svelte/icons/file-edit-line';
	import FileTextLine from 'remixicon-svelte/icons/file-text-line';
	import { Badge } from '$lib/components/ui/badge';
	import { ScrollArea } from '$lib/components/ui/scroll-area';
	import type { FileChange } from '$lib/data/types';

	let {
		files,
		selectedPath = $bindable('')
	}: {
		files: FileChange[];
		selectedPath?: string;
	} = $props();

	function verb(action: FileChange['action']) {
		return action === 'read' ? 'saw' : 'changed';
	}

	function toggle(path: string) {
		selectedPath = selectedPath === path ? '' : path;
	}
</script>

<ScrollArea class="h-full">
	{#if files.length === 0}
		<p class="px-3 py-6 font-mono text-[11px] text-muted-foreground">No files in this trace.</p>
	{:else}
		<ul class="flex flex-col py-1">
			{#each files as file (file.path)}
				<li>
					<button
						type="button"
						onclick={() => toggle(file.path)}
						aria-pressed={selectedPath === file.path}
						class={[
							'flex w-full items-start gap-2 border-l-2 px-3 py-2 text-left transition-colors',
							'hover:bg-accent/70 focus-visible:bg-accent/70 focus-visible:ring-2 focus-visible:ring-ring/40 focus-visible:outline-none',
							selectedPath === file.path ? 'border-l-primary bg-accent/80' : 'border-l-transparent'
						]}
					>
						{#if file.action === 'read'}
							<EyeLine class="mt-0.5 size-3.5 shrink-0 text-muted-foreground" />
						{:else if file.action === 'create'}
							<FileTextLine class="mt-0.5 size-3.5 shrink-0 text-muted-foreground" />
						{:else}
							<FileEditLine class="mt-0.5 size-3.5 shrink-0 text-muted-foreground" />
						{/if}
						<span class="min-w-0 flex-1">
							<span class="block truncate font-mono text-[12px]">{file.path}</span>
							<span class="mt-1 flex flex-wrap items-center gap-1.5">
								<Badge
									variant="outline"
									class="h-4 rounded-sm px-1 font-mono text-[10px] font-normal"
								>
									{verb(file.action)}
								</Badge>
								{#if file.additions}
									<span class="font-mono text-[10px] text-[oklch(0.42_0.1_145)]"
										>+{file.additions}</span
									>
								{/if}
								{#if file.deletions}
									<span class="font-mono text-[10px] text-[oklch(0.5_0.14_25)]"
										>−{file.deletions}</span
									>
								{/if}
							</span>
						</span>
					</button>
				</li>
			{/each}
		</ul>
	{/if}
</ScrollArea>
