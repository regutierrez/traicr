import {test} from 'node:test';
import assert from 'node:assert/strict';
import {buildTranscriptSession,loadTranscriptSession} from './transcript-data.js';

const metadata={nativeId:'session',harness:'pi',title:'A whole conversation',createdAt:'2026-09-06T10:00:00Z',traceId:'1'};
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
