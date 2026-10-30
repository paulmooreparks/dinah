// The person attaches files to a group chat's messages as in a single chat
// (docs/plans/group-attachments.md). The group's box takes a pasted
// picture, files dropped on the window and files chosen with Attach…, as
// the chat's chips with Remove; a file over 25 MB is refused when it is
// added. Sending gives the files to the window's send, and a message of
// files alone may be sent. A sent message shows its pictures as thumbnails
// the media viewer opens and its other files as links that open them, and
// a removed file as removed. The files are not kept in the draft. Settings
// removes the attachments older than an age, saying how many and how much
// space before it asks.
const path = require('path');
const { launch, OUT } = require('./lib/page');
(async () => {
  const t = await launch({
    viewport: { width: 1440, height: 900 },
    state: { workspace: 'C:/Work/docs', workspaces: ['C:/Work/docs'], recent: ['C:/Work/docs'], promptSendKey: 'enter' },
    app: () => {
      // A one-pixel PNG, so that a thumbnail loads and the viewer opens it.
      const pixel = 'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg==';
      window.__pixel = pixel;
      const msg = (id, from, text, extra) => Object.assign({ id, createdAt: '2026-10-28T08:00:00Z', kind: 'text', from, fromPerson: from === 'Paul', fromSession: from === 'Paul' ? '' : 'window-' + id, fromWorkspace: from === 'Paul' ? '' : 'docs', to: 'Release Review', room: 'release-review', roomTitle: 'Release Review', mentions: [], replyTo: 0, text, attachments: [], unread: false }, extra);
      const person = { id: 'paul', chat: '', title: 'Paul', workspace: '', address: '', state: 'person' };
      const docs = { id: 'window-d1', chat: 'chat-docs', title: 'Docs Pass', workspace: 'docs', address: 'Docs Pass (docs)', state: 'open' };
      const messages = [
        msg(1, 'Docs Pass', 'Ready when you are.'),
        msg(2, 'Paul', 'Here are the shots.', { attachments: [
          { index: 1, rel: 'shot.png', size: 70, mime: 'image/png', stored: true, picture: true, src: 'data:image/png;base64,' + pixel, uri: 'file:///C:/Users/paul/.dinah/attachments/2026-10-28/shot.png' },
          { index: 2, rel: 'notes.txt', size: 2048, mime: 'text/plain', stored: true, uri: 'file:///C:/Users/paul/.dinah/attachments/2026-10-28/notes.txt' },
          { index: 3, rel: 'old.pdf', size: 4096, mime: 'application/pdf', stored: true, removed: true }] })
      ];
      window.__messages = messages;
      let next = 200;
      return {
        ReadUIState: async () => ({ version: 1, values: { 'dinah-workbench-layout': '?open=workspace' } }), SaveUIState: async () => {},
        BusOverview: async () => ({ inbox: [], unread: 0, rooms: [{ name: 'release-review', title: 'Release Review', lastAt: '', unread: 0, members: [docs, person] }], workspaces: [{ name: 'docs', label: 'docs', path: 'C:/Work/docs', chats: [{ id: 'window-d1', chat: 'chat-docs', title: 'Docs Pass' }] }], links: [], defaultHops: 10, defaultMax: 3, noHopLimit: -1 }),
        BusGroupMessages: async () => messages.slice(),
        BusMarkGroupRead: async () => {},
        PickAttachments: async () => { __calls.push(['pick']); return [{ kind: 'file', name: 'picked.csv', mimeType: 'text/csv', data: btoa('a,b\n1,2\n'), size: 8 }]; },
        // Go turns a file address into its path and says where it opens
        // (document_links.go); here, the system's program.
        ResolveDocumentLink: async (cwd, ref) => ({ ref, found: true, opens: 'system', path: decodeURIComponent(ref.replace(/^file:\/\/\//i, '')).replace(/\//g, '\\') }),
        OpenPath: async p => { __calls.push(['open', p]); return 'opened'; },
        BusSendToGroup: async (key, text, mentions, files) => {
          __calls.push(['group-send', key, text, mentions, files]);
          const m = msg(next++, 'Paul', text, { attachments: files.map((f, i) => ({ index: i + 1, rel: f.name, size: atob(f.data).length, mime: f.mimeType, stored: true, uri: 'file:///C:/Users/paul/.dinah/attachments/2026-10-28/' + f.name })) });
          messages.push(m);
          return m;
        },
        OldAttachments: async days => { __calls.push(['count', days]); return days === 7 ? { files: 0, bytes: 0 } : { files: 2, bytes: 3 * 1048576 }; },
        RemoveOldAttachments: async days => { __calls.push(['remove', days]); return { files: 2, bytes: 3 * 1048576 }; }
      };
    },
    init: () => { window.__calls = []; window.__toasts = []; },
  });
  const { page: p, ok } = t;
  try {
    await p.locator('[data-win="workspace"]').waitFor({ state: 'attached' });
    await p.evaluate(() => { const t = window.pudlToast; window.pudlToast = (text, o) => { __toasts.push(String(text)); return t && t(text, o); }; });
    await p.evaluate(() => dinahBus.openGroup('release-review'));
    const room = p.locator('.win[data-win-kind="bus-room"]');
    await room.locator('[data-bus-message="2"]').waitFor();
    const box = room.locator('textarea');
    const chips = room.locator('.group-attachments .attachment');
    const calls = kind => p.evaluate(k => __calls.filter(c => c[0] === k), kind);

    // Paste, drop and Attach….
    await box.fill('With files');
    await box.evaluate((el, pixel) => {
      const bytes = Uint8Array.from(atob(pixel), c => c.charCodeAt(0));
      const dt = new DataTransfer(); dt.items.add(new File([bytes], 'pasted.png', { type: 'image/png' }));
      el.dispatchEvent(new ClipboardEvent('paste', { clipboardData: dt, bubbles: true, cancelable: true }));
    }, await p.evaluate(() => __pixel));
    await chips.first().waitFor();
    ok('a pasted picture is a chip with its thumbnail, name and size', await chips.count() === 1 && await chips.first().locator('img').count() === 1 &&
      await chips.first().locator('.attachment-name').innerText() === 'pasted.png' && await box.inputValue() === 'With files');
    await room.locator('.bus-conversation').evaluate(el => {
      const dt = new DataTransfer(); dt.items.add(new File(['dropped words'], 'dropped.txt', { type: 'text/plain' }));
      el.dispatchEvent(new DragEvent('dragover', { dataTransfer: dt, bubbles: true, cancelable: true }));
      el.dispatchEvent(new DragEvent('drop', { dataTransfer: dt, bubbles: true, cancelable: true }));
    });
    await p.waitForFunction(() => document.querySelectorAll('.win[data-win-kind="bus-room"] .group-attachments .attachment').length === 2);
    ok('a file dropped on the window is a chip, and the outline goes', await chips.nth(1).locator('.attachment-name').innerText() === 'dropped.txt' &&
      !(await room.locator('.bus-conversation').getAttribute('class')).includes('dropping'));
    await room.getByRole('button', { name: 'Attach…' }).click();
    await p.waitForFunction(() => document.querySelectorAll('.win[data-win-kind="bus-room"] .group-attachments .attachment').length === 3);
    ok('Attach… adds the files chosen on the computer', (await calls('pick')).length === 1 && await chips.nth(2).locator('.attachment-name').innerText() === 'picked.csv');
    await box.evaluate(el => {
      const dt = new DataTransfer(); dt.items.add(new File([new Uint8Array(25 * 1024 * 1024 + 1)], 'huge.bin', { type: 'application/octet-stream' }));
      el.dispatchEvent(new ClipboardEvent('paste', { clipboardData: dt, bubbles: true, cancelable: true }));
    });
    await p.waitForFunction(() => __toasts.some(t => t.includes('huge.bin is larger than 25 MB')));
    ok('a file over 25 MB is refused when added, with the reason', await chips.count() === 3);
    const refused = room.locator('.group-attachment-error');
    await refused.waitFor();
    // Removing the transient toast leaves the actionable failure in the composer.
    await p.evaluate(() => document.querySelectorAll('.toast').forEach(n => n.remove()));
    ok('the attachment failure stays in a danger notice after the toast is gone', await refused.isVisible() && /huge.bin/.test(await refused.locator('.notice-body').textContent()));
    await t.shot('group-attachments-refused');
    await refused.getByRole('button', { name: 'Dismiss' }).click();
    ok('the reader dismisses the failure without changing the attached files', await refused.count() === 0 && await chips.count() === 3);

    await chips.nth(1).getByRole('button', { name: 'Remove dropped.txt' }).click();
    ok('Remove takes a chip away', await chips.count() === 2 && !(await room.locator('.group-attachments').innerText()).includes('dropped.txt'));

    // The draft keeps its words but not its files.
    const key = await room.getAttribute('data-win');
    await p.evaluate(k => pudlWindows.close(k), key);
    await room.waitFor({ state: 'detached' });
    await p.evaluate(() => dinahBus.openGroup('release-review'));
    await room.locator('[data-bus-message="2"]').waitFor();
    ok('reopening the group keeps the words of the draft but not its files', await box.inputValue() === 'With files' && await chips.count() === 0 && await room.locator('.group-attachments').isHidden());

    // Sending.
    await box.fill('');
    await box.evaluate((el, pixel) => {
      const bytes = Uint8Array.from(atob(pixel), c => c.charCodeAt(0));
      const dt = new DataTransfer(); dt.items.add(new File([bytes], 'only.png', { type: 'image/png' }));
      el.dispatchEvent(new ClipboardEvent('paste', { clipboardData: dt, bubbles: true, cancelable: true }));
    }, await p.evaluate(() => __pixel));
    await chips.first().waitFor();
    ok('Send is offered for a message of files alone', await room.getByRole('button', { name: 'Send', exact: true }).isEnabled());
    await box.press('Enter');
    await p.waitForFunction(() => __calls.some(c => c[0] === 'group-send'));
    const sent = (await calls('group-send'))[0];
    ok('the files go to the group\'s send with their names, types and contents', sent[2] === '' && sent[4].length === 1 && sent[4][0].name === 'only.png' &&
      sent[4][0].mimeType === 'image/png' && sent[4][0].kind === 'file' && sent[4][0].data === await p.evaluate(() => __pixel), JSON.stringify(sent).slice(0, 300));
    await p.waitForFunction(() => document.querySelectorAll('.win[data-win-kind="bus-room"] .group-attachments .attachment').length === 0);
    ok('sending empties the chips', await room.locator('.group-attachments').isHidden());

    // A sent message's attachments.
    const shown = room.locator('[data-bus-message="2"] .msg-attachments');
    const thumb = shown.locator('.sent-images img');
    await thumb.waitFor();
    ok('a picture is shown as a thumbnail under the text', await thumb.count() === 1 && await thumb.getAttribute('alt') === 'shot.png');
    await p.waitForFunction(() => !!document.querySelector('.win[data-win-kind="bus-room"] [data-bus-message="2"] .sent-images img[data-media-expand]'));
    await thumb.click();
    await p.locator('#media-viewer').waitFor({ state: 'visible' });
    ok('the thumbnail opens in the media viewer', await p.locator('#media-viewer .lightbox-img').count() === 1);
    await p.keyboard.press('Escape');
    await p.locator('#media-viewer').waitFor({ state: 'hidden' });
    const link = shown.locator('a.smart-chip', { hasText: 'notes.txt' });
    ok('another file is a link with its size', await link.count() === 1 && (await shown.locator('.msg-file').first().innerText()).includes('2 KB'));
    await link.click();
    await p.waitForFunction(() => __calls.some(c => c[0] === 'open'));
    ok('the link opens the file as a single chat\'s files open', (await calls('open'))[0][1] === 'C:\\Users\\paul\\.dinah\\attachments\\2026-10-28\\notes.txt', JSON.stringify(await calls('open')));
    const removed = shown.locator('.msg-file-removed');
    ok('a removed file keeps its name, marked removed, and opens nothing', await removed.count() === 1 && (await removed.innerText()).includes('old.pdf') &&
      (await removed.innerText()).includes('Removed') && await removed.locator('a').count() === 0);
    ok('the sent message with a picture alone is drawn', await room.locator('[data-bus-message="200"] .msg-files a', { hasText: 'only.png' }).count() === 1);
    await room.screenshot({ path: path.join(OUT, 'group-attachments-window.png') });

    // Settings: remove the attachments older than an age.
    await p.evaluate(() => dinahWorkbench.open('settings'));
    const settings = p.locator('#settings');
    await settings.getByRole('tab', { name: 'Chat', exact: true }).click();
    const remove = settings.getByRole('button', { name: 'Remove Attachments…' });
    ok('Settings offers the command, with an age to choose', await remove.isVisible() && await settings.locator('#set-attachment-age').inputValue() === '90');
    await settings.locator('#set-attachment-age').selectOption('7');
    await remove.click();
    await p.waitForFunction(() => __toasts.some(t => t.includes('nothing to remove')));
    ok('with nothing that old, it says so and asks nothing', !(await p.locator('#ask').isVisible()) && (await calls('remove')).length === 0);
    await settings.locator('#set-attachment-age').selectOption('30');
    await remove.click();
    await p.locator('#ask').waitFor({ state: 'visible' });
    const body = await p.locator('#ask-body').innerText();
    ok('it asks first, saying how many files and how much space', await p.locator('#ask-title').innerText() === 'Remove 2 Attachments?' && body.includes('2 files') && body.includes('3.0 MB') && body.includes('older than 1 month'), body);
    await p.locator('#ask-no').click();
    ok('Cancel removes nothing', (await calls('remove')).length === 0);
    await remove.click();
    await p.locator('#ask').waitFor({ state: 'visible' });
    await p.locator('#ask-yes').click();
    await p.waitForFunction(() => __toasts.some(t => t.startsWith('Removed 2 files')));
    ok('confirming removes them and says what was freed', JSON.stringify(await calls('remove')) === JSON.stringify([['remove', 30]]));
    await t.shot('group-attachments');
  } finally { await t.close(); }
})();
