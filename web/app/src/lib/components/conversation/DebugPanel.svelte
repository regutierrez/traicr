<script lang="ts">
	import { resolve } from '$app/paths';
	import type { LoadedSession } from '$lib/transcript/types';

	let { session }: { session: LoadedSession } = $props();

	const ms = (value: number) => `${Math.round(value).toLocaleString()} ms`;
</script>

<!-- Shown with ?debug=1. The same report reaches GET /api/v1/logs. -->
<section class="debug" aria-label="Load diagnostics">
	<p>
		Loaded {session.timing.events.toLocaleString()} records in {session.timing.pages} pages · fetch {ms(session.timing.fetchMs)}
		· adapter {ms(session.timing.adapterMs)} · {session.entries.length.toLocaleString()} entries
	</p>
	{#if session.anomalies.length}
		<ul>
			{#each session.anomalies as anomaly (anomaly.kind)}
				<li>
					<code>{anomaly.kind}</code>
					{anomaly.message}
					{#if anomaly.eventIds.length}
						<span class="ids">
							events
							{#each anomaly.eventIds as id (id)}
								<a href={resolve('/events/[id]/sources', { id: String(id) })}>{id}</a>
							{/each}
							{#if anomaly.count > anomaly.eventIds.length}…{/if}
						</span>
					{/if}
				</li>
			{/each}
		</ul>
	{:else}
		<p>No adapter anomalies.</p>
	{/if}
</section>

<style>
	.debug {
		margin: 0.5rem 1.25rem 0;
		padding: 0.5rem 0.75rem;
		border: 1px dashed var(--border);
		border-radius: var(--radius);
		color: var(--muted-foreground);
		font-size: 12px;
		line-height: 1.5;
	}

	p,
	ul {
		margin: 0;
	}

	ul {
		padding-left: 1rem;
	}

	code {
		color: var(--foreground);
		font-family: var(--font-mono);
	}

	.ids a {
		margin-left: 0.3rem;
		color: var(--primary);
		font-family: var(--font-mono);
	}
</style>
