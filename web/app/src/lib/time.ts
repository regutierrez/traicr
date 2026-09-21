export function relativeTime(value: string, now = Date.now()) {
	const then = Date.parse(value);
	if (Number.isNaN(then)) return value;
	const minutes = Math.round((now - then) / 60000);
	if (Math.abs(minutes) < 1) return 'just now';
	if (Math.abs(minutes) < 60) return `${minutes}m ago`;
	const hours = Math.round(minutes / 60);
	if (Math.abs(hours) < 24) return `${hours}h ago`;
	const days = Math.round(hours / 24);
	if (Math.abs(days) < 30) return `${days}d ago`;
	const months = Math.round(days / 30);
	if (Math.abs(months) < 12) return `${months}mo ago`;
	return `${Math.round(months / 12)}y ago`;
}

export function absoluteTime(value: string) {
	const date = new Date(value);
	if (Number.isNaN(date.getTime())) return value;
	return new Intl.DateTimeFormat('en-US', {
		month: 'short',
		day: 'numeric',
		hour: 'numeric',
		minute: '2-digit'
	}).format(date);
}

export function dayGroup(value: string, now = new Date()) {
	const date = new Date(value);
	if (Number.isNaN(date.getTime())) return 'Undated';
	const start = (item: Date) => new Date(item.getFullYear(), item.getMonth(), item.getDate()).getTime();
	const days = Math.round((start(now) - start(date)) / 86400000);
	if (days === 0) return 'Today';
	if (days === 1) return 'Yesterday';
	if (days > 1 && days < 7) return new Intl.DateTimeFormat('en-US', { weekday: 'long' }).format(date);
	return new Intl.DateTimeFormat('en-US', { month: 'short', day: 'numeric', year: 'numeric' }).format(date);
}

export function repoLabel(value: string) {
	const repo = value.trim();
	if (!repo) return '';
	try {
		if (repo.startsWith('http://') || repo.startsWith('https://')) {
			const url = new URL(repo);
			return url.pathname.replace(/^\//, '').replace(/\.git$/, '') || url.host;
		}
	} catch {
		// Keep the raw identity when the recorded value is not a URL.
	}
	const parts = repo.split(/[/\\]/).filter(Boolean);
	return parts.slice(-2).join('/') || repo;
}
