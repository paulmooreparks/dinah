// Shared chat rows give the title every width but one fixed status square and the count reserve, which never move.
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
        await p.evaluate(()=>new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(r))));
        const data = await row.evaluate(row=>{
          const link=row.querySelector('a[data-session]'),line=link.querySelector('.browser-row-line'),title=line.querySelector('.session-title'),badge=line.querySelector('.session-badge');
          const rect=el=>{const r=el.getBoundingClientRect();return {x:r.x,y:r.y,width:r.width,height:r.height,right:r.right,bottom:r.bottom};};
          return {line:rect(line),title:rect(title),badge:rect(badge),emptyAggregate:!!line.querySelector('.team-state-slot:empty'),reserve:!!line.querySelector('.session-attention-reserve'),focused:document.activeElement===link,label:link.getAttribute('aria-label'),badgeClass:badge.className,badgeLabel:badge.getAttribute('aria-label'),reason:link.querySelector('.session-reason').title,actions:row.querySelectorAll('[data-row-actions]').length};
        });
        const label=[skin.id,mode,width,surface,state].join('/'); records.push({label,...data});
        ok(label+' has no empty team column',!data.emptyAggregate);
        ok(label+' keeps the count reserve',data.reserve);
        ok(label+' puts a square status mark at the trailing content edge',Math.abs(data.badge.right-data.line.right)<1&&Math.abs(data.badge.width-data.badge.height)<1,JSON.stringify(data));
        ok(label+' gives the remaining width to the title without overlap',data.title.width>data.line.width/2&&data.title.right<=data.badge.x+1,JSON.stringify(data));
        if(!first)first=data;
        ok(label+' holds the title and mark where the first state put them',['x','y','width','height'].every(k=>Math.abs(data.title[k]-first.title[k])<=.02&&Math.abs(data.badge[k]-first.badge[k])<=.02),JSON.stringify({first:[first.title,first.badge],now:[data.title,data.badge]}));
        ok(label+' reflects its live status update',data.badgeClass.includes(['closed','retired','elsewhere'].includes(state)?'chat-lifecycle':'chat-state-'+state));
        ok(label+' retains focus and complete accessible state',data.focused&&!!data.badgeLabel&&data.label.includes('A very long chat title'));
        if(surface==='workspace')ok(label+' retains its actual row action',data.actions===1);
      }
      if(width===560)await t.shot('list-layout-'+[skin.id,mode,surface].join('-'));
    }
  } finally {fs.writeFileSync(path.join(OUT,'list-row-layout.json'),JSON.stringify(records,null,2));await t.close();}
})().catch(error=>{console.error(error);process.exitCode=1;});
