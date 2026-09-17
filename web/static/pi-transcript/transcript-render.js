function details(label, body, escape) {
  return `<details class="amp-details"><summary>${escape(label)}</summary>${body}</details>`;
}

function fields(values, escape) {
  return '<dl class="amp-fields">' + Object.entries(values).filter(([,v]) => v !== undefined && v !== null && v !== '').map(([key,value]) => `<dt>${escape(key)}</dt><dd>${escape(typeof value === 'object' ? JSON.stringify(value) : value)}</dd>`).join('') + '</dl>';
}

function jsonDetails(label, value, escape) {
  return value == null ? '' : details(label, `<pre>${escape(JSON.stringify(value,null,2))}</pre>`, escape);
}

export function parseSkillBlock(text) {
  const match = text.match(/^<skill name="([^"]+)" location="([^"]+)">\n([\s\S]*?)\n<\/skill>(?:\n\n([\s\S]+))?$/);
  if (!match) return null;
  return {name:match[1], location:match[2], content:match[3], userMessage:match[4]?.trim()};
}

function threadLinks(id, label, escape) {
  if (typeof id !== 'string' || !id) return '';
  const match = id.match(/^https:\/\/ampcode\.com\/threads\/([^/?#]+)/);
  if (match) id = match[1];
  const original = id.startsWith('T-') ? ` · <a href="https://ampcode.com/threads/${encodeURIComponent(id)}" target="_blank" rel="noreferrer">Original in Amp ↗</a>` : '';
  return `<div class="amp-related">${escape(label)}: <a href="/traces/resolve?native_id=${encodeURIComponent(id)}">${escape(id)}</a>${original}</div>`;
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
    return `<div class="amp-attachment">External reference · <a href="${escape(url.href)}" target="_blank" rel="noreferrer">${escape(name)} ↗</a><span> Bytes are not archived; access may require sign-in to the source service.</span></div>`;
  }
  return `<div class="amp-attachment">${escape(name)} · Attachment unavailable; inspect the source block.</div>`;
}

function codeBlock(text, path, {escape,highlight}) {
  const languages = {ts:'typescript',tsx:'typescript',js:'javascript',jsx:'javascript',py:'python',rb:'ruby',rs:'rust',go:'go',java:'java',c:'c',cpp:'cpp',h:'c',hpp:'cpp',cs:'csharp',php:'php',sh:'bash',bash:'bash',zsh:'bash',sql:'sql',html:'html',css:'css',scss:'scss',json:'json',yaml:'yaml',yml:'yaml',xml:'xml',md:'markdown',dockerfile:'dockerfile'};
  const language = typeof path === 'string' && languages[path.split('.').pop().toLowerCase()];
  return typeof language === 'string' && highlight ? `<pre><code class="hljs">${highlight(text,language)}</code></pre>` : `<pre>${escape(text)}</pre>`;
}

function diffBlock(text, escape) {
  return `<pre class="tool-diff">${text.split('\n').map(line => `<div class="${line.startsWith('+') ? 'diff-added' : line.startsWith('-') ? 'diff-removed' : 'diff-context'}">${escape(line)}</div>`).join('')}</pre>`;
}

function renderTranscriptTool(call, resultEntry, renderers) {
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
  const name = call.name.toLowerCase();
  const path = args.file_path ?? args.path;
  if (['shell_command','bash'].includes(name)) body += `<pre class="tool-command">$ ${escape(args.command || '')}</pre>` + fields({'Working directory':args.workdir},escape);
  if (['read','write','edit','ls','glob','grep','find'].includes(name)) body += fields({File:path,'Start line':args.offset,'Line limit':args.limit},escape);
  if (name === 'write' && typeof args.content === 'string') body += details('Requested content',codeBlock(args.content,path,renderers),escape);
  if (name === 'edit') {
    const before = args.oldText ?? args.old_string;
    const after = args.newText ?? args.new_string;
    if (typeof before === 'string' && typeof after === 'string') body += details('Requested edit',diffBlock(before.split('\n').map(line=>'-'+line).concat(after.split('\n').map(line=>'+'+line)).join('\n'),escape),escape);
  }
  if (call.name === 'skill') body += fields({Skill:args.name},escape);
  body += jsonDetails('Arguments',args,escape);
  body += fields({'Process ID':payload?.pid},escape);
  if (typeof payload?.exitCode === 'number') body += `<span class="amp-state">Exit code: ${escape(payload.exitCode)}</span>`;
  for (const key of ['thread','threadID','thread_id']) if (typeof args[key] === 'string') body += threadLinks(args[key],call.name === 'send_message_to_thread' ? 'Message to thread' : 'Referenced thread',escape);
  if (result) {
    const text = result.content.filter(b => b.type === 'text').map(b => b.text).join('\n');
    const output = ['shell_command','shell_command_status','shell_command_kill','bash','read','edit','write','ls','glob','grep','find'].includes(name) ? codeBlock(text,name === 'read' ? path : null,renderers) : `<div class="markdown-content">${markdown(text)}</div>`;
    if (text) body += details('Tool output',output,escape);
    else body += '<div class="amp-state">No text output recorded.</div>';
    if (Array.isArray(payload?.files)) {
      for (const file of payload.files) {
        if (!file || typeof file !== 'object') continue;
        const diff = typeof file.diff === 'string' ? diffBlock(file.diff,escape) : '';
        body += details(`${file.uri || file.path || 'Changed file'} · +${file.additions ?? '?'} −${file.deletions ?? '?'}`, diff,escape);
      }
    }
    body += result.content.filter(b => b.type === 'attachment').map(a => attachmentHTML(a,escape)).join('');
    if (payload?.threadID) body += threadLinks(payload.threadID,call.name === 'create_thread' ? 'Spawned thread' : 'Referenced thread',escape);
    if (payload && typeof payload === 'object') body += jsonDetails('Structured result',payload,escape);
  }
  return `<div class="tool-execution ${result?.isError ? 'error' : status === 'done' ? 'success' : 'pending'}" id="tool-call-${escape(call.id)}">${body}</div>`;
}

export function renderTranscriptEntry(entry, renderers) {
  const {escape,markdown,copyLink,toolResults,toolCalls} = renderers;
  const timestamp = entry.timestamp ? `<div class="message-timestamp">${escape(new Date(entry.timestamp).toLocaleString())}</div>` : '';
  if (entry.type === 'session_info' || entry.type === 'label') return '';
  if (entry.type === 'model_change') return `<div class="model-change" id="entry-${escape(entry.id)}">${timestamp}Switched to model: <span class="model-name">${escape([entry.provider,entry.modelId].filter(Boolean).join('/'))}</span></div>`;
  if (entry.type === 'thinking_level_change') return `<div class="model-change" id="entry-${escape(entry.id)}">${timestamp}Thinking level: ${escape(entry.thinkingLevel)}</div>`;
  if (entry.type === 'compaction' || entry.type === 'branch_summary') return `<section class="amp-summary" id="entry-${escape(entry.id)}">${timestamp}${details(entry.type === 'compaction' ? 'Compaction summary' : 'Branch summary',`<div class="markdown-content">${markdown(entry.summary)}</div>`,escape)}</section>`;
  if (entry.type === 'custom_message') {
    const unknown = entry.customType === 'unknown';
    const content = unknown ? `<pre>${escape(entry.content)}</pre>` : `<div class="markdown-content">${markdown(entry.content)}</div>`;
    return `<section class="hook-message" id="entry-${escape(entry.id)}">${timestamp}<strong>${unknown ? 'Unsupported content · ' : ''}${escape(entry.sources?.[0]?.details?.native_type || entry.customType)}</strong>${content}</section>`;
  }
  const message = entry.message;
  if (!message) return '';
  if (message.role === 'toolResult') {
    if (toolCalls?.has(message.toolCallId)) return '';
    return renderTranscriptTool({id:message.toolCallId || entry.id,name:message.toolName || 'Unpaired tool result'},entry,renderers);
  }
  const origin = message.nativeDetails?.message_meta;
  const phase = origin?.openAIResponsePhase;
  let body = copyLink(entry.id) + `<div class="message-timestamp">${escape(message.role)}${phase ? ' · ' + escape(phase === 'final_answer' ? 'Final answer' : phase) : ''}${entry.timestamp ? ' · ' + escape(new Date(entry.timestamp).toLocaleString()) : ''}</div>`;
  if (origin?.fromExecutorThreadID) body += threadLinks(origin.fromExecutorThreadID,'From thread',escape);
  if (origin?.fromAutomation) body += '<div class="amp-state">Automation message</div>';
  for (const block of message.content) {
    if (block.type === 'text') {
      const skill = message.role === 'user' && parseSkillBlock(block.text);
      if (skill) {
        body += `<div class="skill-invocation">${details(`Skill: ${skill.name}`,`<div class="markdown-content">${markdown(skill.content)}</div>`,escape)}</div>`;
        if (skill.userMessage) body += `<div class="markdown-content">${markdown(skill.userMessage)}</div>`;
      } else body += `<div class="markdown-content">${markdown(block.text)}</div>`;
    }
    else if (block.type === 'hidden') body += details('Hidden context',`<pre>${escape(block.text)}</pre>`,escape);
    else if (block.type === 'thinking') body += `<div class="thinking-block"><div class="thinking-text">${escape(block.thinking || 'Reasoning text not included in export.')}</div><div class="thinking-collapsed">Thinking …</div></div>`;
    else if (block.type === 'attachment') body += attachmentHTML(block,escape);
    else if (block.type === 'toolCall') body += renderTranscriptTool(block,toolResults.get(block.id),renderers);
  }
  if (message.stopReason && !['stop','toolUse','complete'].includes(message.stopReason)) body += `<div class="error-text">Recorded response state: ${escape(message.stopReason)}</div>`;
  return `<section class="${message.role === 'user' ? 'user-message' : 'assistant-message'}" id="entry-${escape(entry.id)}">${body}</section>`;
}

export function renderTranscriptHeaderDetails(header, entries, {escape}) {
  const messages = entries.filter(e => e.message?.role === 'assistant').map(e => e.message);
  if (!header.native && !messages.some(message => message.usage)) return '';
  const totals = {};
  for (const [key,label] of [['input','Input tokens'],['output','Output tokens'],['cacheRead','Cache reads'],['cacheWrite','Cache writes']]) {
    const values = messages.map(m => m.usage?.[key]).filter(Number.isFinite);
    totals[label] = values.length ? `${values.reduce((a,b)=>a+b,0)}${values.length < messages.length ? ' (partial)' : ''}` : 'Unknown';
  }
  const native = header.native || {};
  return details('Usage & environment', '<p>Totals cover loaded assistant messages in this view, once per message. Costs are not provided by this export.</p>' + fields(totals,escape) + fields({'Agent mode':native.agentMode,Features:native.meta?.features,Executor:native.meta?.executorType,Archived:native.archived},escape) + jsonDetails('Activated skills',native.activatedSkills,escape) + jsonDetails('Recorded environment',native.env,escape),escape);
}
