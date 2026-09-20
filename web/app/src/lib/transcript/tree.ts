import type { TranscriptEntry } from './types';

export type TreeNode = {
	entry: TranscriptEntry;
	children: TreeNode[];
};

export function graphHasFork(entries: TranscriptEntry[]) {
	const childCount = new Map<string, number>();
	let roots = 0;
	for (const entry of entries) {
		if (!entry.parentId || entry.parentId === entry.id) {
			roots += 1;
			if (roots > 1) return true;
			continue;
		}
		const count = (childCount.get(entry.parentId) ?? 0) + 1;
		childCount.set(entry.parentId, count);
		if (count > 1) return true;
	}
	return false;
}

export function buildTree(entries: TranscriptEntry[]): TreeNode[] {
	const nodeMap = new Map<string, TreeNode>();
	const roots: TreeNode[] = [];
	for (const entry of entries) {
		nodeMap.set(entry.id, { entry, children: [] });
	}
	for (const entry of entries) {
		const node = nodeMap.get(entry.id);
		if (!node) continue;
		if (entry.parentId === null || entry.parentId === undefined || entry.parentId === entry.id) {
			roots.push(node);
			continue;
		}
		const parent = nodeMap.get(entry.parentId);
		if (parent) parent.children.push(node);
		else roots.push(node);
	}
	return roots;
}

export function buildActivePathIds(entries: TranscriptEntry[], targetId: string) {
	const byId = new Map(entries.map((entry) => [entry.id, entry]));
	const ids = new Set<string>();
	let current = byId.get(targetId);
	while (current) {
		ids.add(current.id);
		if (!current.parentId || current.parentId === current.id) break;
		current = byId.get(current.parentId);
	}
	return ids;
}

export function getPath(entries: TranscriptEntry[], targetId: string) {
	const byId = new Map(entries.map((entry) => [entry.id, entry]));
	const path: TranscriptEntry[] = [];
	let current = byId.get(targetId);
	while (current) {
		path.push(current);
		if (!current.parentId || current.parentId === current.id) break;
		current = byId.get(current.parentId);
	}
	return path.reverse();
}

export function findNewestLeaf(entries: TranscriptEntry[], nodeId: string) {
	const tree = buildTree(entries);
	const nodeMap = new Map<string, TreeNode>();
	const pending = [...tree];
	while (pending.length) {
		const node = pending.pop();
		if (!node) continue;
		nodeMap.set(node.entry.id, node);
		pending.push(...node.children);
	}
	let current = nodeMap.get(nodeId);
	if (!current) return nodeId;
	while (current.children.length > 0) {
		current = current.children[current.children.length - 1];
	}
	return current.entry.id;
}

export function flattenTree(roots: TreeNode[], activePathIds: Set<string>) {
	type Flat = { node: TreeNode; indent: number };
	const result: Flat[] = [];
	const containsActive = new Map<TreeNode, boolean>();
	const traversal: TreeNode[] = [];
	const pending = [...roots];
	while (pending.length) {
		const node = pending.pop();
		if (!node) continue;
		traversal.push(node);
		pending.push(...node.children);
	}
	for (const node of traversal.reverse()) {
		containsActive.set(
			node,
			activePathIds.has(node.entry.id) || node.children.some((child) => containsActive.get(child))
		);
	}
	const stack: { node: TreeNode; indent: number }[] = [...roots]
		.sort((a, b) => Number(containsActive.get(b)) - Number(containsActive.get(a)))
		.reverse()
		.map((node) => ({ node, indent: 0 }));
	while (stack.length) {
		const item = stack.pop();
		if (!item) continue;
		result.push(item);
		const children = [...item.node.children].sort(
			(a, b) => Number(containsActive.get(b)) - Number(containsActive.get(a))
		);
		const childIndent = children.length > 1 ? item.indent + 1 : item.indent;
		for (let i = children.length - 1; i >= 0; i--) {
			stack.push({ node: children[i], indent: childIndent });
		}
	}
	return result;
}

export function entrySearchText(entry: TranscriptEntry) {
	const parts: string[] = [entry.type];
	if (entry.message?.role) parts.push(entry.message.role);
	for (const block of entry.message?.content ?? []) {
		if (block.type === 'text') parts.push(block.text);
		if (block.type === 'toolCall') parts.push(block.name, JSON.stringify(block.arguments));
		if (block.type === 'thinking') parts.push(block.thinking);
	}
	if (entry.summary) parts.push(entry.summary);
	if (entry.modelId) parts.push(entry.modelId);
	if (entry.content) parts.push(entry.content);
	return parts.join(' ').toLowerCase();
}

export function treeLabel(entry: TranscriptEntry) {
	const message = entry.message;
	if (message?.role === 'user') {
		const text = message.content
			.filter((block) => block.type === 'text')
			.map((block) => block.text)
			.join(' ')
			.replace(/\s+/g, ' ')
			.trim();
		return text ? `user · ${text.slice(0, 80)}` : 'user';
	}
	if (message?.role === 'assistant') {
		const text = message.content
			.filter((block) => block.type === 'text')
			.map((block) => block.text)
			.join(' ')
			.replace(/\s+/g, ' ')
			.trim();
		if (text) return `assistant · ${text.slice(0, 80)}`;
		const tools = message.content.filter((block) => block.type === 'toolCall').map((block) => block.name);
		if (tools.length) return `assistant · ${tools.join(', ')}`;
		return 'assistant';
	}
	if (message?.role === 'toolResult') return `tool · ${message.toolName || 'result'}`;
	if (entry.type === 'model_change') return `model · ${[entry.provider, entry.modelId].filter(Boolean).join('/')}`;
	if (entry.type === 'compaction') return 'compaction';
	if (entry.type === 'branch_summary') return `branch · ${(entry.summary || '').slice(0, 80)}`;
	return entry.customType || entry.type;
}
