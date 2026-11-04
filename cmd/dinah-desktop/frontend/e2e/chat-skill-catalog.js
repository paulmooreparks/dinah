const { launch } = require('./lib/page');
const { openMenu } = require('./lib/commands');
(async () => {
  const t = await launch({
    viewport: { width: 1280, height: 820 },
    app: () => {
      const ws='C:/Test';
      window.__prepared=[]; window.__skillFail=false;
      return {
        State:async()=>({workspace:ws,recent:[ws],defaultProvider:'codex',catalog:[],defaults:{},pinned:[],titles:{}}),
        AgentProviders:async()=>[window.__provider('codex')],
        SkillCatalog:async()=>({skills:[],problems:[]}),
        ChatSkillCatalog:async()=>{
          if(window.__skillFail)throw new Error('Discovery failed');
          return {skills:[{id:'personal-one',name:'review',source:'personal',scope:'personal',description:'Personal review',revision:'one'},
            {id:'codex-two',name:'review',source:'imported',scope:'imported',origin:'Codex',description:'Codex review',revision:'two'}],problems:[]};
        },
        NewSessionAt:async()=>({sessionId:'codex:test',provider:'codex',cwd:ws,title:'',open:true,working:false,queued:[]}),
        PrepareSkill:async()=>{throw new Error('Legacy preparation used');},
        PrepareChatSkill:async(...args)=>{window.__prepared.push(args);return 'Prepared '+args[0];},
        Send:async()=>{},
      };
    }
  });
  try {
    const p=t.page, chat=p.locator('.win[data-chat-current]'), prompt=chat.locator('[data-view-id=prompt]');
    await p.waitForFunction(()=>!!window.dinahSkills);
    const menu=await openMenu(p,chat,'skills');
    t.ok('Skills menu lists both personal and Codex skills',await menu.locator('.menu-action').filter({hasText:'/review'}).count()===2);
    await p.keyboard.press('Escape');
    await prompt.fill('/rev');
    await chat.locator('[data-view-id=slash-pop] .skill-row').first().waitFor();
    t.ok('slash picker lists same-name packages separately',await chat.locator('.skill-row').count()===2);
    await p.keyboard.press('ArrowDown'); await p.keyboard.press('Enter');
    await p.keyboard.press('Control+Enter');
    await p.waitForFunction(()=>window.__prepared.length===1);
    t.ok('keyboard selection prepares the second package by ID and provider',await p.evaluate(()=>__prepared[0][0]==='codex-two'&&__prepared[0][3]==='codex'));
    await p.evaluate(()=>{window.__skillFail=true;window.__emit('agent:commands',[]);});
    await p.waitForTimeout(100);
    await prompt.fill('/');
    t.ok('discovery failures are visible in slash picker',await chat.locator('[data-view-id=slash-pop]').textContent().then(s=>s.includes('Discovery failed')));
    await p.keyboard.press('Escape');
    const failed=await openMenu(p,chat,'skills');
    t.ok('skills menu exposes discovery failure and retry',await failed.textContent().then(s=>s.includes('Skills could not be fully loaded')&&s.includes('Refresh Skills')));
  } finally {await t.close();}
})();
