<script lang="ts">
	import { page } from '$app/state';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { submitGoForm } from '$lib/forms';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();
	let error = $state('');
	let pending = $state(false);

	async function unlock(event: SubmitEvent) {
		event.preventDefault();
		const form = event.currentTarget;
		if (!(form instanceof HTMLFormElement)) return;
		pending = true;
		error = '';
		try {
			const result = await submitGoForm(form);
			if (result.ok) {
				location.assign(result.url);
				return;
			}
			error = result.message;
		} catch {
			error = 'Could not reach the server. Check that it is running, then try again.';
		} finally {
			pending = false;
		}
	}
</script>

<svelte:head>
	<title>Open your archive · Traicr</title>
</svelte:head>

<section class="login">
	<svg class="mark" viewBox="0 0 20 20" fill="none" aria-hidden="true"><path d="M3 14.5 7.5 6l4 7 4.5-8.5" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"/><circle cx="3" cy="14.5" r="1.6" fill="currentColor"/><circle cx="16" cy="4.5" r="1.6" fill="currentColor"/></svg>
	<h1>Unlock your archive</h1>
	<p class="lede">Enter the admin token configured on this server.</p>
	<form class="form" method="post" action="/login" onsubmit={unlock} aria-busy={pending}>
		<input type="hidden" name="csrf" value={data.csrf} />
		<!-- The page that sent them here; the server checks it stays inside Traicr. -->
		<input type="hidden" name="next" value={page.url.searchParams.get('next') ?? ''} />
		<label class="label" for="token">Admin token</label>
		<Input
			id="token"
			name="token"
			type="password"
			autocomplete="current-password"
			required
			aria-invalid={error ? true : undefined}
			aria-describedby={error ? 'token-error token-hint' : 'token-hint'}
		/>
		{#if error}
			<p class="error" id="token-error" role="alert">{error}</p>
		{/if}
		<Button type="submit" disabled={pending}>{pending ? 'Unlocking…' : 'Unlock archive'}</Button>
	</form>
	<p class="hint" id="token-hint">Use only over your private network or HTTPS.</p>
</section>

<style>
	.login {
		width: min(24rem, 100%);
		padding: 2rem;
		border: 1px solid var(--border);
		border-radius: 0.75rem;
		background: var(--card);
	}

	.mark {
		width: 28px;
		height: 28px;
		margin-bottom: 1.25rem;
		color: var(--primary);
	}

	h1 {
		margin: 0;
		font-size: 1.35rem;
		font-weight: 600;
		letter-spacing: -0.02em;
	}

	.lede,
	.hint {
		margin: 0.5rem 0 0;
		color: var(--muted-foreground);
		font-size: 13px;
	}

	.form {
		display: flex;
		flex-direction: column;
		gap: 0.6rem;
		margin-top: 1.5rem;
	}

	.label {
		font-size: 12px;
		font-weight: 500;
	}

	.error {
		margin: 0;
		color: var(--destructive);
		font-size: 12px;
	}
</style>
