// A link to another site never replaces the program's own page. It opens in
// the person's browser, whichever part of the page it is in and whichever
// keys are down.
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
  ok('no other page was opened',page.context().pages().length===1);
  ok('the page is the program\'s own at the end',page.url().split('#')[0]===start.split('#')[0]&&await page.locator('[data-menubar]').count()===1);
 }finally{await t.close();}
})().catch(e=>{console.error(e);process.exit(1)});
