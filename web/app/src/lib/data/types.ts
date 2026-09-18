export type Harness =
	| 'amp'
	| 'claude-code'
	| 'codex'
	| 'cursor'
	| 'cursor-agent'
	| 'grok-build'
	| 'opencode'
	| 'pi';

export type EventRole = 'user' | 'assistant' | 'tool' | 'system';
export type EventKind =
	| 'message'
	| 'reasoning'
	| 'tool_call'
	| 'tool_result'
	| 'model_change'
	| 'delegation';

export type ChildStatus = 'unread' | 'running' | 'finished';

export type ChildTrace = {
	id: string;
	title: string;
	harness: Harness;
	status: ChildStatus;
	duration: string;
	model: string;
	preview: string;
};

export type FileChange = {
	path: string;
	action: 'read' | 'edit' | 'create' | 'delete';
	additions?: number;
	deletions?: number;
};

export type TraceEvent = {
	id: string;
	role: EventRole;
	kind: EventKind;
	timestamp: string;
	content: string;
	model?: string;
	tool?: string;
	args?: string;
	output?: string;
	path?: string;
	durationMs?: number;
	child?: ChildTrace;
	diff?: string;
};

export type Session = {
	id: string;
	title: string;
	harness: Harness;
	nativeTraceId: string;
	repository: string;
	cwd: string;
	machine: string;
	model: string;
	updatedAt: string;
	day: 'today' | 'yesterday' | 'earlier';
	time: string;
	eventCount: number;
	revisionCount: number;
	preview: string;
	matchSnippet?: string;
	children: ChildTrace[];
	files: FileChange[];
	commit?: string;
	warnings: string[];
};

export type ConceptId = 'inbox' | 'studio' | 'ledger';
