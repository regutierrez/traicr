<script lang="ts">
	import { onMount } from 'svelte';

	// The transcript viewer is still served by Go. A client-side hit reloads onto that page.
	onMount(() => {
		location.reload();
	});
</script>

<p class="text-muted-foreground">Opening transcript…</p>
