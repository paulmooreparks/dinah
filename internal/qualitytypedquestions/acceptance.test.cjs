'use strict';
const assert = require('node:assert/strict');
const { test } = require('node:test');
const path = require('node:path');
const options = [{optionId:'b',name:'Beta'}, {optionId:'a',name:'Alpha'}, {optionId:'c',name:'Gamma'}];
const text = (minLength=0,maxLength=4) => ({title:'Invented question',answer:{type:'text',minLength,maxLength}});
const multiple = (minSelections=0,maxSelections=3) => ({title:'Invented choices',answer:{type:'multiple',options:structuredClone(options),minSelections,maxSelections}});
async function fixture(t) {
  assert.ok(process.env.QUALITY_TYPED_QUESTION_BINDING, 'Typed-question acceptance is unbound');
  const f = await require(path.resolve(process.env.QUALITY_TYPED_QUESTION_BINDING)).open();
  t.after(() => f.close());
  return f;
}
async function pending(f, request) {
  const response = await f.ask(request);
  assert.equal(response.ok,true);
  const rows = await f.pending('notifications');
  const row = rows.find(q => q.key === response.key);
  assert.ok(row && row.key && row.owner && row.askedAt);
  assert.equal(row.text,request.title);
  return row;
}
async function reject(f,key,raw,bytes=false) {
  const before = await f.snapshot();
  const response = await (bytes ? f.answerBytes(key,raw) : f.answer(key,raw));
  assert.equal(response.ok,false);
  assert.ok(response.error);
  assert.deepEqual(await f.snapshot(),before);
}
async function quiet(f) {
  const e = await f.observations();
  assert.ok(e.artifactDirectory);
  assert.deepEqual(e.providerRequests,[]);
  assert.deepEqual(e.actionRequests,[]);
  assert.equal(e.remoteWriteAttempts,0);
  assert.deepEqual(e.protectedAfter,e.protectedBefore);
}
test('invalid declarations refuse before a pending question or external work',async t => {
  const f=await fixture(t);
  const invalid=[
    {title:'Q',choices:['Beta','Alpha','Gamma'],answer:text().answer},
    {title:'Q',answer:{type:'text'}},text(0,0),text(0,16385),text(-1,4),text(5,4),text(0,1.5),
    {title:'Q',answer:{...text().answer,options}},
    {title:'Q',answer:{...text().answer,minSelections:0}},
    {title:'Q',answer:{type:'unknown'}},
    {title:'Q',answer:{...multiple().answer,options:[]}},
    {title:'Q',answer:{...multiple().answer,options:Array.from({length:17},(_,i)=>({optionId:String(i),name:String(i)}))}},
    {title:'Q',answer:{...multiple().answer,options:[options[0],options[0]]}},
    {title:'Q',answer:{...multiple().answer,options:[{optionId:'',name:'X'}]}},
    {title:'Q',answer:{...multiple().answer,options:[{optionId:'x',name:''}]}},
    multiple(-1,2),multiple(2,1),multiple(0,0),multiple(0,4),multiple(0,1.5),
    {title:'Q',answer:{...multiple().answer,maxLength:4}}
  ];
  for(const request of invalid) {
    const before=await f.snapshot();
    const r=await f.ask(request);
    assert.equal(r.ok,false,JSON.stringify(request));
    assert.deepEqual(await f.snapshot(),before);
  }
  await quiet(f);
});
test('text counts Unicode scalars and preserves all submitted bytes',async t => {
  const f=await fixture(t);
  for(const value of ['', ' a\n', '😀😀😀😀', 'e\u0301', 'é']) {
    const q=await pending(f,text());
    assert.equal(q.answerType,'text');
    assert.equal(q.minLength,0);assert.equal(q.maxLength,4);
    assert.equal((await f.answer(q.key,JSON.stringify({type:'text',text:value}))).ok,true);
    assert.deepEqual(await f.result(q.key),{answer:{type:'text',text:value}});
  }
  const q=await pending(f,text(2,4));
  await reject(f,q.key,JSON.stringify({type:'text',text:'😀'}));
  await reject(f,q.key,JSON.stringify({type:'text',text:'😀😀😀😀😀'}));
  assert.equal((await f.answer(q.key,JSON.stringify({type:'text',text:'e\u0301'}))).ok,true);
  const big=await pending(f,text(16384,16384));
  const exact='😀'.repeat(16384);
  assert.equal((await f.answer(big.key,JSON.stringify({type:'text',text:exact}))).ok,true);
  assert.deepEqual(await f.result(big.key),{answer:{type:'text',text:exact}});
  await quiet(f);
});
test('strict answer shapes and invalid Unicode leave the same question pending',async t => {
  const f=await fixture(t),q=await pending(f,text());
  for(const raw of ['{}','null','[]','{"type":"text"}','{"type":"text","text":1}',
    '{"type":"multiple","optionIds":[]}','{"type":"text","text":"x","person":true}',
    '{"type":"text","text":"x","owner":"invented"}',
    '{"type":"text","text":"x","optionIds":[]}',
    '{"type":"text","text":"x"} {}','{"type":"text","text":"\\ud800"}',
    '{"type":"text","text":"\\udc00"}', ' '.repeat(131073)+'{}']) await reject(f,q.key,raw);
  await reject(f,q.key,[...Buffer.from('{"type":"text","text":"'),255,...Buffer.from('"}')],true);
  const body='{"type":"text","text":"ok"}';
  assert.equal((await f.answer(q.key,body+' '.repeat(131072-Buffer.byteLength(body)))).ok,true);
  await quiet(f);
});
test('multiple answers validate offered IDs and return offered order',async t => {
  const f=await fixture(t),q=await pending(f,multiple(1,2));
  assert.equal(q.answerType,'multiple');
  assert.deepEqual(q.options,options);
  assert.equal(q.minSelections,1);assert.equal(q.maxSelections,2);
  for(const value of [[],['b','a','c'],['b','b'],['foreign'],[1],null])
    await reject(f,q.key,JSON.stringify({type:'multiple',optionIds:value}));
  await reject(f,q.key,'{"type":"multiple"}');
  await reject(f,q.key,'{"type":"multiple","optionIds":["a"],"text":"x"}');
  assert.equal((await f.answer(q.key,'{"type":"multiple","optionIds":["c","b"]}')).ok,true);
  assert.deepEqual(await f.result(q.key),{answer:{type:'multiple',optionIds:['b','c']}});
  const empty=await pending(f,multiple());
  assert.equal((await f.answer(empty.key,'{"type":"multiple","optionIds":[]}')).ok,true);
  const sixteen=multiple(16,16);
  sixteen.answer.options=Array.from({length:16},(_,i)=>({optionId:'id'+i,name:'Name '+i}));
  const full=await pending(f,sixteen),ids=sixteen.answer.options.map(o=>o.optionId);
  assert.equal((await f.answer(full.key,JSON.stringify({type:'multiple',optionIds:[...ids].reverse()}))).ok,true);
  assert.deepEqual(await f.result(full.key),{answer:{type:'multiple',optionIds:ids}});
  await quiet(f);
});
test('legacy and typed submission paths remain disjoint',async t => {
  const f=await fixture(t),legacy=await pending(f,{title:'Legacy',choices:['Beta','Alpha','Gamma']});
  for(const field of ['answerType','minLength','maxLength','minSelections','maxSelections']) assert.equal(Object.hasOwn(legacy,field),false);
  await reject(f,legacy.key,'{"type":"multiple","optionIds":["b"]}');
  assert.equal((await f.answerLegacy(legacy.key,'Beta')).ok,true);
  assert.deepEqual(await f.result(legacy.key),{answer:'Beta'});
  const typed=await pending(f,text());
  const before=await f.snapshot();
  assert.equal((await f.answerLegacy(typed.key,'b')).ok,false);
  assert.deepEqual(await f.snapshot(),before);
  await quiet(f);
});
test('copied pending schemas cannot widen authority',async t => {
  const f=await fixture(t),q=await pending(f,multiple(1,1));
  await f.mutatePendingCopy(q.key,{options:[{optionId:'foreign',name:'Foreign'}],minSelections:0,maxSelections:3});
  await reject(f,q.key,'{"type":"multiple","optionIds":["foreign"]}');
  await reject(f,q.key,'{"type":"multiple","optionIds":[]}');
  await reject(f,q.key,'{"type":"multiple","optionIds":["a","b"]}');
  assert.deepEqual((await f.pending('decisions')).find(x=>x.key===q.key).options,options);
  await quiet(f);
});
test('views share one question and settle it once under concurrent answers',async t => {
  const f=await fixture(t),q=await pending(f,text());
  assert.deepEqual(await f.pending('decisions'),await f.pending('notifications'));
  const answers=await Promise.all(['one','two'].map(value=>f.answer(q.key,JSON.stringify({type:'text',text:value}))));
  assert.equal(answers.filter(x=>x.ok).length,1);
  assert.equal(answers.filter(x=>!x.ok).length,1);
  assert.equal((await f.pending('decisions')).some(x=>x.key===q.key),false);
  const e=await f.settlement(q.key);
  assert.equal(e.durableResults.length,1);assert.equal(e.answeredEvents.length,1);
  assert.ok(['one','two'].includes(e.durableResults[0].answer.text));
  await reject(f,q.key,'{"type":"text","text":"late"}');
  await quiet(f);
});
for(const kind of ['cancel','generation-change','removal','shutdown']) test(`${kind} withdraws typed questions without replay`,async t=>{
  const f=await fixture(t),q=await pending(f,text());
  await f.withdraw(kind,q.key);
  assert.equal((await f.pending('notifications')).some(x=>x.key===q.key),false);
  await reject(f,q.key,'{"type":"text","text":"late"}');
  const e=await f.settlement(q.key);
  assert.equal(e.answeredEvents.length,1);
  assert.equal(e.durableResults.filter(x=>x.answer).length,0);
  await f.restart();
  assert.equal((await f.pending('decisions')).some(x=>x.key===q.key),false);
  assert.equal((await f.observations()).questionReplayCount,0);
  await quiet(f);
});
test('service calls cannot answer and repeated asking has another key',async t=>{
  const f=await fixture(t),q=await pending(f,text());
  for(const name of ['AnswerTaskQuestion','Answer','flows.Answer']) {
    const before=await f.snapshot();
    assert.equal((await f.callService(name,{key:q.key,answer:{type:'text',text:'bad'},person:true})).ok,false);
    assert.deepEqual(await f.snapshot(),before);
  }
  assert.equal((await f.answer(q.key,'{"type":"text","text":"yes"}')).ok,true);
  const next=await pending(f,text());assert.notEqual(next.key,q.key);
  await reject(f,q.key,'{"type":"text","text":"late"}');
  await quiet(f);
});
