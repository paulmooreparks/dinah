// The host's menus after their first tidy (docs/design/menus.md): View shows
// and hides and starts nothing, the commands that left it have a row
// elsewhere, the themes are a set with one mark, and the launcher lists no
// commands.
const { launch } = require('./lib/page');
(async () => {
  const t = await launch(); const p = t.page;
  try {
    await p.waitForFunction(() => document.querySelector('[data-command-menu="view"] li[data-generated]'));
    // A menu's rows as PUDL will read them: a label, "-" for a separator,
    // "[words]" for a heading, and "words >" for a cascade.
    const rows = (selector) => p.evaluate((selector) => Array.from(document.querySelector(selector).children).map((li) => {
      if (li.hasAttribute('data-commands')) return null;
      const nested = li.querySelector(':scope > ul'), button = li.querySelector(':scope > button, :scope > a');
      if (nested) return Array.from(li.childNodes).filter((n) => n !== nested).map((n) => n.textContent).join('').trim() + ' >';
      if (button) return button.textContent.trim();
      const words = li.textContent.trim();
      return words === '-' ? '-' : '[' + words + ']';
    }).filter((r) => r !== null), selector);
    const view = await rows('[data-command-menu="view"]');
    t.ok('View begins with the six panes under their heading', view.slice(0, 8).join('|') === '[Panes]|Workspace|Chats|In Progress|Skills|Notifications|Group Chats|-');
    // Full Screen is the native window's, which this page stands in for without.
    t.ok('View then has the display switches and the colours', view.slice(8).filter((r) => r !== 'Full Screen').join('|') === 'Show the Agent\'s Thinking|-|Color Mode >|Themes >|Skin >');
    t.ok('View holds no command that starts or does something', !view.some((r) => /Decisions|Org Chart|Clean Up|Broadcast|Find and Open|Commands|Go to/.test(r)));
    const documents = await rows('[data-command-menu="documents"]');
    t.ok('Documents has Find and Open after Open, and no In Progress', documents.join('|').startsWith('New Document|Open…|Find and Open…') && !documents.includes('In Progress'));
    const help = await rows('[data-command-menu="help"]');
    t.ok('Help has Commands after its help rows', help.join('|') === 'Keyboard Shortcuts|Working with Chats|Commands…|-|About Dinah');
    const win = await rows('#host-windows');
    const at = win.indexOf('Previous Window');
    t.ok('Window has the two Go to rows after Previous Window', at >= 0 && win[at + 1] === 'Go to the Question' && win[at + 2] === 'Go to the Chat' && win[at + 3] === '-');
    const workspaces = await rows('[data-command-menu="workspace"]');
    t.ok('Workspaces has Refresh Workspace and Remove Workspace', workspaces.includes('Refresh Workspace') && workspaces.join('|').includes('-|Remove Workspace…|-|Recent Workspaces >'));
    t.ok('Remove Workspace is marked dangerous', await p.locator('[data-command-menu="workspace"] [data-cmd="remove-workspace"][data-menu-danger]').count() === 1);
    // The themes: one mark among them, a heading in place of "(built in)".
    const themes = await p.evaluate(() => {
      const list = document.getElementById('host-themes');
      return { radio: list.hasAttribute('data-menu-radio'), on: list.querySelectorAll('button[aria-checked="true"]').length, first: list.children[0].textContent.trim(),
        labels: Array.from(list.querySelectorAll('button')).map((b) => b.textContent.trim()) };
    });
    t.ok('the themes are a set with one mark', themes.radio && themes.on === 1);
    t.ok('the built-in themes stand under a heading', themes.first === 'Built In' && !themes.labels.some((l) => /built in/i.test(l)));
    t.ok('Edit Current Theme asks for nothing more and has no ellipsis', themes.labels.includes('Edit Current Theme') && themes.labels.includes('Appearance Settings…'));
    // The keys keep working from their new menus.
    t.ok('the moved commands keep their keys', await p.evaluate(() => {
      const all = window.dinahCommands.searchable(), key = (label) => (all.find((c) => c.label === label) || {}).shortcut;
      return key('Find and Open…') === 'Ctrl+P' && key('Commands…') === 'Ctrl+Shift+P' && key('Go to the Question') === 'F9' && key('Go to the Chat') === 'Ctrl+J';
    }));
    t.ok('Commands files chat commands under Chat and tools under Tools', await p.evaluate(() => {
      const all = window.dinahCommands.searchable(), menu = (label) => (all.find((c) => c.label === label) || {}).menu;
      return menu('Rename Chat…') === 'Chat' && menu('Delete Chat…') === 'Chat' && menu('Org Chart') === 'Tools' && menu('Clean Up Chats') === 'Tools' && menu('Remove Workspace…') === 'Workspaces' && menu('Notifications') === 'View';
    }));
    // The launcher lists no commands.
    const launcher = await p.evaluate(() => ({
      names: Array.from(document.querySelectorAll('#dinah-launcher [data-start-item] .start-name')).map((n) => n.textContent.trim()),
      categories: Array.from(document.querySelectorAll('#dinah-launcher [data-start-category-key]')).map((a) => a.dataset.startCategoryKey)
    }));
    t.ok('the launcher lists no commands', !launcher.names.some((n) => /^New |Import Workspace|Open Document|Find a Tool|Keyboard Shortcuts|^About |Refresh Shells/.test(n)));
    t.ok('the launcher keeps its applications, panes and the Terminal', ['Docs', 'Editor', 'Files', 'Chats', 'Workspace', 'Default Shell', 'Install Packages…'].every((n) => launcher.names.includes(n)));
    t.ok('the launcher has no Help category', !launcher.categories.includes('help'));
    // Its categories (Paul, 19 November 2026): the panes have one of their own, and one with nothing in it is not listed.
    const cats = await p.evaluate(() => {
      const of = (key) => Array.from(document.querySelectorAll('#dinah-launcher [data-start-item][data-start-category="' + key + '"] .start-name')).map((n) => n.textContent.trim()).sort().join('|');
      return { panes: of('panes'), chats: of('chats'), settings: of('settings'), documents: of('documents'),
        listed: Array.from(document.querySelectorAll('#dinah-launcher [data-start-category-key]')).filter((a) => !a.closest('li').hidden).map((a) => a.dataset.startCategoryKey).join('|') };
    });
    t.ok('the six panes are under Panes', cats.panes === 'Chats|Group Chats|In Progress|Notifications|Skills|Workspace', cats.panes);
    t.ok('the chat tools are under Chats and the settings tools under Settings', cats.chats === 'Clean Up Chats|Decisions|Org Chart' && cats.settings === 'Extensions|Install Packages…|Theme Editor', cats.chats + ' / ' + cats.settings);
    t.ok('Files is with the document applications', cats.documents.split('|').includes('Files') && cats.documents.split('|').includes('Docs'), cats.documents);
    t.ok('an empty category is not listed', cats.listed === '|documents|chats|panes|terminals|settings', cats.listed);
    t.ok('the way out is called Exit', await p.evaluate(() => document.querySelector('#dinah-launcher [data-cmd="quit"]').textContent.trim() === 'Exit Dinah' && window.dinahCommands.describe('quit').label === 'Exit Dinah'));
    // The Chats pane's own menu has Broadcast.
    await p.evaluate(() => window.dinahWorkbench.open('chat-browser'));
    await p.waitForFunction(() => Array.from(document.querySelectorAll('nav.menubar .menubar-front .menubar-title')).some((b) => b.textContent.trim() === 'Actions'));
    await p.locator('nav.menubar .menubar-front .menubar-title', { hasText: 'Actions' }).click();
    const actions = await p.evaluate(() => Array.from(document.querySelectorAll('#menubar-panel > .menu-action .menubar-words')).map((w) => w.textContent.trim()));
    t.ok('the Chats pane\'s Actions has the two Broadcast rows', actions.includes('Broadcast to Active…') && actions.includes('Broadcast to All…'));
    await p.keyboard.press('Escape');
    await t.shot('host-menus-order');
  } finally { await t.close(); }
})();
