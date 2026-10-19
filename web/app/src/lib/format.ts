const time = new Intl.DateTimeFormat(undefined, { hour: 'numeric', minute: '2-digit' });
const weekday = new Intl.DateTimeFormat(undefined, { weekday: 'long', month: 'short', day: 'numeric' });
const fullDay = new Intl.DateTimeFormat(undefined, { month: 'short', day: 'numeric', year: 'numeric' });
const dateTime = new Intl.DateTimeFormat(undefined, { dateStyle: 'medium', timeStyle: 'short' });
const number = new Intl.NumberFormat();

function startOfDay(date: Date) {
	return new Date(date.getFullYear(), date.getMonth(), date.getDate()).getTime();
}

/** "Today", "Yesterday", "Monday, Oct 12", or "Oct 12, 2025" for older years. */
export function dayLabel(value: string, now = new Date()) {
	const date = new Date(value);
	const days = Math.round((startOfDay(now) - startOfDay(date)) / 86_400_000);
	if (days === 0) return 'Today';
	if (days === 1) return 'Yesterday';
	return date.getFullYear() === now.getFullYear() ? weekday.format(date) : fullDay.format(date);
}

export function clockTime(value: string) {
	return time.format(new Date(value));
}

export function dateAndTime(value: string) {
	return dateTime.format(new Date(value));
}

/** "1 record", "880 records". */
export function count(value: number, singular: string, plural = `${singular}s`) {
	return `${number.format(value)} ${value === 1 ? singular : plural}`;
}

/** "https://github.com/owner/repo" becomes "owner/repo"; local identities pass through. */
export function shortRepository(value = '') {
	const match = value.match(/^(?:https?:\/\/|git@)[^/:]+[/:](.+?)(?:\.git)?\/?$/);
	return match ? match[1] : value.replace(/^file:\/\//, '');
}
