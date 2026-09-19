<script lang="ts">
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
			error = 'Could not reach the server';
		} finally {
			pending = false;
		}
	}
</script>

<svelte:head>
	<title>Open your archive · Traicr</title>
</svelte:head>

<section class="login">
	<p class="meta">Private archive</p>
	<h1>Unlock Traicr</h1>
	<p class="lede">Use the admin token configured on this server. The original records stay yours.</p>
	<form class="form" method="post" action="/login" onsubmit={unlock} aria-busy={pending}>
		<input type="hidden" name="csrf" value={data.csrf} />
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
		width: min(22rem, 100%);
	}

	.meta {
		margin: 0 0 0.4rem;
		color: var(--muted-foreground);
		font-family: var(--font-mono);
		font-size: 11px;
		letter-spacing: 0.08em;
		text-transform: uppercase;
	}

	h1 {
		margin: 0;
		font-size: 1.35rem;
		font-weight: 600;
		letter-spacing: -0.02em;
	}

	.lede,
	.hint {
		margin: 0.55rem 0 0;
		color: var(--muted-foreground);
		font-size: 12px;
	}

	.form {
		display: flex;
		flex-direction: column;
		gap: 0.55rem;
		margin-top: 1.15rem;
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
