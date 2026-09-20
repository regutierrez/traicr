import type { Attachment } from './types';

const PREVIEW_LIMIT = 10 * 1024 * 1024;

export type AttachmentView =
	| {
			kind: 'archived';
			name: string;
			preview?: string;
			download: string;
			oversized: boolean;
	  }
	| { kind: 'external'; name: string; href: string }
	| { kind: 'missing'; name: string };

export function attachmentName(attachment: Attachment) {
	return attachment.name || attachment.path || attachment.media_type || 'Attachment';
}

export function attachmentView(attachment: Attachment): AttachmentView {
	const name = attachmentName(attachment);
	if ((attachment.inline || attachment.archived_path) && attachment.revisionId && attachment.source_pointer) {
		const preview = `/revisions/${encodeURIComponent(String(attachment.revisionId))}/attachment?pointer=${encodeURIComponent(attachment.source_pointer)}`;
		const download = attachment.archived_path
			? `/revisions/${encodeURIComponent(String(attachment.revisionId))}/file?path=${encodeURIComponent(attachment.archived_path)}&download=1`
			: `${preview}&download=1`;
		return {
			kind: 'archived',
			name,
			preview: (attachment.size ?? 0) > PREVIEW_LIMIT ? undefined : preview,
			download,
			oversized: (attachment.size ?? 0) > PREVIEW_LIMIT
		};
	}
	try {
		const url = new URL(attachment.url ?? '');
		if (url.protocol === 'https:' || url.protocol === 'http:') {
			return { kind: 'external', name, href: url.href };
		}
	} catch {
		// Not a URL.
	}
	return { kind: 'missing', name };
}
