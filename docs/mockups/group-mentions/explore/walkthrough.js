// Uses the group mentions mockup headless the way Paul would and checks it:
// the @ list offers members first and then everything a one-on-one chat
// offers; the keyboard picks; a member mentioned is asked and changes who the
// message reaches, a reference does not; the sent message shows the
// difference in its marks and in words; a team; the phone.
//   pwsh scripts/with-test-lock.ps1 node docs/mockups/group-mentions/explore/walkthrough.js
const { PAGE, withBrowser, newPage } = require('./common');
let failures = 0;
function check(ok, what) { console.log((ok ? 'ok   ' : 'FAIL ') + what); if (!ok) failures++; }
withBrowser(async browser => {
  const page = await newPage(browser, 1440, 900);
  page.setDefaultTimeout(8000);
  const settle = () => page.waitForFunction(() => document.documentElement.dataset.mockSettled === 'true');
  const heads = () => page.evaluate(() => [...document.querySelectorAll('#group-pop .md-section-label')].map(h => h.textContent));
  const rows = () => page.evaluate(() => [...document.querySelectorAll('#group-pop [data-mention]')].map(r => r.dataset.mention));
  const reach = () => page.locator('.group-reach').textContent();
  const box = page.locator('#group-text');

  await page.goto(PAGE + '?state=index');
  check(await page.evaluate(() => document.body.className === 'index'), '?state=index lists the states');
  await page.goto(PAGE); await settle();
  check(await page.locator('#group-pop').isVisible(), 'the plain file path opens on a group chat with the @ list open');
  check(JSON.stringify(await heads()) === JSON.stringify(['Members, asked to answer', 'Windows', 'Applets', 'Open Documents', 'Other Chats, referred to and not asked']), 'the list offers members first, then windows, applets, open documents and other chats');
  const all = await rows();
  check(all.includes('member:c02') && all.includes('everyone:everyone') && all.includes('window:docs') && all.includes('applet:git') && all.includes('document:d1') && all.includes('chat:c14'), 'it has every kind a one-on-one chat offers, and the group’s members and everyone');
  check(!all.includes('chat:c02'), 'a member chat is listed once, as a member');

  // Typing searches all of them, and the keyboard picks.
  await box.fill(''); await box.type('Please see @rel');
  check(JSON.stringify(await rows()) === JSON.stringify(['document:d1']), 'typing @rel finds the open document');
  await page.keyboard.press('Enter');
  check((await box.inputValue()) === 'Please see @release-notes.md ' && await page.locator('#group-pop').isHidden(), 'Enter puts the reference in the box and closes the list');
  const reachBefore = await reach();
  check(reachBefore.startsWith('Reaches Front End, the member in conversation') && reachBefore.includes('1 reference asks no one'), 'a reference does not change who the message reaches, and the line says it asks no one');
  await box.type(' and @Docs');
  const docs = await rows();
  check(docs[0] === 'member:c06' && docs.includes('window:docs'), '@Docs offers the member Docs Lead first and the Docs window after');
  await page.keyboard.press('Enter');
  check((await reach()).startsWith('Reaches Docs Lead'), 'mentioning a member changes who the message reaches');
  await box.type('please check it.');
  await page.locator('#send').click();
  const sent = page.locator('.msg[data-sent]').last();
  check(await sent.locator('.mention').textContent() === '@Docs Lead' && await sent.locator('.mention-ref').count() === 1, 'in the sent message the member is a mention and the document a reference');
  const looks = await sent.evaluate(el => { const m = getComputedStyle(el.querySelector('.mention')), r = getComputedStyle(el.querySelector('.mention-ref')); return { differ: m.color !== r.color && m.fontWeight !== r.fontWeight, glyph: !!el.querySelector('.mention-ref .glyph') }; });
  check(looks.differ && looks.glyph, 'they differ by weight, colour and a glyph on the reference');
  check((await sent.locator('[data-asked]').textContent()).includes('Asked to answer: Docs Lead.') && (await sent.locator('[data-referred]').textContent()).includes('Referred to, not asked: release-notes.md.'), 'the lines under the message say who was asked and what was only referred to');
  check((await box.inputValue()) === '' && (await reach()).startsWith('Reaches Front End'), 'after sending the box is empty and the line is back to the default');

  // A chat outside the group is a reference, never asked.
  await box.type('@Spread'); await page.keyboard.press('Enter'); await box.type('has the numbers.');
  await page.locator('#send').click();
  const s2 = page.locator('.msg[data-sent]').last();
  check((await s2.locator('[data-referred]').textContent()).includes('Spreadsheet Helper (harbor)') && !(await s2.locator('[data-asked]').textContent()).includes('Spreadsheet Helper'), 'a chat outside the group is referred to and not asked');

  // Arrow keys and Escape.
  await box.type('@');
  await page.keyboard.press('ArrowDown');
  check(await page.locator('#group-pop [aria-selected="true"]').getAttribute('data-mention') === 'member:c02', 'Down moves to the next row');
  await page.keyboard.press('Escape');
  check(await page.locator('#group-pop').isHidden(), 'Escape closes the list');

  // A team chat.
  await page.goto(PAGE + '?state=team'); await settle();
  check((await rows()).includes('applet:git') && (await page.locator('#group-pop').textContent()).includes('the team’s lead'), 'a team chat has the same list, with its lead marked');
  await box.fill('Status, please.'); await box.dispatchEvent('input');
  check((await reach()).startsWith('Reaches the lead, Desktop Lead'), 'a team message with no member mentioned goes to the lead');

  // The Add button opens the same list.
  await page.goto(PAGE + '?state=empty'); await settle();
  await page.locator('[data-add]').click();
  check(await page.locator('#group-pop').isVisible() && (await box.inputValue()) === '@', 'Add… puts an @ in the box and opens the list');

  // The phone.
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto(PAGE + '?state=list'); await settle();
  const fits = await page.locator('#group-pop').evaluate(p => { const r = p.getBoundingClientRect(); return r.left >= 0 && r.right <= innerWidth; });
  check(fits, 'on a phone the list fits the window');
  check(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth), 'the page has no sideways overflow');

  check(page.errors.length === 0, 'no page errors' + (page.errors.length ? ': ' + page.errors.join(' | ') : ''));
  console.log(failures ? failures + ' FAILED' : 'all passed');
  process.exitCode = failures ? 1 : 0;
}).catch(e => { console.error(e); process.exitCode = 1; });
