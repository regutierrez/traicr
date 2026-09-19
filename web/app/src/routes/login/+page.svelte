<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '$lib/components/ui/card';
	import { Input } from '$lib/components/ui/input';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();
</script>

<svelte:head>
	<title>Open your archive · Traicr</title>
</svelte:head>

<section class="max-w-md flex-1">
	<p class="text-primary mb-2 font-mono text-[11px] tracking-widest">A MEMORY FOR YOUR WORK</p>
	<h1 class="text-4xl font-semibold tracking-tight">Every trace.<br />All in one place.</h1>
	<p class="text-muted-foreground mt-4">Your coding history belongs to you. Preserve the original records and find the details worth coming back to.</p>
</section>

<Card class="w-full max-w-md">
	<CardHeader>
		<CardTitle>Open your archive</CardTitle>
		<CardDescription>Use the admin token configured on your Traicr server.</CardDescription>
	</CardHeader>
	<CardContent>
		<form class="flex flex-col gap-3" method="post" action="/login" data-sveltekit-reload>
			<input type="hidden" name="csrf" value={data.csrf} />
			<label class="text-sm font-medium" for="token">Admin token</label>
			<Input id="token" name="token" type="password" autocomplete="current-password" required />
			<Button type="submit" size="lg">Unlock archive</Button>
		</form>
		<p class="text-muted-foreground mt-3 text-xs">Use only over your private network or HTTPS.</p>
	</CardContent>
</Card>
