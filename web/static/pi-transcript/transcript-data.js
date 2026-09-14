function nativeMessageKey(event) {
  if (event.metadata?.transcript_message) return event.metadata.transcript_message;
  const key = event.key || String(event.id);
  const native = key.slice(key.indexOf(":") + 1);
  if (native.startsWith("sha256:")) return key;
  return native.replace(/:\d+$/, "");
}

function blockIndex(event) {
  if (Number.isInteger(event.metadata?.transcript_block)) return event.metadata.transcript_block;
  const match = event.key?.match(/:(\d+)$/);
  return match ? Number(match[1]) : event.id;
}

export function buildTranscriptSession(events, metadata) {
  const groups = new Map();
  for (const event of events) {
    const key = nativeMessageKey(event);
    if (!groups.has(key)) groups.set(key, []);
    groups.get(key).push(event);
  }
  const ordered = [...groups.values()].sort((a, b) => {
    if (Number.isInteger(a[0].metadata?.transcript_order) && Number.isInteger(b[0].metadata?.transcript_order)) {
      return a[0].metadata.transcript_order - b[0].metadata.transcript_order;
    }
    return (a[0].timestamp || "").localeCompare(b[0].timestamp || "") || a.reduce((id,e)=>Math.min(id,e.id),Infinity) - b.reduce((id,e)=>Math.min(id,e.id),Infinity);
  });
  const entries = [];
  const eventEntries = new Map();
  const groupEntries = new Map();
  const groupParents = new Map();
  let previous = null;
  for (const group of ordered) {
    group.sort((a, b) => blockIndex(a) - blockIndex(b) || a.id - b.id);
    const first = group[0];
    const groupKey = nativeMessageKey(first);
    let messageEntry = null;
    const added = [];
    function append(entry) {
      entry.id = String(entry.id);
      entry.parentId = added.at(-1)?.id || null;
      entries.push(entry);
      added.push(entry);
      return entry;
    }
    for (const event of group) {
      const text = String(event.text || "");
      let entry;
      if (["message", "reasoning", "tool_call", "attachment"].includes(event.kind) && (!event.role || ["user", "assistant"].includes(event.role) || event.kind === "tool_call")) {
        const role = event.role === "user" ? "user" : "assistant";
        if (!messageEntry || messageEntry.message.role !== role) {
          messageEntry = append({id: event.id, type: "message", timestamp: event.timestamp,
            message: {role, content: [], model: event.model, provider: event.provider, stopReason: "stop"}});
        }
        entry = messageEntry;
        if (event.kind === "reasoning") entry.message.content.push({type: "thinking", thinking: text});
        else if (event.kind === "tool_call") {
          let args;
          try { args = JSON.parse(text); } catch { args = {raw: text}; }
          if (!args || typeof args !== "object") args = {raw: text};
          entry.message.content.push({type: "toolCall", id: event.call_id || event.key, name: event.tool || "tool", arguments: args});
        } else if (event.kind === "attachment") {
          entry.message.content.push({type: "text", text: (event.attachments || []).map(a => `Attachment: ${a.name || a.path || a.url || a.media_type || "See source records"}`).join("\n")});
        } else entry.message.content.push({type: "text", text});
      } else if (event.kind === "tool_result") {
        const existing = added.find(e => e.message?.role === "toolResult" && e.message.toolCallId === event.call_id);
        entry = existing || append({id: event.id, type: "message", timestamp: event.timestamp,
          message: {role: "toolResult", toolCallId: event.call_id, toolName: event.tool, content: []}});
        entry.message.content.push({type: "text", text});
        messageEntry = null;
      } else {
        messageEntry = null;
        if (event.kind === "model_change") entry = append({id:event.id,type:"model_change",timestamp:event.timestamp,modelId:event.model || text,provider:event.provider || ""});
        else if (event.kind === "compaction" || event.kind === "branch_summary") entry = append({id:event.id,type:event.kind,timestamp:event.timestamp,summary:text});
        else if (event.kind === "thinking_level_change") entry = append({id:event.id,type:event.kind,timestamp:event.timestamp,thinkingLevel:text});
        else if (event.kind === "session_info" || event.kind === "label") entry = append({id:event.id,type:event.kind,timestamp:event.timestamp});
        else entry = append({id:event.id,type:"custom_message",timestamp:event.timestamp,customType:event.kind,display:true,content:text});
      }
      eventEntries.set(String(event.id), entry.id);
      eventEntries.set(event.key, entry.id);
    }
    if (added.length) {
      groupEntries.set(groupKey, {first:added[0],last:added.at(-1)});
      groupParents.set(groupKey, {key:first.parent_key,nativeParent:first.metadata?.transcript_parent,previous});
      previous = added.at(-1).id;
    }
  }
  const entryMap = new Map(entries.map(e => [e.id,e]));
  for (const [key, group] of groupEntries) {
    const parent = groupParents.get(key);
    const nativeParent = parent.nativeParent || (parent.key && nativeMessageKey({key:parent.key}));
    group.first.parentId = groupEntries.get(nativeParent)?.last.id || eventEntries.get(parent.key) || parent.previous;
  }
  // A malformed native graph must not hang Pi's parent walkers.
  for (const entry of entries) {
    const seen = new Set([entry.id]);
    let node = entry;
    while (node.parentId && entryMap.has(node.parentId)) {
      if (seen.has(node.parentId)) { node.parentId = null; break; }
      seen.add(node.parentId);
      node = entryMap.get(node.parentId);
    }
  }
  return {header: {id:metadata.nativeId, harness:metadata.harness, title:metadata.title, timestamp:entries.find(e => e.timestamp)?.timestamp, cwd:metadata.cwd},
    entries, leafId:entries.at(-1)?.id, eventEntries};
}

export async function loadTranscriptSession(metadata) {
  const events = [];
  let cursor = "";
  const cursors = new Set();
  do {
    const url = `/traces/${encodeURIComponent(metadata.traceId)}/events?limit=200&cursor=${encodeURIComponent(cursor)}`;
    const response = await fetch(url, {headers:{Accept:"application/json"}});
    if (!response.ok || !response.headers.get("content-type")?.includes("application/json")) throw new Error("Transcript load failed. Reload the page and sign in again.");
    const page = await response.json();
    events.push(...page.events);
    cursor = page.next_cursor || "";
    if (cursor && cursors.has(cursor)) throw new Error("Transcript pagination returned a repeated cursor.");
    cursors.add(cursor);
    document.getElementById("transcript-status").textContent = `Loading transcript… ${events.length} records`;
  } while (cursor);
  const data = buildTranscriptSession(events, metadata);
  document.getElementById("transcript-status").textContent = events.length ? "Normalized transcript · Original records and parsing warnings are in Session details." : "No normalized messages. Open Session details to inspect the retained source files.";
  return data;
}
