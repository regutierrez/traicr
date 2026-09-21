export function latestRevisionId(
	revisions: { id: number; native_updated_at?: string; collected_at?: string }[] | undefined
) {
	if (!revisions?.length) return '';
	const time = (item: { native_updated_at?: string; collected_at?: string }) => {
		const parsed = Date.parse(item.native_updated_at || item.collected_at || '');
		return Number.isNaN(parsed) ? 0 : parsed;
	};
	const best = revisions.reduce((left, right) => {
		const delta = time(right) - time(left);
		if (delta !== 0) return delta > 0 ? right : left;
		return right.id > left.id ? right : left;
	});
	return String(best.id);
}
