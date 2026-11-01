<script lang="ts">
	import { page } from '$app/state';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { submitGoForm } from '$lib/forms';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();
	let error = $state('');
	let pending = $state(false);
	// The token crosses the network in clear text only on plain HTTP to another computer.
	const plainHTTP = location.protocol === 'http:' && !['localhost', '127.0.0.1', '[::1]'].includes(location.hostname);
	let describedBy = $derived([error && 'token-error', plainHTTP && 'token-hint'].filter(Boolean).join(' ') || undefined);

	async function unlock(event: SubmitEvent) {
		event.preventDefault();
		const form = event.currentTarget;
		if (!(form instanceof HTMLFormElement)) return;
		pending = true;
		error = '';
		try {
			let result = await submitGoForm(form);
			// A tab left open past the login cookie's 24 hours: GET /login mints a fresh one, then try once more.
			if (!result.ok && result.code === 'login_expired') {
				await fetch('/login', { credentials: 'same-origin' });
				result = await submitGoForm(form);
			}
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
	<title>Unlock · Traicr</title>
</svelte:head>

<section class="login">
	<p class="brand"><svg class="mark" viewBox="0 0 20 20" fill="none" aria-hidden="true"><path d="M3 14.5 7.5 6l4 7 4.5-8.5" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"/><circle cx="3" cy="14.5" r="1.6" fill="currentColor"/><circle cx="16" cy="4.5" r="1.6" fill="currentColor"/></svg><span>Traicr</span></p>
	<h1>Unlock your archive</h1>
	<p class="lede">Enter the admin token configured on this server.</p>
	<form class="form" method="post" action="/login" onsubmit={unlock} aria-busy={pending}>
		<input type="hidden" name="csrf" value={data.csrf} />
		<!-- The page that sent them here; the server checks it stays inside Traicr. -->
		<input type="hidden" name="next" value={page.url.searchParams.get('next') ?? ''} />
		<label class="label" for="token">Admin token</label>
		<!-- The token box is the only thing to do here. SvelteKit resets focus after navigation unless an element has autofocus. -->
		<Input
			autofocus
			id="token"
			name="token"
			type="password"
			autocomplete="current-password"
			required
			aria-invalid={error ? true : undefined}
			aria-describedby={describedBy}
		/>
		{#if error}
			<p class="error" id="token-error" role="alert">{error}</p>
		{/if}
		<Button type="submit" disabled={pending}>{pending ? 'Unlocking…' : 'Unlock'}</Button>
	</form>
	{#if plainHTTP}
		<p class="hint" id="token-hint">This connection is not encrypted. Use it only on your private network, or set up HTTPS.</p>
	{/if}
</section>

<style>
	.login {
		width: min(24rem, 100%);
		padding: 2rem;
		border: 1px solid var(--border);
		border-radius: 0.75rem;
		background: var(--card);
	}

	.brand {
		display: flex;
		align-items: center;
		gap: 0.5rem;
		margin: 0 0 1.25rem;
		font-size: 15px;
		font-weight: 600;
		letter-spacing: -0.01em;
	}

	.mark {
		width: 24px;
		height: 24px;
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
