import {test} from 'node:test';
import assert from 'node:assert/strict';
import {renderTranscriptEntry,renderTranscriptHeaderDetails} from './transcript-render.js';
import {buildTranscriptSession} from './transcript-data.js';

const escape = value => String(value ?? '').replace(/[&<>"']/g, c => ({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));
const renderers = {escape,markdown:escape,copyLink:()=>'',toolResults:new Map()};

test('shared renderer preserves Pi skills, model changes, summaries and custom entries',()=>{
 const session=buildTranscriptSession([
  {id:1,key:'message:a:0',kind:'message',role:'user',text:'<skill name="review" location="/skills/review">\nCheck <script> safely.\n</skill>\n\nReview the changes.'},
  {id:2,key:'model_change:b',kind:'model_change',provider:'example',model:'model-v2'},
  {id:3,key:'branch_summary:c',kind:'branch_summary',text:'Alternate branch summary'},
  {id:4,key:'compaction:d',kind:'compaction',text:'Earlier work summarized'},
  {id:5,key:'custom_message:e',kind:'custom_message',text:'Subagent finished <script>'},
 ],{harness:'pi'});
 const html=session.entries.map(e=>renderTranscriptEntry(e,renderers)).join('');
 assert.match(html,/<summary>Skill: review<\/summary>/);
 assert.match(html,/Check &lt;script&gt; safely/);
 assert.match(html,/Review the changes\./);
 assert.match(html,/Switched to model:.*example\/model-v2/);
 assert.match(html,/Branch summary.*Alternate branch summary/);
 assert.match(html,/Compaction summary.*Earlier work summarized/);
 assert.match(html,/Subagent finished &lt;script&gt;/);
 assert.doesNotMatch(html,/<script>|<skill|<details[^>]*open/);
});

test('shared tools keep original names and display shell and file operations consistently',()=>{
 const render=(name,args,text='output <tag>')=>renderTranscriptEntry({id:'1',message:{role:'assistant',content:[{type:'toolCall',id:'call',name,arguments:args}]}},{...renderers,toolResults:new Map([['call',{message:{role:'toolResult',content:[{type:'text',text}]}}]])});
 for(const name of ['bash','Bash','shell_command']) {
  const html=render(name,{command:'echo <value>'});
  assert.ok(html.includes(`class="tool-header">${name} `));
  assert.match(html,/<pre class="tool-command">\$ echo &lt;value&gt;<\/pre>/);
  assert.match(html,/<summary>Tool output<\/summary><pre>output &lt;tag&gt;<\/pre>/);
 }
 for(const name of ['read','Read']) {
  const html=render(name,{file_path:'src/main.js',offset:3,limit:7});
  assert.match(html,/File<\/dt><dd>src\/main.js/);
  assert.match(html,/Start line<\/dt><dd>3/);
  assert.match(html,/Line limit<\/dt><dd>7/);
 }
 for(const name of ['write','Write']) {
  assert.match(render(name,{path:'index.html',content:'<h1>Hello</h1>'}),/<summary>Requested content<\/summary><pre>&lt;h1&gt;Hello&lt;\/h1&gt;<\/pre>/);
 }
 for(const [name,args] of [['edit',{oldText:'old <tag>',newText:'new & value'}],['Edit',{old_string:'old <tag>',new_string:'new & value'}]]) {
  const html=render(name,{path:'file.txt',...args});
  assert.match(html,/<summary>Requested edit<\/summary>/);
  assert.match(html,/diff-removed">-old &lt;tag&gt;/);
  assert.match(html,/diff-added">\+new &amp; value/);
  assert.doesNotMatch(html,/<tag>/);
 }
});

test('Claude Bash calls show a command and collapsed arguments and output',()=>{
 const data=buildTranscriptSession([
  {id:1,key:'tool_call:a:0',kind:'tool_call',role:'assistant',call_id:'bash',tool:'Bash',text:'{"command":"printf \'<tag>\\n\'","description":"Inspect output"}'},
  {id:2,key:'tool_result:r:0',parent_key:'tool_call:a:0',kind:'tool_result',call_id:'bash',text:'<tag>\nsecond line'},
 ],{harness:'claude-code'});
 const toolResults=new Map([['bash',data.entries[1]]]);
 const html=renderTranscriptEntry(data.entries[0],{...renderers,toolResults});
 assert.match(html,/<pre class="tool-command">\$ printf/);
 assert.match(html,/<details class="amp-details"><summary>Arguments<\/summary>/);
 assert.match(html,/<details class="amp-details"><summary>Tool output<\/summary><pre>&lt;tag&gt;\nsecond line<\/pre>/);
 assert.doesNotMatch(html,/<details[^>]*open|<tag>|Message details/);
});

test('Claude empty reasoning is explained and Agent results appear once beneath their call',()=>{
 const data=buildTranscriptSession([
  {id:1,key:'reasoning:a:0',kind:'reasoning',role:'assistant',text:''},
  {id:2,key:'tool_call:b:0',parent_key:'reasoning:a:0',kind:'tool_call',role:'assistant',call_id:'agent',tool:'Agent',text:'{"description":"Review tests","prompt":"Check coverage"}'},
  {id:3,key:'tool_result:c:0',parent_key:'tool_call:b:0',kind:'tool_result',call_id:'agent',text:'Coverage report'},
 ],{harness:'claude-code'});
 const options={...renderers,toolResults:new Map([['agent',data.entries[2]]]),toolCalls:new Set(['agent'])};
 const reasoning=renderTranscriptEntry(data.entries[0],options);
 assert.match(reasoning,/<div class="thinking-text">Reasoning text not included in export\.<\/div>/);
 assert.match(reasoning,/<div class="thinking-collapsed">Thinking/);
 const call=renderTranscriptEntry(data.entries[1],options);
 assert.match(call,/Agent <span/);
 assert.match(call,/<summary>Arguments<\/summary>/);
 assert.match(call,/<summary>Tool output<\/summary>.*Coverage report/);
 assert.equal(renderTranscriptEntry(data.entries[2],options),'');
 assert.match(renderTranscriptEntry(data.entries[2],{...options,toolCalls:new Set()}),/Coverage report/);
});

test('Amp recorded tool states preserve failure and running precedence',()=>{
 for (const [result,complete,status] of [
  [{isError:true,run:{status:'done',result:{running:true}}},true,'failed'],
  [{run:{status:'done',result:{running:true}}},true,'running'],
  [{run:{status:'cancelled'}},true,'cancelled'],
  [{},false,'unknown'],
  [undefined,false,'incomplete arguments'],
  [undefined,true,'result not collected'],
 ]) {
  const entry={id:'1',message:{role:'assistant',content:[{type:'toolCall',id:'call',name:'tool',details:{complete}}]}};
  const toolResults=new Map(result ? [['call',{id:'2',message:{...result,role:'toolResult',content:[]}}]] : []);
  assert.ok(renderTranscriptEntry(entry,{...renderers,toolResults}).includes(`Recorded: ${status}`),status);
 }
});

test('Amp skill and subagent results stay paired and thread origins remain distinct',()=>{
 const events=[
  {id:1,key:'tool_call:1:0',kind:'tool_call',role:'assistant',call_id:'TU-skill',tool:'skill',text:'{"name":"tdd"}'},
  {id:2,key:'tool_call:1:1',kind:'tool_call',role:'assistant',call_id:'TU-task',tool:'Task',text:'{"description":"Review boundaries","prompt":"Inspect range.go; treat <script> as text."}'},
  {id:3,key:'tool_result:2:0',kind:'tool_result',call_id:'TU-task',tool:'Task',text:'Subagent answer: include 3, exclude 4.',metadata:{run:{status:'done',result:'Subagent answer: include 3, exclude 4.'}}},
  {id:4,key:'tool_result:2:1',kind:'tool_result',call_id:'TU-skill',tool:'skill',text:'Skill instructions: write a failing test.',metadata:{run:{status:'done',result:{content:[{type:'text',text:'Skill instructions: write a failing test.'}]}}}},
  {id:5,key:'tool_call:3:0',kind:'tool_call',role:'assistant',call_id:'TU-thread',tool:'create_thread',text:'{"prompt":"Review documentation"}'},
  {id:6,key:'tool_result:4:0',kind:'tool_result',call_id:'TU-thread',tool:'create_thread',text:'Thread created',metadata:{run:{status:'done',result:{threadID:'T-child'}}}},
  {id:7,key:'message:5:0',kind:'message',role:'user',text:'Documentation agrees.',metadata:{message_meta:{fromExecutorThreadID:'T-child',fromAutomation:true}}},
 ];
 const session=buildTranscriptSession(events,{harness:'amp',nativeId:'fixture',traceId:'1'});
 const toolResults=new Map(session.entries.filter(e=>e.message?.role==='toolResult').map(e=>[e.message.toolCallId,e]));
 const toolCalls=new Set(['TU-skill','TU-task','TU-thread']);
 const rendered=session.entries.map(e=>renderTranscriptEntry(e,{...renderers,toolResults,toolCalls}));
 const paired=rendered[0];
 const skill=paired.slice(paired.indexOf('id="tool-call-TU-skill"'),paired.indexOf('id="tool-call-TU-task"'));
 const task=paired.slice(paired.indexOf('id="tool-call-TU-task"'));
 assert.match(skill,/Skill<\/dt><dd>tdd/);
 assert.match(skill,/<details class="amp-details"><summary>Tool output<\/summary>/);
 assert.match(skill,/Skill instructions: write a failing test/);
 assert.doesNotMatch(skill,/Subagent answer/);
 assert.match(task,/Review boundaries/);
 assert.match(task,/Subagent answer: include 3, exclude 4/);
 assert.match(task,/&lt;script&gt;/);
 assert.doesNotMatch(task,/<script>|Skill instructions/);
 assert.ok(session.entries.filter(e=>e.message?.role==='toolResult').every(e=>renderTranscriptEntry(e,{...renderers,toolResults,toolCalls})===''));
 const html=rendered.join('');
 assert.equal((html.match(/id="tool-call-/g)||[]).length,3);
 assert.match(html,/Spawned thread:/);
 assert.match(html,/From thread:/);
 assert.match(html,/Automation message/);
 assert.match(html,/href="\/traces\/resolve\?native_id=T-child"/);
 assert.match(html,/href="https:\/\/ampcode.com\/threads\/T-child"/);
});

test('Amp downloaded images use local previews and keep large originals downloadable',()=>{
 const image={type:'attachment',name:'screenshot.png',media_type:'image/png',url:'https://ampcode.com/user-content/attachments/fixture.png',archived_path:'source/attachments/hash',size:10*1024*1024,revisionId:9,source_pointer:'/messages/0/content/0'};
 const render=image=>renderTranscriptEntry({id:'1',message:{role:'user',content:[image]}},renderers);
 const html=render(image);
 assert.match(html,/Archived image: screenshot.png/);
 assert.match(html,/<img[^>]*src="\/revisions\/9\/attachment\?pointer=/);
 assert.match(html,/\/revisions\/9\/file\?path=source%2Fattachments%2Fhash&download=1/);
 assert.doesNotMatch(html,/ampcode.com|External reference/);
 const large=render({...image,size:image.size+1});
 assert.match(large,/10 MiB preview limit/);
 assert.match(large,/Download image/);
 assert.doesNotMatch(large,/<img/);
});

test('Amp unfamiliar file and skill details remain inspectable without breaking the viewer',()=>{
 const entry={id:'1',message:{role:'toolResult',run:{status:'done',result:{files:[null,'future-file-shape']}},content:[]}};
 const html=renderTranscriptEntry(entry,renderers);
 assert.match(html,/future-file-shape/);
 assert.match(renderTranscriptHeaderDetails({native:{activatedSkills:{future:'skill-shape'}}},[],renderers),/skill-shape/);
});

test('Amp renders failed tool output, diffs and historical status without unsafe HTML',()=>{
 const result={id:'2',message:{role:'toolResult',toolCallId:'TU-1',isError:true,run:{status:'done',result:{exitCode:2,files:[{uri:'file:///repo/test.go',diff:'-old\n+<script>bad</script>',additions:1,deletions:1}]}},content:[{type:'text',text:'failure <img src=x>'}]},sources:[]};
 const entry={id:'1',message:{role:'assistant',content:[{type:'toolCall',id:'TU-1',name:'apply_patch',arguments:{patchText:'patch'}}]},sources:[]};
 const html=renderTranscriptEntry(entry,{...renderers,toolResults:new Map([['TU-1',result]])});
 assert.match(html,/Recorded: failed/);
 assert.match(html,/Exit code: 2/);
 assert.match(html,/diff-added/);
 assert.match(html,/&lt;script&gt;/);
 assert.doesNotMatch(html,/<script>|<img src=x>/);
});

test('Amp keeps external images and hidden context without per-message source panels',()=>{
 const entry={id:'1',message:{role:'user',nativeDetails:{message_meta:{fromExecutorThreadID:'T-child'}},content:[{type:'hidden',text:'hidden context'},{type:'attachment',url:'javascript:alert(1)',path:'unsafe.png'},{type:'attachment',url:'https://example.com/private.png',path:'private.png'}]},sources:[{revisionId:8,pointer:'/messages/0/content/1',eventId:1}]};
 const html=renderTranscriptEntry(entry,renderers);
 assert.match(html,/<details[^>]*><summary>Hidden context/);
 assert.match(html,/External reference/);
 assert.match(html,/From thread/);
 assert.doesNotMatch(html,/Message details &amp; source|\/revisions\/8\/block\?pointer=/);
 assert.doesNotMatch(html,/<img|javascript:|<details[^>]*open/);
});

test('Amp usage totals count messages once and missing usage remains unknown',()=>{
 const entries=[{message:{role:'assistant',usage:{input:12,output:7,cacheRead:100,cacheWrite:0},content:[{type:'thinking'},{type:'text'}]}},{message:{role:'assistant',usage:{input:5,output:11,cacheRead:0},content:[]}}];
 const html=renderTranscriptHeaderDetails({native:{agentMode:'high'}},entries,renderers);
 assert.match(html,/Input tokens<\/dt><dd>17/);
 assert.match(html,/Output tokens<\/dt><dd>18/);
 assert.match(html,/Cache writes<\/dt><dd>0 \(partial\)/);
 assert.doesNotMatch(html,/\$0/);
});

test('optional details do not invent Amp metadata for other harnesses',()=>{
 assert.equal(renderTranscriptHeaderDetails({harness:'pi'},[],renderers),'');
 const entry={id:'1',message:{role:'assistant',content:[{type:'toolCall',id:'call',name:'inspect_thread',arguments:{thread_id:'session-other'}}]}};
 const html=renderTranscriptEntry(entry,renderers);
 assert.match(html,/\/traces\/resolve\?native_id=session-other/);
 assert.doesNotMatch(html,/ampcode.com/);
});
