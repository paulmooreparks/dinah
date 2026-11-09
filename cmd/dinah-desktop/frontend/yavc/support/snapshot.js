/* Copyright 2026 Paul Parks. SPDX-License-Identifier: Apache-2.0 */
/* Lifted from parkscomputing.com (wwwroot/js/snapshot.js) with its global
   renamed to yavcSnapshot; host.js gives it to packages as pudlHost.snapshot. */
/* Snapshots: where the reader was, brought back after the page reloads.

   The address already keeps which windows are open and where, and each
   applet's state string (PUDL's state() and setState()) keeps what can be
   linked to, such as the file in front. A snapshot keeps the rest, the
   things a reload loses and no link should carry: how far each window is
   scrolled, which window had the keyboard, and whatever an applet says it
   was in the middle of, such as an editor's cursor and unsaved text.

   This is the site's trial of a contract meant for PUDL. Until PUDL has
   it, an applet takes part through window.yavcSnapshot:

     var snap = pcSnapshot.join(root, take)
       take()           returns what the applet would need to carry on,
                        as plain JSON, or null for nothing
       snap.kept        what it returned before the reload, or null
       snap.focused     whether the keyboard was in it then
       snap.changed()   says something changed, so it is kept soon
       snap.leave()     stops; the applet's destroy calls it

   Snapshots are kept in this browser tab's session storage, which a
   reload keeps and the browser brings back with a tab it reopens. Two
   tabs keep apart. A window's snapshot goes when the window closes, so a
   window opened again later starts afresh unless its primary applet opts
   into a remembered workspace; an applet on a page of its own
   keeps its snapshot under that page's address. Load before pudl-applets.js,
   without defer, beside js/continuity.js. */
