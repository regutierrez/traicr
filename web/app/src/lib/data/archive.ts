import type { Session, TraceEvent } from './types';

export const sessions: Session[] = [
	{
		id: 'billing-csv',
		title: 'Add CSV export to billing',
		harness: 'pi',
		nativeTraceId: '4d1a8c2e-91b0-4f11-a7e2-0b3c9d12aa01',
		repository: 'github.com/regutierrez/ledger',
		cwd: '~/src/ledger',
		machine: 'macbook',
		model: 'claude-opus-4',
		updatedAt: '2026-09-18T14:22:00Z',
		day: 'today',
		time: '2:22 PM',
		eventCount: 42,
		revisionCount: 3,
		preview:
			'Plan the export flow, then wire a billing CSV that escapes newlines and keeps existing invoice totals intact.',
		children: [
			{
				id: 'csv-tests',
				title: 'Unit tests for CSV',
				harness: 'pi',
				status: 'finished',
				duration: '4m 12s',
				model: 'claude-sonnet-4',
				preview: 'Added table-driven tests for quoting, newlines, and empty invoices.'
			},
			{
				id: 'csv-docs',
				title: 'API docs for export',
				harness: 'pi',
				status: 'unread',
				duration: '2m 08s',
				model: 'claude-sonnet-4',
				preview: 'Documented GET /billing/export.csv and the quoting rules.'
			},
			{
				id: 'csv-flaky',
				title: 'Flaky pipeline probe',
				harness: 'pi',
				status: 'running',
				duration: '1m 40s',
				model: 'claude-haiku-4',
				preview: 'Reproducing the export job flake on empty date ranges.'
			}
		],
		files: [
			{ path: 'report/export.go', action: 'create', additions: 148, deletions: 0 },
			{ path: 'report/csv.go', action: 'edit', additions: 36, deletions: 8 },
			{ path: 'web/billing/page.tsx', action: 'edit', additions: 22, deletions: 4 },
			{ path: 'report/export_test.go', action: 'create', additions: 91, deletions: 0 }
		],
		commit: '8f21c0a',
		warnings: []
	},
	{
		id: 'voltage-errors',
		title: 'Diagnose voltage error reporting',
		harness: 'claude-code',
		nativeTraceId: 'cc-voltage-102',
		repository: 'github.com/acme/gridkit',
		cwd: '~/work/gridkit',
		machine: 'studio',
		model: 'claude-opus-4',
		updatedAt: '2026-09-18T12:45:00Z',
		day: 'today',
		time: '12:45 PM',
		eventCount: 102,
		revisionCount: 2,
		preview:
			'Trace why undervoltage events lose their source span after compaction, then keep the original probe id.',
		children: [],
		files: [
			{ path: 'internal/alerts/voltage.go', action: 'edit', additions: 41, deletions: 19 },
			{ path: 'internal/alerts/voltage_test.go', action: 'edit', additions: 27, deletions: 3 }
		],
		commit: 'c41e9aa',
		warnings: ['One observation conflicted across revisions']
	},
	{
		id: 'hero-video',
		title: 'HeroVideo.tsx terminal-like preview',
		harness: 'claude-code',
		nativeTraceId: 'cc-hero-13',
		repository: 'github.com/market/traces',
		cwd: '~/src/traces',
		machine: 'macbook',
		model: 'claude-sonnet-4',
		updatedAt: '2026-09-18T12:45:00Z',
		day: 'today',
		time: '12:45 PM',
		eventCount: 13,
		revisionCount: 1,
		preview: 'Tighten the homepage terminal preview so session rows stay one line at 1280px.',
		children: [],
		files: [{ path: 'web/components/HeroVideo.tsx', action: 'edit', additions: 18, deletions: 11 }],
		warnings: []
	},
	{
		id: 'cursor-terminal',
		title: 'Terminal not working troubleshooting',
		harness: 'cursor',
		nativeTraceId: 'cursor-term-10',
		repository: 'github.com/regutierrez/traicr',
		cwd: '~/src/traicr',
		machine: 'studio',
		model: 'gpt-5',
		updatedAt: '2026-09-18T12:25:00Z',
		day: 'today',
		time: '12:25 PM',
		eventCount: 10,
		revisionCount: 1,
		preview: 'PTY spawn failed after the collector updated PATH; restore the shell without rewriting session files.',
		children: [],
		files: [{ path: 'internal/collector/adapters/commands.go', action: 'read' }],
		warnings: []
	},
	{
		id: 'chat-intro',
		title: 'Light chat intro clarifications',
		harness: 'codex',
		nativeTraceId: 'codex-intro-112',
		repository: 'github.com/regutierrez/traicr',
		cwd: '~/src/traicr',
		machine: 'macbook',
		model: 'gpt-5-codex',
		updatedAt: '2026-09-18T12:44:00Z',
		day: 'today',
		time: '12:44 PM',
		eventCount: 112,
		revisionCount: 4,
		preview: 'Keep the login card quiet and move the private-archive copy into the eyebrow.',
		children: [],
		files: [
			{ path: 'web/templates/login.html', action: 'edit', additions: 14, deletions: 9 },
			{ path: 'web/static/app.css', action: 'edit', additions: 21, deletions: 6 }
		],
		commit: 'b19d441',
		warnings: []
	},
	{
		id: 'homehero-fade',
		title: 'HomeHero background dot fade',
		harness: 'amp',
		nativeTraceId: 'amp-fade-74',
		repository: 'github.com/market/traces',
		cwd: '~/src/traces',
		machine: 'macbook',
		model: 'claude-sonnet-4',
		updatedAt: '2026-09-18T11:47:00Z',
		day: 'today',
		time: '11:47 AM',
		eventCount: 74,
		revisionCount: 2,
		preview: 'Fade the hero dots without shifting layout; keep the first paint on one composited layer.',
		children: [],
		files: [{ path: 'web/components/HomeHero.tsx', action: 'edit', additions: 9, deletions: 7 }],
		warnings: []
	},
	{
		id: 'schema-parts',
		title: 'Schema validation error in parts table',
		harness: 'opencode',
		nativeTraceId: 'oc-schema-59',
		repository: 'github.com/acme/parts',
		cwd: '~/work/parts',
		machine: 'studio',
		model: 'kimi-k2',
		updatedAt: '2026-09-18T10:00:00Z',
		day: 'today',
		time: '10:00 AM',
		eventCount: 59,
		revisionCount: 1,
		preview: 'Reject empty part SKUs before insert and keep the existing unique index.',
		children: [],
		files: [{ path: 'db/parts.sql', action: 'edit', additions: 6, deletions: 1 }],
		warnings: ['Partial parse: one unknown OpenCode record retained']
	},
	{
		id: 'onboarding',
		title: 'Improving app onboarding and hero section',
		harness: 'grok-build',
		nativeTraceId: 'grok-onboard-386',
		repository: 'github.com/regutierrez/traicr',
		cwd: '~/src/traicr',
		machine: 'studio',
		model: 'grok-4',
		updatedAt: '2026-09-18T11:28:00Z',
		day: 'today',
		time: '11:28 AM',
		eventCount: 386,
		revisionCount: 6,
		preview: 'Replace the empty-state card with a quieter first-run list and keep collect/upload as copyable commands.',
		children: [],
		files: [
			{ path: 'web/templates/search.html', action: 'edit', additions: 48, deletions: 33 },
			{ path: 'web/static/app.css', action: 'edit', additions: 27, deletions: 12 }
		],
		commit: 'e1ff7d6',
		warnings: []
	},
	{
		id: 'commit-order',
		title: 'Logical commit order strategy',
		harness: 'pi',
		nativeTraceId: 'pi-commits-18',
		repository: 'github.com/regutierrez/traicr',
		cwd: '~/src/traicr',
		machine: 'macbook',
		model: 'claude-opus-4',
		updatedAt: '2026-09-18T12:02:00Z',
		day: 'today',
		time: '12:02 PM',
		eventCount: 18,
		revisionCount: 1,
		preview: 'Split collector, normalizer, and viewer changes so each commit stays bisectable.',
		children: [],
		files: [],
		warnings: []
	},
	{
		id: 'home-feed',
		title: 'HomeFeed traces list redesign',
		harness: 'amp',
		nativeTraceId: 'amp-feed-255',
		repository: 'github.com/market/traces',
		cwd: '~/src/traces',
		machine: 'macbook',
		model: 'claude-opus-4',
		updatedAt: '2026-09-17T12:31:00Z',
		day: 'yesterday',
		time: '12:31 PM',
		eventCount: 255,
		revisionCount: 5,
		preview: 'Move from cards to a day-grouped feed without losing harness, repo, or match context.',
		children: [],
		files: [{ path: 'web/components/HomeFeed.tsx', action: 'edit', additions: 120, deletions: 86 }],
		commit: '025bd08',
		warnings: []
	},
	{
		id: 'og-images',
		title: 'Next.js OpenGraph image route',
		harness: 'cursor-agent',
		nativeTraceId: 'ca-og-14',
		repository: 'github.com/market/traces',
		cwd: '~/src/traces',
		machine: 'studio',
		model: 'composer',
		updatedAt: '2026-09-17T15:31:00Z',
		day: 'yesterday',
		time: '3:31 PM',
		eventCount: 14,
		revisionCount: 1,
		preview: 'Render OG images from session title and harness without fetching remote fonts.',
		children: [],
		files: [{ path: 'app/s/[id]/opengraph-image.tsx', action: 'create', additions: 84, deletions: 0 }],
		warnings: []
	},
	{
		id: 'density-chart',
		title: 'Fix WeeklyDensityChart data rendering',
		harness: 'codex',
		nativeTraceId: 'codex-chart-72',
		repository: 'github.com/acme/gridkit',
		cwd: '~/work/gridkit',
		machine: 'studio',
		model: 'gpt-5-codex',
		updatedAt: '2026-09-16T16:56:00Z',
		day: 'earlier',
		time: '4:56 PM',
		eventCount: 72,
		revisionCount: 2,
		preview: 'Zero values were plotting as gaps; keep them on the baseline.',
		children: [],
		files: [{ path: 'web/charts/WeeklyDensityChart.tsx', action: 'edit', additions: 17, deletions: 9 }],
		warnings: []
	}
];

