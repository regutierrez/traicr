<script lang="ts">
	let { text, tail = 0 }: { text: string; tail?: number } = $props();

	// Keep the end that tells values apart: the last path segment, or the last
	// `tail` characters of a hash. The start gives way first: "/home/user/wo…/repo".
	let split = $derived.by(() => {
		if (tail) return Math.max(0, text.length - tail);
		const cut = text.replace(/\/+$/, '').lastIndexOf('/');
		return cut > 0 ? cut + 1 : text.length;
	});
</script>

<span class="truncate-middle" title={text}><span class="head">{text.slice(0, split)}</span><span class="tail">{text.slice(split)}</span></span>

<style>
	.truncate-middle {
		display: inline-flex;
		max-width: 100%;
		min-width: 0;
		vertical-align: bottom;
		white-space: nowrap;
	}

	.head,
	.tail {
		overflow: hidden;
		text-overflow: ellipsis;
	}

	.head {
		min-width: 0;
		flex-shrink: 1;
	}

	/* The tail shrinks only once the head is gone. */
	.tail {
		max-width: 100%;
		flex-shrink: 0;
	}
</style>
