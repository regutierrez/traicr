function details(label, body, escape) {
  return `<details class="amp-details"><summary>${escape(label)}</summary>${body}</details>`;
}

function fields(values, escape) {
  return '<dl class="amp-fields">' + Object.entries(values).filter(([,v]) => v !== undefined && v !== null && v !== '').map(([key,value]) => `<dt>${escape(key)}</dt><dd>${escape(typeof value === 'object' ? JSON.stringify(value) : value)}</dd>`).join('') + '</dl>';
}

function jsonDetails(label, value, escape) {
  return value == null ? '' : details(label, `<pre>${escape(JSON.stringify(value,null,2))}</pre>`, escape);
}

function threadLinks(id, label, escape) {
  if (typeof id !== 'string' || !id) return '';
  const match = id.match(/^https:\/\/ampcode\.com\/threads\/([^/?#]+)/);
  if (match) id = match[1];
  return `<div class="amp-related">${escape(label)}: <a href="/traces/resolve?native_id=${encodeURIComponent(id)}">${escape(id)}</a> · <a href="https://ampcode.com/threads/${encodeURIComponent(id)}" target="_blank" rel="noreferrer">Original in Amp ↗</a></div>`;
}

function attachmentHTML(attachment, escape) {
  const name = attachment.name || attachment.path || attachment.media_type || 'Attachment';
  if ((attachment.inline || attachment.archived_path) && attachment.revisionId && attachment.source_pointer) {
    const url = `/revisions/${encodeURIComponent(attachment.revisionId)}/attachment?pointer=${encodeURIComponent(attachment.source_pointer)}`;
    const download = attachment.archived_path ? `/revisions/${encodeURIComponent(attachment.revisionId)}/file?path=${encodeURIComponent(attachment.archived_path)}&download=1` : `${url}&download=1`;
    const preview = attachment.size > 10*1024*1024 ? '<p>Image exceeds the 10 MiB preview limit. The original is archived.</p>' : `<a href="${url}" target="_blank" rel="noreferrer"><img loading="lazy" src="${url}" alt="${escape(name)}" class="message-image"></a>`;
    return details(`Archived image: ${name}`, `${preview}<a href="${download}">Download image</a>`, escape);
  }
  let url;
  try { url = new URL(attachment.url); } catch { url = null; }
  if (url && ['https:','http:'].includes(url.protocol)) {
    return `<div class="amp-attachment">External reference · <a href="${escape(url.href)}" target="_blank" rel="noreferrer">${escape(name)} ↗</a><span> Bytes are not archived; access may require Amp sign-in.</span></div>`;
  }
  return `<div class="amp-attachment">${escape(name)} · Attachment unavailable; inspect the source block.</div>`;
}

function sourceDetails(entry, escape) {
  const links = (entry.sources || []).map(source => {
    const url = source.revisionId && source.pointer ? `/revisions/${encodeURIComponent(source.revisionId)}/block?pointer=${encodeURIComponent(source.pointer)}` : `/events/${encodeURIComponent(source.eventId)}/sources`;
    return `<li><a href="${url}" target="_blank" rel="noreferrer">${escape(source.pointer || source.key || 'Source records')}</a></li>`;
  }).join('');
  const message = entry.message;
  const native = message?.nativeDetails || {};
  const starts = (entry.sources || []).map(s => Date.parse(s.details?.start_time)).filter(Number.isFinite);
  const ends = (entry.sources || []).map(s => Date.parse(s.details?.final_time)).filter(Number.isFinite);
  const duration = starts.length && ends.length ? Math.max(...ends) - Math.min(...starts) : undefined;
  return details('Message details & source', fields({Model:message?.model,Provider:message?.provider,Timestamp:entry.timestamp,'Generation span':duration === undefined ? undefined : `${duration} ms`,State:native.state,'Context tokens':native.usage?.totalInputTokens,'Context limit':native.usage?.maxInputTokens,Usage:native.usage},escape) + `<ul>${links}</ul>`, escape);
}

function renderAmpTool(call, resultEntry, renderers) {
  const {escape,markdown} = renderers;
  const result = resultEntry?.message;
  const run = result?.run;
  const payload = run?.result;
  let status = 'result not collected';
  if (result?.isError) status = 'failed';
  else if (payload?.running) status = 'running';
  else if (run?.status) status = run.status;
  else if (result) status = 'unknown';
  else if (call.details?.complete === false) status = 'incomplete arguments';
  let body = `<div class="tool-header">${escape(call.name)} <span class="amp-state">Recorded: ${escape(status)}</span></div>`;
  const args = call.arguments || {};
  if (call.name === 'shell_command') body += `<pre class="tool-command">$ ${escape(args.command || '')}</pre>` + fields({'Working directory':args.workdir},escape);
  if (call.name === 'skill') body += fields({Skill:args.name},escape);
  body += jsonDetails('Arguments',args,escape);
  body += fields({'Process ID':payload?.pid},escape);
  if (typeof payload?.exitCode === 'number') body += `<span class="amp-state">Exit code: ${escape(payload.exitCode)}</span>`;
  for (const key of ['thread','threadID','thread_id']) if (typeof args[key] === 'string') body += threadLinks(args[key],call.name === 'send_message_to_thread' ? 'Message to thread' : 'Referenced thread',escape);
  if (result) {
    const text = result.content.filter(b => b.type === 'text').map(b => b.text).join('\n');
    const output = ['shell_command','shell_command_status','shell_command_kill'].includes(call.name) ? `<pre>${escape(text)}</pre>` : `<div class="markdown-content">${markdown(text)}</div>`;
    if (text) body += text.length > 1600 || call.name === 'skill' ? details('Tool output',output,escape) : `<div class="tool-output">${output}</div>`;
    else body += '<div class="amp-state">No text output recorded.</div>';
    if (Array.isArray(payload?.files)) {
      for (const file of payload.files) {
        if (!file || typeof file !== 'object') continue;
        const diff = typeof file.diff === 'string' ? file.diff.split('\n').map(line => `<div class="${line.startsWith('+') ? 'diff-added' : line.startsWith('-') ? 'diff-removed' : 'diff-context'}">${escape(line)}</div>`).join('') : '';
        body += details(`${file.uri || file.path || 'Changed file'} · +${file.additions ?? '?'} −${file.deletions ?? '?'}`, `<pre class="tool-diff">${diff}</pre>`,escape);
      }
    }
    body += result.content.filter(b => b.type === 'attachment').map(a => attachmentHTML(a,escape)).join('');
    if (payload?.threadID) body += threadLinks(payload.threadID,call.name === 'create_thread' ? 'Spawned thread' : 'Referenced thread',escape);
    if (payload && typeof payload === 'object') body += jsonDetails('Structured result',payload,escape);
    body += sourceDetails(resultEntry,escape);
  }
  return `<div class="tool-execution ${result?.isError ? 'error' : status === 'done' ? 'success' : 'pending'}" id="tool-call-${escape(call.id)}">${body}</div>`;
}

export function renderAmpEntry(entry, renderers) {
  const {escape,markdown,copyLink,toolResults,toolCalls} = renderers;
  if (entry.type === 'session_info') return '';
  if (entry.type === 'compaction') return `<section class="amp-summary" id="entry-${escape(entry.id)}">${details('Compaction summary',`<div class="markdown-content">${markdown(entry.summary)}</div>` + sourceDetails(entry,escape),escape)}</section>`;
  if (entry.type === 'custom_message') return `<section class="hook-message" id="entry-${escape(entry.id)}"><strong>Unsupported content · ${escape(entry.sources?.[0]?.details?.native_type || entry.customType)}</strong><pre>${escape(entry.content)}</pre>${sourceDetails(entry,escape)}</section>`;
  const message = entry.message;
  if (!message) return '';
  if (message.role === 'toolResult') {
    if (toolCalls?.has(message.toolCallId)) return '';
    return renderAmpTool({id:message.toolCallId || entry.id,name:message.toolName || 'Unpaired tool result'},entry,renderers);
  }
  const origin = message.nativeDetails?.message_meta;
  const phase = origin?.openAIResponsePhase;
  let body = copyLink(entry.id) + `<div class="message-timestamp">${escape(message.role)}${phase ? ' · ' + escape(phase === 'final_answer' ? 'Final answer' : phase) : ''}${entry.timestamp ? ' · ' + escape(new Date(entry.timestamp).toLocaleString()) : ''}</div>`;
  if (origin?.fromExecutorThreadID) body += threadLinks(origin.fromExecutorThreadID,'From thread',escape);
  if (origin?.fromAutomation) body += '<div class="amp-state">Automation message</div>';
  for (const block of message.content) {
    if (block.type === 'text') body += `<div class="markdown-content">${markdown(block.text)}</div>`;
    else if (block.type === 'hidden') body += details('Hidden context',`<pre>${escape(block.text)}</pre>`,escape);
    else if (block.type === 'thinking') body += `<div class="thinking-block"><div class="thinking-text">${escape(block.thinking || 'Reasoning text not included in export.')}</div><div class="thinking-collapsed">Thinking …</div></div>`;
    else if (block.type === 'attachment') body += attachmentHTML(block,escape);
    else if (block.type === 'toolCall') body += renderAmpTool(block,toolResults.get(block.id),renderers);
  }
  if (message.stopReason && !['stop','toolUse','complete'].includes(message.stopReason)) body += `<div class="error-text">Recorded response state: ${escape(message.stopReason)}</div>`;
  body += sourceDetails(entry,escape);
  return `<section class="${message.role === 'user' ? 'user-message' : 'assistant-message'}" id="entry-${escape(entry.id)}">${body}</section>`;
}

export function renderAmpHeaderDetails(header, entries, {escape}) {
  const messages = entries.filter(e => e.message?.role === 'assistant').map(e => e.message);
  const totals = {};
  for (const [key,label] of [['input','Input tokens'],['output','Output tokens'],['cacheRead','Cache reads'],['cacheWrite','Cache writes']]) {
    const values = messages.map(m => m.usage?.[key]).filter(Number.isFinite);
    totals[label] = values.length ? `${values.reduce((a,b)=>a+b,0)}${values.length < messages.length ? ' (partial)' : ''}` : 'Unknown';
  }
  const native = header.native || {};
  return details('Usage & environment', '<p>Totals cover loaded assistant messages in this view, once per message. Costs are not provided by this export.</p>' + fields(totals,escape) + fields({'Agent mode':native.agentMode,Features:native.meta?.features,Executor:native.meta?.executorType,Archived:native.archived},escape) + jsonDetails('Activated skills',native.activatedSkills,escape) + jsonDetails('Recorded environment',native.env,escape),escape);
}
