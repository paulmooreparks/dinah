// A preview may change its draft while its isolated document is still loading.
const {launch,OUT,FRONTEND}=require('./lib/page'),fs=require('fs'),path=require('path');
(async()=>{const t=await launch();try{
 await t.page.evaluate(()=>dinahAppearance.ready);
 let release,seen;const gate=new Promise(resolve=>{release=resolve;}),requested=new Promise(resolve=>{seen=resolve;});let held=false;
 await t.context.route('http://dinah.test/pudl/pudl.css',async route=>{if(!held){held=true;seen();await gate;}await route.fulfill({path:path.join(FRONTEND,'pudl/pudl.css')});});
 await t.page.evaluate(()=>{window.__auditDraft=dinahAppearance.document('mineral');__auditDraft.name='In-flight contrast snapshot';window.__auditPending=dinahSkinContrast.check(__auditDraft);});
 await t.deadline('the isolated audit stylesheet request',requested,10000);
 await t.page.evaluate(()=>{__auditDraft.palettes.light.text='#ffffff';});release();
 const first=await t.page.evaluate(()=>__auditPending);
 await t.page.evaluate(()=>{__auditDraft.palettes.light.text=dinahAppearance.builtins.mineral.palettes.light.text;});
 const cached=await t.page.evaluate(()=>dinahSkinContrast.check(__auditDraft));
 const fresh=await t.page.evaluate(()=>{const document=JSON.parse(JSON.stringify(__auditDraft));document.name='Fresh comparison';return dinahSkinContrast.check(document);});
 fs.writeFileSync(path.join(OUT,'skin-audit-cache.json'),JSON.stringify({first,cached,fresh},null,2));
 t.ok('an in-flight audit retains its original input',!first.length,JSON.stringify(first));
 t.ok('a later edit cannot poison the original cache entry',!cached.length,JSON.stringify(cached));
 t.ok('the fresh valid comparison passes',!fresh.length,JSON.stringify(fresh));
 const ordinary=await t.page.evaluate(()=>{const document=dinahAppearance.document('mineral');document.name='Ordinary invalid palette';document.palettes.light.text='#ffffff';return dinahSkinContrast.check(document);});
 await t.page.emulateMedia({forcedColors:'active'});
 const forced=await t.page.evaluate(()=>{const document=dinahAppearance.document('mineral');document.name='Forced-colors invalid palette';document.palettes.light.text='#ffffff';return dinahSkinContrast.check(document);});
 t.ok('forced colors cannot hide an invalid underlying palette from the audit',ordinary.length>0&&JSON.stringify(forced)===JSON.stringify(ordinary),JSON.stringify({ordinary,forced}));
 t.ok('the visible application retains its forced-color behavior',await t.page.evaluate(()=>getComputedStyle(document.documentElement).getPropertyValue('--category-1').trim().toLowerCase()==='canvastext'));
 await t.page.emulateMedia({forcedColors:'none'});
}finally{await t.close();}})().catch(error=>{console.error(error);process.exitCode=1;});
