// Named document state survives a fresh browser profile. The document
// applet is loaded on its own into a bare page rather than into index.html,
// so the stubbed window is launched without going anywhere first.
const { launch, FRONTEND } = require('./lib/page');
const path = require('path');
(async () => {
  const t = await launch({
    viewport: { width: 1280, height: 720 },
    app: () => ({
      ReadUIState: async () => ({version:1, values:{
        'dinah-scroll:doc-one:document': {top:650,left:0}
      }}), SaveUIState: async value => { window.__saved = value; },
      PreviewWorkspaceResource: () => new Promise(resolve => { window.__documentReady = resolve; })
    }),
    init: () => {
      window.dinahWorkbench = {namespaceSVG() {}, resourceRef(item) { return { workspaceId: 'w1', sourceId: 'local', path: item.path }; }, register(kind, definition) { if (kind === 'workspace-resource') window.__documentApplet = definition; }};
      window.pudlWindows = {retitle() {}};
    },
    goto: false,
  });
  const { page, ok } = t;
  try {
    await page.setContent('<style>.doc-scroll{height:200px;overflow:auto}</style><section data-win="doc-one"><div id="doc"></div></section>');
    for (const file of ['helpers.js', 'pudl/pudl-commands.js', 'ui-state.js', 'safe-html.js', 'document-applet.js']) {
      await page.addScriptTag({path:path.join(FRONTEND, file)});
    }
    await page.evaluate(async () => {
      await dinahUIState.ready;
      dinahUIState.restoreLayout();
      window.__doc = __documentApplet.init(document.getElementById('doc'), {state:'problems'}, {kind:'workspace-resource',workspace:'w1',path:'sample.md'});
    });
    ok('delayed document does not replace saved scroll with zero', await page.evaluate(() => dinahUIState.get('dinah-scroll:doc-one:document').top === 650));
    await page.evaluate(async () => {
      __documentReady({resource:{ref:{workspaceId:'w1',sourceId:'local',path:'sample.md'},title:'Sample',revision:'1',actions:['preview']},blocks:[{line:1,html:'<div style="height:1600px">Sample</div>'}],problems:[{line:1,severity:'warning',message:'Example problem'}]});
      await __doc.ready;
    });
    ok('document restores scroll after content arrives', await page.locator('[data-doc-scroll]').evaluate(el => el.scrollTop === 650));
    ok('document restores expanded problems', await page.locator('[data-doc-problems]').isVisible());
    await page.getByRole('button', {name:'1 Problem',exact:true}).click();
    await page.evaluate(() => dinahUIState.flush());
    ok('document problem visibility has restorable state', await page.evaluate(() => __doc.state() === null));
    await page.evaluate(() => { __doc.destroy(); });
  } finally { await t.close(); }
})().catch(error => { console.error(error); process.exitCode = 1; });
