const {start,settle,geometry}=require('./lib/updates-fixture');
const notes=['## Changes','A **clear** explanation.','- First change','- Second change','','```powershell','command '+ 'long-argument-'.repeat(70)+'TAIL','```','','| Name | Value |','| --- | --- |','| Example | '+ 'wide-value-'.repeat(70)+' |','','[Relative](../CHANGELOG.md) [File](file:///C:/example.txt) [Web](https://example.test/notes) [Unsafe](javascript:alert(1))','','<img src=x onerror=alert(1)>'].join('\n');
async function check(t,selector,label){
 const result=await t.page.locator(selector).evaluate(root=>{
  const pre=root.querySelector('pre'); pre.focus(); pre.scrollLeft=pre.scrollWidth;
  const table=root.querySelector('.msg-table'); table.scrollLeft=table.scrollWidth;
  return {heading:!!root.querySelector('h2'),strong:!!root.querySelector('strong'),list:root.querySelectorAll('li').length,
   local:root.querySelectorAll('[data-chip-local],[data-chip-file]').length,web:root.querySelector('a')?.getAttribute('href'),
   unsafe:root.querySelectorAll('img,script,[onerror],a[href^="javascript:"]').length,
   code:pre.scrollWidth>pre.clientWidth&&pre.scrollLeft>0&&getComputedStyle(pre).overflowX==='auto',
   table:table.scrollWidth<=table.clientWidth||table.scrollLeft>0,focus:document.activeElement===pre,rootWidth:root.scrollWidth<=root.clientWidth+1};
 });
 t.ok(label+' renders Markdown',result.heading&&result.strong&&result.list===2);
 t.ok(label+' leaves relative and file labels as text',result.local===0&&result.web==='https://example.test/notes');
 t.ok(label+' keeps unsafe content inert',result.unsafe===0);
 t.ok(label+' code scrolls and tables fit or scroll inside the panel',result.code&&result.table&&result.rootWidth,JSON.stringify(result));
 t.ok(label+' code region accepts keyboard focus',result.focus);
}
async function run(){
 const t=await start({release:{current:'2.0.0',latest:'2.0.1',available:true,notes}});
 await settle(t);
 try {
  await t.page.evaluate(()=>pudlWindows.snap('settings','maximized'));
  for(const width of [1440,390])for(const mode of ['light','dark']){
   await t.page.setViewportSize({width,height:width===390?844:1000});
   await t.page.evaluate(mode=>pudlSetTheme(mode),mode);
   await check(t,'#updates-notes',mode+' '+width);
   await t.shot('release-notes-'+mode+'-'+width);
  }
  const before=await geometry(t);
  t.ok('unchanged refresh preserves nodes, scroll and focus',await t.page.evaluate(()=>{
   const root=document.querySelector('#updates-notes'),node=root.firstChild,pre=root.querySelector('pre');
   pre.focus();pre.scrollLeft=90;root.scrollTop=30;
   dinahReleaseNotes.set(root,__u.release.notes);
   return root.firstChild===node&&pre.scrollLeft===90&&root.scrollTop===30&&document.activeElement===pre;
  }));
  await t.page.evaluate(()=>dinahReleaseNotes.set(document.querySelector('#updates-notes'),'## Another change\nShort notes.'));
  t.ok('changed notes keep surrounding geometry',JSON.stringify(before)===JSON.stringify(await geometry(t)));
  await t.page.evaluate(()=>dinahReleaseNotes.set(document.querySelector('#updates-notes'),'[Web](https://example.test/notes)'));
  await t.page.locator('#updates-notes a').click();
  t.ok('web link opens through the existing browser action',await t.page.evaluate(()=>__runtime.some(c=>c[0]==='BrowserOpenURL'&&c[1]==='https://example.test/notes')));
  t.ok('reading notes performs no download or restart',!(await t.page.evaluate(()=>__updateCalls)).some(c=>/^(Download|Restart|RestartToUpdate)$/.test(c[0])));
 } finally {await t.close();}
}
module.exports={notes,check};
if(require.main===module)run().catch(error=>{console.error(error);process.exitCode=1;});
