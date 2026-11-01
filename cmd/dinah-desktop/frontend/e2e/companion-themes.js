const {launch, OUT} = require('./lib/page');
const fs = require('fs'), path = require('path');
(async () => {
  const choices=JSON.parse(fs.readFileSync(path.join(__dirname,'../pudl/pudl-skins.json'),'utf8')).skins.filter(s=>s.admitted).map(({id,label})=>({id,label}));
  const t = await launch({viewport:{width:1360,height:920}, args:{choices}, app:({choices}) => {
    let state = null, revision = 0;
    const copy = x => JSON.parse(JSON.stringify(x));
    const snapshot = () => {
      if (!state) state = JSON.parse(localStorage.getItem('test-appearance') || 'null') || {
        version:1, active:'mineral', skin:'standard',
        themes:[{id:'custom-retained', document:{...copy(dinahThemeBuiltins.mineral),name:'My retained theme'}}],
        overrides:{chat:{size:18}}
      };
      return {state:copy(state), revision:String(revision),skins:choices};
    };
    return {
      ReadAppearance:async()=>snapshot(),
      SaveAppearance:async(next, expected)=>{
        if (expected !== String(revision)) throw Error('Revision conflict');
        state=copy(next); revision++; localStorage.setItem('test-appearance',JSON.stringify(state)); return snapshot();
      }
    };
  }});
  const {page:p,ok}=t, records=[];
  try {
    await p.evaluate(()=>dinahAppearance.ready);
    await p.evaluate(()=>dinahWorkbench.open('settings'));
    const audit = await p.evaluate(async()=>{const out={};for(const id of ['bevel','gel','glaze'])out[id]=await dinahSkinContrast.check(dinahAppearance.builtins[id]);return out;}); console.log('AUDIT',JSON.stringify(audit));
    const baseline = await p.evaluate(()=>JSON.stringify(dinahAppearance.snapshot().state.themes));
    ok('all three themes are selectable in Settings', await p.locator('#appearance-theme option').evaluateAll(options=>['bevel','gel','glaze'].every(id=>options.some(o=>o.value===id))));
    for(const theme of ['bevel','gel','glaze']) {
      await p.locator('#appearance-theme').selectOption(theme);
      try { await p.waitForFunction(id=>dinahAppearance.snapshot().state.active===id,theme); } catch(error) { console.log('SELECTION', JSON.stringify(await p.evaluate(()=>({snapshot:dinahAppearance.snapshot(),status:document.getElementById('appearance-status').textContent})))); throw error; }
      await p.evaluate(id=>dinahAppearance.selectSkin(id),theme);
      for(const mode of ['light','dark']) for(const motion of ['no-preference','reduce']) {
        await p.emulateMedia({reducedMotion:motion});
        await p.evaluate(mode=>pudlSetTheme(mode),mode);
        await p.waitForFunction(mode=>document.documentElement.dataset.theme===mode,mode);
        const result=await p.evaluate(({theme,mode})=>{
          const doc=dinahAppearance.builtins[theme], style=getComputedStyle(document.documentElement);
          return {theme,mode,failures:dinahAppearance.contrast(doc),mismatches:Object.entries(doc.palettes[mode]).filter(([key,value])=>style.getPropertyValue('--'+key).trim().toLowerCase()!==value.toLowerCase()), background:style.getPropertyValue('--bg').trim(), skin:pudlSkins.current(), active:dinahAppearance.snapshot().state.active};
        },{theme,mode});
        records.push({...result,motion});
        ok(theme+'/'+mode+'/'+motion+' renders the complete admitted palette',!result.failures.length&&!result.mismatches.length&&result.active===theme&&result.skin===theme,JSON.stringify(result));
        if(motion==='no-preference') await t.shot('companion-'+theme+'-'+mode);
      }
      await p.evaluate(()=>dinahAppearance.selectSkin('standard'));
      ok(theme+' palette is independent of skin and mode', await p.evaluate(id=>dinahAppearance.snapshot().state.active===id&&pudlSkins.current()==='standard'&&pudlThemePreference()==='dark',theme));
      await p.evaluate(()=>dinahAppearance.reload());
      ok(theme+' persists independently through authoritative reload', await p.evaluate(id=>dinahAppearance.snapshot().state.active===id,theme));
    }
    await p.reload(); await p.evaluate(()=>dinahAppearance.ready);
    ok('saved theme survives a page restart', await p.evaluate(()=>dinahAppearance.snapshot().state.active==='glaze'));
    ok('custom documents and overrides survive all selections',await p.evaluate(expected=>JSON.stringify(dinahAppearance.snapshot().state.themes)===expected&&dinahAppearance.snapshot().state.overrides.chat.size===18,baseline));
    await p.evaluate(()=>dinahAppearance.select('custom-retained'));
    ok('retained custom theme remains selectable',await p.evaluate(()=>dinahAppearance.snapshot().state.active==='custom-retained'));
  } finally {
    fs.mkdirSync(OUT,{recursive:true});fs.writeFileSync(path.join(OUT,'companion-themes.json'),JSON.stringify(records,null,2));
    await t.close();
  }
})().catch(error=>{console.error(error);process.exitCode=1;});
