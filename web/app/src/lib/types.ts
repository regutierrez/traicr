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
	/** Set when the search matched a Child Trace listed under this Trace. */
	matched_child?: { id: number; title?: string };
	/** With a query, the Event that matched, in matched_child when that is set. */
	matched_event_id?: number;
};

export type TranscriptCardPage = {
	cards: TranscriptCard[];
	next_cursor?: string;
	/** The search stopped at its time limit; next_cursor continues it. */
	partial?: boolean;
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
	/** False while the import runs or after it was interrupted. */
	complete: boolean;
	traces?: {
		trace_id?: number;
		harness: string;
		native_trace_id: string;
		title?: string;
		revision_digest: string;
		status: string;
		warnings?: Warning[];
		error?: string;
	}[];
};

export type ImportPage = {
	// The list leaves out per-Trace outcomes; /api/v1/imports/{id} has them.
	imports: Omit<ImportReport, 'traces'>[];
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

/** GET /api/v1/status: archive health. */
export type ArchiveStatus = {
	running_import_id?: number;
	machines: {
		id: string;
		hostname: string;
		last_import_at: string;
		last_import_id: number;
		last_import_complete: boolean;
		last_import_failed_traces: number;
	}[];
	imports: { total: number; with_failed_traces: number; incomplete: number };
	traces: number;
	events: number;
	titles: { enabled: boolean; pending: number };
	storage: { database_bytes: number; objects_bytes: number };
};

/** GET /api/v1/activity: distinct sessions, each a session-list card, active in a range. */
export type Activity = {
	sessions: number;
	/** One harness per session; adds up to `sessions`. */
	harnesses: { name: string; sessions: number }[];
	/** A session counts under every model it used, so these can add up to more than `sessions`. */
	models: { name: string; sessions: number }[];
	without_model: number;
	/** Days with activity in the requested time zone, oldest first. */
	days: { date: string; sessions: number }[];
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
	parent_native_trace_id?: string;
	parents?: TraceRef[];
	children?: TraceRef[];
};

/** One file edit a tool call made, from GET /api/v1/traces/{id}/patches. */
export type Patch = {
	path: string;
	tool: string;
	event_id: number;
	timestamp?: string;
	operation: 'edit' | 'write' | 'delete';
	/** Unified diff hunks without file headers. */
	diff?: string;
	additions: number;
	deletions: number;
	/** The call's own text, kept when no diff could be built. */
	arguments?: string;
	result?: string;
	failed?: boolean;
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
	attachments?: {
		name?: string;
		media_type?: string;
		path?: string;
		url?: string;
		source_pointer?: string;
		inline?: boolean;
		archived_path?: string;
		size?: number;
	}[];
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

/** GET /api/v1/wrapped: a playful recap of the Events in a range. */
export type Wrapped = {
	sessions: number;
	prompts: number;
	tool_calls: number;
	active_hours: number;
	active_days: number;
	longest_streak: number;
	/** Prompts by local hour, midnight first. */
	hours: number[];
	/** Prompts by local weekday, Sunday first. */
	weekdays: number[];
	busiest_day?: { date: string; prompts: number };
	tools: { name: string; count: number }[];
	files: { name: string; count: number }[];
	commands: { name: string; count: number }[];
	models: { name: string; sessions: number }[];
	rabbit_hole?: { trace_id: number; prompt: string; tool_calls: number; minutes: number };
	marathon?: { trace_id: number; title: string; active_hours: number };
	polite_prompts: number;
	shortest_prompt?: { trace_id: number; text: string; chars: number };
	longest_prompt?: { trace_id: number; text: string; chars: number };
	/** The prompt latest in the night, counting a day as ending at 5 AM. */
	latest_prompt?: { trace_id: number; text: string; chars: number; at: string };
};