(function () {
    'use strict';
    if (window.yavcSnapshot) { return; }

    var PREFIX = 'pc-snap:';
    var RESUME_PREFIX = 'pc-resume:';
    function remembered(key) {
        var best = null, prefix = RESUME_PREFIX + key;
        try {
            for (var i = 0; i < localStorage.length; i++) {
                var name = localStorage.key(i);
                if (name !== prefix && name.indexOf(prefix + ':') !== 0) continue;
                var entry;
                try { entry = JSON.parse(localStorage.getItem(name)); } catch (err) { continue; }
                if (entry && typeof entry.t === 'number' && 'v' in entry && (!best || entry.t > best.t)) best = Object.assign({},entry,{key:name});
            }
        } catch (err) { return null; }
        return best;
    }
    function seal(j) {
        if(!j.remember)return;
        try {var entry=JSON.parse(localStorage.getItem(j.remember));if(entry && entry.sealed===false){entry.sealed=true;localStorage.setItem(j.remember,JSON.stringify(entry));}}catch(err){/* A retained unsealed copy is safer than deleting an uncertain writer. */}
    }
    function retireParent(j) {
        var parent=j.parent;j.parent=null;if(!parent)return;
        try {var entry=JSON.parse(localStorage.getItem(parent.key));if(entry && entry.sealed===true && entry.t===parent.t && JSON.stringify(entry.v)===parent.text)localStorage.removeItem(parent.key);}catch(err){/* Recovery copies survive cleanup failures. */}
    }
    var joined = [];        /* { root, key, take, dirty } */
    var timer = null;

    function read(key) {
        try { var v = sessionStorage.getItem(PREFIX + key); return v ? JSON.parse(v) : null; } catch (err) { return null; }
    }
    function write(key, value) {
        try {
            if (value == null) { sessionStorage.removeItem(PREFIX + key); }
            else { sessionStorage.setItem(PREFIX + key, JSON.stringify(value)); }
            return true;
        } catch (err) { return false; }
    }
    /* An applet's snapshot is kept with the time it was taken, so of two
       for the same applet the newer is known. */
    function readApplet(key) {
        var e = read(key);
        return e && typeof e === 'object' && typeof e.t === 'number' && 'v' in e ? e : null;
    }
    function keys() {
        var out = [];
        try { for (var i = 0; i < sessionStorage.length; i++) { var k = sessionStorage.key(i); if (k && k.indexOf(PREFIX) === 0) { out.push(k.slice(PREFIX.length)); } } } catch (err) { }
        return out;
    }

    /* A window's things go by its key; an applet on a page of its own by
       the page and the applet. */
    function winOf(el) { return el && el.closest ? el.closest('.win[data-win]') : null; }
    function keyOf(root) {
        var win = winOf(root);
        if (win) { return 'win:' + win.getAttribute('data-win') + ':' + (root.getAttribute('data-applet') || ''); }
        return 'page:' + location.pathname + ':' + (root.getAttribute('data-applet') || '');
    }

    /* The window or applet the keyboard was in when the page went. */
    var focus = read('focus');
    write('focus', null);
    function focusNow() {
        var el = document.activeElement;
        if (!el || el === document.body) { return null; }
        var win = winOf(el);
        if (win) { return 'win:' + win.getAttribute('data-win'); }
        var mount = el.closest && el.closest('[data-applet]');
        return mount ? keyOf(mount) : null;
    }

    function keepApplet(j) {
        var value = null;
        try { value = j.take(); } catch (err) { j.dirty = true; if (window.console) { console.warn('snapshot:', err); } return false; }
        var serialized;
        if (j.remember) {
            try { serialized = JSON.stringify(value); } catch (err) { j.dirty = true; return false; }
            if (serialized !== j.rememberedText) j.changedAt = Math.max(Date.now(), j.changedAt + 1);
        }
        var kept = write(j.key, value == null ? null : { t: j.remember ? j.changedAt : Date.now(), v: value });
        if (kept && j.remember) {
            try {
                localStorage.setItem(j.remember, JSON.stringify({ t: j.changedAt, v: value,sealed:false }));
                j.rememberedText = serialized;
                retireParent(j);
            }
            catch (err) { kept = false; }
        }
        j.dirty = !kept;
        if (kept) j.pendingCleanup = j.pendingCleanup.filter(function (key) { return !write(key, null); });
        if (!kept && window.console) {
            console.warn('snapshot: ' + j.key + ' is too large to keep');
        }
        return kept;
    }
    /* A snapshot is taken once a run of changes pauses, and at least every
       few seconds through a long one, rather than every half second while
       typing goes on (performance review, 2026-11-03); leaving or hiding
       the page still takes one at once (keepAll). */
    var firstDirty = 0;
    function keepSoon() {
        var now = Date.now();
        if (!firstDirty) { firstDirty = now; }
        clearTimeout(timer);
        timer = now - firstDirty >= 5000 ? (keepDirty(), null) : setTimeout(keepDirty, 800);
    }
    function keepDirty() {
        clearTimeout(timer);
        timer = null;
        firstDirty = 0;
        joined.forEach(function (j) { if (j.dirty) { keepApplet(j); } });
    }
    function keepAll() {
        clearTimeout(timer);
        timer = null;
        joined.forEach(keepApplet);
        keepScrolls();
        write('focus', focusNow());
    }

    /* Switching between the window view and the classic view moves an
       applet from its window to its own page, or back, under another key.
       Its first instance takes the newest snapshot of the applet in either
       view, and the others are let go, so an older one can't come back
       later. Further instances, such as a second terminal, have no page of
       their own, so they keep to their own. */
    function newest(key, root) {
        var name = root.getAttribute('data-applet') || '';
        var win = winOf(root);
        var own = readApplet(key);
        if (!name || (win && win.getAttribute('data-win') !== name)) { return { entry: own, remove: [] }; }
        var best = own, remove = [];
        keys().forEach(function (k) {
            if (k === key || !(k === 'win:' + name + ':' + name || (k.indexOf('page:') === 0 && k.slice(-(name.length + 1)) === ':' + name))) { return; }
            var e = readApplet(k);
            if (e && (!best || e.t > best.t)) { best = e; }
            remove.push(k);
        });
        return { entry: best, remove: remove };
    }

    function join(root, take, options) {
        var key = keyOf(root);
        var plan = newest(key, root), entry = plan.entry;
        var windows = window.pudlHost && window.pudlHost.documentWindows;
        var initial = windows && windows.initial(root);
        if (initial) { entry = initial.value ? {t: Date.now(), v: initial.value} : null; plan.remove = []; }
        var resumeKey = options && options.remember ? root.getAttribute('data-applet') : null;
        var resume = resumeKey ? remembered(resumeKey) : null;
        if (resume && !entry && !initial) entry = resume;
        // Validation precedes registration and cleanup of other snapshots.
        if (options && typeof options.validate === 'function') options.validate(entry ? entry.v : null);
        var j = { root: root, key: key, take: take, dirty: false, active: true, pendingCleanup: plan.remove,
            remember: resumeKey ? RESUME_PREFIX + resumeKey + ':' + crypto.randomUUID() : null,
            rememberedText: entry ? JSON.stringify(entry.v) : null, changedAt: entry ? entry.t : 0,
            parent:resume && entry && JSON.stringify(resume.v)===JSON.stringify(entry.v)?{key:resume.key,t:resume.t,text:JSON.stringify(resume.v)}:null };
        joined.push(j);
        var win = winOf(root);
        var wasFocused = !!focus && (focus === key || (win && focus === 'win:' + win.getAttribute('data-win')));
        return {
            kept: entry ? entry.v : null,
            focused: wasFocused,
            changed: function () { j.dirty = true; keepSoon(); },
            flush: function () { return j.active ? keepApplet(j) : false; },
            leave: function () {
                j.active = false;
                seal(j);
                var i = joined.indexOf(j);
                if (i >= 0) { joined.splice(i, 1); }
            }
        };
    }

    /* === Scrolling ======================================================

       How far each window's body is scrolled, and the page inside a framed
       window when it is the site's own; and the article list. Applets that
       scroll inside themselves keep that in their own snapshots. A place is
       put back once there is enough content to scroll to, which for a
       window fetched after the page loads is a moment later. */
    /* An element with no layout, such as the body of a minimised window,
       says it is not scrolled at all; its place is the one last seen, or
       the one still waiting to be put back. */
    function shown(el) { return el.getClientRects().length > 0; }
    var waiting = {};       /* window key -> the place still to put back */
    function scrollers() {
        var out = {}, before = read('scroll') || {};
        document.querySelectorAll('.win[data-win]').forEach(function (win) {
            var key = win.getAttribute('data-win'), body = win.querySelector('.win-body');
            if (!body) { return; }
            if (waiting[key] || !shown(body)) { out[key] = waiting[key] || before[key] || {}; return; }
            var place = { body: body.scrollTop };
            var frame = body.querySelector('iframe.win-app-frame');
            try { if (frame && frame.contentWindow && frame.contentDocument) { place.frame = frame.contentWindow.scrollY; } } catch (err) { /* Another site's page. */ }
            out[key] = place;
        });
        /* A window still on its way keeps the place it had. */
        Object.keys(waiting).forEach(function (key) { if (!out[key]) { out[key] = waiting[key]; } });
        return out;
    }
    function keepScrolls() { write('scroll', scrollers()); }

    /* A place waits for enough content to scroll to. The reader scrolling
       the element first, while it waits, wins: its scroll is then not
       where it was left. */
    function putBack(el, top, getHeight) {
        if (!top) { return; }
        var until = Date.now() + 8000, left = el.scrollTop;
        (function attempt() {
            if (!el.isConnected || el.scrollTop !== left) { return; }
            var room = getHeight() - el.clientHeight;
            if (room >= top || Date.now() > until) {
                el.scrollTop = Math.min(top, Math.max(0, room));
                return;
            }
            setTimeout(attempt, 100);
        })();
    }
    function putFrameBack(frame, y) {
        if (!y) { return; }
        function go() {
            try {
                var w = frame.contentWindow, d = frame.contentDocument;
                if (!w || !d || !d.documentElement) { return; }
                var until = Date.now() + 8000;
                (function attempt() {
                    var room = d.documentElement.scrollHeight - w.innerHeight;
                    if (room >= y || Date.now() > until) { w.scrollTo(0, Math.min(y, Math.max(0, room))); return; }
                    setTimeout(attempt, 100);
                })();
            } catch (err) { /* Another site's page. */ }
        }
        var d = null;
        try { d = frame.contentDocument; } catch (err) { return; }
        if (d && d.readyState === 'complete' && d.location.href !== 'about:blank') { go(); } else { frame.addEventListener('load', go, { once: true }); }
    }

    /* A window arrives whole once it has been fetched, so each is looked
       for until it does; one that is minimised waits until it is shown. */
    function restoreScrolls() {
        var kept = read('scroll');
        if (!kept) { return; }
        Object.keys(kept).forEach(function (key) {
            var place = kept[key] || {};
            if (key === ' list') {
                // Ignore the former application sidebar's saved scroll.
                return;
            }
            if (place.body || place.frame) { waiting[key] = place; }
        });
        tryWaiting();
        var until = Date.now() + 15000;
        (function poll() {
            tryWaiting();
            if (Object.keys(waiting).length && Date.now() < until) { setTimeout(poll, 150); }
        })();
    }
    function tryWaiting() {
        Object.keys(waiting).forEach(function (key) {
            var win = document.querySelector('.win[data-win="' + CSS.escape(key) + '"]');
            var body = win && win.querySelector('.win-body');
            if (!body || !shown(body)) { return; }
            var place = waiting[key];
            delete waiting[key];
            var frame = body.querySelector('iframe.win-app-frame');
            if (frame) { putFrameBack(frame, place.frame); }
            putBack(body, place.body, function () { return body.scrollHeight; });
        });
    }

    /* === Windows closing ================================================

       A window closed on purpose forgets its snapshot. Only a window this
       page has seen open and then closed counts: until the window layer has
       read the address, every window looks closed. */
    var seenOpen = {};
    function prune() {
        if (!window.pudlWindows) { return; }
        var open = window.pudlWindows.state().open;
        var closed = Object.keys(seenOpen).filter(function (k) { return open.indexOf(k) < 0; });
        open.forEach(function (k) { seenOpen[k] = true; });
        closed.forEach(function (k) { delete seenOpen[k]; delete waiting[k]; });
        if (!closed.length) { return; }
        keys().forEach(function (k) {
            var m = /^win:([^:]+):/.exec(k);
            if (m && closed.indexOf(m[1]) >= 0) { write(k, null); }
        });
    }

    /* The page is going, or may be: everything is kept as it is now. */
    window.addEventListener('pagehide', function (event) {
        keepAll();
        if(!event.persisted){joined.forEach(function (j) {j.active=false;seal(j);});joined=[];}
    });
    document.addEventListener('visibilitychange', function () { if (document.visibilityState === 'hidden') { keepAll(); } });
    var scrollTimer = null;
    document.addEventListener('scroll', function () {
        clearTimeout(scrollTimer);
        scrollTimer = setTimeout(keepScrolls, 300);
    }, true);
    /* A minimised window shown again, or one closed. */
    document.addEventListener('pudl:windows-change', function () {
        setTimeout(function () {
            prune();
            tryWaiting();
        }, 0);
    });

    /* The windows the address opened count as seen open, so closing one
       of them forgets it. The window layer reads the address as the page
       starts, just after this. */
    function start() {
        restoreScrolls();
        setTimeout(function () {
            if (window.pudlWindows) { window.pudlWindows.state().open.forEach(function (k) { seenOpen[k] = true; }); }
        }, 0);
    }
    if (document.readyState === 'loading') { document.addEventListener('DOMContentLoaded', start); } else { start(); }

    window.yavcSnapshot = { version: 2, remembersClosed: true, join: join, keepAll: keepAll };
})();
