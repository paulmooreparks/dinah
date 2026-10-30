// Screenshots the group chat mockups with Edge, headless, from the file
// system. Usage, from this folder:
//   node make-shots.mjs [page.html ...]
// playwright-core comes from the desktop frontend's end-to-end tests, so
// nothing is installed here.
import { createRequire } from 'node:module';
import { mkdirSync } from 'node:fs';
import { fileURLToPath, pathToFileURL } from 'node:url';
import path from 'node:path';

const here = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(here, '../../../cmd/dinah-desktop/frontend/e2e/package.json'));
const { chromium } = require('playwright-core');

const EDGE = 'C:/Program Files (x86)/Microsoft/Edge/Application/msedge.exe';
const shots = [
  { page: 'chat-actions.html', out: 'chat-actions.png', width: 1280, height: 800 },
  { page: 'start-group.html', out: 'start-group.png', width: 1280, height: 800 },
  { page: 'group-window.html', out: 'group-window.png', width: 1280, height: 800 },
  { page: 'group-window.html', query: '?list=closed', out: 'group-window-conversation.png', width: 1280, height: 800 },
  { page: 'bus-pane.html', out: 'bus-pane.png', width: 1280, height: 800 },
  // The narrow shot is tall enough to show the whole pane, all four
  // sections, without scrolling.
  { page: 'bus-pane.html', out: 'bus-pane-narrow.png', width: 420, height: 1180 },
  // The index shows the other shots, so it is taken last.
  { page: 'index.html', out: 'index.png', width: 1280, height: 800 }
];

mkdirSync(path.join(here, 'shots'), { recursive: true });
const browser = await chromium.launch({ executablePath: EDGE, headless: true });
try {
  // Pages given on the command line are the only ones taken.
  const only = process.argv.slice(2);
  for (const s of shots.filter(x => !only.length || only.includes(x.page))) {
    const page = await browser.newPage({ viewport: { width: s.width, height: s.height } });
    await page.goto(pathToFileURL(path.join(here, s.page)).href + (s.query || ''));
    await page.evaluate(() => document.fonts.ready);
    await page.waitForTimeout(150);
    await page.screenshot({ path: path.join(here, 'shots', s.out) });
    await page.close();
    console.log('shots/' + s.out);
  }
} finally {
  await browser.close();
}
