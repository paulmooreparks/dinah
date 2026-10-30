// Pressing a command of a folding toolbar as a person would: its control
// when the toolbar shows it, or, when its group has folded, the row of the
// menu it folded into (the group's own menu, or More at the row's end).
// scope is a Playwright locator around the toolbar, such as a window.
//
// A control is drawn from its command in a frame after it enters the page,
// and the toolbar folds again in the frame after that, so a control is
// looked at only once it has been drawn and the toolbar has settled.
async function settled(page, scope, id) {
  const ctl = scope.locator('[data-command="' + id + '"]').first();
  await ctl.waitFor({ state: 'attached' });
  await ctl.locator(':scope > .toolbar-words').or(ctl.locator(':scope[data-command-own]')).first().waitFor({ state: 'attached' });
  await page.evaluate(() => new Promise(r => requestAnimationFrame(() => requestAnimationFrame(r))));
  return ctl;
}
async function press(page, scope, id) {
  const ctl = await settled(page, scope, id);
  if (await ctl.isVisible()) return ctl.click();
  const found = await ctl.evaluate(el => {
    const cmd = window.pudlCommands.find(el.dataset.command, el), group = el.closest('.toolbar-group');
    return { label: window.pudlCommands.value(cmd, 'label') || el.dataset.command, stage: group ? group.dataset.toolbarStage : '' };
  });
  if (found.stage === '5') await ctl.locator('xpath=ancestor::*[contains(concat(" ", @class, " "), " toolbar ")][1]').locator(':scope > .toolbar-more > .menu-btn').click();
  else await ctl.locator('xpath=ancestor::*[contains(concat(" ", @class, " "), " toolbar-group ")][1]').locator(':scope > .toolbar-fold > .menu-btn').click();
  await page.locator('.menu-panel:popover-open .menu-action', { hasText: found.label }).first().click();
}
// Opening the menu that holds a menu command's rows: its own menu button
// when the toolbar shows it, or the menu its group folded into, where the
// rows are listed under the group's name. Returns the open menu.
async function openMenu(page, scope, id) {
  const ctl = await settled(page, scope, id);
  if (await ctl.isVisible()) await ctl.click();
  else {
    const stage = await ctl.evaluate(el => { const g = el.closest('.toolbar-group'); return g ? g.dataset.toolbarStage : ''; });
    if (stage === '5') await ctl.locator('xpath=ancestor::*[contains(concat(" ", @class, " "), " toolbar ")][1]').locator(':scope > .toolbar-more > .menu-btn').click();
    else await ctl.locator('xpath=ancestor::*[contains(concat(" ", @class, " "), " toolbar-group ")][1]').locator(':scope > .toolbar-fold > .menu-btn').click();
  }
  const menu = page.locator('.menu-panel:popover-open');
  await menu.waitFor();
  return menu;
}
// Choosing a row of a menu command, by the row's words, whether the menu
// is shown or folded. Folded, the command is a submenu of its group's menu,
// which is opened first.
async function choose(page, scope, id, words) {
  const menu = await openMenu(page, scope, id);
  const label = await scope.locator('[data-command="' + id + '"]').first().evaluate(el => {
    const cmd = window.pudlCommands.find(el.dataset.command, el);
    return window.pudlCommands.value(cmd, 'label') || el.dataset.command;
  });
  const direct = menu.locator('.menu-action', { hasText: words });
  if (!await direct.count()) await menu.locator('.menu-action', { hasText: label }).first().click();
  await page.locator('.menu-panel:popover-open .menu-action', { hasText: words }).last().click();
}
// The words a command's control shows now, once its command has drawn it.
async function shown(scope, id) {
  const words = scope.locator('[data-command="' + id + '"] .toolbar-words').first();
  await words.waitFor({ state: 'attached' });
  return (await words.textContent()).trim();
}
// The words of a menu command's rows, with its menu left open: all of its
// own menu's rows, or, when its group has folded, the rows under the
// command's heading in the group's menu.
async function rowsOf(page, scope, id) {
  const menu = await openMenu(page, scope, id);
  // The labels of the commands in the command's group, whose headings
  // divide a folded group's menu; a heading among a command's own rows is
  // not one of them.
  const [label, labels, folded] = await scope.locator('[data-command="' + id + '"]').first().evaluate(el => {
    const C = window.pudlCommands, labelOf = b => { const c = C.find(b.dataset.command, b); return (c && C.value(c, 'label')) || b.dataset.command; };
    const group = el.closest('.toolbar-group');
    return [labelOf(el), group ? Array.from(group.querySelectorAll('[data-command]:not([hidden])'), labelOf) : [], !el.getClientRects().length];
  });
  return menu.evaluate((panel, [label, labels, folded]) => {
    const out = [];
    // A folded group of one menu lists that menu's rows with no heading.
    let mine = !folded || labels.length === 1;
    for (const el of panel.children) {
      if (folded && labels.length > 1 && el.classList.contains('md-section-label') && labels.includes(el.textContent)) { mine = el.textContent === label; continue; }
      if (mine && el.classList.contains('menu-action')) out.push(el.querySelector('.menu-words').textContent);
    }
    return out;
  }, [label, labels, folded]);
}
// Whether a command's control comes to show these words within a few
// seconds; its control is drawn again in the frame after a change.
function shows(scope, id, words) {
  const exact = new RegExp('^\\s*' + words.replace(/[.*+?^${}()|[\]\\]/g, '\\$&') + '\\s*$');
  return scope.locator('[data-command="' + id + '"] .toolbar-words').filter({ hasText: exact }).first()
    .waitFor({ state: 'attached', timeout: 3000 }).then(() => true, () => false);
}
module.exports = { press, openMenu, choose, shown, shows, rowsOf };
