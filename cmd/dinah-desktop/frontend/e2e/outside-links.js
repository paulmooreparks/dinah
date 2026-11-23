// A link to another site never replaces the program's own page. It opens in
// the person's browser, whichever part of the page it is in and whichever
// keys are down, unless a product registered an opener for its address
// (docs/design/product-applets.md, "A Product's Opener for a Web Address").
// An opener takes a plain press, and the browser keeps a press with Shift,
// Control or the middle button.
const {launch}=require('./lib/page');
(async()=>{
 const t=await launch({viewport:{width:1200,height:800},ready:'workbench'});
 const {page,ok}=t;
 try {
  await page.route('https://elsewhere.example/**',route=>route.abort());
  const start=page.url();
  await page.evaluate(()=>{
   window.__opened=[];
   window.runtime=Object.assign(window.runtime||{},{BrowserOpenURL:function(u){window.__opened.push(u);}});
   const box=document.createElement('div');
   box.id='outside-links';
   box.style.cssText='position:fixed;left:8px;top:8px;z-index:99999;background:#fff;padding:8px';
   box.innerHTML='<a id="ol-plain" href="https://elsewhere.example/a">a</a> '+
    '<a id="ol-blank" target="_blank" href="https://elsewhere.example/b">b</a> '+
    '<a id="ol-handled" href="https://elsewhere.example/c">c</a> '+
    '<a id="ol-fragment" href="#ol-plain">d</a> '+
    '<a id="ol-download" download="x.txt" href="https://elsewhere.example/e">e</a>';
   document.body.appendChild(box);
   document.getElementById('ol-handled').addEventListener('click',e=>e.preventDefault());
   document.getElementById('ol-download').addEventListener('click',e=>{window.__download=!e.defaultPrevented;e.preventDefault();},true);
  });
  const opened=()=>page.evaluate(()=>window.__opened.splice(0));
  await page.click('#ol-plain');
  ok('a plain press opens the link in the browser',JSON.stringify(await opened())==='["https://elsewhere.example/a"]');
  ok('the page is still the program\'s own',page.url().split('#')[0]===start.split('#')[0]);
  await page.click('#ol-plain',{modifiers:['Control']});
  ok('a press with Control opens the link in the browser',JSON.stringify(await opened())==='["https://elsewhere.example/a"]');
  await page.click('#ol-plain',{modifiers:['Shift']});
  ok('a press with Shift opens the link in the browser',JSON.stringify(await opened())==='["https://elsewhere.example/a"]');
  await page.click('#ol-plain',{button:'middle'});
  ok('a press of the middle button opens the link in the browser',JSON.stringify(await opened())==='["https://elsewhere.example/a"]');
  await page.click('#ol-blank');
  ok('a link meant for a new window opens in the browser',JSON.stringify(await opened())==='["https://elsewhere.example/b"]');
  await page.click('#ol-handled');
  ok('a link its own part handles is left to that part',(await opened()).length===0);
  await page.click('#ol-fragment');
  ok('a link within the page is left alone',(await opened()).length===0);
  await page.click('#ol-download');
  ok('a link that saves a file is left alone',(await opened()).length===0&&await page.evaluate(()=>window.__download===true));

  // A product's opener (dinahLinks, helpers.js) takes a plain press on an
  // address it matches, and the browser keeps every other press.
  await page.route('https://tracker.example/**',route=>route.abort());
  await page.evaluate(()=>{
   window.__taken=[];
   document.getElementById('outside-links').insertAdjacentHTML('beforeend',' <a id="ol-issue" href="https://tracker.example/issues/7">f</a> <a id="ol-other" href="https://tracker.example/wiki/7">g</a>');
   window.__unregisterFirst=dinahLinks.registerOpener({id:'sample-issues',title:'Open in Issue Window',glyph:'ticket',
    match:url=>url.hostname==='tracker.example'&&url.pathname.startsWith('/issues/'),
    open:(href,event)=>{__taken.push(['first',href,event?event.type:'none']);}});
  });
  const taken=()=>page.evaluate(()=>window.__taken.splice(0));
  await page.click('#ol-issue');
  ok('a plain press on an address an opener matches goes to the opener',JSON.stringify(await taken())==='[["first","https://tracker.example/issues/7","click"]]'&&(await opened()).length===0);
  for(const [how,opts] of [['Control',{modifiers:['Control']}],['Shift',{modifiers:['Shift']}],['the middle button',{button:'middle'}]]){
   await page.click('#ol-issue',opts);
   ok('a press with '+how+' on such an address opens the browser',(await taken()).length===0&&JSON.stringify(await opened())==='["https://tracker.example/issues/7"]');
  }
  await page.click('#ol-other');
  ok('an address no opener matches opens the browser',(await taken()).length===0&&JSON.stringify(await opened())==='["https://tracker.example/wiki/7"]');
  ok('openerFor returns the opener with its title and glyph, and null for another address',await page.evaluate(()=>{const o=dinahLinks.openerFor('https://tracker.example/issues/7');return !!o&&o.id==='sample-issues'&&o.title==='Open in Issue Window'&&o.glyph==='ticket'&&dinahLinks.openerFor('https://tracker.example/wiki/7')===null;}));
  ok('a match that returns a promise is no match',await page.evaluate(()=>{const off=dinahLinks.registerOpener({id:'sample-promise',match:()=>Promise.resolve(true),open:()=>{}});const none=dinahLinks.openerFor('https://elsewhere.example/a')===null;off();return none;}));
  ok('a registration without a match function is refused',await page.evaluate(()=>{const real=console.error;let said='';console.error=t=>{said=String(t);};const off=dinahLinks.registerOpener({id:'sample-bad',open:()=>{}});console.error=real;off();return /needs an id, a match function and an open function/.test(said)&&dinahLinks.openerFor('https://tracker.example/issues/7').id==='sample-issues';}));
  await page.evaluate(()=>{window.__unregisterSecond=dinahLinks.registerOpener({id:'sample-issues',title:'Open in Second Window',
   match:url=>url.hostname==='tracker.example'&&url.pathname.startsWith('/issues/'),open:href=>{__taken.push(['second',href]);}});});
  await page.click('#ol-issue');
  ok('a second registration of an id replaces the first',JSON.stringify(await taken())==='[["second","https://tracker.example/issues/7"]]');
  await page.evaluate(()=>window.__unregisterFirst());
  await page.click('#ol-issue');
  ok('the first registration\'s unregister does nothing once it was replaced',JSON.stringify(await taken())==='[["second","https://tracker.example/issues/7"]]');
  await page.evaluate(()=>window.__unregisterSecond());
  await page.click('#ol-issue');
  ok('once unregistered, a plain press opens the browser again',(await taken()).length===0&&JSON.stringify(await opened())==='["https://tracker.example/issues/7"]'&&await page.evaluate(()=>dinahLinks.openerFor('https://tracker.example/issues/7')===null));

  // A link in a chat's message: its tooltip and menu say where a plain press
  // opens it. The message is drawn before the opener is registered, so the
  // tooltip is read again when the pointer reaches the link.
  await page.evaluate(()=>{
   const root=document.createElement('div');root.id='outside-chips';
   root.style.cssText='position:fixed;left:8px;top:80px;z-index:99999;background:#fff;padding:16px';
   root.appendChild(dinahChatText.nodes('[Issue](https://tracker.example/issues/7) and [Site](https://elsewhere.example/a)',null,'C:/Test'));
   document.body.appendChild(root);dinahChatText.openLinks(root,()=>'C:/Test');
  });
  const issueChip=page.locator('#outside-chips a.smart-chip').first(),siteChip=page.locator('#outside-chips a.smart-chip').last();
  const tip=el=>el.evaluate(a=>a.getAttribute('data-tooltip')||a.getAttribute('title'));
  ok('with no opener, a chip\'s tooltip says the browser',await tip(issueChip)==='Open in the Browser');
  await page.evaluate(()=>{window.__unregisterChip=dinahLinks.registerOpener({id:'sample-issues',title:'Open in Issue Window',glyph:'ticket',
   match:url=>url.hostname==='tracker.example'&&url.pathname.startsWith('/issues/'),open:(href,event)=>{__taken.push(['chip',href,event?event.type:'none']);}});});
  await issueChip.hover();
  ok('once an opener is registered, the chip\'s tooltip is the opener\'s title when the pointer reaches it',await tip(issueChip)==='Open in Issue Window');
  await siteChip.hover();
  ok('a chip for another address still says the browser',await tip(siteChip)==='Open in the Browser');
  ok('a message drawn after the registration has the opener\'s title at once',await page.evaluate(()=>{const n=dinahChatText.nodes('[Issue](https://tracker.example/issues/8)',null,'C:/Test');const a=n.querySelector?n.querySelector('a.smart-chip'):null;return !!a&&a.getAttribute('title')==='Open in Issue Window';}));
  await issueChip.click();
  ok('a plain press on the chip goes to the opener',JSON.stringify(await taken())==='[["chip","https://tracker.example/issues/7","none"]]'&&(await opened()).length===0);
  await issueChip.click({modifiers:['Control']});
  await issueChip.click({modifiers:['Shift']});
  await issueChip.click({button:'middle'});
  ok('Control, Shift and the middle button on the chip open the browser',(await taken()).length===0&&JSON.stringify(await opened())==='["https://tracker.example/issues/7","https://tracker.example/issues/7","https://tracker.example/issues/7"]');
  const menu=page.locator('.context-menu:popover-open');
  const menuRows=()=>menu.first().evaluate(panel=>Array.from(panel.children).filter(el=>!el.matches('hr')).map(el=>({label:el.querySelector('.menubar-words').textContent,glyph:el.querySelector('.glyph')?el.querySelector('.glyph').style.getPropertyValue('--glyph'):''})));
  const choose=async label=>{await menu.first().locator('.menu-action').filter({has:page.locator('.menubar-words',{hasText:new RegExp('^'+label+'$')})}).click();};
  await issueChip.click({button:'right'});
  await menu.first().waitFor();
  let rows=await menuRows();
  ok('the chip\'s menu has the opener\'s row with its glyph first, then Open in Browser',JSON.stringify(rows.map(r=>r.label))==='["Open in Issue Window","Open in Browser","Copy Link Address"]'&&rows[0].glyph==='var(--glyph-ticket)',JSON.stringify(rows));
  await choose('Open in Issue Window');
  ok('the opener\'s row opens the link in the opener',JSON.stringify(await taken())==='[["chip","https://tracker.example/issues/7","none"]]');
  await issueChip.click({button:'right'});
  await menu.first().waitFor();
  await choose('Open in Browser');
  ok('Open in Browser in the chip\'s menu opens the browser',(await taken()).length===0&&JSON.stringify(await opened())==='["https://tracker.example/issues/7"]');
  await siteChip.click({button:'right'});
  await menu.first().waitFor();
  rows=await menuRows();
  ok('a chip for another address keeps its menu',JSON.stringify(rows.map(r=>r.label))==='["Open in Browser","Copy Link Address"]',JSON.stringify(rows));
  await page.keyboard.press('Escape');
  await page.evaluate(()=>{window.__unregisterChip();document.getElementById('outside-chips').remove();});
  ok('no other page was opened',page.context().pages().length===1);
  ok('the page is the program\'s own at the end',page.url().split('#')[0]===start.split('#')[0]&&await page.locator('[data-menubar]').count()===1);
 }finally{await t.close();}
})().catch(e=>{console.error(e);process.exit(1)});
