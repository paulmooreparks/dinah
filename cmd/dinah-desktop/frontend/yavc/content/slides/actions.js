/* Copyright 2026 Paul Parks. SPDX-License-Identifier: Apache-2.0 */
/* Slides' actions over text alone (actions.json, docs/agents-design.md):
   parse a deck and make one. They use the deck functions slides.js puts
   at window.yavcSlides, loading it beside this module when the applet is
   not on the page. The live actions are answered by the open applet,
   which attaches them as it starts. */
(function () {
    'use strict';

    function lib(ctx) {
        if (window.yavcSlides) { return Promise.resolve(window.yavcSlides); }
        return ctx.load('slides.js').then(function () {
            if (!window.yavcSlides) { throw new Error('slides.js did not give its deck functions'); }
            return window.yavcSlides;
        });
    }

    window.pudlApplets.provide('yavc.slides', {
        'parse@1': function (input, ctx) {
            return lib(ctx).then(function (S) {
                var deck = S.parseDeck(String(input.text));
                return { output: { front: deck.front, slides: deck.slides.map(function (s, i) { return S.slideView(s, i); }) } };
            });
        },
        'make@1': function (input, ctx) {
            return lib(ctx).then(function (S) {
                var deck = { front: Object.assign({}, input.front || {}), slides: [] };
                (input.slides || []).forEach(function (s) {
                    var slide = { text: String(s.text || '').replace(/\s+$/, '') + '\n' };
                    Object.keys(s.settings || {}).forEach(function (k) { S.setSetting(slide, k, s.settings[k]); });
                    if (s.notes) { slide.text += '\n{{notes}}\n' + String(s.notes).replace(/\s+$/, '') + '\n{{/notes}}\n'; }
                    deck.slides.push(slide);
                });
                return { output: { text: S.serializeDeck(deck) } };
            });
        }
    });
})();
