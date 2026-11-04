// Rasterize the canonical SVG with Chromium, then package PNG frames in an ICO.
// Run under the shared browser-test lock. Requires playwright-core and Edge.
const fs = require('node:fs');
const path = require('node:path');
const { chromium } = require('playwright-core');
const root = path.resolve(__dirname, '..');
const mark = fs.readFileSync(path.join(root, 'cmd/dinah-desktop/frontend/dinah-mark.svg'), 'utf8');
const out = path.join(root, 'build/clear-chime-logo');
(async () => {
  fs.mkdirSync(out, { recursive: true });
  const browser = await chromium.launch({ channel: 'msedge', headless: true });
  try {
    const page = await browser.newPage({ deviceScaleFactor: 1 });
    const frames = [];
    for (const size of [16, 20, 24, 32, 40, 48, 64, 128, 256, 512]) {
      await page.setViewportSize({ width: size, height: size });
      await page.setContent('<style>html,body{margin:0;background:transparent}svg{display:block;width:100vw;height:100vh}</style>' + mark);
      const png = await page.screenshot({ omitBackground: true });
      fs.writeFileSync(path.join(out, `icon-${size}.png`), png);
      if (size <= 256) frames.push({ size, png });
      if (size === 512) fs.writeFileSync(path.join(root, 'cmd/dinah-desktop/build/appicon.png'), png);
    }
    const header = Buffer.alloc(6 + frames.length * 16);
    header.writeUInt16LE(1, 2); header.writeUInt16LE(frames.length, 4);
    let offset = header.length;
    frames.forEach(({ size, png }, i) => {
      const at = 6 + i * 16;
      header[at] = header[at + 1] = size === 256 ? 0 : size;
      header.writeUInt16LE(1, at + 4); header.writeUInt16LE(32, at + 6);
      header.writeUInt32LE(png.length, at + 8); header.writeUInt32LE(offset, at + 12);
      offset += png.length;
    });
    fs.writeFileSync(path.join(root, 'cmd/dinah-desktop/build/windows/icon.ico'), Buffer.concat([header, ...frames.map(f => f.png)]));
    fs.writeFileSync(path.join(root, 'internal/web/static/dinah-mark.svg'), mark);
    await page.setViewportSize({ width: 850, height: 300 });
    const sizes = [16, 20, 24, 32, 48, 64, 128];
    await page.setContent('<style>body{margin:0;font:14px Segoe UI}.row{display:flex;align-items:center;gap:32px;padding:20px;height:150px;box-sizing:border-box}.light{background:#faf8f1;color:#243c3c}.dark{background:#282f32;color:#eeefeb}figure{margin:0;text-align:center}svg{display:block;margin:0 auto}figcaption{margin-top:8px}</style>' + ['light', 'dark'].map(mode => '<div class="row ' + mode + '">' + sizes.map(size => '<figure>' + mark.replace('<svg ', '<svg width="' + size + '" height="' + size + '" ') + '<figcaption>' + size + ' px</figcaption></figure>').join('') + '</div>').join(''));
    await page.screenshot({ path: path.join(out, 'icon-sizes.png') });
    console.log('Generated 9 ICO sizes, 512px application PNG, web SVG and size review sheet.');
  } finally { await browser.close(); }
})().catch(error => { console.error(error); process.exitCode = 1; });
