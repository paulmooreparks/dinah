// Transcript commands own keys only when the transcript itself has focus.
const {launch} = require('./lib/page');
(async()=>{
 const t=await launch({viewport:{width:1440,height:1000}}),{page:p,ok}=t;
 try{
  const key=await p.evaluate(()=>dinahWorkbench.open({kind:'chat-session',workspace:'C:/Fixture',path:'keyboard-fixture'}));
  await p.evaluate(key=>{
   const root=document.querySelector('[data-win="'+key+'"]');
   const log=document.createElement('div');log.id='keyboard-transcript';log.style.cssText='position:fixed;left:400px;top:200px;width:400px;height:250px;overflow:auto;z-index:10000;background:var(--surface)';
   const content=log.appendChild(document.createElement('div'));
   const spacer=content.appendChild(document.createElement('div'));spacer.style.height='1600px';
   const markup='<input id="nested-input" value="abcdef"><textarea id="nested-textarea">abcdef</textarea><select id="nested-select"><option>a</option><option>b</option></select><div id="nested-empty-editable" contenteditable>abcdef</div><div id="nested-plain-editable" contenteditable="plaintext-only">abcdef</div><button id="nested-button">Button</button><a id="nested-link" href="#">Link</a>';
   const controls=content.appendChild(document.createElement('div'));controls.innerHTML=markup;
   root.appendChild(log);
   window.__keyboardNavigation=dinahTranscript.mount({root,log,content,entries:()=>[]});
   window.__keyboardEvents=[];
   log.addEventListener('keydown',e=>__keyboardEvents.push({target:e.target.id,key:e.key,ctrl:e.ctrlKey,meta:e.metaKey,shift:e.shiftKey,prevented:e.defaultPrevented}));
  },key);
  const log=p.locator('#keyboard-transcript');
  const frames=()=>p.evaluate(()=>new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(r))));
  for(const modifier of ['Control','Meta']){
   await log.focus();await p.evaluate(()=>__keyboardNavigation.scroll(true));await p.keyboard.press(modifier+'+Home');await frames();
   ok(modifier+' Home starts at the beginning without recapturing following',await log.evaluate(el=>el.scrollTop===0&&el.dataset.following==='false'));
   await p.evaluate(()=>{document.querySelector('#keyboard-transcript > div > div').style.height='1900px';});
   await p.keyboard.press(modifier+'+End');await frames();
   ok(modifier+' End follows the current extent',await log.evaluate(el=>el.dataset.following==='true'&&el.scrollHeight-el.scrollTop-el.clientHeight<2));
  }
  await log.focus();await p.keyboard.press('Control+Home');await p.keyboard.press('PageDown');
  ok('PageDown moves a focused transcript by its viewport',await log.evaluate(el=>Math.abs(el.scrollTop-el.clientHeight)<2));
  await p.keyboard.press('Control+End');await p.keyboard.press('Control+Home');
  const rapid=[];for(let i=0;i<12;i++){await frames();rapid.push(await log.evaluate(el=>el.scrollTop));}
  ok('an earlier page command cannot move a later Home position',rapid.every(top=>top===0),JSON.stringify(rapid));
  for(const id of ['nested-input','nested-textarea','nested-select','nested-empty-editable','nested-plain-editable','nested-button','nested-link']){
   await p.locator('#'+id).focus();await p.evaluate(()=>__keyboardNavigation.scroll(true));
   await p.keyboard.press('Control+Home');await frames();
   const event=await p.evaluate(()=>__keyboardEvents.filter(e=>e.key==='Home').at(-1));
   ok(id+' retains its native shortcut ownership',event.target===id&&!event.prevented,JSON.stringify(event));
  }
  await log.focus();await p.evaluate(()=>{
   __keyboardNavigation.scroll(true);
   const log=document.getElementById('keyboard-transcript');
   log.addEventListener('keydown',function handled(e){if(e.key==='Home'){e.preventDefault();log.removeEventListener('keydown',handled,true);}},true);
  });
  await p.keyboard.press('Control+Home');await frames();
  ok('an already-handled log key cannot change transcript following',await log.evaluate(el=>el.dataset.following==='true'&&el.scrollHeight-el.scrollTop-el.clientHeight<2));
  await p.keyboard.press('Control+Shift+Home');
  const selected=await p.evaluate(()=>__keyboardEvents.filter(e=>e.key==='Home').at(-1));
  ok('selection shortcuts retain the browser default',selected.shift&&!selected.prevented,JSON.stringify(selected));
  await t.shot('transcript-keyboard-boundary');
 }finally{await p.evaluate(()=>__keyboardNavigation?.destroy());await t.close();}
})().catch(error=>{console.error(error);process.exitCode=1;});
