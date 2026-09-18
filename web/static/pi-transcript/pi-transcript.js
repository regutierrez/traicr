    // Adapted from Pi 0.85.1's MIT-licensed HTML export. See pi-LICENSE.txt.
    import { loadTranscriptSession } from './transcript-data.js';
    import { renderTranscriptEntry, renderTranscriptHeaderDetails, parseSkillBlock } from './transcript-render.js';
    import { registerShortcuts, toast } from '../shortcuts.js';
    (async function() {
      'use strict';

      // ============================================================
      // DATA LOADING
      // ============================================================

      const metadata = {...document.getElementById('transcript-meta').dataset};
      metadata.revision = new URLSearchParams(window.location.search).get('revision') || '';
      const data = await loadTranscriptSession(metadata);
      const requested = new URLSearchParams(window.location.search);
      const requestedIDs = ['leafId','targetId','event','key'].map(key => requested.get(key)).filter(Boolean);
      while (data.hasMore && requestedIDs.some(id => !data.eventEntries.has(id))) await data.loadMore();
      let { header, entries } = data;
      const { leafId: defaultLeafId, systemPrompt, tools } = data;

      // ============================================================
      // URL PARAMETER HANDLING
      // ============================================================

      // Parse URL parameters for deep linking: leafId and targetId
      // Check for injected params (when loaded in iframe via srcdoc) or use window.location
      const injectedParams = document.querySelector('meta[name="pi-url-params"]');
      const searchString = injectedParams ? injectedParams.content : window.location.search.substring(1);
      const urlParams = new URLSearchParams(searchString);
      const urlLeafId = data.eventEntries.get(urlParams.get('leafId')) || urlParams.get('leafId');
      const urlTargetId = data.eventEntries.get(urlParams.get('targetId')) || urlParams.get('targetId') || data.eventEntries.get(urlParams.get('event') || urlParams.get('key'));
      // Use URL leafId if provided, otherwise fall back to session default
      const leafId = urlLeafId || defaultLeafId;

      // ============================================================
      // DATA STRUCTURES
      // ============================================================

      // Entry lookup by ID
      const byId = new Map();
      for (const entry of entries) {
        byId.set(entry.id, entry);
      }

      // Tool call lookup (toolCallId -> {name, arguments})
      const toolCallMap = new Map();
      const toolResultEntries = new Map();
      const visibleToolCalls = new Set();
      for (const entry of entries) {
        if (entry.type === 'message' && entry.message.role === 'assistant') {
          const content = entry.message.content;
          if (Array.isArray(content)) {
            for (const block of content) {
              if (block.type === 'toolCall') {
                toolCallMap.set(block.id, { name: block.name, arguments: block.arguments });
              }
            }
          }
        }
      }

      // Label lookup (entryId -> label string)
      // Labels are stored in 'label' entries that reference their target via targetId
      const labelMap = new Map();
      for (const entry of entries) {
        if (entry.type === 'label' && entry.targetId && entry.label) {
          labelMap.set(entry.targetId, entry.label);
        }
      }

      // ============================================================
      // TREE DATA PREPARATION (no DOM, pure data)
      // ============================================================

      /**
       * Build tree structure from flat entries.
       * Returns array of root nodes, each with { entry, children, label }.
       */
      function buildTree() {
        const nodeMap = new Map();
        const roots = [];

        // Create nodes
        for (const entry of entries) {
          nodeMap.set(entry.id, {
            entry,
            children: [],
            label: labelMap.get(entry.id)
          });
        }

        // Build parent-child relationships
        for (const entry of entries) {
          const node = nodeMap.get(entry.id);
          if (entry.parentId === null || entry.parentId === undefined || entry.parentId === entry.id) {
            roots.push(node);
          } else {
            const parent = nodeMap.get(entry.parentId);
            if (parent) {
              parent.children.push(node);
            } else {
              roots.push(node);
            }
          }
        }

        return roots;
      }

      /**
       * Build set of entry IDs on path from root to target.
       */
      function buildActivePathIds(targetId) {
        const ids = new Set();
        let current = byId.get(targetId);
        while (current) {
          ids.add(current.id);
          // Stop if no parent or self-referencing (root)
          if (!current.parentId || current.parentId === current.id) {
            break;
          }
          current = byId.get(current.parentId);
        }
        return ids;
      }

      /**
       * Get array of entries from root to target (the conversation path).
       */
      function getPath(targetId) {
        const path = [];
        let current = byId.get(targetId);
        while (current) {
          path.push(current);
          // Stop if no parent or self-referencing (root)
          if (!current.parentId || current.parentId === current.id) {
            break;
          }
          current = byId.get(current.parentId);
        }
        return path.reverse();
      }

      // Tree node lookup for finding leaves
      let treeNodeMap = null;

      /**
       * Find the newest leaf node reachable from a given node.
       * This allows clicking any node in a branch to show the full branch.
       * Children are sorted by timestamp, so the newest is always last.
       */
      function findNewestLeaf(nodeId) {
        // Build tree node map lazily
        if (!treeNodeMap) {
          treeNodeMap = new Map();
          const tree = buildTree();
          const pending = [...tree];
          while (pending.length) {
            const node = pending.pop();
            treeNodeMap.set(node.entry.id, node);
            for (const child of node.children) pending.push(child);
          }
        }

        const node = treeNodeMap.get(nodeId);
        if (!node) return nodeId;

        // Follow the newest (last) child at each level
        let current = node;
        while (current.children.length > 0) {
          current = current.children[current.children.length - 1];
        }
        return current.entry.id;
      }

      /**
       * Flatten tree into list with indentation and connector info.
       * Returns array of { node, indent, showConnector, isLast, gutters, isVirtualRootChild, multipleRoots }.
       * Matches tree-selector.ts logic exactly.
       */
      function flattenTree(roots, activePathIds) {
        const result = [];
        const multipleRoots = roots.length > 1;

        // Mark which subtrees contain the active leaf
        const containsActive = new Map();
        const pending = [...roots];
        const traversal = [];
        while (pending.length) {
          const node = pending.pop();
          traversal.push(node);
          for (const child of node.children) pending.push(child);
        }
        for (const node of traversal.reverse()) containsActive.set(node, activePathIds.has(node.entry.id) || node.children.some(child => containsActive.get(child)));

        // Stack: [node, indent, justBranched, showConnector, isLast, gutters, isVirtualRootChild]
        const stack = [];

        // Add roots (prioritize branch containing active leaf)
        const orderedRoots = [...roots].sort((a, b) =>
          Number(containsActive.get(b)) - Number(containsActive.get(a))
        );
        for (let i = orderedRoots.length - 1; i >= 0; i--) {
          const isLast = i === orderedRoots.length - 1;
          stack.push([orderedRoots[i], multipleRoots ? 1 : 0, multipleRoots, multipleRoots, isLast, [], multipleRoots]);
        }

        while (stack.length > 0) {
          const [node, indent, justBranched, showConnector, isLast, gutters, isVirtualRootChild] = stack.pop();

          result.push({ node, indent, showConnector, isLast, gutters, isVirtualRootChild, multipleRoots });

          const children = node.children;
          const multipleChildren = children.length > 1;

          // Order children (active branch first)
          const orderedChildren = [...children].sort((a, b) =>
            Number(containsActive.get(b)) - Number(containsActive.get(a))
          );

          // Calculate child indent (matches tree-selector.ts)
          let childIndent;
          if (multipleChildren) {
            // Parent branches: children get +1
            childIndent = indent + 1;
          } else if (justBranched && indent > 0) {
            // First generation after a branch: +1 for visual grouping
            childIndent = indent + 1;
          } else {
            // Single-child chain: stay flat
            childIndent = indent;
          }

          // Build gutters for children
          const connectorDisplayed = showConnector && !isVirtualRootChild;
          const currentDisplayIndent = multipleRoots ? Math.max(0, indent - 1) : indent;
          const connectorPosition = Math.max(0, currentDisplayIndent - 1);
          const childGutters = connectorDisplayed
            ? [...gutters, { position: connectorPosition, show: !isLast }]
            : gutters;

          // Add children in reverse order for stack
          for (let i = orderedChildren.length - 1; i >= 0; i--) {
            const childIsLast = i === orderedChildren.length - 1;
            stack.push([orderedChildren[i], childIndent, multipleChildren, multipleChildren, childIsLast, childGutters, false]);
          }
        }

        return result;
      }

      /**
       * Build ASCII prefix string for tree node.
       */
      function buildTreePrefix(flatNode) {
        const { indent, showConnector, isLast, gutters, isVirtualRootChild, multipleRoots } = flatNode;
        const displayIndent = multipleRoots ? Math.max(0, indent - 1) : indent;
        const connector = showConnector && !isVirtualRootChild ? (isLast ? '└─ ' : '├─ ') : '';
        const connectorPosition = connector ? displayIndent - 1 : -1;

        const totalChars = displayIndent * 3;
        const prefixChars = [];
        for (let i = 0; i < totalChars; i++) {
          const level = Math.floor(i / 3);
          const posInLevel = i % 3;

          const gutter = gutters.find(g => g.position === level);
          if (gutter) {
            prefixChars.push(posInLevel === 0 ? (gutter.show ? '│' : ' ') : ' ');
          } else if (connector && level === connectorPosition) {
            if (posInLevel === 0) {
              prefixChars.push(isLast ? '└' : '├');
            } else if (posInLevel === 1) {
              prefixChars.push('─');
            } else {
              prefixChars.push(' ');
            }
          } else {
            prefixChars.push(' ');
          }
        }
        return prefixChars.join('');
      }

      // ============================================================
      // FILTERING (pure data)
      // ============================================================

      let filterMode = 'default';
      let searchQuery = '';

      function hasTextContent(content) {
        if (typeof content === 'string') return content.trim().length > 0;
        if (Array.isArray(content)) {
          for (const c of content) {
            if (c.type === 'text' && c.text && c.text.trim().length > 0) return true;
          }
        }
        return false;
      }

      function extractContent(content) {
        if (typeof content === 'string') return content;
        if (Array.isArray(content)) {
          return content
            .filter(c => c.type === 'text' && c.text)
            .map(c => c.text)
            .join('');
        }
        return '';
      }

      function getSearchableText(entry, label) {
        const parts = [];
        if (label) parts.push(label);

        switch (entry.type) {
          case 'message': {
            const msg = entry.message;
            parts.push(msg.role);
            if (msg.content) parts.push(extractContent(msg.content));
            if (msg.role === 'bashExecution' && msg.command) parts.push(msg.command);
            break;
          }
          case 'custom_message':
            parts.push(entry.customType);
            parts.push(typeof entry.content === 'string' ? entry.content : extractContent(entry.content));
            break;
          case 'compaction':
            parts.push('compaction');
            break;
          case 'branch_summary':
            parts.push('branch summary', entry.summary);
            break;
          case 'model_change':
            parts.push('model', entry.modelId);
            break;
          case 'thinking_level_change':
            parts.push('thinking', entry.thinkingLevel);
            break;
        }

        return parts.join(' ').toLowerCase();
      }

      /**
       * Filter flat nodes based on current filterMode and searchQuery.
       */
      function filterNodes(flatNodes, currentLeafId) {
        const searchTokens = searchQuery.toLowerCase().split(/\s+/).filter(Boolean);

        const filtered = flatNodes.filter(flatNode => {
          const entry = flatNode.node.entry;
          const label = flatNode.node.label;
          const isCurrentLeaf = entry.id === currentLeafId;

          // Always show current leaf
          if (isCurrentLeaf) return true;

          if (entry.type === 'message' && entry.message.role === 'assistant') {
            const msg = entry.message;
            const hasText = hasTextContent(msg.content);
            const hasToolCalls = msg.content.some(block => block.type === 'toolCall');
            const isErrorOrAborted = msg.stopReason && msg.stopReason !== 'stop' && msg.stopReason !== 'toolUse';
            if (!hasText && !hasToolCalls && !isErrorOrAborted && filterMode !== 'all') return false;
          }

          // Apply filter mode
          const isSettingsEntry = ['label', 'custom', 'model_change', 'thinking_level_change'].includes(entry.type);
          let passesFilter = true;

          switch (filterMode) {
            case 'user-only':
              passesFilter = entry.type === 'message' && entry.message.role === 'user';
              break;
            case 'no-tools':
              passesFilter = !isSettingsEntry && !(entry.type === 'message' && (entry.message.role === 'toolResult' || (!hasTextContent(entry.message.content) && entry.message.content.some(block => block.type === 'toolCall'))));
              break;
            case 'labeled-only':
              passesFilter = label !== undefined;
              break;
            case 'all':
              passesFilter = true;
              break;
            default: // 'default'
              passesFilter = !isSettingsEntry;
              break;
          }

          if (!passesFilter) return false;

          // Apply search filter
          if (searchTokens.length > 0) {
            const nodeText = getSearchableText(entry, label);
            if (!searchTokens.every(t => nodeText.includes(t))) return false;
          }

          return true;
        });

        // Recalculate visual structure based on visible tree
        recalculateVisualStructure(filtered, flatNodes);

        return filtered;
      }

      /**
       * Recompute indentation/connectors for the filtered view
       *
       * Filtering can hide intermediate entries; descendants attach to the nearest visible ancestor.
       * Keep indentation semantics aligned with flattenTree() so single-child chains don't drift right.
       */
      function recalculateVisualStructure(filteredNodes, allFlatNodes) {
        if (filteredNodes.length === 0) return;

        const visibleIds = new Set(filteredNodes.map(n => n.node.entry.id));

        // Build entry map for parent lookup (using full tree)
        const entryMap = new Map();
        for (const flatNode of allFlatNodes) {
          entryMap.set(flatNode.node.entry.id, flatNode);
        }

        // Find nearest visible ancestor for a node
        function findVisibleAncestor(nodeId) {
          let currentId = entryMap.get(nodeId)?.node.entry.parentId;
          while (currentId != null) {
            if (visibleIds.has(currentId)) {
              return currentId;
            }
            currentId = entryMap.get(currentId)?.node.entry.parentId;
          }
          return null;
        }

        // Build visible tree structure
        const visibleParent = new Map();
        const visibleChildren = new Map();
        visibleChildren.set(null, []); // root-level nodes

        for (const flatNode of filteredNodes) {
          const nodeId = flatNode.node.entry.id;
          const ancestorId = findVisibleAncestor(nodeId);
          visibleParent.set(nodeId, ancestorId);

          if (!visibleChildren.has(ancestorId)) {
            visibleChildren.set(ancestorId, []);
          }
          visibleChildren.get(ancestorId).push(nodeId);
        }

        // Update multipleRoots based on visible roots
        const visibleRootIds = visibleChildren.get(null);
        const multipleRoots = visibleRootIds.length > 1;

        // Build a map for quick lookup: nodeId → FlatNode
        const filteredNodeMap = new Map();
        for (const flatNode of filteredNodes) {
          filteredNodeMap.set(flatNode.node.entry.id, flatNode);
        }

        // DFS traversal of visible tree, applying same indentation rules as flattenTree()
        // Stack items: [nodeId, indent, justBranched, showConnector, isLast, gutters, isVirtualRootChild]
        const stack = [];

        // Add visible roots in reverse order (to process in forward order via stack)
        for (let i = visibleRootIds.length - 1; i >= 0; i--) {
          const isLast = i === visibleRootIds.length - 1;
          stack.push([
            visibleRootIds[i],
            multipleRoots ? 1 : 0,
            multipleRoots,
            multipleRoots,
            isLast,
            [],
            multipleRoots
          ]);
        }

        while (stack.length > 0) {
          const [nodeId, indent, justBranched, showConnector, isLast, gutters, isVirtualRootChild] = stack.pop();

          const flatNode = filteredNodeMap.get(nodeId);
          if (!flatNode) continue;

          // Update this node's visual properties
          flatNode.indent = indent;
          flatNode.showConnector = showConnector;
          flatNode.isLast = isLast;
          flatNode.gutters = gutters;
          flatNode.isVirtualRootChild = isVirtualRootChild;
          flatNode.multipleRoots = multipleRoots;

          // Get visible children of this node
          const children = visibleChildren.get(nodeId) || [];
          const multipleChildren = children.length > 1;

          // Calculate child indent using same rules as flattenTree():
          // - Parent branches (multiple children): children get +1
          // - Just branched and indent > 0: children get +1 for visual grouping
          // - Single-child chain: stay flat
          let childIndent;
          if (multipleChildren) {
            childIndent = indent + 1;
          } else if (justBranched && indent > 0) {
            childIndent = indent + 1;
          } else {
            childIndent = indent;
          }

          // Build gutters for children (same logic as flattenTree)
          const connectorDisplayed = showConnector && !isVirtualRootChild;
          const currentDisplayIndent = multipleRoots ? Math.max(0, indent - 1) : indent;
          const connectorPosition = Math.max(0, currentDisplayIndent - 1);
          const childGutters = connectorDisplayed
            ? [...gutters, { position: connectorPosition, show: !isLast }]
            : gutters;

          // Add children in reverse order (to process in forward order via stack)
          for (let i = children.length - 1; i >= 0; i--) {
            const childIsLast = i === children.length - 1;
            stack.push([
              children[i],
              childIndent,
              multipleChildren,
              multipleChildren,
              childIsLast,
              childGutters,
              false
            ]);
          }
        }
      }

      // ============================================================
      // TREE DISPLAY TEXT (pure data -> string)
      // ============================================================

      function shortenPath(p) {
        if (typeof p !== 'string') return '';
        if (p.startsWith('/Users/')) {
          const parts = p.split('/');
          if (parts.length > 2) return '~' + p.slice(('/Users/' + parts[2]).length);
        }
        if (p.startsWith('/home/')) {
          const parts = p.split('/');
          if (parts.length > 2) return '~' + p.slice(('/home/' + parts[2]).length);
        }
        return p;
      }

      function formatToolCall(name, args) {
        switch (name) {
          case 'read': {
            const path = shortenPath(String(args.path || args.file_path || ''));
            const offset = args.offset;
            const limit = args.limit;
            let display = path;
            if (offset !== undefined || limit !== undefined) {
              const start = offset ?? 1;
              const end = limit !== undefined ? start + limit - 1 : '';
              display += `:${start}${end ? `-${end}` : ''}`;
            }
            return `[read: ${display}]`;
          }
          case 'write':
            return `[write: ${shortenPath(String(args.path || args.file_path || ''))}]`;
          case 'edit':
            return `[edit: ${shortenPath(String(args.path || args.file_path || ''))}]`;
          case 'bash': {
            const rawCmd = String(args.command || '');
            const cmd = rawCmd.replace(/[\n\t]/g, ' ').trim().slice(0, 50);
            return `[bash: ${cmd}${rawCmd.length > 50 ? '...' : ''}]`;
          }
          case 'grep':
            return `[grep: /${args.pattern || ''}/ in ${shortenPath(String(args.path || '.'))}]`;
          case 'find':
            return `[find: ${args.pattern || ''} in ${shortenPath(String(args.path || '.'))}]`;
          case 'ls':
            return `[ls: ${shortenPath(String(args.path || '.'))}]`;
          default: {
            const argsStr = JSON.stringify(args).slice(0, 40);
            return `[${name}: ${argsStr}${JSON.stringify(args).length > 40 ? '...' : ''}]`;
          }
        }
      }

      function escapeHtml(text) {
        return String(text)
          .replace(/&/g, '&amp;')
          .replace(/</g, '&lt;')
          .replace(/>/g, '&gt;')
          .replace(/"/g, '&quot;')
          .replace(/'/g, '&#39;');
      }

      function sanitizeMarkdownUrl(value) {
        const href = String(value || '').trim().replace(/[\x00-\x1f\x7f]/g, '');
        if (!href) return href;

        const scheme = href.match(/^([A-Za-z][A-Za-z0-9+.-]*):/);
        if (scheme && !/^(https?|mailto|tel|ftp)$/i.test(scheme[1])) {
          return null;
        }

        return href;
      }

      /**
       * Truncate string to maxLen chars, append "..." if truncated.
       */
      function truncate(s, maxLen = 100) {
        if (s.length <= maxLen) return s;
        return s.slice(0, maxLen) + '...';
      }

      /**
       * Get display text for tree node (returns HTML string).
       */
      function getTreeNodeDisplayHtml(entry, label) {
        const normalize = s => s.replace(/[\n\t]/g, ' ').trim();
        const labelHtml = label ? `<span class="tree-label">[${escapeHtml(label)}]</span> ` : '';

        switch (entry.type) {
          case 'message': {
            const msg = entry.message;
            if (msg.role === 'user') {
              const rawContent = extractContent(msg.content);
              const skillBlock = parseSkillBlock(rawContent);
              if (skillBlock) {
                let treeHtml = labelHtml + `<span class="tree-role-skill">skill:</span> ${escapeHtml(skillBlock.name)}`;
                if (skillBlock.userMessage) {
                  treeHtml += ` · <span class="tree-role-user">user:</span> ${escapeHtml(truncate(normalize(skillBlock.userMessage)))}`;
                }
                return treeHtml;
              }
              const content = truncate(normalize(rawContent));
              return labelHtml + `<span class="tree-role-user">user:</span> ${escapeHtml(content)}`;
            }
            if (msg.role === 'assistant') {
              const textContent = truncate(normalize(extractContent(msg.content)));
              if (textContent) {
                return labelHtml + `<span class="tree-role-assistant">assistant:</span> ${escapeHtml(textContent)}`;
              }
              const calls = msg.content.filter(block => block.type === 'toolCall');
              if (calls.length) return labelHtml + `<span class="tree-role-tool">${escapeHtml(calls.map(call => formatToolCall(call.name,call.arguments)).join(' · '))}</span>`;
              if (msg.stopReason === 'aborted') {
                return labelHtml + `<span class="tree-role-assistant">assistant:</span> <span class="tree-muted">(aborted)</span>`;
              }
              if (msg.errorMessage) {
                return labelHtml + `<span class="tree-role-assistant">assistant:</span> <span class="tree-error">${escapeHtml(truncate(msg.errorMessage))}</span>`;
              }
              return labelHtml + `<span class="tree-role-assistant">assistant:</span> <span class="tree-muted">(no text)</span>`;
            }
            if (msg.role === 'toolResult') {
              const toolCall = msg.toolCallId ? toolCallMap.get(msg.toolCallId) : null;
              if (toolCall) {
                return labelHtml + `<span class="tree-role-tool">${escapeHtml(formatToolCall(toolCall.name, toolCall.arguments))}</span>`;
              }
              return labelHtml + `<span class="tree-role-tool">[${escapeHtml(msg.toolName || 'tool')}]</span>`;
            }
            if (msg.role === 'bashExecution') {
              const cmd = truncate(normalize(msg.command || ''));
              return labelHtml + `<span class="tree-role-tool">[bash]:</span> ${escapeHtml(cmd)}`;
            }
            return labelHtml + `<span class="tree-muted">[${escapeHtml(msg.role)}]</span>`;
          }
          case 'compaction':
            return labelHtml + `<span class="tree-compaction">[compaction]</span>`;
          case 'branch_summary': {
            const summary = truncate(normalize(entry.summary || ''));
            return labelHtml + `<span class="tree-branch-summary">[branch summary]:</span> ${escapeHtml(summary)}`;
          }
          case 'custom_message': {
            const content = typeof entry.content === 'string' ? entry.content : extractContent(entry.content);
            return labelHtml + `<span class="tree-custom">[${escapeHtml(entry.customType)}]:</span> ${escapeHtml(truncate(normalize(content)))}`;
          }
          case 'model_change':
            return labelHtml + `<span class="tree-muted">[model: ${escapeHtml(entry.modelId)}]</span>`;
          case 'thinking_level_change':
            return labelHtml + `<span class="tree-muted">[thinking: ${escapeHtml(entry.thinkingLevel)}]</span>`;
          default:
            return labelHtml + `<span class="tree-muted">[${escapeHtml(entry.type)}]</span>`;
        }
      }

      // ============================================================
      // TREE RENDERING (DOM manipulation)
      // ============================================================

      let currentLeafId = leafId;
      let currentTargetId = urlTargetId || leafId;
      let treeRendered = false;

      function renderTree() {
        const tree = buildTree();
        const activePathIds = buildActivePathIds(currentLeafId);
        const flatNodes = flattenTree(tree, activePathIds);
        const filtered = filterNodes(flatNodes, currentLeafId);
        const container = document.getElementById('tree-container');

        // Full render only on first call or when filter/search changes
        if (!treeRendered) {
          container.innerHTML = '';

          for (const flatNode of filtered) {
            const entry = flatNode.node.entry;
            const isOnPath = activePathIds.has(entry.id);
            const isTarget = entry.id === currentTargetId;

            const div = document.createElement('div');
            div.className = 'tree-node';
            if (isOnPath) div.classList.add('in-path');
            if (isTarget) div.classList.add('active');
            div.dataset.id = entry.id;

            const prefix = buildTreePrefix(flatNode);
            const prefixSpan = document.createElement('span');
            prefixSpan.className = 'tree-prefix';
            prefixSpan.textContent = prefix;

            const marker = document.createElement('span');
            marker.className = 'tree-marker';
            marker.textContent = isOnPath ? '•' : ' ';

            const content = document.createElement('span');
            content.className = 'tree-content';
            content.innerHTML = getTreeNodeDisplayHtml(entry, flatNode.node.label);

            div.appendChild(prefixSpan);
            div.appendChild(marker);
            div.appendChild(content);
            // Navigate to the newest leaf through this node, but scroll to the clicked node
            div.addEventListener('click', () => {
              if (window.getSelection().toString()) return;
              const leafId = findNewestLeaf(entry.id);
              navigateTo(leafId, 'target', entry.id);
            });

            container.appendChild(div);
          }

          treeRendered = true;
        } else {
          // Just update markers and classes
          const nodes = container.querySelectorAll('.tree-node');
          for (const node of nodes) {
            const id = node.dataset.id;
            const isOnPath = activePathIds.has(id);
            const isTarget = id === currentTargetId;

            node.classList.toggle('in-path', isOnPath);
            node.classList.toggle('active', isTarget);

            const marker = node.querySelector('.tree-marker');
            if (marker) {
              marker.textContent = isOnPath ? '•' : ' ';
            }
          }
        }

        document.getElementById('tree-status').textContent = `${filtered.length} / ${flatNodes.length} entries`;

        // Scroll active node into view after layout
        setTimeout(() => {
          const activeNode = container.querySelector('.tree-node.active');
          if (activeNode) {
            activeNode.scrollIntoView({ block: 'nearest' });
          }
        }, 0);
      }

      function forceTreeRerender() {
        treeRendered = false;
        renderTree();
      }

      /**
       * Download the session data as a JSONL file.
       * Reconstructs the original format: header line + entry lines.
       */
      window.downloadSessionJson = function() {
        // Build JSONL content: header first, then all entries
        const lines = [];
        if (header) {
          lines.push(JSON.stringify({ type: 'header', ...header }));
        }
        for (const entry of entries) {
          lines.push(JSON.stringify(entry));
        }
        const jsonlContent = lines.join('\n');

        // Create download
        const blob = new Blob([jsonlContent], { type: 'application/x-ndjson' });
        const url = URL.createObjectURL(blob);
        const a = document.createElement('a');
        a.href = url;
        a.download = `${header?.id || 'session'}.jsonl`;
        document.body.appendChild(a);
        a.click();
        document.body.removeChild(a);
        URL.revokeObjectURL(url);
      }

      /**
       * Build a shareable URL for a specific message.
       * URL format: base?gistId&leafId=<leafId>&targetId=<entryId>
       */
      function buildShareUrl(entryId) {
        // Check for injected base URL (used when loaded in iframe via srcdoc)
        const baseUrlMeta = document.querySelector('meta[name="pi-share-base-url"]');
        const baseUrl = baseUrlMeta ? baseUrlMeta.content : window.location.href.split('?')[0];

        const url = new URL(window.location.href);
        // Find the gist ID (first query param without value, e.g., ?abc123)
        const gistId = Array.from(url.searchParams.keys()).find(k => !url.searchParams.get(k));

        // Build the share URL
        const params = new URLSearchParams();
        params.set('leafId', currentLeafId);
        params.set('targetId', entryId);
        if (metadata.revision) params.set('revision', metadata.revision);

        // If we have an injected base URL (iframe context), use it directly
        if (baseUrlMeta) {
          return `${baseUrl}&${params.toString()}`;
        }

        // Otherwise build from current location (direct file access)
        url.search = gistId ? `?${gistId}&${params.toString()}` : `?${params.toString()}`;
        return url.toString();
      }

      /**
       * Copy text to clipboard with visual feedback.
       * Uses navigator.clipboard with fallback to execCommand for HTTP contexts.
       */
      async function copyToClipboard(text, button) {
        let success = false;
        try {
          if (navigator.clipboard && navigator.clipboard.writeText) {
            await navigator.clipboard.writeText(text);
            success = true;
          }
        } catch (err) {
          // Clipboard API failed, try fallback
        }

        // Fallback for HTTP or when Clipboard API is unavailable
        if (!success) {
          try {
            const textarea = document.createElement('textarea');
            textarea.value = text;
            textarea.style.position = 'fixed';
            textarea.style.opacity = '0';
            document.body.appendChild(textarea);
            textarea.select();
            success = document.execCommand('copy');
            document.body.removeChild(textarea);
          } catch (err) {
            console.error('Failed to copy:', err);
          }
        }

        if (success) toast('Link copied');
        else toast('Copy failed. Your browser blocked clipboard access.', 'alert-error');
        if (success && button) {
          const originalHtml = button.innerHTML;
          button.innerHTML = '✓';
          button.classList.add('copied');
          setTimeout(() => {
            button.innerHTML = originalHtml;
            button.classList.remove('copied');
          }, 1500);
        }
      }

      /**
       * Render the copy-link button HTML for a message.
       */
      function renderCopyLinkButton(entryId) {
        return `<button class="copy-link-btn" data-entry-id="${escapeHtml(entryId)}" title="Copy link to this message">
          <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <path d="M10 13a5 5 0 0 0 7.54.54l3-3a5 5 0 0 0-7.07-7.07l-1.72 1.71"/>
            <path d="M14 11a5 5 0 0 0-7.54-.54l-3 3a5 5 0 0 0 7.07 7.07l1.71-1.71"/>
          </svg>
        </button>`;
      }

      // ============================================================
      // HEADER / STATS
      // ============================================================

      function computeStats(entryList) {
        let userMessages = 0, assistantMessages = 0, toolResults = 0;
        let customMessages = 0, compactions = 0, branchSummaries = 0, toolCalls = 0;
        const tokens = { input: 0, output: 0, cacheRead: 0, cacheWrite: 0 };
        const cost = { input: 0, output: 0, cacheRead: 0, cacheWrite: 0 };
        const models = new Set();

        for (const entry of entryList) {
          if (entry.type === 'message') {
            const msg = entry.message;
            if (msg.role === 'user') userMessages++;
            if (msg.role === 'assistant') {
              assistantMessages++;
              if (msg.model) models.add(msg.provider ? `${msg.provider}/${msg.model}` : msg.model);
              if (msg.usage) {
                tokens.input += msg.usage.input || 0;
                tokens.output += msg.usage.output || 0;
                tokens.cacheRead += msg.usage.cacheRead || 0;
                tokens.cacheWrite += msg.usage.cacheWrite || 0;
                if (msg.usage.cost) {
                  cost.input += msg.usage.cost.input || 0;
                  cost.output += msg.usage.cost.output || 0;
                  cost.cacheRead += msg.usage.cost.cacheRead || 0;
                  cost.cacheWrite += msg.usage.cost.cacheWrite || 0;
                }
              }
              toolCalls += msg.content.filter(c => c.type === 'toolCall').length;
            }
            if (msg.role === 'toolResult') toolResults++;
          } else if (entry.type === 'compaction') {
            compactions++;
          } else if (entry.type === 'branch_summary') {
            branchSummaries++;
          } else if (entry.type === 'custom_message') {
            customMessages++;
          }
        }

        return { userMessages, assistantMessages, toolResults, customMessages, compactions, branchSummaries, toolCalls, tokens, cost, models: Array.from(models) };
      }

      let globalStats = computeStats(entries);

      function renderHeader() {
        const msgParts = [];
        if (globalStats.userMessages) msgParts.push(`${globalStats.userMessages} user`);
        if (globalStats.assistantMessages) msgParts.push(`${globalStats.assistantMessages} assistant`);
        if (globalStats.toolResults) msgParts.push(`${globalStats.toolResults} tool results`);
        if (globalStats.customMessages) msgParts.push(`${globalStats.customMessages} custom`);
        if (globalStats.compactions) msgParts.push(`${globalStats.compactions} compactions`);
        if (globalStats.branchSummaries) msgParts.push(`${globalStats.branchSummaries} branch summaries`);

        const kbd = (key) => `<kbd class="kbd kbd-xs">${escapeHtml(key)}</kbd>`;
        let html = `
          <div class="header">
            <div class="session-eyebrow"><span class="harness-badge" aria-label="Harness">${escapeHtml(header?.harness || 'unknown')}</span><span>${header?.timestamp ? escapeHtml(new Date(header.timestamp).toLocaleString()) : 'Date unknown'}</span></div>
            <h1>${escapeHtml(header?.title || header?.id || 'Untitled session')}</h1>
            <div class="help-bar">
              <span class="help-hint">${kbd('J')}${kbd('K')} move between messages, ${kbd('T')} reasoning, ${kbd('O')} tool output, ${kbd('B')} branch tree, ${kbd('?')} all shortcuts</span>
              <div class="help-actions">
                <button type="button" class="header-toggle-btn" data-action="toggle-thinking" aria-pressed="${thinkingExpanded}" title="Show or hide reasoning (T)">Reasoning</button>
                <button type="button" class="header-toggle-btn" data-action="toggle-tools" aria-pressed="${toolOutputsExpanded}" title="Expand or collapse tool output (O)">Tool output</button>
                <button type="button" class="download-json-btn" data-action="download-session" title="Download the loaded normalized viewer entries, not native source records">Download viewer JSONL${data.hasMore ? ' (loaded only)' : ''}</button>
              </div>
            </div>
            <div class="header-info">
              <div class="info-item"><span class="info-label">Models</span><span class="info-value">${escapeHtml(globalStats.models.join(', ') || 'unknown')}</span></div>
              <div class="info-item"><span class="info-label">Messages</span><span class="info-value">${msgParts.join(', ') || '0'}</span></div>
              <div class="info-item"><span class="info-label">Tool calls</span><span class="info-value">${globalStats.toolCalls}</span></div>
              <div class="info-item"><span class="info-label">Directory</span><span class="info-value">${escapeHtml(header?.cwd || 'unknown')}</span></div>
            </div>
          </div>`;

        html += renderTranscriptHeaderDetails(header, entries, {escape:escapeHtml});

        // Render system prompt (user's base prompt, applies to all providers)
        if (systemPrompt) {
          const lines = systemPrompt.split('\n');
          const previewLines = 10;
          if (lines.length > previewLines) {
            const preview = lines.slice(0, previewLines).join('\n');
            const remaining = lines.length - previewLines;
            html += `<div class="system-prompt expandable" data-expand="expanded">
              <div class="system-prompt-header">System Prompt</div>
              <div class="system-prompt-preview">${escapeHtml(preview)}</div>
              <div class="system-prompt-expand-hint">... (${remaining} more lines, click to expand)</div>
              <div class="system-prompt-full">${escapeHtml(systemPrompt)}</div>
            </div>`;
          } else {
            html += `<div class="system-prompt">
              <div class="system-prompt-header">System Prompt</div>
              <div class="system-prompt-full" style="display: block">${escapeHtml(systemPrompt)}</div>
            </div>`;
          }
        }

        if (tools && tools.length > 0) {
          html += `<div class="tools-list">
            <div class="tools-header">Available Tools</div>
            <div class="tools-content">
              ${tools.map(t => {
                const hasParams = t.parameters && typeof t.parameters === 'object' && t.parameters.properties && Object.keys(t.parameters.properties).length > 0;
                if (!hasParams) {
                  return `<div class="tool-item"><span class="tool-item-name">${escapeHtml(t.name)}</span> - <span class="tool-item-desc">${escapeHtml(t.description)}</span></div>`;
                }
                const params = t.parameters;
                const properties = params.properties;
                const required = params.required || [];
                let paramsHtml = '';
                for (const [name, prop] of Object.entries(properties)) {
                  const isRequired = required.includes(name);
                  const typeStr = prop.type || 'any';
                  const reqLabel = isRequired ? '<span class="tool-param-required">required</span>' : '<span class="tool-param-optional">optional</span>';
                  paramsHtml += `<div class="tool-param"><span class="tool-param-name">${escapeHtml(name)}</span> <span class="tool-param-type">${escapeHtml(typeStr)}</span> ${reqLabel}`;
                  if (prop.description) {
                    paramsHtml += `<div class="tool-param-desc">${escapeHtml(prop.description)}</div>`;
                  }
                  paramsHtml += `</div>`;
                }
                return `<div class="tool-item" data-expand="params-expanded"><span class="tool-item-name">${escapeHtml(t.name)}</span> - <span class="tool-item-desc">${escapeHtml(t.description)}</span> <span class="tool-params-hint"></span><div class="tool-params-content">${paramsHtml}</div></div>`;
              }).join('')}
            </div>
          </div>`;
        }

        return html;
      }

      // ============================================================
      // NAVIGATION
      // ============================================================

      function getScrollTargetElementId(entryId) {
        const entry = byId.get(entryId);
        if (entry?.type === 'message' && entry.message.role === 'toolResult' && entry.message.toolCallId) {
          return `tool-call-${entry.message.toolCallId}`;
        }
        return `entry-${entryId}`;
      }

      function renderEntryToNode(entry) {
        const html = renderTranscriptEntry(entry, {
          escape:escapeHtml, markdown:safeMarkedParse, copyLink:renderCopyLinkButton,
          toolResults:toolResultEntries, toolCalls:visibleToolCalls,
          highlight:(text,language)=>hljs.getLanguage(language) ? hljs.highlight(text,{language}).value : escapeHtml(text),
        });
        if (!html) return null;

        const template = document.createElement('template');
        template.innerHTML = html;
        return template.content.firstElementChild;
      }

      let pageStartShown = 0, pageEndShown = 0, pathLength = 0;
      let loadMoreRecords = () => {};
      let currentEntryId = null;

      function navigateTo(targetId, scrollMode = 'target', scrollToEntryId = null, pageStart = null) {
        currentLeafId = targetId;
        currentTargetId = scrollToEntryId || targetId;
        const path = getPath(targetId);

        renderTree();

        document.getElementById('header-container').innerHTML = renderHeader();
        attachHeaderHandlers();

        const messagesEl = document.getElementById('messages');
        const fragment = document.createDocumentFragment();

        const targetIndex = path.findIndex(entry => entry.id === (scrollToEntryId || targetId));
        const start = pageStart ?? (scrollMode === 'none' ? 0 : Math.max(0, targetIndex - 100));
        const end = Math.min(path.length, start + 200);
        toolResultEntries.clear();
        for (const entry of path) {
          if (entry.message?.role === 'toolResult') toolResultEntries.set(entry.message.toolCallId,entry);
        }
        visibleToolCalls.clear();
        for (const entry of path.slice(start,end)) {
          for (const block of entry.message?.content || []) if (block.type === 'toolCall') visibleToolCalls.add(block.id);
        }
        const pageButton = (label, key, onClick) => {
          const wrapper = document.createElement('div');
          wrapper.className = 'page-nav';
          const button = document.createElement('button');
          button.type = 'button';
          button.className = 'btn btn-sm btn-outline';
          button.textContent = label;
          button.addEventListener('click', onClick);
          wrapper.appendChild(button);
          if (key) {
            const hint = document.createElement('kbd');
            hint.className = 'kbd kbd-sm';
            hint.textContent = key;
            wrapper.appendChild(hint);
          }
          return wrapper;
        };
        pageStartShown = start; pageEndShown = end; pathLength = path.length;
        if (start > 0) fragment.appendChild(pageButton('Earlier messages', '[', () => navigateTo(targetId, 'none', null, Math.max(0,start - 200))));
        for (const entry of path.slice(start,end)) {
          const node = renderEntryToNode(entry);
          if (node) {
            fragment.appendChild(node);
          }
        }
        if (end < path.length) fragment.appendChild(pageButton('Later messages', ']', () => navigateTo(targetId, 'none', null, end)));
        if (data.hasMore) {
          const wrapper = pageButton('Load next 200 records', 'M', () => loadMoreRecords());
          const button = wrapper.querySelector('button');
          loadMoreRecords = async () => {
            if (button.disabled) return;
            button.disabled = true;
            try {
              await data.loadMore();
              ({header, entries} = data);
              byId.clear(); toolCallMap.clear();
              for (const entry of entries) {
                byId.set(entry.id, entry);
                for (const block of entry.message?.content || []) if (block.type === 'toolCall') toolCallMap.set(block.id, block);
              }
              treeNodeMap = null; treeRendered = false;
              globalStats = computeStats(entries);
              navigateTo(data.leafId, 'none', null, start);
            } catch (error) {
              document.getElementById('transcript-status').textContent = error.message;
              button.disabled = false;
            }
          };
          fragment.appendChild(wrapper);
        } else {
          loadMoreRecords = () => {};
        }

        messagesEl.innerHTML = '';
        messagesEl.appendChild(fragment);
        setThinkingExpanded(thinkingExpanded);
        setToolOutputsExpanded(toolOutputsExpanded);

        // Attach click handlers for copy-link buttons
        messagesEl.querySelectorAll('.copy-link-btn').forEach(btn => {
          btn.addEventListener('click', (e) => {
            e.stopPropagation();
            const entryId = btn.dataset.entryId;
            const shareUrl = buildShareUrl(entryId);
            copyToClipboard(shareUrl, btn);
          });
        });

        // Use setTimeout(0) to ensure DOM is fully laid out before scrolling
        setTimeout(() => {
          const content = document.getElementById('content');
          if (scrollMode === 'bottom') {
            content.scrollTop = content.scrollHeight;
          } else if (scrollMode === 'target') {
            // If scrollToEntryId is provided, scroll to that specific entry.
            // Tool result entries are rendered inside their assistant tool-call block,
            // so route them to the visible tool-call element instead.
            const scrollTargetId = scrollToEntryId || targetId;
            const targetEl = document.getElementById(getScrollTargetElementId(scrollTargetId)) ||
              document.getElementById(`entry-${scrollTargetId}`);
            if (targetEl) {
              targetEl.scrollIntoView({ block: 'center' });
              // Briefly highlight the target message
              if (scrollToEntryId) {
                targetEl.classList.add('highlight');
                setTimeout(() => targetEl.classList.remove('highlight'), 2000);
              }
            }
          }
        }, 0);
      }

      // ============================================================
      // INITIALIZATION
      // ============================================================

      // Configure marked with syntax highlighting and TUI-compatible HTML handling
      const strictStrikethroughRegex = /^(~~)(?=[^\s~])((?:\\.|[^\\])*?(?:\\.|[^\s~\\]))\1(?=[^~]|$)/;

      marked.use({
        breaks: true,
        gfm: true,
        tokenizer: {
          // Treat HTML-like input as plain text so tags are shown verbatim,
          // matching the TUI markdown renderer.
          html() {
            return undefined;
          },
          tag() {
            return undefined;
          },
          del(src) {
            const match = strictStrikethroughRegex.exec(src);
            if (!match) return undefined;
            return {
              type: 'del',
              raw: match[0],
              text: match[2],
              tokens: this.lexer.inlineTokens(match[2])
            };
          }
        },
        renderer: {
          // Sanitize link URLs with a scheme allow-list. Browsers strip C0
          // controls from schemes, so strip them before checking and emitting.
          link(token) {
            const href = sanitizeMarkdownUrl(token.href);
            if (href === null) {
              return this.parser.parseInline(token.tokens);
            }
            let out = '<a href="' + escapeHtml(href) + '"';
            if (token.title) {
              out += ' title="' + escapeHtml(token.title) + '"';
            }
            out += '>' + this.parser.parseInline(token.tokens) + '</a>';
            return out;
          },
          // Sanitize image src URLs with the same scheme allow-list.
          image(token) {
            const href = sanitizeMarkdownUrl(token.href);
            if (href === null) {
              return escapeHtml(token.text || '');
            }
            // Do not fetch URLs embedded in private transcripts automatically.
            return '<a rel="noreferrer" href="' + escapeHtml(href) + '">[Image: ' + escapeHtml(token.text || href) + ']</a>';
          },
          // Code blocks: syntax highlight, no HTML escaping
          code(token) {
            const code = token.text;
            const lang = token.lang;
            let highlighted;
            if (lang && hljs.getLanguage(lang)) {
              try {
                highlighted = hljs.highlight(code, { language: lang }).value;
              } catch {
                highlighted = escapeHtml(code);
              }
            } else {
              // Auto-detect language if not specified
              try {
                highlighted = hljs.highlightAuto(code).value;
              } catch {
                highlighted = escapeHtml(code);
              }
            }
            return `<pre><code class="hljs">${highlighted}</code></pre>`;
          },
          // Inline code: escape HTML
          codespan(token) {
            return `<code>${escapeHtml(token.text)}</code>`;
          }
        }
      });

      // Simple marked parse (escaping handled in renderers)
      function safeMarkedParse(text) {
        return marked.parse(text);
      }

      // Search input
      const searchInput = document.getElementById('tree-search');
      searchInput.addEventListener('input', (e) => {
        searchQuery = e.target.value;
        forceTreeRerender();
      });

      // Filter buttons
      const setFilterMode = (mode) => {
        filterMode = mode;
        document.querySelectorAll('.filter-btn').forEach(b => {
          const active = b.dataset.filter === mode;
          b.classList.toggle('btn-active', active);
          b.classList.toggle('btn-ghost', !active);
          b.setAttribute('aria-pressed', String(active));
        });
        forceTreeRerender();
      };
      document.querySelectorAll('.filter-btn').forEach(btn => {
        btn.addEventListener('click', () => setFilterMode(btn.dataset.filter));
      });

      // Sidebar toggle
      const sidebar = document.getElementById('sidebar');
      const overlay = document.getElementById('sidebar-overlay');
      const hamburger = document.getElementById('hamburger');
      const sidebarResizer = document.getElementById('sidebar-resizer');
      const SIDEBAR_WIDTH_STORAGE_KEY = 'pi-share:v1:sidebar-width';
      const MIN_CONTENT_WIDTH = 320;

      function isMobileLayout() {
        return window.matchMedia('(max-width: 900px)').matches;
      }

      function getSidebarBounds() {
        const rootStyles = getComputedStyle(document.documentElement);
        const minWidth = parseFloat(rootStyles.getPropertyValue('--sidebar-min-width')) || 240;
        const maxWidth = parseFloat(rootStyles.getPropertyValue('--sidebar-max-width')) || 720;
        const viewportMaxWidth = window.innerWidth - MIN_CONTENT_WIDTH;
        return {
          minWidth,
          maxWidth: Math.max(minWidth, Math.min(maxWidth, viewportMaxWidth))
        };
      }

      function clampSidebarWidth(width) {
        const { minWidth, maxWidth } = getSidebarBounds();
        return Math.max(minWidth, Math.min(maxWidth, width));
      }

      function applySidebarWidth(width) {
        document.documentElement.style.setProperty('--sidebar-width', `${Math.round(clampSidebarWidth(width))}px`);
      }

      function loadSidebarWidth() {
        try {
          const raw = localStorage.getItem(SIDEBAR_WIDTH_STORAGE_KEY);
          if (raw === null) return null;
          const width = Number(raw);
          return Number.isFinite(width) ? width : null;
        } catch {
          return null;
        }
      }

      function saveSidebarWidth(width) {
        try {
          localStorage.setItem(SIDEBAR_WIDTH_STORAGE_KEY, String(Math.round(clampSidebarWidth(width))));
        } catch {
          // Ignore storage failures (e.g. private browsing restrictions)
        }
      }

      function setupSidebarResize() {
        const savedWidth = loadSidebarWidth();
        if (savedWidth !== null) {
          applySidebarWidth(savedWidth);
        }

        if (!sidebarResizer) return;

        let cleanupDrag = null;

        const stopDrag = (pointerId) => {
          if (cleanupDrag) {
            cleanupDrag(pointerId);
            cleanupDrag = null;
          }
        };

        sidebarResizer.addEventListener('pointerdown', (e) => {
          if (isMobileLayout()) return;

          e.preventDefault();
          const startX = e.clientX;
          const startWidth = sidebar.getBoundingClientRect().width;
          document.body.classList.add('sidebar-resizing');
          sidebarResizer.setPointerCapture?.(e.pointerId);

          const onPointerMove = (event) => {
            applySidebarWidth(startWidth + (event.clientX - startX));
          };

          cleanupDrag = (pointerIdToRelease) => {
            document.body.classList.remove('sidebar-resizing');
            sidebarResizer.releasePointerCapture?.(pointerIdToRelease);
            window.removeEventListener('pointermove', onPointerMove);
            window.removeEventListener('pointerup', onPointerUp);
            window.removeEventListener('pointercancel', onPointerCancel);
            saveSidebarWidth(sidebar.getBoundingClientRect().width);
          };

          const onPointerUp = (event) => stopDrag(event.pointerId);
          const onPointerCancel = (event) => stopDrag(event.pointerId);

          window.addEventListener('pointermove', onPointerMove);
          window.addEventListener('pointerup', onPointerUp);
          window.addEventListener('pointercancel', onPointerCancel);
        });

        sidebarResizer.addEventListener('dblclick', () => {
          if (isMobileLayout()) return;
          applySidebarWidth(400);
          saveSidebarWidth(400);
        });

        window.addEventListener('resize', () => {
          if (isMobileLayout()) return;
          applySidebarWidth(sidebar.getBoundingClientRect().width);
        });
      }

      setupSidebarResize();

      hamburger.addEventListener('click', () => {
        sidebar.classList.add('open');
        overlay.classList.add('open');
        hamburger.style.display = 'none';
      });

      const closeSidebar = () => {
        sidebar.classList.remove('open');
        overlay.classList.remove('open');
        hamburger.style.display = '';
      };

      overlay.addEventListener('click', closeSidebar);
      document.getElementById('sidebar-close').addEventListener('click', closeSidebar);

      // Toggle states
      let thinkingExpanded = true;
      let toolOutputsExpanded = false;

      const setThinkingExpanded = (expanded) => {
        thinkingExpanded = expanded;
        document.querySelector('[data-action="toggle-thinking"]')?.setAttribute('aria-pressed', String(expanded));
        document.querySelectorAll('.thinking-text').forEach(el => {
          el.style.display = thinkingExpanded ? '' : 'none';
        });
        document.querySelectorAll('.thinking-collapsed').forEach(el => {
          el.style.display = thinkingExpanded ? 'none' : 'block';
        });
      };

      const setToolOutputsExpanded = (expanded) => {
        toolOutputsExpanded = expanded;
        document.querySelector('[data-action="toggle-tools"]')?.setAttribute('aria-pressed', String(expanded));
        document.querySelectorAll('.tool-execution details, .amp-summary details, .skill-invocation details').forEach(element => { element.open = toolOutputsExpanded; });
      };

      const toggleThinking = () => setThinkingExpanded(!thinkingExpanded);
      const toggleToolOutputs = () => setToolOutputsExpanded(!toolOutputsExpanded);

      const attachHeaderHandlers = () => {
        document.querySelector('[data-action="toggle-thinking"]')?.addEventListener('click', toggleThinking);
        document.querySelector('[data-action="toggle-tools"]')?.addEventListener('click', toggleToolOutputs);
        document.querySelector('[data-action="download-session"]')?.addEventListener('click', window.downloadSessionJson);
      };

      const isEditableTarget = (element) => {
        if (!element) return false;
        const tagName = element.tagName;
        if (tagName === 'INPUT' || tagName === 'TEXTAREA' || tagName === 'SELECT' || tagName === 'BUTTON') {
          return true;
        }
        return element.isContentEditable || Boolean(element.closest?.('[contenteditable="true"]'));
      };

      // Keyboard shortcuts, registered with the shared palette and help dialog.
      const contentEl = document.getElementById('content');
      const messageElements = () => Array.from(document.querySelectorAll('#messages [id^="entry-"]'));
      const setCurrentEntry = (element) => {
        document.querySelectorAll('#messages .is-current').forEach(el => el.classList.remove('is-current'));
        currentEntryId = element ? element.id.slice(6) : null;
        element?.classList.add('is-current');
      };
      const stepMessage = (delta) => {
        const elements = messageElements();
        if (!elements.length) return;
        const viewportTop = 0;
        let index = elements.findIndex(el => el.id.slice(6) === currentEntryId);
        const rect = index >= 0 ? elements[index].getBoundingClientRect() : null;
        const inView = rect && rect.bottom > viewportTop && rect.top < window.innerHeight;
        if (!inView) {
          index = elements.findIndex(el => el.getBoundingClientRect().bottom > 60);
          if (index < 0) index = elements.length - 1;
          if (currentEntryId === null && delta > 0) delta = 0;
        }
        const next = Math.min(elements.length - 1, Math.max(0, index + delta));
        setCurrentEntry(elements[next]);
        elements[next].scrollIntoView({ block: 'start' });
        window.scrollBy(0, -64);
      };
      document.getElementById('messages').addEventListener('click', (event) => {
        const section = event.target.closest('[id^="entry-"]');
        if (section) setCurrentEntry(section);
      });
      const toggleSidebar = () => {
        if (isMobileLayout()) {
          if (sidebar.classList.contains('open')) closeSidebar();
          else hamburger.click();
        } else {
          document.body.classList.toggle('sidebar-collapsed');
        }
      };
      const filterKeys = ['default', 'no-tools', 'user-only', 'labeled-only', 'all'];
      const filterLabels = ['Default', 'No tools', 'User', 'Labeled', 'All'];
      registerShortcuts([
        { keys: 'j', label: 'Next message', group: 'Transcript', run: () => stepMessage(1) },
        { keys: 'k', label: 'Previous message', group: 'Transcript', run: () => stepMessage(-1) },
        { keys: 't', label: 'Show or hide reasoning', group: 'Transcript', run: () => toggleThinking() },
        { keys: 'o', label: 'Expand or collapse tool output', group: 'Transcript', run: () => toggleToolOutputs() },
        { keys: 'b', label: 'Toggle the branch tree', group: 'Transcript', run: toggleSidebar },
        { keys: '/', label: 'Search the branch tree', group: 'Transcript', run: () => { document.body.classList.remove('sidebar-collapsed'); if (isMobileLayout()) hamburger.click(); searchInput.focus(); } },
        { keys: '[', label: 'Earlier messages on this branch', group: 'Transcript', run: () => { if (pageStartShown > 0) navigateTo(currentLeafId, 'none', null, Math.max(0, pageStartShown - 200)); } },
        { keys: ']', label: 'Later messages on this branch', group: 'Transcript', run: () => { if (pageEndShown < pathLength) navigateTo(currentLeafId, 'none', null, pageEndShown); } },
        { keys: 'y', label: 'Copy link to the current message', group: 'Transcript', run: () => { if (currentEntryId) copyToClipboard(buildShareUrl(currentEntryId), null); else toast('Select a message first with J or K'); } },
        { keys: 'm', label: 'Load more records', group: 'Transcript', run: () => loadMoreRecords() },
        ...filterKeys.map((mode, index) => ({ keys: String(index + 1), label: `Tree filter: ${filterLabels[index]}`, group: 'Transcript', run: () => setFilterMode(mode), palette: false })),
        { keys: 'escape', label: 'Reset to the latest message', group: 'Transcript', hidden: true, palette: false, run: () => {
          if (document.activeElement === searchInput && !searchInput.value) { searchInput.blur(); return; }
          if (isEditable(document.activeElement)) { document.activeElement.blur(); return; }
          searchInput.value = '';
          searchQuery = '';
          navigateTo(leafId, 'bottom');
        } },
      ]);
      const isEditable = (element) => element && (['INPUT','TEXTAREA','SELECT'].includes(element.tagName) || element.isContentEditable);
      void contentEl;

      // Delegate expansion instead of inline handlers to preserve Traicr's CSP.
      document.addEventListener('click', (event) => {
        const expandable = event.target.closest('[data-expand]');
        if (!expandable || event.target.closest('a, button') || window.getSelection().toString()) return;
        expandable.classList.toggle(expandable.dataset.expand);
      });

      // Initial render
      // If URL has targetId, scroll to that specific message; otherwise stay at top
      if (leafId) {
        if (urlTargetId && byId.has(urlTargetId)) {
          // Deep link: navigate to leaf and scroll to target message
          navigateTo(urlLeafId || findNewestLeaf(urlTargetId), 'target', urlTargetId);
        } else {
          navigateTo(leafId, 'none');
        }
      } else if (entries.length > 0) {
        // Fallback: use last entry if no leafId
        navigateTo(entries[entries.length - 1].id, 'none');
      }
    })().catch((error) => {
      document.getElementById('transcript-status').textContent = error.message;
      console.error('Transcript viewer failed:', error);
    });
