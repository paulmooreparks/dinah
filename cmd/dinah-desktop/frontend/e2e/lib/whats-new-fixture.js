const { launch } = require('./page');
const id = 'whats-new-20261114';
const feature = { id, kind: 'app', severity: 'info', title: "What's New", body: 'Decisions keeps questions and recommendations in one place. Working with Chats explains Broadcast and Pause Chats and Restart. Settings includes Background Chats and Set-aside Programs.', details: [], created: '2026-11-14T00:00:00Z', dismissed: false };
async function start(options = {}) {
  const t = await launch({ nativeScrollbars: true, args: { feature, ...options }, app: data => {
    const clone = value => JSON.parse(JSON.stringify(value));
    if (!sessionStorage.getItem('whats-new-items')) sessionStorage.setItem('whats-new-items', JSON.stringify([{ id: 'earlier-report', kind: 'app', severity: 'info', title: 'An earlier report.', body: 'This existing card keeps its place.', details: [], created: '2026-11-13T00:00:00Z' }, ...(data.later ? [] : [data.feature])]));
    window.__feature = data.feature; window.__featureStore = JSON.parse(sessionStorage.getItem('whats-new-items'));
    window.__featureUpdates = !!data.updates; window.__featureHold = !!data.hold;
    window.__featurePending = [];
    const keep = () => sessionStorage.setItem('whats-new-items', JSON.stringify(__featureStore));
    return {
      Notifications: async () => clone(__featureStore.filter(item => !item.dismissed)),
      DismissNotification: async id => { __featureStore.find(item => item.id === id).dismissed = true; keep(); },
      ClearNotifications: async () => { __featureStore.forEach(item => { item.dismissed = true; }); keep(); },
      ProductServices: async () => {
        if (__featureHold) await new Promise(resolve => { __featurePending.push(resolve); window.__featureRelease = () => __featurePending.splice(0).forEach(done => done()); });
        if (window.__featureDiscoveryFailure) throw new Error('Synthetic discovery failure');
        return __featureUpdates ? ['updates'] : [];
      },
      Decisions: async () => ({ items: [], nextBefore: 0 }),
      ListSetAsidePrograms: async () => ({ items: [], totalBytes: 0, warnings: [] }),
      RemoveSetAsidePrograms: async () => { throw new Error('Reading a feature card must not remove programs.'); },
      CallService: async (_, method) => {
        const values = { Status: { installed: [], running: [], behind: 0 }, Check: { current: '1.0.0', latest: '1.0.0', available: false } };
        return JSON.stringify(values[method] || {});
      }
    };
  } });
  try { await t.page.evaluate(() => dinahWorkbench.open('needs-you')); return t; }
  catch (error) { await t.close(); throw error; }
}
const card = t => t.page.locator('#questions .whats-new-card');
async function quiet(t) {
  const writes = (await t.calls()).filter(call => /^(AnswerDecision|ConfirmDecisionBroadcast|PauseAndRestart|RestartToUpdate|SetBackgroundChatLimit|RemoveSetAsidePrograms|InstallPackages|InstallPersonalPackages|ApplyUpdate)$/.test(call[0]));
  writes.push(...(await t.calls('CallService')).filter(call => !['Status', 'Check'].includes(call[2])));
  t.ok('feature links perform no answer, restart, setting write or removal', writes.length === 0, JSON.stringify(writes));
  t.ok('no native or external action', !(await t.page.evaluate(() => __runtime)).length);
  await t.close();
}
module.exports = { start, card, quiet, feature, id };
