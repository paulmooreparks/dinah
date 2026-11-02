// Shared chat rows give the title every width but the count reserve and one status pill, as wide as its own glyph and word, at the right of a slot as wide as the widest state's pill (c8031ec). Neither moves.
const { launch, FRONTEND, OUT } = require('./lib/page');
const fs = require('fs'), path = require('path');
const skins = JSON.parse(fs.readFileSync(path.join(FRONTEND, 'pudl/pudl-skins.json'), 'utf8')).skins.filter(s => s.admitted).map(({id,label}) => ({id,label}));
(async () => {
  const t = await launch({ viewport: { width: 1440, height: 1000 }, args: {skins}, app: ({skins}) => {
    const cwd = 'C:/Fixture'; let version = 1, state = {life:'open',window:true,exec:{state:'idle',why:''}};
    let appearance = {version:1,active:'mineral',skin:'standard',themes:[],overrides:{}};
    const chat = () => ({sessionId:'layout',title:'A very long chat title that should use the remaining row width',cwd,provider:'codex',open:true,chatState:state});
    window.__layoutState = kind => {
      state = ['closed','retired','elsewhere'].includes(kind) ? {life:kind === 'elsewhere' ? 'open' : kind,window:false} : {life:'open',window:true,exec:kind === 'unknown' ? null : {state:kind,why:'Full accessible state reason'}};
      if (kind === 'elsewhere') state.elsewhere = 'Running in another Dinah window';
      __emit('agent:chatstate',{stateGen:'layout',stateVer:++version,chats:[{sessionId:'layout',...state}],groups:[]});
    };
    const snapshot = () => ({state:appearance,revision:String(version),skins});
    return {State:async()=>({workspace:cwd,recent:[cwd]}),Sessions:async()=>[chat()],SessionsAt:async()=>[chat()],
      ChatStates:async()=>({stateGen:'layout',stateVer:version,chats:[{sessionId:'layout',...state}],groups:[]}),
      OpenSessionAt:async()=>chat(), ReadAppearance:async()=>snapshot(),SaveAppearance:async value=>{appearance=value;version++;return snapshot();},
      ReadUIState:async()=>({version:1,values:{'dinah-workbench-layout':'?open=workspace'}}),SaveUIState:async()=>{}};
  }});
  const {page:p,ok} = t, records = [];
  try {
    await p.evaluate(()=>dinahAppearance.ready);
    for (const skin of skins) for (const mode of ['light','dark']) for (const width of [1440,560]) for (const surface of ['chats','workspace']) {
      await p.setViewportSize({width,height:1000});
      const windowKey = surface === 'chats' ? 'chat-browser' : 'workspace';
      await p.evaluate(async ({windowKey,skin,mode})=>{await dinahWorkbench.open(windowKey);pudlWindows.dock(windowKey,null);pudlWindows.snap(windowKey,'maximized');pudlSetTheme(mode);await dinahAppearance.selectSkin(skin);},{windowKey,skin:skin.id,mode});
      const host = surface === 'chats' ? '#sessions' : '#workspace-chats';
      const row = p.locator(host+' [data-session-row="layout"]'), link = row.locator('a[data-session]');
      await link.waitFor(); await link.focus();
      let first = null;
      for (const state of ['idle','working','waiting','stuck','unknown','closed','retired','elsewhere']) {
        await p.evaluate(state=>__layoutState(state),state);
        // The state arrives through the page's own render job, so the row is measured once it shows it.
        await p.waitForFunction(({host,cls,word})=>{const b=document.querySelector(host+' [data-session-row="layout"] .session-badge > .badge:not(.chat-state-reserve)');return !!b&&b.classList.contains(cls)&&b.textContent===word;},{host,cls:['closed','retired','elsewhere'].includes(state)?'chat-lifecycle':'chat-state-'+state,word:{idle:'Idle',working:'Working',waiting:'Waiting',stuck:'Stuck',unknown:'Unknown',closed:'Closed',retired:'Retired',elsewhere:'Elsewhere'}[state]});
        await p.evaluate(()=>new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(r))));
        const data = await row.evaluate(row=>{
          // .session-badge is the chip's unpainted slot, sized by hidden reserve copies of every state's chip; the shown chip is its own width at the slot's right edge (c8031ec).
          const link=row.querySelector('a[data-session]'),line=link.querySelector('.browser-row-line'),title=line.querySelector('.session-title'),slot=line.querySelector('.session-badge'),badge=slot.querySelector(':scope > .badge:not(.chat-state-reserve)');
          const rect=el=>{const r=el.getBoundingClientRect();return {x:r.x,y:r.y,width:r.width,height:r.height,right:r.right,bottom:r.bottom};};
          const reserves=[...slot.querySelectorAll(':scope > .chat-state-reserve')], text=[...badge.childNodes].find(n=>n.nodeType===3), range=document.createRange(); range.selectNodeContents(text);
          const same=reserves.find(r=>r.textContent===badge.textContent);
          return {line:rect(line),title:rect(title),badgeSlot:rect(slot),badge:rect(badge),emptyAggregate:!!line.querySelector('.team-state-slot:empty'),reserve:!!line.querySelector('.session-attention-reserve'),focused:document.activeElement===link,label:link.getAttribute('aria-label'),badgeClass:badge.className,words:badge.textContent,wordsRight:range.getBoundingClientRect().right,widest:Math.max(...reserves.map(r=>r.getBoundingClientRect().width)),sameWidth:same?same.getBoundingClientRect().width:-1,borderEnd:parseFloat(getComputedStyle(badge).borderRightWidth),lineGap:parseFloat(getComputedStyle(line).columnGap)||0,slot:rect(line.querySelector('.session-attention-slot')),padEnd:parseFloat(getComputedStyle(badge).paddingRight),badgeLabel:badge.getAttribute('aria-label'),reason:link.querySelector('.session-reason').title,actions:row.querySelectorAll('[data-row-actions]').length};
        });
        const label=[skin.id,mode,width,surface,state].join('/'); records.push({label,...data});
        ok(label+' has no empty team column',!data.emptyAggregate);
        ok(label+' keeps the count reserve',data.reserve);
        const short={idle:'Idle',working:'Working',waiting:'Waiting',stuck:'Stuck',unknown:'Unknown',closed:'Closed',retired:'Retired',elsewhere:'Elsewhere'}[state];
        ok(label+' shows its short word, right-aligned in a pill at the trailing content edge',data.words===short&&Math.abs(data.badge.right-data.line.right)<1&&Math.abs(data.badgeSlot.right-data.line.right)<1&&Math.abs(data.wordsRight-(data.badge.right-data.padEnd-data.borderEnd))<1.5,JSON.stringify(data));
        ok(label+' is as wide as its own glyph and word, in a slot as wide as the widest state\'s chip',Math.abs(data.badge.width-data.sameWidth)<1&&Math.abs(data.badgeSlot.width-data.widest)<1&&data.badge.width<=data.badgeSlot.width+.5,JSON.stringify(data));
        ok(label+' gives the title all the width but the count slot and the pill',Math.abs(data.title.x-data.line.x)<1&&Math.abs(data.title.right+data.lineGap-data.slot.x)<1&&Math.abs(data.slot.right+data.lineGap-data.badgeSlot.x)<1,JSON.stringify(data));
        if(!first)first=data;
        ok(label+' holds the title and mark where the first state put them',['x','y','width','height'].every(k=>Math.abs(data.title[k]-first.title[k])<=.02&&Math.abs(data.badgeSlot[k]-first.badgeSlot[k])<=.02)&&Math.abs(data.badge.right-first.badge.right)<=.02&&Math.abs(data.badge.y-first.badge.y)<=.02,JSON.stringify({first:[first.title,first.badgeSlot,first.badge],now:[data.title,data.badgeSlot,data.badge]}));
        ok(label+' reflects its live status update',data.badgeClass.includes(['closed','retired','elsewhere'].includes(state)?'chat-lifecycle':'chat-state-'+state));
        ok(label+' retains focus and complete accessible state',data.focused&&!!data.badgeLabel&&data.label.includes('A very long chat title'));
        if(surface==='workspace')ok(label+' retains its actual row action',data.actions===1);
      }
      if(width===560)await t.shot('list-layout-'+[skin.id,mode,surface].join('-'));
    }
  } finally {fs.writeFileSync(path.join(OUT,'list-row-layout.json'),JSON.stringify(records,null,2));await t.close();}
})().catch(error=>{console.error(error);process.exitCode=1;});