export const featuredEvents: TraceEvent[] = [
	{
		id: 'e1',
		role: 'user',
		kind: 'message',
		timestamp: '14:04',
		content:
			'Add a CSV export to billing. Keep existing invoice totals, escape newlines, and make it easy to download from the billing page.'
	},
	{
		id: 'e2',
		role: 'assistant',
		kind: 'reasoning',
		timestamp: '14:04',
		model: 'claude-opus-4',
		content:
			'The export should live next to invoice rendering, reuse the current total calculation, and quote fields the way spreadsheets expect. Tests first for quoting, then the download control.'
	},
	{
		id: 'e3',
		role: 'assistant',
		kind: 'message',
		timestamp: '14:05',
		model: 'claude-opus-4',
		content:
			'I will keep export next to the existing invoice renderer, reuse the current totals, and add quoting tests before wiring the download button.'
	},
	{
		id: 'e4',
		role: 'assistant',
		kind: 'tool_call',
		timestamp: '14:05',
		tool: 'read',
		path: 'report/render.go',
		args: 'path: report/render.go',
		durationMs: 42,
		content: 'Opened invoice renderer to reuse totals.'
	},
	{
		id: 'e5',
		role: 'tool',
		kind: 'tool_result',
		timestamp: '14:05',
		tool: 'read',
		path: 'report/render.go',
		content: 'func RenderInvoice(inv Invoice) []byte {\n    total := inv.Subtotal + inv.Tax\n    return render(inv, total)\n}'
	},
	{
		id: 'e6',
		role: 'assistant',
		kind: 'tool_call',
		timestamp: '14:07',
		tool: 'edit',
		path: 'report/export.go',
		args: 'create report/export.go',
		durationMs: 310,
		content: 'Wrote CSV exporter with quoting and newline escaping.',
		diff: '+func ExportCSV(invoices []Invoice) []byte {\n+    var b bytes.Buffer\n+    w := csv.NewWriter(&b)\n+    _ = w.Write([]string{"id", "total", "memo"})\n+    for _, inv := range invoices {\n+        _ = w.Write([]string{inv.ID, formatMoney(inv.Total()), escape(inv.Memo)})\n+    }\n+    w.Flush()\n+    return b.Bytes()\n+}'
	},
	{
		id: 'e7',
		role: 'assistant',
		kind: 'delegation',
		timestamp: '14:09',
		content: 'Unit tests for CSV',
		child: {
			id: 'csv-tests',
			title: 'Unit tests for CSV',
			harness: 'pi',
			status: 'finished',
			duration: '4m 12s',
			model: 'claude-sonnet-4',
			preview: 'Added table-driven tests for quoting, newlines, and empty invoices.'
		}
	},
	{
		id: 'e8',
		role: 'assistant',
		kind: 'tool_call',
		timestamp: '14:12',
		tool: 'bash',
		args: 'go test ./report',
		durationMs: 1840,
		content: 'go test ./report',
		output: 'ok  github.com/regutierrez/ledger/report  0.041s'
	},
	{
		id: 'e9',
		role: 'assistant',
		kind: 'tool_call',
		timestamp: '14:14',
		tool: 'edit',
		path: 'web/billing/page.tsx',
		args: 'add download control',
		durationMs: 220,
		content: 'Wired the export button on the billing page.',
		diff: '+<button onClick={downloadCsv}>Export CSV</button>'
	},
	{
		id: 'e10',
		role: 'assistant',
		kind: 'delegation',
		timestamp: '14:15',
		content: 'API docs for export',
		child: {
			id: 'csv-docs',
			title: 'API docs for export',
			harness: 'pi',
			status: 'unread',
			duration: '2m 08s',
			model: 'claude-sonnet-4',
			preview: 'Documented GET /billing/export.csv and the quoting rules.'
		}
	},
	{
		id: 'e11',
		role: 'user',
		kind: 'message',
		timestamp: '14:18',
		content: 'Also escape newlines inside memo fields. The last export broke Numbers.'
	},
	{
		id: 'e12',
		role: 'assistant',
		kind: 'tool_call',
		timestamp: '14:19',
		tool: 'edit',
		path: 'report/csv.go',
		args: 'quote embedded newlines',
		durationMs: 160,
		content: 'Quoted embedded newlines so Numbers keeps one row per invoice.',
		diff: '-return strings.ReplaceAll(memo, "\\n", " ")\n+return strconv.Quote(memo)'
	},
	{
		id: 'e13',
		role: 'assistant',
		kind: 'delegation',
		timestamp: '14:20',
		content: 'Flaky pipeline probe',
		child: {
			id: 'csv-flaky',
			title: 'Flaky pipeline probe',
			harness: 'pi',
			status: 'running',
			duration: '1m 40s',
			model: 'claude-haiku-4',
			preview: 'Reproducing the export job flake on empty date ranges.'
		}
	},
	{
		id: 'e14',
		role: 'assistant',
		kind: 'message',
		timestamp: '14:22',
		model: 'claude-opus-4',
		content:
			'CSV export is on the billing page. Totals come from the existing invoice renderer, memos are quoted, and the child test agent covered empty invoices. Docs and the flaky empty-range job are still open in the pane.'
	}
];

