// The host's menus after the first two steps of their design
// (docs/design/menus.md): the bar reads Chat, Workspace, Documents, View,
// Window, Help; Chat has the commands of the chat in front and greys them
// when none is; View shows and hides and starts nothing; Window has the
// window commands once, Close among them; the themes are a set with one
// mark; the launcher lists no commands; and a window adds one menu with
// its kind's name, or none.
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
    t.ok('View then has the display switch, Refresh and the colours', view.slice(8).filter((r) => r !== 'Full Screen').join('|') === 'Show the Agent\'s Thinking|-|Refresh|-|Color Mode >|Themes >|Skin >', view.slice(8).join('|'));
    t.ok('View holds no command that starts or does something', !view.some((r) => /Decisions|Org Chart|Clean Up|Broadcast|Find and Open|Commands|Go to/.test(r)));
    const documents = await rows('[data-command-menu="documents"]');
    t.ok('Documents has Find and Open after Open, and no In Progress', documents.join('|').startsWith('New Document|Open…|Find and Open…') && !documents.includes('In Progress'));
    const help = await rows('[data-command-menu="help"]');
    t.ok('Help has Commands after its help rows', help.join('|') === 'Keyboard Shortcuts|Working with Chats|Commands…|-|About Dinah');
    const win = await rows('#host-windows');
    const at = win.indexOf('Next Window');
    t.ok('Window moves between windows, then documents, then has the two Go to rows', at >= 0 && win.slice(at, at + 7).join('|') === 'Next Window|Previous Window|Next Document|Previous Document|Go to the Question|Go to the Chat|-', win.join('|'));
    const workspaces = await rows('[data-command-menu="workspace"]');
    t.ok('Workspace begins with New Workspace and ends with Remove', workspaces.join('|') === 'New Workspace…|New Empty Workspace|-|Switch To >|-|Rename…|Refresh|Add a Source…|-|Import…|Export…|-|Move Section Up|Move Section Down|Reorder Sections|-|Remove…', workspaces.join('|'));
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
      return menu('Rename Chat…') === 'Chat' && menu('Delete Chat…') === 'Chat' && menu('Org Chart') === 'Tools' && menu('Clean Up Chats') === 'Tools' && menu('Remove Workspace…') === 'Workspace' && menu('Notifications') === 'View';
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
    // The bar: Chat and Workspace stand first, and nothing is in front.
    const titles = () => p.evaluate(() => Array.from(document.querySelectorAll('nav.menubar .menubar-title')).map((b) => b.textContent.trim()).filter(Boolean).join('|'));
    const frontTitles = () => p.evaluate(() => Array.from(document.querySelectorAll('nav.menubar .menubar-front .menubar-title')).map((b) => b.textContent.trim()).filter(Boolean).join('|'));
    const state = (menu) => p.evaluate((menu) => Array.from(document.querySelectorAll('[data-command-menu="' + menu + '"] > li > button')).map((b) => ({ label: b.textContent.trim(), greyed: b.disabled, reason: b.dataset.menuReason || '', key: b.dataset.shortcut || '' })), menu);
    const settled = () => p.evaluate(() => new Promise((resolve) => { window.dinahCommands.flush(); setTimeout(resolve, 60); }));
    t.ok('the bar reads Chat, Workspace, Documents, View, Window, Help', (await titles()) === 'Dinah|Chat|Workspace|Documents|View|Window|Help', await titles());
    // Chat with no chat in front: its rows are all there, and those that need a chat are greyed with the reason.
    const chat = await rows('[data-command-menu="chat"]');
    t.ok('Chat has its rows in order', chat.join('|') === 'New Chat|New Group Chat…|-|Rename…|Continue in Another Workspace…|Pin to Workspace|-|Provider|Mode|Model|Effort|-|Start a Group Chat…|Add to a Group Chat|Team|-|Prompt History…|My Templates…|Export…|Show the Chat Log|Show Failed Command Summaries|Subagents|-|Ask Me Again About a Full Context|Load Full History|Drop the Queued Messages|-|Stop|Restart the Agent|-|Delete…', chat.join('|'));
    const idle = await state('chat');
    const needy = ['Rename…', 'Continue in Another Workspace…', 'Provider', 'Model', 'Export…', 'Stop', 'Delete…'];
    t.ok('the rows that need a chat are greyed and say why', needy.every((label) => idle.some((r) => r.label === label && r.greyed && r.reason === 'No chat is in front.')), JSON.stringify(idle.filter((r) => needy.includes(r.label))));
    t.ok('the rows for chats as a whole are not greyed', ['New Chat', 'New Group Chat…', 'Prompt History…', 'My Templates…', 'Restart the Agent'].every((label) => idle.some((r) => r.label === label && !r.greyed)));
    t.ok('New Chat has Ctrl+N and Rename has F2', idle.some((r) => r.label === 'New Chat' && r.key === 'Mod+N') && idle.some((r) => r.label === 'Rename…' && r.key === 'F2'));
    t.ok('Commands lists a chat command by its full name', await p.evaluate(() => window.dinahCommands.searchable().some((c) => c.label === 'Rename Chat…' && c.menu === 'Chat' && c.shortcut === 'F2' && !c.enabled && c.reason === 'No chat is in front.')));
    // Ctrl+N starts a chat, and with it in front the provider is offered as a set with one mark.
    await p.keyboard.press('Control+n');
    await p.waitForFunction(() => window.pudlWindows.state().top === 'chat');
    await settled();
    t.ok('a chat window adds no menu of its own', (await frontTitles()) === '', await frontTitles());
    const provider = await p.evaluate(() => { const li = Array.from(document.querySelectorAll('[data-command-menu="chat"] > li')).find((li) => li.querySelector(':scope > ul') && /^Provider/.test(li.textContent.trim())); const ul = li && li.querySelector(':scope > ul'); return ul ? { radio: ul.hasAttribute('data-menu-radio'), on: ul.querySelectorAll('button[aria-checked="true"]').length } : null; });
    t.ok('with a chat in front, Provider is a set with one mark', !!provider && provider.radio && provider.on === 1, JSON.stringify(provider));
    t.ok('a chat that has not started says so on its rows', (await state('chat')).some((r) => r.label === 'Rename…' && r.greyed && r.reason === 'The chat starts with its first message.'));
    // Window has the commands for the window in front once, with Close last of them.
    const withChat = await rows('#host-windows');
    const close = withChat.indexOf('Close');
    t.ok('Close ends the block of commands for the window in front', close > 0 && withChat[close + 1] === '-' && withChat[close + 2] === 'Next Window' && withChat.slice(0, close).every((r) => r !== '-'), withChat.slice(0, close + 3).join('|'));
    // PUDL's window rows are in title case, as every label is, and a set of which one is on has radio marks.
    t.ok('the window rows are in title case', ['Pin to Taskbar', 'Reset Size and Position', 'Snap to a Zone >', 'Dock >'].every((r) => withChat.includes(r)) && !withChat.some((r) => /^(Pin to taskbar|Reset size|Snap to a zone)/.test(r)), withChat.slice(0, close).join('|'));
    const marks = await p.evaluate(() => {
      const list = document.getElementById('host-windows'), sub = (name) => { const li = Array.from(list.children).find((li) => li.querySelector(':scope > ul') && li.textContent.trim().startsWith(name)); return li ? li.querySelector(':scope > ul') : null; };
      const snap = sub('Snap to a Zone'), dock = sub('Dock');
      return { list: list.hasAttribute('data-menu-radio'), marked: Array.from(list.querySelectorAll(':scope > li > button[aria-checked]')).map((b) => b.textContent.trim()), on: list.querySelectorAll(':scope > li > button[aria-checked="true"]').length,
        snap: !!snap && snap.hasAttribute('data-menu-radio'), dock: !!dock && dock.hasAttribute('data-menu-radio'), zones: snap ? Array.from(snap.querySelectorAll('button')).map((b) => b.textContent.trim()).slice(0, 5).join('|') : '' };
    });
    t.ok('the list of open windows is a set with one mark, and only its rows are marked', marks.list && marks.on === 1 && marks.marked.length > 1 && marks.marked.every((l) => /^[1-9][.] /.test(l)), JSON.stringify(marks));
    t.ok('the zones and the dock edges are sets with radio marks, in title case', marks.snap && marks.dock && marks.zones === 'Left Half|Right Half|Top Half|Bottom Half|Top Left Quarter', JSON.stringify(marks));
    // The panes add no menu, and View, Refresh reads a list again where the window in front has one.
    const refresh = async () => (await state('view')).find((r) => r.label === 'Refresh');
    t.ok('Refresh is greyed over a chat and says why', (await refresh()).greyed && (await refresh()).reason === 'Nothing in front is a list that can be read again.' && (await refresh()).key === 'F5');
    for (const pane of ['chat-browser', 'workspace', 'needs-you', 'in-progress', 'skills-browser']) {
      await p.evaluate((pane) => window.dinahWorkbench.open(pane), pane);
      await p.waitForFunction((pane) => window.pudlWindows.state().top === pane, pane);
      await settled();
      t.ok('the ' + pane + ' pane adds no menu', (await frontTitles()) === '', await frontTitles());
      t.ok('Refresh is ' + (pane === 'needs-you' ? 'greyed' : 'offered') + ' over the ' + pane + ' pane', (await refresh()).greyed === (pane === 'needs-you'));
    }
    await p.evaluate(() => window.dinahWorkbench.open('chat-browser'));
    await p.waitForFunction(() => window.pudlWindows.state().top === 'chat-browser');
    await settled();
    const asked = (await t.calls('DeclaredProviders')).length;
    await p.keyboard.press('F5');
    await p.waitForFunction((n) => window.__bridgeCalls.filter((c) => c[0] === 'DeclaredProviders').length > n, asked);
    t.ok('F5 reads the chats again over the Chats pane', true);
    // A tool's one menu has its kind's name, and no menu is called Actions.
    await p.evaluate(() => window.dinahWorkbench.open('bus'));
    await p.waitForFunction(() => window.pudlWindows.state().top === 'bus');
    await settled();
    t.ok('Group Chats adds one menu with its name', (await frontTitles()) === 'Group Chats', await frontTitles());
    await p.locator('nav.menubar .menubar-front .menubar-title', { hasText: 'Group Chats' }).click();
    const own = await p.evaluate(() => Array.from(document.querySelectorAll('#menubar-panel > .menu-action .menubar-words')).map((w) => w.textContent.trim()));
    t.ok('its menu has its own commands and no window commands', own.includes('New Message…') && own.includes('Trust a Link…') && !own.includes('New Group Chat…') && !own.some((l) => /^(Close|Dock|Minimize|Pin to Taskbar)/.test(l)), own.join('|'));
    await p.keyboard.press('Escape');
    await p.evaluate(() => window.dinahCommands.run('own-shell'));
    await p.waitForFunction(() => Array.from(document.querySelectorAll('nav.menubar .menubar-front .menubar-title')).some((b) => b.textContent.trim() === 'Terminal'));
    t.ok('a Terminal adds one menu called Terminal', (await frontTitles()) === 'Terminal', await frontTitles());
    t.ok('no menu is called Actions', !(await titles()).split('|').includes('Actions'));
    await t.shot('host-menus-order');
  } finally { await t.close(); }
})();
