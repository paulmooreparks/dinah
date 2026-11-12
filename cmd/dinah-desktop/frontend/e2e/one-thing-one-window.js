// One thing, one window: a kind that says what its items are the same thing
// by has one window and one pinned button for a thing, however the item was
// spelled, and a pinned button goes with its window to a new key.
const {launch}=require('./lib/page');
(async()=>{const t=await launch({app:()=>({ReadUIState:async()=>JSON.parse(sessionStorage.getItem('one-thing-state')||'{"version":1,"values":{}}'),SaveUIState:async(value)=>sessionStorage.setItem('one-thing-state',JSON.stringify(value))})}),p=t.page;try{
await p.waitForFunction(()=>window.dinahTaskbarPins && pudlWindows.taskbarState().references);
await p.evaluate(()=>{
 // A thing is its path in its folder, the folder compared without its case, as a chat is.
 dinahWorkbench.register('same-fixture',{title:'Same thing',glyph:'file',identity:item=>JSON.stringify([item.path,String(item.workspace||'').toLowerCase()]),init:()=>({ready:Promise.resolve()})});
 dinahWorkbench.register('plain-fixture',{title:'Plain thing',glyph:'file',init:()=>({ready:Promise.resolve()})});
});
const BS=String.fromCharCode(92), small='c:'+BS+'Work', capital='C:'+BS+'Work';
const keys=await p.evaluate(async([small,capital])=>{
 const a=await dinahWorkbench.open({kind:'same-fixture',workspace:small,path:'s1'});
 const b=await dinahWorkbench.open({kind:'same-fixture',workspace:capital,path:'s1'});
 const other=await dinahWorkbench.open({kind:'same-fixture',workspace:capital,path:'s2'});
 const moved=dinahWorkbench.key({kind:'same-fixture',workspace:'C:'+String.fromCharCode(92)+'Elsewhere',path:'s1'});
 const p1=await dinahWorkbench.open({kind:'plain-fixture',workspace:small,path:'s1'});
 const p2=await dinahWorkbench.open({kind:'plain-fixture',workspace:capital,path:'s1'});
 return {a,b,other,moved,p1,p2,key:dinahWorkbench.key({kind:'same-fixture',workspace:capital,path:'s1'}),windows:document.querySelectorAll('[data-win-kind="same-fixture"]').length};
},[small,capital]);
t.ok('two spellings of one thing open one window',keys.a===keys.b&&keys.key===keys.a&&keys.windows===2,JSON.stringify(keys));
t.ok('another thing, and the same thing in another folder, are other windows',keys.other!==keys.a&&keys.moved!==keys.a);
t.ok('a kind that says nothing keeps one window for each spelling',keys.p1!==keys.p2);
// The pinned button of a closed window is found by the other spelling too.
await p.evaluate(async(a)=>{await pudlWindows.refreshTaskbarDestinations();pudlWindows.pinTaskbar(a,true);pudlWindows.close(a);},keys.a);
await p.waitForFunction((a)=>!pudlWindows.state().open.includes(a)&&pudlWindows.taskbarState().pins.includes(a),keys.a);
const again=await p.evaluate(async([capital])=>({key:dinahWorkbench.key({kind:'same-fixture',workspace:capital,path:'s1'}),opened:await dinahWorkbench.open({kind:'same-fixture',workspace:capital,path:'s1'}),ref:dinahWorkbench.pinReference({kind:'same-fixture',workspace:capital,path:'s1'}).id}),[capital]);
t.ok('the other spelling opens the pinned button\'s own window',again.key===keys.a&&again.opened===keys.a&&again.ref===keys.a,JSON.stringify(again));
t.ok('the thing has one pinned button',await p.evaluate(()=>pudlWindows.taskbarState().pins.filter(id=>{const i=dinahWorkbench.item(id);return i&&i.kind==='same-fixture';}).length===1));
// A window given a new key takes its pinned button with it.
const next=await p.evaluate(async([a,capital])=>{const key=dinahWorkbench.rekey(a,{kind:'same-fixture',workspace:capital,path:'s1-continued'});await new Promise(r=>setTimeout(r,100));const s=pudlWindows.taskbarState();return {key,pins:s.pins,order:s.order,references:s.references.map(r=>r.id)};},[keys.a,capital]);
t.ok('a window given a new key keeps its pinned button',next.key!==keys.a&&next.pins.includes(next.key)&&!next.pins.includes(keys.a)&&next.order.includes('pin:'+next.key)&&!next.order.includes('pin:'+keys.a)&&next.references.includes(next.key),JSON.stringify(next));
// A pinned button moved onto a thing that is pinned already does not make two.
const merged=await p.evaluate(async([other,key])=>{await pudlWindows.refreshTaskbarDestinations();pudlWindows.pinTaskbar(other,true);dinahTaskbarPins.move(other,key);return pudlWindows.taskbarState().pins;},[keys.other,next.key]);
t.ok('moving a pinned button onto a pinned thing leaves one',merged.filter(id=>id===next.key).length===1&&!merged.includes(keys.other),JSON.stringify(merged));
await p.evaluate(()=>dinahUIState.flush());
t.ok('the saved pins are the moved ones',await p.evaluate(([key,a])=>{const s=dinahUIState.get('dinah-taskbar-pins-v1');return s.pins.includes(key)&&!s.pins.includes(a);},[next.key,keys.a]));
}finally{await t.close()}})();
