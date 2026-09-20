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

export type TranscriptSession = {
	header: TranscriptHeader;
	entries: TranscriptEntry[];
	leafId?: string;
	eventEntries: Map<string, string>;
};

export type LoadedSession = TranscriptSession & {
	hasMore: boolean;
	status: string;
	cursor: string;
	events: TranscriptEvent[];
	seenCursors: string[];
	metadata: SessionMetadata;
};

export type Turn = {
	id: string;
	role: 'user' | 'assistant' | 'system';
	entries: TranscriptEntry[];
};
