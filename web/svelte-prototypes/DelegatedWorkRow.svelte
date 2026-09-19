<script lang="ts">
	type DelegationStatus = 'unread' | 'running' | 'finished';

	interface Props {
		title: string;
		harness: string;
		status: DelegationStatus;
		duration?: string;
		selected?: boolean;
		onselect?: () => void;
	}

	let {
		title,
		harness,
		status,
		duration = '',
		selected = false,
		onselect
	}: Props = $props();
</script>

<button
	type="button"
	class={['delegated-work-row', selected && 'selected']}
	onclick={onselect}
	aria-current={selected ? 'true' : undefined}
>
	<span class="prefix" aria-hidden="true">↳</span>
	<span class="title">{title}</span>
	<span class="meta">
		<span class="status status-{status}" aria-label={status}></span>
		{harness}{duration ? ` · ${duration}` : ''}
		<span class="chevron" aria-hidden="true">›</span>
	</span>
</button>

<style>
	.delegated-work-row {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 12px;
		width: 100%;
		padding: 10px 12px;
		border: 1px solid transparent;
		border-radius: 6px;
		background: transparent;
		color: var(--ink, #202b26);
		font: 0.875rem/1.4 var(--font-mono, ui-monospace, monospace);
		text-align: left;
		cursor: pointer;
	}

	.delegated-work-row:hover,
	.delegated-work-row.selected {
		border-color: var(--green, #25583e);
		background: var(--soft, #eaf0e7);
	}

	.title {
		flex: 1;
		min-width: 0;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
		font-family: var(--font-body, system-ui, sans-serif);
	}

	.meta {
		display: inline-flex;
		align-items: center;
		gap: 8px;
		color: var(--muted, #606d65);
		white-space: nowrap;
	}

	.status {
		width: 8px;
		height: 8px;
		border-radius: 50%;
	}

	.status-unread {
		background: var(--green, #25583e);
	}

	.status-running {
		border: 1.5px solid var(--green, #25583e);
		background: transparent;
	}

	.status-finished::after {
		content: '✓';
		font-size: 0.75rem;
		color: var(--green, #25583e);
	}
</style>
