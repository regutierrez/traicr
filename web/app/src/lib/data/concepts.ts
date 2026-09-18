import type { ConceptId } from './types';

export const concepts: {
	id: ConceptId;
	name: string;
	eyebrow: string;
	summary: string;
	inspiredBy: string;
	homeHref: string;
	viewerHref: string;
}[] = [
	{
		id: 'inbox',
		name: 'Inbox',
		eyebrow: 'Linear × traces.com',
		summary:
			'A compact transcript inbox: day-grouped rows, command search, and a contextual right pane. The viewer keeps the parent stream live while delegated work opens beside it.',
		inspiredBy: 'Linear density, traces.com feed, issue #9 shell',
		homeHref: '/inbox',
		viewerHref: '/inbox/billing-csv'
	},
	{
		id: 'studio',
		name: 'Studio',
		eyebrow: 'Cursor × AgentTrace',
		summary:
			'A dark agent studio: history rail, composer-like transcript, inline tool cards, and a timing waterfall. Built for reading one session the way Cursor reads an agent.',
		inspiredBy: 'Cursor desktop/web, AgentTrace dashboard',
		homeHref: '/studio',
		viewerHref: '/studio/billing-csv'
	},
	{
		id: 'ledger',
		name: 'Ledger',
		eyebrow: 'OpenTraces × traces.com',
		summary:
			'An evidence ledger: files, patches, and git anchors sit beside the conversation. The homepage previews what the agent saw, did, and changed.',
		inspiredBy: 'OpenTraces evidence layer, traces.com session cards',
		homeHref: '/ledger',
		viewerHref: '/ledger/billing-csv'
	}
];
