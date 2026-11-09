/* Copyright 2026 Paul Parks. SPDX-License-Identifier: Apache-2.0 */
(function () {
    'use strict';
    var count = 0;
    /* ECMAScript GetSubstitution, using the captures and context from the
       full input. This also handles lookaround and named captures. */
    function substitute(value, match, input) {
        return value.replace(/\$(\$|&|`|'|<([^>]*)>|[0-9]{1,2})/g, function (token, key, name) {
            if (key === '$') return '$';
            if (key === '&') return match[0];
            if (key === '`') return input.slice(0, match.index);
            if (key === "'") return input.slice(match.index + match[0].length);
            if (key.charAt(0) === '<') return match.groups ? match.groups[name] || '' : token;
            var index = Number(key);
            if (index > 0 && index < match.length) return match[index] || '';
            index = Number(key.charAt(0));
            if (key.length === 2 && index > 0 && index < match.length) return (match[index] || '') + key.charAt(1);
            return token;
        });
    }
    function attach(o) {
        var id = 'slide-search-scope-' + (++count);
        var bar = null, scope = 'slide', query = null, matches = [], at = -1, error = false, revision = 0;
        function expression() {
            if (!query || !query.search) return null;
            var pattern = query.regexp ? query.search : query.search.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
            if (query.wholeWord) pattern = '(?<![\\p{L}\\p{N}_])(?:' + pattern + ')(?![\\p{L}\\p{N}_])';
            return new RegExp(pattern, 'gu' + (query.caseSensitive ? '' : 'i'));
        }
        function collect() {
            matches = []; error = false;
            try {
                var re = expression(); if (!re) return;
                o.deck().slides.forEach(function (slide, index) {
                    if (scope !== 'deck' && slide !== o.current()) return;
                    re.lastIndex = 0; var m;
                    while ((m = re.exec(slide.text)) && matches.length < 10000) {
                        if (!m[0]) { re.lastIndex += slide.text.codePointAt(re.lastIndex) > 65535 ? 2 : 1; continue; }
                        matches.push({ slide: slide, index: index, from: m.index, to: m.index + m[0].length, text: slide.text });
                    }
                });
            } catch (err) { error = true; }
        }
        function replacement(match) {
            var value = query.replace || '';
            if (!query.regexp) return value;
            var re = expression(); re.lastIndex = match.from;
            return substitute(value, re.exec(match.text), match.text);
        }
        async function show(index) {
            var match = matches[index], deck = o.deck(), ticket = ++revision;
            if (!match) return;
            await o.textPane();
            if (!o.alive() || deck !== o.deck() || ticket !== revision) return;
            if (o.current() !== match.slide) await o.go(match.index);
            if (!o.alive() || deck !== o.deck() || ticket !== revision || o.current() !== match.slide || match.slide.text !== match.text) return;
            at = index;
            o.source().dispatch({ selection: { anchor: match.from, head: match.to }, scrollIntoView: true });
            if (bar) bar.refresh();
        }
        function move(index) { show(index).catch(function (err) { if (o.alive()) o.say(err.message, true); }); }
        async function replace(all) {
            if (o.readOnly && o.readOnly()) return;
            if (!matches.length || error) return;
            var deck = o.deck(), owner = o.current(), savedQuery = query, savedScope = scope, selectedAt = at;
            var savedMatch = matches[Math.max(0, Math.min(at, matches.length - 1))];
            await o.textPane();
            if (!o.alive() || deck !== o.deck() || owner !== o.current() || query !== savedQuery || scope !== savedScope) return;
            if (!all && savedMatch.slide.text !== savedMatch.text) {
                collect(); at = -1; if (bar) bar.refresh(); o.say('The slide changed. Review the match before replacing.', true); return;
            }
            collect(); at = selectedAt;
            if (!matches.length || error) { if (bar) bar.refresh(); return; }
            if (all && matches.length >= 10000) { o.say('There are too many matches. Refine the search before replacing all.', true); return; }
            var selected = all ? matches.slice() : [matches[Math.max(0, Math.min(at, matches.length - 1))]];
            if (selected.some(function (m) { return o.deck().slides[m.index] !== m.slide || m.slide.text !== m.text || (scope !== 'deck' && m.slide !== o.current()); })) { collect(); at = -1; if (bar) bar.refresh(); o.say('The deck changed. Review the matches before replacing.', true); return; }
            var edits = new Map(), total = o.text().length, budget = Math.max(1048576, total);
            if ((query.replace || '').length > 262144) { o.say('The replacement text is too large.', true); return; }
            for (var m of selected) {
                if (!edits.has(m.slide)) edits.set(m.slide, []);
                var insert = replacement(m); total += insert.length - (m.to - m.from);
                if (total > budget) { o.say('The replacement would make the deck too large.', true); return; }
                edits.get(m.slide).push({ from: m.from, to: m.to, insert: insert });
            }
            o.replace(edits); collect(); at = -1;
            if (matches.length) move(0);
            if (bar) bar.refresh();
            o.say('Replaced ' + selected.length + (selected.length === 1 ? ' match.' : ' matches.') + (all ? ' Undo is available on each affected slide in Text view.' : ''));
        }
        var driver = {
            set: function (q) { query = q; revision++; collect(); at = -1; if (matches.length) move(0); },
            next: function () { collect(); if (matches.length) move((at + 1) % matches.length); },
            prev: function () { collect(); if (matches.length) move((at - 1 + matches.length) % matches.length); },
            replace: function () { replace(false).catch(function (err) { if (o.alive()) o.say(err.message, true); }); }, replaceAll: function () { replace(true).catch(function (err) { if (o.alive()) o.say(err.message, true); }); },
            readOnly: function () { return !!(o.readOnly && o.readOnly()); }, count: function () { return { total: matches.length, at: at + 1, error: error }; },
            close: function () { revision++; o.place.hidden = true; if (o.source()) o.source().focus(); }
        };
        return {
            open: async function () {
                var deck = o.deck(); await o.textPane(); if (!o.alive() || deck !== o.deck()) return;
                if (!bar) {
                    o.place.innerHTML = '<div class="deck-find-scope"><label class="form-label" for="' + id + '">Search</label><div class="form-select-wrap"><select class="form-select" id="' + id + '"><option value="slide">Current Slide</option><option value="deck">Whole Deck</option></select></div><span class="form-help">Searches Markdown, including notes. Results open in the text pane.</span></div>';
                    o.place.querySelector('select').addEventListener('change', function (e) { scope = e.target.value; driver.set(query); bar.refresh(); });
                    bar = window.yavcMdEditor.findBar(driver); o.place.appendChild(bar.dom);
                }
                o.place.hidden = false; driver.set(query); bar.refresh(); bar.focus();
            },
            reset: function () { revision++; matches = []; at = -1; if (bar) bar.refresh(); o.place.hidden = true; },
            changed: function (cancel) { if (cancel !== false) revision++; if (!o.place.hidden && bar) { collect(); at = -1; bar.refresh(); } }
        };
    }
    window.yavcSlideSearch = { attach: attach, substitute: substitute };
})();
