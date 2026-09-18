<script lang="ts">
	import GitCommitLine from 'remixicon-svelte/icons/git-commit-line.svelte';
	import { Kbd } from '$lib/components/ui/kbd';

	let {
		diff,
		path,
		commit,
		highlighted = false
	}: {
		diff: string;
		path?: string;
		commit?: string;
		highlighted?: boolean;
	} = $props();

	const lines = $derived(diff.split('\n'));

	function tone(line: string) {
		if (line.startsWith('+')) {
			return 'bg-[oklch(0.74_0.08_145/0.14)] text-[oklch(0.38_0.09_145)]';
		}
		if (line.startsWith('-')) {
			return 'bg-[oklch(0.72_0.1_25/0.14)] text-[oklch(0.46_0.13_25)]';
		}
		return 'text-muted-foreground';
	}
</script>

<figure
	class={[
		'overflow-hidden rounded-sm border bg-card',
		highlighted ? 'border-primary/50 ring-1 ring-primary/20' : 'border-border'
	]}
>
	{#if path || commit}
		<figcaption
			class="flex items-center gap-2 border-b border-border px-2.5 py-1.5 font-mono text-[10px] text-muted-foreground"
		>
			{#if path}
				<span class="min-w-0 truncate">{path}</span>
			{/if}
			{#if commit}
				<span class="ml-auto inline-flex items-center gap-1">
					<GitCommitLine class="size-3" />
					<Kbd class="h-4 min-w-0 rounded-sm px-1 font-mono text-[10px]">{commit}</Kbd>
				</span>
			{/if}
		</figcaption>
	{/if}
	<pre class="overflow-x-auto p-0 text-[11px] leading-5"><code class="block"
			>{#each lines as line, index (`${index}:${line}`)}<span
					class={['block px-2.5 whitespace-pre', tone(line)]}>{line || ' '}</span
				>{/each}</code
		></pre>
</figure>
