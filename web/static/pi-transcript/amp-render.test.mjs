import {test} from 'node:test';
import assert from 'node:assert/strict';
import {renderAmpEntry,renderAmpHeaderDetails} from './amp-render.js';
import {buildTranscriptSession} from './transcript-data.js';

const escape = value => String(value ?? '').replace(/[&<>"']/g, c => ({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));
const renderers = {escape,markdown:escape,copyLink:()=>'',toolResults:new Map()};

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
  assert.ok(renderAmpEntry(entry,{...renderers,toolResults}).includes(`Recorded: ${status}`),status);
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
 const rendered=session.entries.map(e=>renderAmpEntry(e,{...renderers,toolResults,toolCalls}));
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
 assert.ok(session.entries.filter(e=>e.message?.role==='toolResult').every(e=>renderAmpEntry(e,{...renderers,toolResults,toolCalls})===''));
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
 const render=image=>renderAmpEntry({id:'1',message:{role:'user',content:[image]}},renderers);
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
 const html=renderAmpEntry(entry,renderers);
 assert.match(html,/future-file-shape/);
 assert.match(renderAmpHeaderDetails({native:{activatedSkills:{future:'skill-shape'}}},[],renderers),/skill-shape/);
});

test('Amp renders failed tool output, diffs and historical status without unsafe HTML',()=>{
 const result={id:'2',message:{role:'toolResult',toolCallId:'TU-1',isError:true,run:{status:'done',result:{exitCode:2,files:[{uri:'file:///repo/test.go',diff:'-old\n+<script>bad</script>',additions:1,deletions:1}]}},content:[{type:'text',text:'failure <img src=x>'}]},sources:[]};
 const entry={id:'1',message:{role:'assistant',content:[{type:'toolCall',id:'TU-1',name:'apply_patch',arguments:{patchText:'patch'}}]},sources:[]};
 const html=renderAmpEntry(entry,{...renderers,toolResults:new Map([['TU-1',result]])});
 assert.match(html,/Recorded: failed/);
 assert.match(html,/Exit code: 2/);
 assert.match(html,/diff-added/);
 assert.match(html,/&lt;script&gt;/);
 assert.doesNotMatch(html,/<script>|<img src=x>/);
});

test('Amp keeps external images explicit, hidden context collapsed and source blocks accessible',()=>{
 const entry={id:'1',message:{role:'user',nativeDetails:{message_meta:{fromExecutorThreadID:'T-child'}},content:[{type:'hidden',text:'hidden context'},{type:'attachment',url:'javascript:alert(1)',path:'unsafe.png'},{type:'attachment',url:'https://example.com/private.png',path:'private.png'}]},sources:[{revisionId:8,pointer:'/messages/0/content/1',eventId:1}]};
 const html=renderAmpEntry(entry,renderers);
 assert.match(html,/<details[^>]*><summary>Hidden context/);
 assert.match(html,/External reference/);
 assert.match(html,/From thread/);
 assert.match(html,/\/revisions\/8\/block\?pointer=/);
 assert.doesNotMatch(html,/<img|javascript:|<details[^>]*open/);
});

test('Amp usage totals count messages once and missing usage remains unknown',()=>{
 const entries=[{message:{role:'assistant',usage:{input:12,output:7,cacheRead:100,cacheWrite:0},content:[{type:'thinking'},{type:'text'}]}},{message:{role:'assistant',usage:{input:5,output:11,cacheRead:0},content:[]}}];
 const html=renderAmpHeaderDetails({native:{agentMode:'high'}},entries,renderers);
 assert.match(html,/Input tokens<\/dt><dd>17/);
 assert.match(html,/Output tokens<\/dt><dd>18/);
 assert.match(html,/Cache writes<\/dt><dd>0 \(partial\)/);
 assert.doesNotMatch(html,/\$0/);
});
