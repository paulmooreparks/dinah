// What each skin paints a window with: whether a window can be seen through, and how its corners are rounded.
//   PROBE=bench/probes/window-paint.js node bench/scaling.js [out] 2
module.exports = async ({ p, settle }) => {
  const skins = await p.evaluate(() => [...document.querySelectorAll('link[data-pudl-skin]')].map(l => l.getAttribute('data-pudl-skin')));
  for (const skin of ['(none)'].concat(skins)) for (const theme of ['light', 'dark']) {
    await p.evaluate(([skin, theme]) => { document.documentElement.dataset.theme = theme; if (skin === '(none)') delete document.documentElement.dataset.skin; else document.documentElement.dataset.skin = skin; }, [skin, theme]);
    await settle(p);
    console.log('PROBE ' + skin.padEnd(11) + theme.padEnd(6) + JSON.stringify(await p.evaluate(() => {
      const win = document.querySelector('.win[data-win-kind="chat-session"]'), s = getComputedStyle(win), b = getComputedStyle(win.querySelector('.win-body')), h = getComputedStyle(win.querySelector('.win-head'));
      return { parent: win.parentElement.className, applied: document.documentElement.dataset.skin || '', win: s.backgroundColor, image: s.backgroundImage.slice(0, 30), opacity: s.opacity, radius: s.borderTopLeftRadius, backdrop: s.backdropFilter, filter: s.filter, transform: s.transform, body: b.backgroundColor, head: h.backgroundColor + ' ' + h.backgroundImage.slice(0, 24), z: s.zIndex, position: s.position, isolation: s.isolation };
    })));
  }
};
