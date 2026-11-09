/* Copyright 2026 Paul Parks. SPDX-License-Identifier: Apache-2.0 */
/* Presenting a deck (docs/slides-design.md). The stage holds one
   .deck-slide element per slide, already drawn; this shows one at a time,
   with transitions, the steps of a slide that reveals its points in turn,
   a progress bar, the slide number, and the keys every deck tool uses:
   the arrows, Space, Page Up and Down, Home and End, a number then Enter,
   and Escape to leave. Slides, the applet, uses it in its window and full
   screen; the exported page embeds it and runs it alone.

     var show = yavcPresent.attach(stage, { onChange: function (slide, step) {}, onLeave: function () {} });
     show.goTo(i, step); show.next(); show.prev(); show.position() -> { slide, step, steps, count }; show.destroy();

   A slide marked data-reveal="steps" shows its top-level list items one
   at a time: step 0 shows none of them after the first. */
(function () {
    'use strict';
    if (window.yavcPresent) { return; }

    function attach(stage, o) {
        o = o || {};
        var slides = Array.prototype.slice.call(stage.querySelectorAll('.deck-slide'));
        var at = 0, step = 0, typed = '', typedTimer = 0, destroyed = false;
        var bar = document.createElement('div');
        bar.className = 'deck-progress';
        var counter = document.createElement('div');
        counter.className = 'deck-counter';
        stage.appendChild(bar);
        stage.appendChild(counter);
        stage.classList.add('deck-presenting');
        stage.setAttribute('tabindex', '0');
        if (!stage.getAttribute('role')) { stage.setAttribute('role', 'region'); }
        if (!stage.getAttribute('aria-label')) { stage.setAttribute('aria-label', document.title || 'The deck'); }

        function stepsOf(s) {
            if (!s || s.getAttribute('data-reveal') !== 'steps') { return []; }
            return Array.prototype.slice.call(s.querySelectorAll('.deck-body > ul > li, .deck-body > ol > li, .deck-body .dinah-src > ul > li, .deck-body .dinah-src > ol > li'));
        }
        function draw() {
            slides.forEach(function (s, i) {
                var was = s.classList.contains('deck-current');
                s.classList.toggle('deck-current', i === at);
                s.classList.toggle('deck-before', i < at);
                s.classList.toggle('deck-after', i > at);
                if (i === at && !was) { s.dispatchEvent(new CustomEvent('deck:shown', { bubbles: true })); }
                if (i !== at && was) { s.dispatchEvent(new CustomEvent('deck:hidden', { bubbles: true })); }
            });
            var items = stepsOf(slides[at]);
            items.forEach(function (li, i) { li.classList.toggle('deck-step-hidden', i >= step); });
            bar.style.width = slides.length > 1 ? (100 * at / (slides.length - 1)) + '%' : '100%';
            counter.textContent = (at + 1) + ' / ' + slides.length;
            if (o.onChange) { o.onChange(at, step); }
        }
        function goTo(i, s) {
            if (destroyed || !slides.length) { return; }
            at = Math.max(0, Math.min(slides.length - 1, i | 0));
            var max = stepsOf(slides[at]).length;
            step = s == null ? 0 : Math.max(0, Math.min(max, s | 0));
            draw();
        }
        function next() {
            var max = stepsOf(slides[at]).length;
            if (step < max) { step++; draw(); return; }
            if (at < slides.length - 1) { goTo(at + 1, 0); }
        }
        function prev() {
            if (step > 0) { step--; draw(); return; }
            if (at > 0) { goTo(at - 1, stepsOf(slides[at - 1]).length); }
        }
        function keys(e) {
            if (e.defaultPrevented || e.ctrlKey || e.metaKey || e.altKey) { return; }
            var t = e.target;
            if (t && t.closest && t.closest('input, textarea, select, [contenteditable="true"]')) { return; }
            switch (e.key) {
                case 'ArrowRight': case 'ArrowDown': case ' ': case 'PageDown': case 'Enter':
                    if (e.key === 'Enter' && typed) { goTo(parseInt(typed, 10) - 1, 0); typed = ''; }
                    else { next(); }
                    break;
                case 'ArrowLeft': case 'ArrowUp': case 'PageUp': case 'Backspace': prev(); break;
                case 'Home': goTo(0, 0); break;
                case 'End': goTo(slides.length - 1, 0); break;
                case 'Escape': if (o.onLeave) { o.onLeave(); } break;
                default:
                    if (/^[0-9]$/.test(e.key)) {
                        typed += e.key;
                        clearTimeout(typedTimer);
                        typedTimer = setTimeout(function () { typed = ''; }, 1500);
                    }
                    return;
            }
            e.preventDefault();
        }
        function click(e) {
            if (e.button !== 0 || e.defaultPrevented) { return; }
            var t = e.target;
            if (t && t.closest && t.closest('a, button, input, textarea, select, [data-applet], iframe, details, summary')) { return; }
            if (e.clientX < stage.getBoundingClientRect().left + stage.clientWidth * 0.2) { prev(); } else { next(); }
        }
        stage.addEventListener('keydown', keys);
        stage.addEventListener('click', click);
        goTo(o.start || 0, o.step || 0);
        return {
            goTo: goTo, next: next, prev: prev,
            position: function () { return { slide: at, step: step, steps: stepsOf(slides[at]).length, count: slides.length }; },
            slides: function () { return slides; },
            destroy: function () {
                destroyed = true;
                stage.removeEventListener('keydown', keys);
                stage.removeEventListener('click', click);
                bar.remove(); counter.remove();
                stage.classList.remove('deck-presenting');
                slides.forEach(function (s) { s.classList.remove('deck-current', 'deck-before', 'deck-after'); s.querySelectorAll('.deck-step-hidden').forEach(function (li) { li.classList.remove('deck-step-hidden'); }); });
            }
        };
    }

    /* The exported page: the stage fills the window and presents itself. */
    function start(stage) {
        var show = attach(stage, {});
        function fit() {
            var canvas = stage.querySelectorAll('.deck-canvas');
            var w = stage.clientWidth, h = stage.clientHeight;
            canvas.forEach(function (c) {
                var cw = c.offsetWidth || 1280, ch = c.offsetHeight || 720, k = Math.min(w / cw, h / ch);
                c.style.transform = 'translate(-50%, -50%) scale(' + k + ')';
            });
        }
        window.addEventListener('resize', fit);
        fit();
        stage.focus();
        return show;
    }

    window.yavcPresent = { attach: attach, start: start };
})();
