// A window whose content area is wholly covered by the windows above it is
// not drawn, and one of which anything shows is (covered-windows.js).
const { launch } = require('./lib/page');
(async () => {
  const t = await launch({
    viewport: { width: 1600, height: 1000 },
    app: () => {
      const info = (id, ws) => ({ sessionId: id, title: id, provider: 'codex', cwd: ws, open: true, working: false, queued: [] });
      return {
        OpenSessionAt: async (ws, id) => {
          for (let i = 0; i < 30; i++) window.__emit('agent:update', { sessionId: id, update: { sessionUpdate: i % 2 ? 'agent_message_chunk' : 'user_message_chunk', content: { type: 'text', text: 'Row ' + i + ' of ' + id + '. ' + 'Words of an answer.\n\n'.repeat(4) } } });
          return info(id, ws);
        },
      };
    },
  });
  const { page: p, ok } = t;
  try {
    const key = id => p.evaluate(id => dinahWorkbench.key({ kind: 'chat-session', workspace: 'C:/Fixture', path: id }), id);
    for (const id of ['codex:under', 'codex:over']) {
      await p.evaluate(id => dinahWorkbench.open({ kind: 'chat-session', workspace: 'C:/Fixture', path: id }), id);
      await p.waitForFunction(k => document.querySelectorAll('[data-win="' + k + '"] .msg-text').length >= 30, await key(id));
    }
    const under = await key('codex:under'), over = await key('codex:over');
    const frames = () => p.evaluate(() => new Promise(r => requestAnimationFrame(() => requestAnimationFrame(r))));
    // Places are written to the address, as the window layer keeps them, and a frame is let pass.
    const place = async places => {
      await p.evaluate(places => {
        const query = new URLSearchParams(location.search);
        for (const [k, v] of Object.entries(places)) query.set('p.' + k, 'floating:' + v);
        history.replaceState(history.state, '', location.pathname + '?' + query.toString() + location.hash);
        window.dispatchEvent(new PopStateEvent('popstate'));
      }, places);
      await frames();
    };
    const covered = k => p.evaluate(k => document.querySelector('[data-win="' + k + '"]').hasAttribute('data-dinah-covered'), k);
    const drawn = k => p.evaluate(k => getComputedStyle(document.querySelector('[data-win="' + k + '"] > .win-body')).contentVisibility, k);

    // The geometry by itself.
    ok('one rectangle over the whole target covers it', await p.evaluate(() => dinahCovered.covers({ left: 10, top: 10, right: 110, bottom: 110 }, [{ left: 0, top: 0, right: 200, bottom: 200 }])));
    ok('two rectangles that meet cover a target neither covers alone', await p.evaluate(() => dinahCovered.covers({ left: 0, top: 0, right: 100, bottom: 100 }, [{ left: -5, top: -5, right: 50, bottom: 105 }, { left: 50, top: -5, right: 105, bottom: 105 }])));
    ok('a gap one pixel wide between them leaves the target showing', await p.evaluate(() => !dinahCovered.covers({ left: 0, top: 0, right: 100, bottom: 100 }, [{ left: -5, top: -5, right: 50, bottom: 105 }, { left: 51, top: -5, right: 105, bottom: 105 }])));
    ok('a target that sticks out by a pixel is not covered', await p.evaluate(() => !dinahCovered.covers({ left: 0, top: 0, right: 101, bottom: 100 }, [{ left: 0, top: 0, right: 100, bottom: 100 }])));
    ok('nothing above covers nothing', await p.evaluate(() => !dinahCovered.covers({ left: 0, top: 0, right: 10, bottom: 10 }, [])));

    // A window that lies wholly within the one above it. BIG is larger than
    // SMALL by more than a corner's rounding on every side.
    const SMALL = '0.3,0.1,0.4,0.6', BIG = '0.28,0.07,0.44,0.66';
    await place({ [under]: SMALL, [over]: BIG });
    ok('a window wholly within the one above is marked covered and its content is not drawn', await covered(under) && await drawn(under) === 'hidden');
    ok('the window in front is drawn', !await covered(over) && await drawn(over) !== 'hidden');
    ok('a covered window keeps its frame and title bar', await p.evaluate(k => { const w = document.querySelector('[data-win="' + k + '"]'), h = w.querySelector('.win-head'); return getComputedStyle(w).display !== 'none' && h.getBoundingClientRect().height > 0 && getComputedStyle(h).contentVisibility !== 'hidden'; }, under));

    // A strip of the content shows.
    await place({ [under]: '0.26,0.1,0.4,0.6' });
    ok('a window of which a strip shows is drawn', !await covered(under) && await drawn(under) !== 'hidden');
    await place({ [under]: '0.3,0.1,0.4,0.66' });
    ok('a window that shows below the one in front is drawn', !await covered(under));

    // Rounded corners. A window's corner is cut away, so content beneath it
    // there can be seen. The lower window is moved so that the bottom left
    // corner of its content area sits one pixel inside the corner of the
    // window above.
    const corner = await p.evaluate(k => parseFloat(getComputedStyle(document.querySelector('[data-win="' + k + '"]')).borderTopLeftRadius), over);
    ok('this skin rounds a window\'s corners', corner >= 4, String(corner));
    // A place is a share of the room the layer leaves a window, so how far one hundredth moves a window is measured first.
    const left = k => p.evaluate(k => { const r = document.querySelector('[data-win="' + k + '"]').getBoundingClientRect(); return [r.left, r.top]; }, k);
    await place({ [under]: '0.3,0.1,0.4,0.6' }); const at = await left(under);
    await place({ [under]: '0.31,0.11,0.4,0.6' }); const moved = await left(under);
    const perPixel = [0.01 / (moved[0] - at[0]), 0.01 / (moved[1] - at[1])];
    // The place that moves a window from where SMALL puts it by so many pixels.
    const shifted = (dx, dy) => [(0.3 + dx * perPixel[0]).toFixed(6), (0.1 + dy * perPixel[1]).toFixed(6), '0.4', '0.6'].join(',');
    await place({ [under]: SMALL });
    const gap = await p.evaluate(([u, o]) => {
      const upper = document.querySelector('[data-win="' + o + '"]').getBoundingClientRect(), body = document.querySelector('[data-win="' + u + '"] > .win-body').getBoundingClientRect();
      return [(upper.left + 1) - body.left, (upper.bottom - 1) - body.bottom];
    }, [under, over]);
    await place({ [under]: shifted(gap[0], gap[1]) });
    const reached = await p.evaluate(([u, o]) => {
      const upper = document.querySelector('[data-win="' + o + '"]').getBoundingClientRect(), body = document.querySelector('[data-win="' + u + '"] > .win-body').getBoundingClientRect();
      return [body.left - upper.left, upper.bottom - body.bottom];
    }, [under, over]);
    ok('the lower window\'s content area now reaches into the corner of the window above', reached.every(v => v >= 0 && v < 3), JSON.stringify(reached));
    ok('content that reaches into the rounded corner of the window above is drawn', !await covered(under));

    // Raising the covered window draws it, at once, and it takes the keyboard.
    await place({ [under]: SMALL });
    ok('covered again', await covered(under));
    await place({ [over]: SMALL, [under]: BIG });
    await p.evaluate(k => pudlWindows.raise(k), under);
    ok('a raised window is drawn in the same task, and the one it now covers is not', await p.evaluate(([u, o]) => !document.querySelector('[data-win="' + u + '"]').hasAttribute('data-dinah-covered') && document.querySelector('[data-win="' + o + '"]').hasAttribute('data-dinah-covered'), [under, over]));
    ok('the raised window holds the keyboard focus', await p.evaluate(k => document.querySelector('[data-win="' + k + '"]').contains(document.activeElement), under));

    // Text streamed into a covered chat is there, at the end, when the chat is raised.
    await p.evaluate(() => { for (let i = 0; i < 40; i++) __emit('agent:update', { sessionId: 'codex:over', update: { sessionUpdate: 'agent_message_chunk', content: { type: 'text', text: 'Streamed while covered ' + i + '.\n\n' } } }); });
    await frames();
    ok('the chat is still covered while it streams', await covered(over));
    await p.evaluate(k => pudlWindows.raise(k), over);
    ok('raising the smaller window leaves the larger one showing around it', !await covered(under) && !await covered(over));
    await p.waitForFunction(k => { const log = document.querySelector('[data-win="' + k + '"] [data-view-id="activity"]'); return log.scrollHeight - log.scrollTop - log.clientHeight < 24 && /Streamed while covered 39/.test(log.textContent); }, over);
    ok('a chat that streamed while covered shows its latest text at the end when raised', true);

    // Minimising the window in front draws the one beneath.
    await place({ [under]: SMALL, [over]: BIG });
    ok('the smaller is covered again', await covered(under));
    await p.evaluate(k => pudlWindows.minimize(k), over);
    await frames();
    ok('minimising the window in front draws the one beneath', !await covered(under));
    await p.evaluate(k => pudlWindows.raise(k), over);
    ok('and restoring it covers that one again', await covered(under));

    // Dragging the window in front away shows the one beneath during the drag, before the pointer is released.
    const head = await p.evaluate(k => { const r = document.querySelector('[data-win="' + k + '"] .win-title').getBoundingClientRect(); return [r.left + 20, r.top + r.height / 2]; }, over);
    await p.mouse.move(head[0], head[1]); await p.mouse.down();
    await p.mouse.move(head[0] + 120, head[1] + 90, { steps: 4 });
    await frames();
    const during = await covered(under);
    await p.mouse.up();
    ok('a window is drawn as soon as a drag uncovers any of it', !during);
    await frames();
    ok('and stays drawn where the drag left the windows', !await covered(under));

    // The Glass skin leaves a window's frame clear, so only its content area covers.
    const glass = async on => {
      // The skin is chosen through the library's own setter. Written by hand, the mark is sometimes written back by the library.
      await p.evaluate(on => window.pudlSkins.select(on ? 'glass' : 'standard'), on);
      await p.waitForFunction(([k, on]) => (getComputedStyle(document.querySelector('[data-win="' + k + '"]')).backgroundColor === 'rgba(0, 0, 0, 0)') === on, [over, on]);
      await frames();
    };
    await place({ [under]: SMALL, [over]: BIG });
    ok('within the larger window again', await covered(under));
    await glass(true);
    ok('under a clear frame, a window within the content area above is still covered', await covered(under));
    await glass(false);
    // The lower window is raised a little, so that the top of its content
    // area lies ten pixels above the content area of the window over it,
    // which is under that window's title bar.
    const rise = await p.evaluate(([u, o]) => (document.querySelector('[data-win="' + o + '"] > .win-body').getBoundingClientRect().top - 10) - document.querySelector('[data-win="' + u + '"] > .win-body').getBoundingClientRect().top, [under, over]);
    await place({ [under]: shifted(0, rise) });
    ok('the lower content area now starts under the title bar above', await p.evaluate(([u, o]) => { const top = document.querySelector('[data-win="' + u + '"] > .win-body').getBoundingClientRect().top, upper = document.querySelector('[data-win="' + o + '"]'); return top > upper.getBoundingClientRect().top + 14 && top < upper.querySelector('.win-body').getBoundingClientRect().top; }, [under, over]), String(rise));
    ok('content under an opaque title bar is covered', await covered(under));
    await glass(true);
    ok('the same content under a clear title bar is drawn', !await covered(under));
    await glass(false);
    ok('and is covered again when the skin paints the frame', await covered(under));

    // A window sized by its content takes no part.
    await place({ [under]: SMALL, [over]: BIG });
    ok('covered before it is sized by its content', await covered(under));
    await p.evaluate(k => document.querySelector('[data-win="' + k + '"]').setAttribute('data-win-size', 'content'), under);
    await frames();
    ok('a window sized by its content is never left undrawn', !await covered(under));
  } finally { await t.close(); }
})();
