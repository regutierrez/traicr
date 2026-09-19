export type TranscriptCard = {
	id: number;
	title?: string;
	harness: string;
	native_trace_id: string;
	repository?: string;
	updated_at: string;
	snippet?: string;
	event_count: number;
	revision_count: number;
};

export type TranscriptCardPage = {
	cards: TranscriptCard[];
	next_cursor?: string;
};

export type Warning = { code: string; message: string };

export type ImportReport = {
	id: number;
	created_at: string;
	source_machine: { id: string; hostname: string; os: string; arch: string };
	imported: number;
	updated: number;
	unchanged: number;
	partially_parsed: number;
	unsupported: number;
	failed: number;
	traces?: {
		trace_id?: number;
		harness: string;
		native_trace_id: string;
		revision_digest: string;
		status: string;
		warnings?: Warning[];
		error?: string;
	}[];
};

export type ImportPage = {
	imports: ImportReport[];
	next_cursor?: string;
};

export type Machine = {
	id: string;
	hostname: string;
	os: string;
	arch: string;
	first_seen: string;
	last_seen: string;
};

export type TraceRef = { id: number; native_trace_id: string; title?: string };

export type Revision = {
	id: number;
	digest: string;
	native_updated_at?: string;
	collected_at: string;
	status: string;
	diagnostics?: Warning[];
};

export type Trace = {
	id: number;
	harness: string;
	native_trace_id: string;
	title?: string;
	working_directory?: string;
	repository?: string;
	created_at: string;
	updated_at: string;
	revisions: Revision[];
	parents?: TraceRef[];
	children?: TraceRef[];
};

export type TraceEvent = {
	id: number;
	key: string;
	parent_key?: string;
	branch?: string;
	kind: string;
	role?: string;
	model?: string;
	provider?: string;
	tool?: string;
	call_id?: string;
	timestamp?: string;
	text: string;
	revision_id?: number;
	aliases?: string[];
	attachments?: Record<string, unknown>[];
	metadata?: Record<string, unknown>;
};

export type EventPage = {
	events: TraceEvent[];
	next_cursor?: string;
};

export type SourceFile = {
	revision_id: number;
	path: string;
	digest: string;
	size: number;
};

export type FilePreview = {
	path: string;
	content: string;
	truncated: boolean;
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
];
