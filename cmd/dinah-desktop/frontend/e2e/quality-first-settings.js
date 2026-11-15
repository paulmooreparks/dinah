const { measure: capture } = require('./lib/quality-frames');
async function measure(t, name, ...args) {
  const result = await capture(t, name, ...args);
  if (name.startsWith('frames-update-') || name === 'frames-restart-asking' || name === 'frames-restart-cancel') {
    const moving = result.changes.filter(frame => frame.moved.length);
    t.ok(name + ' keeps retained elements fixed through every frame', moving.length === 0);
  }
  return result;
}
const updates = require('./lib/updates-fixture'), aside = require('./lib/set-aside-fixture');
(async()=>{
  let t = await updates.start();
  try {
    const p=t.page, roots=['#settings','.win-bar','#ask'];
    await updates.settle(t); await p.locator("#updates-check").scrollIntoViewIfNeeded();
    const check = async(name,value) => {
      await p.evaluate(v=>{__u.release={current:'2.0.0',...v};__hold.Check=true;},value);
      await measure(t,name+'-checking',roots,()=>p.locator('#updates-check').evaluate(e=>e.click()),()=>p.waitForFunction(()=>__pending.Check?.length));
      await measure(t,name+'-reply',roots,()=>p.evaluate(()=>{__hold.Check=false;__pending.Check.splice(0).forEach(r=>r());}),()=>updates.settle(t));
    };
    await check('frames-update-current',{latest:'2.0.0',available:false});
    await check('frames-update-available',{latest:'2.0.1',available:true,notes:'# Changes\n\nA release note with a [link](https://example.com).\n\n| Item | Change |\n| --- | --- |\n| Chats | Kept |'});
    await p.evaluate(()=>{__hold.Download=true;});
    await measure(t,'frames-update-install-request',roots,()=>p.locator('#updates-install').evaluate(e=>e.click()),()=>p.waitForFunction(()=>__pending.Download?.length));
    await measure(t,'frames-update-downloading',roots,()=>p.evaluate(()=>{__hold.Download=false;__pending.Download.splice(0).forEach(r=>r());}),()=>p.waitForFunction(()=>document.querySelector('#updates-release-status').textContent.includes('Downloading')));
    await measure(t,'frames-update-progress',roots,()=>p.evaluate(()=>{__u.release.received=50;__u.release.size=100;}),()=>p.waitForFunction(()=>document.querySelector('#updates-progress').value===50));
    await measure(t,'frames-update-installed',roots,()=>p.evaluate(()=>{__u.release={current:'2.0.0',latest:'2.0.1',available:true,ready:true,installed:true,downloading:false};}),()=>p.waitForFunction(()=>document.querySelector('#updates-release-status').textContent.includes('is installed')));
    await p.evaluate(()=>{__fail.Check='Invented check failure';});
    await measure(t,'frames-update-error',roots,()=>p.locator('#updates-check').evaluate(e=>e.click()),()=>p.waitForFunction(()=>document.querySelector('#updates-release-status').textContent.includes('Could not check')));
  } finally { await t.close(); }
  t=await aside.start();
  try {
    const p=t.page, roots=['#settings','.win-bar','#ask'];
    await aside.check(t,1).check(); await p.locator("#set-aside-remove").scrollIntoViewIfNeeded();
    await measure(t,'frames-aside-confirm',roots,()=>p.locator('#set-aside-remove').evaluate(e=>e.click()),()=>p.locator('#ask[open]').waitFor());
    await p.evaluate(()=>{__asideHold.remove=true;});
    await measure(t,'frames-aside-removing',roots,()=>p.locator('#ask-yes').evaluate(e=>e.click()),()=>p.waitForFunction(()=>!!__asideReleaseRemove));
    await measure(t,'frames-aside-removed',roots,()=>p.evaluate(()=>{__asideHold.remove=false;__asideReleaseRemove();}),()=>aside.ready(t));
  } finally { await t.close(); }
})().catch(e=>{console.error(e);process.exitCode=1;});
