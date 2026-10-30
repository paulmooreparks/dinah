// Model defaults and favorites belong to the selected provider.
const { launch } = require('./lib/page');
(async () => {
  const t=await launch({
    viewport:{width:1280,height:900},
    app:()=>{
      window.__calls=[];window.__reads=[];window.__writes=[];
      const records={};
      for(const provider of ['devin','codex']) records[provider]={provider,catalog:[{id:'model',category:'model',name:'Model',type:'select',currentValue:provider+'-one',options:[{value:provider+'-one',name:provider==='devin'?'Devin One':'Codex One'},{value:provider+'-two',name:provider==='devin'?'Devin Two':'Codex Two'}]}],defaults:{model:provider+'-one'},pinned:[provider+'-one']};
      const copy=value=>JSON.parse(JSON.stringify(value));
      window.__records=records;
      async function write(provider,kind,value,on) {
        __calls.push([kind,provider,value,on]);
        if(window.__holdWrite===provider)await new Promise(resolve=>__writes.push(resolve));
        if(window.__failWrite)throw new Error('Fixture preferences could not be saved');
        const record=records[provider];
        if(kind==='default'){if(value==null)delete record.defaults.model;else record.defaults.model=value;return copy(record.defaults);}
        record.pinned=record.pinned.filter(item=>item!==value);if(on)record.pinned.push(value);return copy(record.pinned);
      }
      return {
        State:async()=>({workspace:'C:/Fixture',recent:['C:/Fixture'],defaultProvider:'devin',catalog:copy(records.devin.catalog),defaults:copy(records.devin.defaults),pinned:copy(records.devin.pinned)}),
        AgentProviders:async()=>[window.__provider('devin'),window.__provider('codex',{available:false})],
        AgentCheck:async()=>{throw new Error('Preferences must not connect to an agent');},
        SetDefaultProvider:async()=>{throw new Error('Model preferences must not change the default agent');},
        ProviderModelPreferences:async provider=>{
          __calls.push(['read',provider]);
          if(window.__failRead===provider)throw new Error('Fixture preferences unavailable');
          const result=copy(records[provider]);if(window.__empty===provider)result.catalog=[];
          if(window.__holdRead===provider)await new Promise(resolve=>__reads.push(resolve));
          return result;
        },
        SetDefaultForProvider:async(provider,id,value)=>{if(id!=='model')throw new Error('Unexpected model option');return write(provider,'default',value);},
        SetPinnedForProvider:async(provider,value,on)=>write(provider,'pin',value,on)
      };
    },
  });
  const {page,ok}=t;
  try {
    await page.evaluate(()=>document.querySelector('[data-cmd="settings"]').click());
    await page.locator('#settings').getByRole('tab',{name:'Chat',exact:true}).click();
    await page.locator('#defaults [data-open="model-prefs"]').click();
    const dialog=page.locator('#model-prefs'),select=dialog.locator('#prefs-provider');
    const choice=name=>dialog.locator('#prefs-list a').filter({hasText:name});
    const pin=name=>dialog.locator('.model-row').filter({hasText:name}).locator('.model-pin');
    const selectProvider=async(id,label)=>{await select.selectOption(id);await choice(label).waitFor();};
    await choice('Devin One').waitFor();
    ok('model preferences initially select the default agent',await select.inputValue()==='devin');
    ok('a provider the engine does not list is not offered',await select.locator('option[value="gemini"]').count()===0);
    ok('supported offline providers can edit cached preferences',!await select.locator('option[value="codex"]').isDisabled());
    await selectProvider('codex','Codex One');
    ok('the provider switch replaces models, default and favorites',await dialog.locator('#prefs-list a[aria-current]').textContent()==='Codex One' && await pin('Codex One').getAttribute('aria-pressed')==='true' && !(await dialog.locator('#prefs-list').textContent()).includes('Devin'));
    await choice('Codex Two').click();
    await page.waitForFunction(()=>__records.codex.defaults.model==='codex-two');
    await pin('Codex Two').click();
    await page.waitForFunction(()=>__records.codex.pinned.includes('codex-two'));
    ok('model and favorite writes target only Codex',await page.evaluate(()=>__records.devin.defaults.model==='devin-one' && __records.devin.pinned.join('|')==='devin-one' && __calls.some(x=>x[0]==='default'&&x[1]==='codex') && __calls.some(x=>x[0]==='pin'&&x[1]==='codex')));
    await dialog.locator('#prefs-own-default').click();
    await page.waitForFunction(()=>!__records.codex.defaults.model);
    ok('Use Own Default resets only the selected provider',await page.evaluate(()=>__records.devin.defaults.model==='devin-one') && (await dialog.locator('#prefs-own-default').textContent()).includes('Codex'));
    await page.evaluate(()=>{__failWrite=true;});
    await choice('Codex One').click();
    await dialog.locator('#prefs-status').getByText(/could not be saved/).waitFor();
    ok('failed default writes preserve the stored selection',await page.evaluate(()=>!__records.codex.defaults.model) && await dialog.locator('#prefs-list a[aria-current]').count()===0);
    await pin('Codex Two').click();
    await dialog.locator('#prefs-status').getByText(/could not be saved/).waitFor();
    ok('failed favorite writes preserve the existing pin',await pin('Codex Two').getAttribute('aria-pressed')==='true');
    await page.evaluate(()=>{__failWrite=false;__holdRead='codex';});
    await selectProvider('devin','Devin One');
    await select.selectOption('codex');
    await page.waitForFunction(()=>__reads.length===1);
    ok('loading another provider clears selectable stale models',await dialog.locator('#prefs-list a').count()===0);
    await selectProvider('devin','Devin One');
    await page.evaluate(async()=>{__holdRead=null;__reads[0]();await new Promise(resolve=>requestAnimationFrame(resolve));});
    ok('late reads cannot replace the current provider',await select.inputValue()==='devin' && (await dialog.locator('#prefs-list').textContent()).includes('Devin One') && !(await dialog.locator('#prefs-list').textContent()).includes('Codex'));
    await selectProvider('codex','Codex One');
    await page.evaluate(()=>{__holdWrite='codex';});
    await choice('Codex Two').click();
    await page.waitForFunction(()=>__writes.length===1);
    ok('a pending write leaves the provider selector usable',await select.isEnabled());
    await selectProvider('devin','Devin One');
    await page.evaluate(async()=>{__holdWrite=null;__writes[0]();await new Promise(resolve=>requestAnimationFrame(resolve));});
    ok('late writes do not redraw a different provider',await select.inputValue()==='devin' && await dialog.locator('#prefs-list a[aria-current]').textContent()==='Devin One');
    await selectProvider('codex','Codex One');
    ok('returning to a provider shows its completed write',await dialog.locator('#prefs-list a[aria-current]').textContent()==='Codex Two');
    await t.shot('provider-model-prefs');
    await dialog.getByRole('button',{name:'Close',exact:true}).click();
    ok('editing another provider leaves the app default and summary unchanged',await page.locator('#default-provider').inputValue()==='devin' && (await page.locator('#defaults').textContent()).includes('Devin One'));
    await page.locator('#favs-summary + [data-open="model-prefs"]').click();
    await choice('Codex Two').waitFor();
    ok('reopening retains the explicitly selected provider',await select.inputValue()==='codex');
    await page.evaluate(()=>{__failRead='codex';});
    await selectProvider('devin','Devin One');await select.selectOption('codex');
    await dialog.locator('#prefs-status').getByText(/unavailable/).waitFor();
    ok('read errors remove stale model choices',await dialog.locator('#prefs-list a').count()===0);
    await page.evaluate(()=>{__failRead=null;__empty='codex';});
    await selectProvider('devin','Devin One');await select.selectOption('codex');
    await dialog.locator('#prefs-status').getByText(/No saved model catalog/).waitFor();
    ok('an empty catalog is explained without connecting',await dialog.locator('#prefs-list a').count()===0 && (await dialog.locator('#prefs-status').textContent()).trim().length>0);
    await selectProvider('devin','Devin One');
    await page.setViewportSize({width:560,height:650});
    await t.shot('provider-model-prefs-narrow');
    ok('provider preferences fit a narrow display',await dialog.evaluate(el=>el.scrollWidth<=el.clientWidth+1));
    ok('the model list and close command remain visible on a short display',await choice('Devin One').isVisible() && await dialog.getByRole('button',{name:'Close',exact:true}).evaluate(el=>{const rect=el.getBoundingClientRect();return rect.top>=0&&rect.bottom<=innerHeight;}));
  }finally{await t.close();}
})().catch(error=>{console.error(error);process.exitCode=1;});
