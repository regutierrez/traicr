import {test} from 'node:test';
import assert from 'node:assert/strict';
import {buildTranscriptSession,loadTranscriptSession} from './transcript-data.js';

const metadata={nativeId:'session',harness:'pi',title:'A whole conversation',createdAt:'2026-09-06T10:00:00Z',traceId:'1'};
test('Amp asks for reload without declaring completion when history changes between pages',async()=>{
 const originalFetch=globalThis.fetch;const originalDocument=globalThis.document;
 const status={textContent:''};let calls=0;
 globalThis.document={getElementById:()=>status};
 globalThis.fetch=async()=>++calls===1
  ? new Response(JSON.stringify({events:[{id:1,key:'message:1:0',kind:'message',text:'Before rebuild'}],next_cursor:'old'}),{headers:{'content-type':'application/json'}})
  : new Response(JSON.stringify({error:{code:'transcript_changed'}}),{status:409,headers:{'content-type':'application/json'}});
 try {
  const data=await loadTranscriptSession({...metadata,harness:'amp'});
  await assert.rejects(data.loadMore(),/Transcript changed.*Reload/);
  assert.equal(data.hasMore,true);
  assert.equal(data.entries[0].message.content[0].text,'Before rebuild');
  assert.ok(!status.textContent.includes('All records loaded'));
 } finally {globalThis.fetch=originalFetch;globalThis.document=originalDocument;}
});
test('all harnesses preserve attachment fields for the shared renderer',()=>{
 for(const harness of ['amp','pi','claude-code','opencode']) {
  const data=buildTranscriptSession([{id:1,key:'attachment:a:0',kind:'attachment',role:'user',revision_id:7,attachments:[{path:'image.png',url:'https://example.com/image.png'}]}],{...metadata,harness});
  assert.deepEqual(data.entries[0].message.content,[{type:'attachment',path:'image.png',url:'https://example.com/image.png',revisionId:7}]);
 }
});
test('splitting an Amp message around unknown blocks does not duplicate its usage',()=>{
 const events=['message','unknown','message'].map((kind,index)=>({id:index+1,key:`${kind}:1:${index}`,kind,role:'assistant',text:'content',metadata:{transcript_message:'amp:1',transcript_order:0,transcript_block:index,usage:{inputTokens:13,outputTokens:7}}}));
 const data=buildTranscriptSession(events,{...metadata,harness:'amp'});
 assert.equal(data.entries.reduce((total,entry)=>total+(entry.message?.usage?.input || 0),0),13);
});
test('Amp preserves completion reasons and gives cancellation and errors precedence',()=>{
 for (const [state,reason] of [
  [{type:'cancelled',stopReason:'end_turn'},'aborted'],
  [{type:'error',stopReason:'end_turn'},'error'],
  [{type:'complete',stopReason:'max_tokens'},'max_tokens'],
  [{type:'complete',stopReason:'tool_use'},'toolUse'],
  [{type:'complete',stopReason:'end_turn'},'stop'],
  [{type:'complete'},'complete'],
  [undefined,undefined],
 ]) {
  const data=buildTranscriptSession([{id:1,key:'message:1:0',kind:'message',role:'assistant',text:'answer',metadata:{state}}],{...metadata,harness:'amp'});
  assert.equal(data.entries[0].message.stopReason,reason);
 }
});
test('Amp loads incrementally, keeps the revision and joins messages across pages',async()=>{
 const originalFetch=globalThis.fetch;const originalDocument=globalThis.document;
 let calls=0;
 globalThis.document={getElementById:()=>({textContent:''})};
 globalThis.fetch=async(url)=>{
  calls++;
  assert.ok(url.startsWith('/traces/1/transcript?'));
  assert.equal(new URL(url,'https://example.com').searchParams.get('revision'),'7');
  return {ok:true,headers:new Headers({'content-type':'application/json'}),json:async()=>({events:[{id:calls,key:`message:m:${calls}`,kind:'message',role:'assistant',text:`Block ${calls}`,metadata:{transcript_message:'m',transcript_order:0,transcript_block:calls}}],next_cursor:calls===1?'second-page':''})};
 };
 try {
  const data=await loadTranscriptSession({...metadata,harness:'amp',revision:'7'});
  assert.equal(calls,1);assert.equal(data.hasMore,true);
  await data.loadMore();
  assert.equal(data.hasMore,false);assert.equal(calls,2);
  assert.equal(data.entries.length,1);assert.equal(data.entries[0].message.content.length,2);
 } finally {globalThis.fetch=originalFetch;globalThis.document=originalDocument;}
});
test('retains Amp details once per message, hidden context, attachments and execution states',()=>{
 const events=[
  {id:1,key:'session_info:s',kind:'session_info',metadata:{session:{title:'Native title',agentMode:'high'},transcript_order:-1}},
  {id:2,key:'reasoning:1:0',kind:'reasoning',role:'assistant',text:'Think',model:'model-a',metadata:{transcript_message:'amp:1',transcript_order:1,transcript_block:0,usage:{inputTokens:12,outputTokens:7,cacheReadInputTokens:100},state:{type:'cancelled'},message_meta:{openAIResponsePhase:'commentary'}}},
  {id:3,key:'message:1:1',kind:'message',role:'assistant',text:'Hidden instructions',metadata:{transcript_message:'amp:1',transcript_order:1,transcript_block:1,hidden:true,usage:{inputTokens:12,outputTokens:7,cacheReadInputTokens:100}}},
  {id:4,key:'tool_result:2:0',kind:'tool_result',call_id:'TU-a',tool:'shell_command',text:'failed',revision_id:8,aliases:['old-hash','94'],metadata:{transcript_message:'amp:2',transcript_order:2,run:{status:'done',result:{exitCode:2,output:'failed'}},source_pointer:'/messages/2/content/0'},attachments:[{url:'https://example.com/picture.png',path:'picture.png'}]},
 ];
 const data=buildTranscriptSession(events,{...metadata,harness:'amp'});
 const assistant=data.entries.find(e=>e.message?.role==='assistant').message;
 assert.equal(assistant.stopReason,'aborted');
 assert.equal(assistant.usage.output,7);
 assert.equal(assistant.content[1].type,'hidden');
 const result=data.entries.find(e=>e.message?.role==='toolResult');
 assert.equal(result.message.isError,true);
 assert.equal(result.message.content[1].type,'attachment');
 assert.equal(result.sources[0].revisionId,8);
 assert.equal(data.header.native.agentMode,'high');
 assert.equal(data.eventEntries.get('94'),result.id);
});
test('groups message blocks and preserves branches and tool results',()=>{
 const events=[
  {id:1,key:'message:u:0',kind:'message',role:'user',text:'Build this',timestamp:'2026-09-06T10:00:00Z'},
  {id:2,key:'reasoning:a:0',parent_key:'message:u:0',kind:'reasoning',role:'assistant',text:'Think first',timestamp:'2026-09-06T10:01:00Z'},
  {id:3,key:'message:a:1',parent_key:'message:u:0',kind:'message',role:'assistant',text:'I will inspect it',timestamp:'2026-09-06T10:01:00Z'},
  {id:4,key:'tool_call:a:2',parent_key:'message:u:0',kind:'tool_call',role:'assistant',tool:'bash',call_id:'call-1',text:'{"command":"pwd"}',timestamp:'2026-09-06T10:01:00Z'},
  {id:5,key:'tool_result:r:0',parent_key:'reasoning:a:0',kind:'tool_result',call_id:'call-1',tool:'bash',text:'/repo',timestamp:'2026-09-06T10:02:00Z'},
  {id:6,key:'message:branch:0',parent_key:'message:u:0',kind:'message',role:'user',text:'Another path',timestamp:'2026-09-06T10:03:00Z'},
 ];
 const data=buildTranscriptSession(events,metadata);
 assert.equal(data.entries.length,4);
 assert.equal(data.header.harness,'pi');
 assert.deepEqual(data.entries[1].message.content.map(b=>b.type),['thinking','text','toolCall']);
 assert.equal(data.entries[1].parentId,'1');
 assert.equal(data.entries[2].parentId,'2');
 assert.equal(data.entries[3].parentId,'1');
 assert.equal(data.entries[2].message.toolCallId,'call-1');
 assert.equal(data.eventEntries.get('4'),'2');
});
test('uses native Amp message order even when timestamps and IDs are absent',()=>{
 const events=[
  {id:2,key:'reasoning:sha256:abc',kind:'reasoning',role:'assistant',text:'Think',metadata:{transcript_message:'amp:a',transcript_order:1,transcript_block:0}},
  {id:3,key:'message:sha256:def',kind:'message',role:'assistant',text:'Reply',metadata:{transcript_message:'amp:a',transcript_order:1,transcript_block:1}},
  {id:1,key:'message:sha256:ghi',kind:'message',role:'user',text:'Prompt',timestamp:'2026-09-06T10:00:00Z',metadata:{transcript_message:'amp:u',transcript_order:0,transcript_block:0}},
 ];
 const data=buildTranscriptSession(events,metadata);
 assert.equal(data.entries.length,2);
 assert.equal(data.entries[0].message.role,'user');
 assert.deepEqual(data.entries[1].message.content.map(b=>b.type),['thinking','text']);
 assert.equal(data.header.timestamp,'2026-09-06T10:00:00Z');
});
test('retains explicit Amp branch parents instead of flattening them',()=>{
 const data=buildTranscriptSession([
  {id:1,key:'message:u:0',kind:'message',role:'user',text:'Start',metadata:{transcript_message:'amp-message:u',transcript_order:0,transcript_block:0}},
  {id:2,key:'message:a:0',kind:'message',role:'assistant',text:'First branch',metadata:{transcript_message:'amp-message:a',transcript_parent:'amp-message:u',transcript_order:1,transcript_block:0}},
  {id:3,key:'message:b:0',kind:'message',role:'assistant',text:'Second branch',metadata:{transcript_message:'amp-message:b',transcript_parent:'amp-message:u',transcript_order:2,transcript_block:0}},
 ],metadata);
 assert.equal(data.entries[1].parentId,'1');assert.equal(data.entries[2].parentId,'1');
});
test('cycles cannot trap the viewer parent traversal',()=>{
 const data=buildTranscriptSession([
  {id:1,key:'message:a:0',parent_key:'message:b:0',kind:'message',role:'user',text:'a'},
  {id:2,key:'message:b:0',parent_key:'message:a:0',kind:'message',role:'user',text:'b'},
 ],metadata);
 assert.ok(data.entries.some(e=>e.parentId===null));
});
test('loads every page rather than rendering only the first 200 records',async()=>{
 const originalFetch=globalThis.fetch;const originalDocument=globalThis.document;
 let calls=0;
 globalThis.document={getElementById:()=>({textContent:''})};
 globalThis.fetch=async(url)=>{
  calls++;
  assert.ok(url.startsWith('/traces/1/events?'));
  return {ok:true,headers:new Headers({'content-type':'application/json'}),json:async()=>({events:[{id:calls,key:`message:m${calls}:0`,kind:'message',role:'user',text:`Page ${calls}`}],next_cursor:calls===1?'second-page':''})};
 };
 try {const data=await loadTranscriptSession(metadata);assert.equal(calls,2);assert.equal(data.entries.length,2);} finally {globalThis.fetch=originalFetch;globalThis.document=originalDocument;}
});
