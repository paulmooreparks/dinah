// Count-only rehearsal on an already prepared temporary database copy.
// The caller owns backup, credential removal and deletion of the temporary home.
const fs = require('node:fs'), path = require('node:path');
const os = require('node:os');
const { spawn } = require('node:child_process');
const { chromium } = require('playwright-core');
const [installed, root] = process.argv.slice(2);
if (!installed || !root) throw Error('Supply installed executable and prepared temporary folder');
const relative = path.relative(fs.realpathSync(os.tmpdir()), fs.realpathSync(root));
if (!relative || relative.startsWith('..') || path.isAbsolute(relative)) throw Error('The prepared folder must be inside the temporary directory');
const home = path.join(root, 'home'), exe = path.join(root, 'dinah-desktop.exe');
const entry = path.join(root, 'entry.txt');
if (fs.existsSync(entry) || fs.existsSync(exe)) throw Error('Use a fresh prepared folder');
if (fs.realpathSync(home) !== path.join(fs.realpathSync(root), 'home')) throw Error('The home must not redirect outside the prepared folder');
const stats = {}; let child, browser, page;
const until = async (fn, label) => {
  const end = Date.now() + 120000;
  while (Date.now() < end) { if (await fn()) return; await new Promise(r => setTimeout(r, 100)); }
  throw Error(label);
};
(async () => {
  try {
    fs.copyFileSync(installed, exe);
    for (const name of ['profile', 'appdata', 'local', 'tmp', 'workspace']) fs.mkdirSync(path.join(root, name), { recursive: true });
    fs.writeFileSync(path.join(home, 'settings.json'), JSON.stringify({hostNotificationsOff:true, mentionBaseline:0, attentionReadSafe:false, workspace:path.join(root,'workspace')}));
    const env = {SystemRoot:process.env.SystemRoot, WINDIR:process.env.WINDIR,
      PATH:path.join(process.env.SystemRoot,'System32'), COMSPEC:process.env.COMSPEC,
      USERPROFILE:path.join(root,'profile'), HOME:path.join(root,'profile'),
      APPDATA:path.join(root,'appdata'), LOCALAPPDATA:path.join(root,'local'),
      TEMP:path.join(root,'tmp'), TMP:path.join(root,'tmp'),
      CODEX_HOME:path.join(root,'profile','.codex'), CLAUDE_CONFIG_DIR:path.join(root,'profile','.claude'),
      DINAH_HOME:home, DINAH_SHELL_ENTRY_FILE:entry};
    const start = performance.now();
    child = spawn(exe, [], {env, cwd:root, windowsHide:true, stdio:'ignore'});
    await until(() => fs.existsSync(entry), 'hidden entry did not appear');
    stats.entryMs = Math.round(performance.now()-start);
    browser = await chromium.launch({headless:true, channel:'msedge'});
    page = await browser.newPage(); stats.pageErrors = 0;
    page.on('pageerror', () => stats.pageErrors++);
    await page.route('**/*', route => {
      const u = new URL(route.request().url());
      return ['127.0.0.1','localhost','[::1]'].includes(u.hostname) ? route.continue() : route.abort();
    });
    await page.goto(fs.readFileSync(entry,'utf8').trim());
    await page.waitForFunction(() => window.dinahUIState && window.dinahWorkbench);
    await page.evaluate(() => dinahUIState.ready);
    stats.readyMs = Math.round(performance.now()-start);
    await until(() => JSON.parse(fs.readFileSync(path.join(home,'settings.json'),'utf8')).attentionReadSafe, 'attention migration incomplete');
    stats.attentionMigrationCompleteMs = Math.round(performance.now()-start);
    Object.assign(stats, await page.evaluate(async () => {
      const entryStart = performance.now();
      const index = await go.main.App.DecisionEntries();
      const entriesMs = performance.now()-entryStart;
      const start = performance.now(); let before=0, rows=0, pages=0, mentions=0;
      do { const p=await go.main.App.Decisions(before); pages++; rows+=(p.items||[]).length; mentions+=(p.items||[]).filter(x=>x.kind==='mention').length; before=p.nextBefore; } while(before);
      const n=await go.main.App.Notifications();
      const kept=n.filter(x=>x.kind==='mention'&&!x.dismissed);
      return {decisionRows:rows, decisionEntryCount:index.count, decisionEntriesMs:entriesMs, decisionPages:pages, plainMentionRows:mentions, decisionAndNotificationReadMs:performance.now()-start, keptMentionRecords:kept.length, keptMentionSources:new Set(kept.map(x=>x.sessionId||x.sourceId)).size};
    }));
    await page.evaluate(() => dinahWorkbench.open('needs-you'));
    await until(async () => await page.locator('#questions .attention-kept').count() === stats.keptMentionSources, 'grouped attention count differs');
    stats.attentionCards = await page.locator('#questions .attention-kept').count();
    await page.evaluate(() => dinahWorkbench.open('decisions'));
    if(stats.decisionRows) await page.waitForSelector('.decisions-row');
    else await page.getByText('No open requests.', {exact:true}).waitFor();
    stats.initialRenderedDecisionRows = await page.locator('.decisions-row').count();
    stats.workingChats = (await page.evaluate(() => go.main.App.UpdateState())).working;
    stats.statusCount = await page.locator('#status-decisions').getAttribute('data-status-count');
    if (stats.decisionRows !== stats.decisionEntryCount || Number(stats.statusCount) !== stats.decisionRows) throw Error('Decision counts differ');
    if (stats.pageErrors || stats.workingChats) throw Error('Unexpected page error or working chat');
    stats.passed = true;
  } catch (e) { stats.passed=false; stats.failureStage=String(e.message).split('\n')[0]; process.exitCode=1;
  } finally {
    if(browser) await browser.close();
    if(child) { child.kill(); await new Promise(resolve => child.exitCode !== null ? resolve() : child.once('exit',resolve)); }
    fs.writeFileSync(path.join(root,'counts.json'),JSON.stringify(stats,null,2));
    console.log(JSON.stringify(stats));
  }
})();
