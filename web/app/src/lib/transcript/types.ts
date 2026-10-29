export type EventMetadata = {
	transcript_message?: string;
	transcript_block?: number;
	transcript_order?: number;
	transcript_parent?: string;
	session?: SessionNative;
	state?: { type?: string; stopReason?: string };
	usage?: {
		inputTokens?: number;
		outputTokens?: number;
		cacheReadInputTokens?: number;
		cacheCreationInputTokens?: number;
	};
	hidden?: boolean;
	run?: ToolRun;
	source_pointer?: string;
	message_meta?: {
		openAIResponsePhase?: string;
		fromExecutorThreadID?: string;
		fromAutomation?: boolean;
	};
	native_type?: string;
	complete?: boolean;
	[key: string]: unknown;
};

export type SessionNative = {
	title?: string;
	created?: string;
	agentMode?: string;
	archived?: boolean;
	activatedSkills?: unknown;
	env?: { initial?: { workingDirectory?: string } };
	meta?: { features?: unknown; executorType?: string };
	[key: string]: unknown;
};

export type ToolRun = {
	status?: string;
	result?: ToolResultPayload;
};

export type ToolResultPayload = {
	pid?: number;
	exitCode?: number;
	running?: boolean;
	output?: string;
	threadID?: string;
	files?: unknown;
	content?: unknown;
	[key: string]: unknown;
};

export type Attachment = {
	name?: string;
	media_type?: string;
	path?: string;
	url?: string;
	source_pointer?: string;
	inline?: boolean;
	archived_path?: string;
	size?: number;
	revisionId?: number;
	type?: string;
};

export type TranscriptEvent = {
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
	text?: string;
	revision_id?: number;
	aliases?: string[];
	attachments?: Attachment[];
	metadata?: EventMetadata;
};

export type SessionMetadata = {
	traceId: string;
	nativeId: string;
	harness: string;
	title?: string;
	cwd?: string;
	createdAt?: string;
	revision?: string;
};

export type Usage = {
	input?: number;
	output?: number;
	cacheRead?: number;
	cacheWrite?: number;
};

export type TextBlock = { type: 'text'; text: string };
export type HiddenBlock = { type: 'hidden'; text: string };
export type ThinkingBlock = { type: 'thinking'; thinking: string };
export type ToolCallBlock = {
	type: 'toolCall';
	id: string;
	name: string;
	arguments: Record<string, unknown>;
	details?: EventMetadata;
};
export type AttachmentBlock = Attachment & { type: 'attachment'; revisionId?: number };

export type ContentBlock = TextBlock | HiddenBlock | ThinkingBlock | ToolCallBlock | AttachmentBlock;

export type Message = {
	role: 'user' | 'assistant' | 'toolResult';
	content: ContentBlock[];
	model?: string;
	provider?: string;
	stopReason?: string;
	nativeDetails?: EventMetadata;
	usage?: Usage;
	toolCallId?: string;
	toolName?: string;
	run?: ToolRun;
	isError?: boolean;
};

export type EntrySource = {
	eventId: number;
	key: string;
	revisionId?: number;
	pointer?: string;
	details?: EventMetadata;
};

export type TranscriptEntry = {
	id: string;
	parentId: string | null;
	type: string;
	timestamp?: string;
	message?: Message;
	modelId?: string;
	provider?: string;
	summary?: string;
	tokensBefore?: number;
	thinkingLevel?: string;
	customType?: string;
	display?: boolean;
	content?: string;
	sources?: EntrySource[];
};

export type TranscriptHeader = {
	id: string;
	harness: string;
	title?: string;
	timestamp?: string;
	cwd?: string;
	native?: SessionNative;
};

// Places where the adapter could not show the records as the harness meant
// them. Each kind names what the reader sees instead.
export type AnomalyKind =
	// A tool result on the shown path whose call is not on that path. It renders as "Unpaired tool result".
	| 'unpaired_tool_result'
	// An event kind or role the adapter does not render natively. It shows as a raw note.
	| 'unrendered_event'
	// Tool arguments that are not a JSON object. They show as { raw: text }.
	| 'tool_args_not_json'
	// A message names a parent that is not loaded. It is attached to the previous message instead.
	| 'missing_parent'
	// Parent links that loop. The loop is cut at this entry.
	| 'parent_cycle'
	// Entries that are not on the path the page shows first.
	| 'off_path';

export type TranscriptAnomaly = {
	kind: AnomalyKind;
	message: string;
	// Up to ANOMALY_SAMPLE event IDs, so the records can be found on the Records page.
	eventIds: number[];
	count: number;
};

export type TranscriptSession = {
	header: TranscriptHeader;
	entries: TranscriptEntry[];
	leafId?: string;
	eventEntries: Map<string, string>;
	anomalies: TranscriptAnomaly[];
};

export type LoadTiming = {
	// Time spent waiting for and parsing transcript pages.
	fetchMs: number;
	// Time buildTranscriptSession took on the loaded events.
	adapterMs: number;
	pages: number;
	events: number;
};

export type LoadedSession = TranscriptSession & {
	events: TranscriptEvent[];
	metadata: SessionMetadata;
	timing: LoadTiming;
};

export type Turn = {
	id: string;
	role: 'user' | 'assistant' | 'system';
	entries: TranscriptEntry[];
};