export const childTranscripts: Record<string, TraceEvent[]> = {
	'csv-tests': [
		{
			id: 'c1',
			role: 'user',
			kind: 'message',
			timestamp: '14:09',
			content: 'Write table-driven tests for CSV quoting, newlines, and empty invoices.'
		},
		{
			id: 'c2',
			role: 'assistant',
			kind: 'tool_call',
			timestamp: '14:10',
			tool: 'edit',
			path: 'report/export_test.go',
			content: 'Created export tests.',
			diff: '+func TestExportCSV(t *testing.T) {\n+    // quoting, newlines, empty set\n+}'
		},
		{
			id: 'c3',
			role: 'assistant',
			kind: 'message',
			timestamp: '14:13',
			content: 'Tests cover quoting, embedded newlines, and an empty invoice list.'
		}
	],
	'csv-docs': [
		{
			id: 'd1',
			role: 'assistant',
			kind: 'message',
			timestamp: '14:16',
			content: 'Documented GET /billing/export.csv. Quoted fields, UTF-8, and one row per invoice.'
		}
	],
	'csv-flaky': [
		{
			id: 'f1',
			role: 'assistant',
			kind: 'message',
			timestamp: '14:21',
			content: 'Empty date ranges still trip the nightly export job. Reproducing against last Tuesday’s fixture.'
		}
	]
};

export const harnesses = [
	'amp',
	'claude-code',
	'codex',
	'cursor',
	'cursor-agent',
	'grok-build',
	'opencode',
	'pi'
] as const;

export function sessionById(id: string): Session | undefined {
	return sessions.find((session) => session.id === id);
}

export function groupSessions(list: Session[]) {
	return {
		today: list.filter((session) => session.day === 'today'),
		yesterday: list.filter((session) => session.day === 'yesterday'),
		earlier: list.filter((session) => session.day === 'earlier')
	};
}
