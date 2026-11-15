/* A static proposal. Diagrams supplies the box and connector renderer. */
(function () {
  'use strict';
  document.documentElement.dataset.theme = new URLSearchParams(location.search).get('theme') === 'dark' ? 'dark' : 'light';
  const style = getComputedStyle(document.documentElement);
  const token = name => style.getPropertyValue(name).trim();
  const palette = { bg: token('--bg'), surface: token('--surface'), text: token('--text'), muted: token('--text-muted'), line: token('--text-muted'), accent: token('--accent'), presets: [] };
  const narrow = innerWidth < 700, width = narrow ? innerWidth - 24 : innerWidth - 32;
  const nodes = [], edges = [];
  const text = (id, x, y, w, h, value, extra) => nodes.push({ id, type: 'text', x, y, width: w, height: h, text: value, yavc: Object.assign({ shape: 'text', fontSize: 14, padding: 0, textColor: palette.text, lineHeight: 1.3 }, extra) });
  function group(id, title, x, y, w, h) { nodes.push({ id, type: 'group', x, y, width: w, height: h, label: title, yavc: { strokeColor: token('--border'), line: 'solid', lineWidth: 1, radius: 6 } }); }
  const cards = {};
  function chat(id, title, role, program, state, x, y, w, selected) {
    const h = 92; cards[id] = { x, y, w, h };
    nodes.push({ id, type: 'text', x, y, width: w, height: h, text: '', yavc: { radius: 5, fillColor: palette.surface, strokeColor: selected ? palette.accent : palette.line, lineWidth: selected ? 2 : 1 } });
    text(id + '-title', x + 12, y + 10, w - 24, 22, title, { fontWeight: 600, fontSize: 15 });
    text(id + '-role', x + 12, y + 34, w - 24, 18, role + ' / ' + program, { fontSize: 12, textColor: palette.muted });
    const glyph = state === 'Working' ? '▶' : state === 'Waiting' ? '?' : '○';
    const colour = state === 'Working' ? palette.accent : state === 'Waiting' ? token('--warn') : token('--positive');
    text(id + '-glyph', x + 12, y + 62, 18, 20, glyph, { fontWeight: 600, textColor: colour });
    text(id + '-state', x + 34, y + 62, w - 46, 20, state, { fontSize: 13 });
  }
  function edge(id, from, to, dotted, fromSide, toSide) { edges.push({ id, fromNode: from, fromSide: fromSide || 'bottom', fromEnd: 'none', toNode: to, toSide: toSide || 'top', toEnd: 'none', yavc: { line: dotted ? 'dotted' : 'solid', lineWidth: 2, route: 'elbow' } }); }
  function reviewBox(x, y, w) {
    nodes.push({ id: 'release', type: 'text', x, y, width: w, height: 122, text: '', yavc: { radius: 5, fillColor: palette.surface } });
    text('release-title', x + 14, y + 14, w - 28, 24, 'Release Review', { fontWeight: 600, fontSize: 15 });
    text('release-lead', x + 14, y + 47, w - 28, 40, 'Led by Lena from Delivery', { fontSize: 13 });
    text('release-members', x + 14, y + 90, w - 28, 20, 'Lena, Nina and Tess', { fontSize: 13, textColor: palette.muted });
  }
  function viaGutter(id, from, x) {
    const source = cards[from];
    text(id + '-bend', x, source.y + source.h / 2, 1, 1, '');
    edge(id + '-out', from, id + '-bend', true, 'right', 'left');
    edge(id + '-down', id + '-bend', 'release', true, 'bottom', 'right');
  }
  let height, menu;
  if (narrow) {
    const w = width, memberW = (w - 48) / 2;
    group('platform', 'Platform team (Harbor)', 0, 26, w, 300);
    chat('mara', 'Mara', 'Lead', 'Codex', 'Working', (w - 180) / 2, 50, 180);
    chat('ivo', 'Ivo', 'Builder', 'Claude', 'Idle', 12, 206, memberW);
    chat('nina', 'Nina', 'Reviewer', 'Codex', 'Waiting', w - memberW - 12, 206, memberW, true);
    group('delivery', 'Delivery team (Harbor)', 0, 650, w, 300);
    chat('lena', 'Lena', 'Lead', 'Claude', 'Idle', (w - 180) / 2, 674, 180);
    chat('oren', 'Oren', 'Writer', 'Codex', 'Working', 12, 830, memberW);
    chat('tess', 'Tess', 'Tester', 'Claude', 'Idle', w - memberW - 12, 830, memberW);
    group('cross-functional', 'Cross-functional group', 0, 1030, w, 172);
    reviewBox(28, 1060, w - 56);
    group('folded', 'Research team (Orchard)', 0, 1272, w, 100);
    text('fold-summary', 16, 1296, w - 32, 60, '▸  Research is folded.\nSasha leads 3 members.', { fontSize: 14, lineHeight: 1.6 });
    group('free', 'Free agents', 0, 1438, w, 150);
    chat('cal', 'Cal', 'Research', 'Codex', 'Idle', 12, 1466, memberW);
    chat('uma', 'Uma', 'Designer', 'Claude', 'Working', w - memberW - 12, 1466, memberW);
    height = 1620; menu = { x: w - 204, y: 302 };
  } else {
    const teamW = Math.min(430, (width - 600) / 2), memberW = (teamW - 64) / 2;
    group('platform', 'Platform team (Harbor)', 12, 35, teamW, 330);
    chat('mara', 'Mara', 'Lead', 'Codex', 'Working', 12 + (teamW - 210) / 2, 65, 210);
    chat('ivo', 'Ivo', 'Builder', 'Claude', 'Idle', 36, 242, memberW);
    chat('nina', 'Nina', 'Reviewer', 'Codex', 'Waiting', 12 + teamW - memberW - 24, 242, memberW, true);
    const bx = teamW + 276;
    group('delivery', 'Delivery team (Harbor)', bx, 35, teamW, 330);
    chat('lena', 'Lena', 'Lead', 'Claude', 'Idle', bx + (teamW - 210) / 2, 65, 210);
    chat('oren', 'Oren', 'Writer', 'Codex', 'Working', bx + 24, 242, memberW);
    chat('tess', 'Tess', 'Tester', 'Claude', 'Idle', bx + teamW - memberW - 24, 242, memberW);
    const rx = bx + teamW + 32, rw = width - rx - 12;
    group('folded', 'Research team (Orchard)', rx, 35, rw, 128);
    text('fold-summary', rx + 16, 61, rw - 32, 78, '▸  Research is folded.\nSasha leads 3 members.', { fontSize: 14, lineHeight: 1.6 });
    group('free', 'Free agents', rx, 230, rw, 280);
    chat('cal', 'Cal', 'Research', 'Codex', 'Idle', rx + 16, 260, rw - 32);
    chat('uma', 'Uma', 'Designer', 'Claude', 'Working', rx + 16, 386, rw - 32);
    group('cross-functional', 'Cross-functional group', 12, 485, bx + teamW - 12, 200);
    reviewBox(bx + (teamW - 210) / 2 + 210 - 24 - (teamW - 140), 555, teamW - 140);
    height = 730; menu = { x: 12 + teamW + 16, y: cards.nina.y + 10 };
  }
  edge('mara-ivo', 'mara', 'ivo'); edge('mara-nina', 'mara', 'nina'); edge('lena-oren', 'lena', 'oren'); edge('lena-tess', 'lena', 'tess');
  if (narrow) { viaGutter('nina-review', 'nina', width - 3); viaGutter('lena-review', 'lena', width - 11); }
  else { edge('nina-review', 'nina', 'release', true, 'bottom', 'left'); viaGutter('lena-review', 'lena', cards.tess.x + cards.tess.w + 38); }
  edge('tess-review', 'tess', 'release', true, 'bottom', 'top');
  text('bounds', 0, 0, width, height, '');
  const doc = { nodes, edges }, checked = yavcCanvas.read(JSON.stringify(doc));
  if (checked.problems.length) throw new Error(JSON.stringify(checked.problems));
  const output = yavcCanvas.render(checked.doc, { palette, pad: 0, font: getComputedStyle(document.body).fontFamily, title: 'Two expanded teams, a folded team, a cross-functional group and free agents.' });
  document.getElementById('drawing').innerHTML = output.svg;
  const popup = document.querySelector('.mock-menu'); popup.style.left = Math.min(width - (narrow ? 198 : 218), menu.x) + 'px'; popup.style.top = menu.y + 'px';
  window.__orgChart = { doc, width, height, narrow, renderer: 'yavcCanvas.render' };
  document.documentElement.dataset.mockReady = 'true';
}());
