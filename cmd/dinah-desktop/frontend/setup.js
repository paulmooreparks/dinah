/* The guided set-up (setup.go; the approved mockup is
   docs/mockups/guided-setup, third form, 20 November 2026).

   A window that opens by itself once and from Help at any time. Step 1
   lists the agent programs and says of each only what the product can tell,
   which is whether its program was found; choosing one tests its
   connection. Step 2 asks a few questions, each with its choices as a list,
   and takes answers from the person's own words when they would sooner say
   them. Step 3 shows every setting before anything is set. Set It Up sets
   them and opens the windows chosen, and Undo, which takes its place, puts
   them back. Two rows say what Undo cannot take back: a sign-in stays, and
   the small questions in the background are turned off again.

   A product that connects to services with a token has a step for each
   after the first (internal/connections; Dinah has none). The step says
   how the token is made, opens the service, and takes the token in a field
   that saves it where the connection keeps its token and signs in with it
   once. A token is saved when Save and Test is pressed, and Undo leaves it.

   The foot has Back and Next at every step. Next is disabled while the
   step cannot be passed and enabled when it can (Paul, 21 November 2026);
   the last step's button is Set It Up, and then Undo. */
(function () {
  'use strict';
  var H = window.dinahHelpers, node = H.node;
  function app() { return H.app(); }
  function P() { return window.pudlWindows; }
  function productName() { return window.dinahProduct ? window.dinahProduct.name : ''; }
  var KIND = 'setup', MIN_W = 600, MIN_H = 480, WIDE = 760, TALL = 540;
  var opening = false;

  function init(root) {
    root.classList.add('setup-window');
    var frame = root.closest('[data-win]'), key = frame ? frame.dataset.win : '';
    if (frame) {
      /* PUDL reads a window's limits when it takes the window in, which is
         before this runs, so both the attribute and the two properties its
         stylesheet reads are set here (as assistant.js does). */
      frame.setAttribute('data-win-min', MIN_W + ',' + MIN_H);
      frame.style.setProperty('--win-min-w', MIN_W + 'px'); frame.style.setProperty('--win-min-h', MIN_H + 'px');
    }
    if (key && window.dinahWorkbench.retitle) window.dinahWorkbench.retitle(key, 'Set Up ' + productName());
    var disposed = false;

    /* ---- The parts, each made once ---------------------------------------- */
    function button(words, cls) { var b = node('button', cls || 'btn', words); b.type = 'button'; return b; }
    var steps = node('ol', 'setup-steps'); steps.setAttribute('aria-label', 'Steps');
    /* The steps are listed when the offer says which connections there are:
       the agent, each connection, the questions, review. Q and R are the
       places of the last two. */
    var Q = 1, R = 2, conns = [];
    function listSteps() {
      var names = ['Agent'].concat(conns.map(function (c) { return 'Connect ' + c.label; }), ['Questions', 'Review']);
      steps.replaceChildren.apply(steps, names.map(function (name, i) {
        var li = node('li'); li.dataset.step = String(i); li.append(node('span', 'n', String(i + 1)), node('span', '', name)); return li;
      }));
    }
    var panes = [node('div', 'setup-pane'), node('div', 'setup-pane'), node('div', 'setup-pane')];
    var agentRows = node('tbody'), agentTable = node('table', 'data-table setup-agents');
    agentTable.setAttribute('aria-label', 'Agents'); agentTable.appendChild(agentRows);
    panes[0].append(node('h2', '', 'Which agent should ' + productName() + ' use?'), agentTable,
      node('p', 'form-help', 'Choosing an agent starts it and tests that it answers. No message is sent to it.'));
    var said = node('div', 'setup-said'); said.setAttribute('role', 'log'); said.setAttribute('aria-label', 'What was said'); said.tabIndex = 0;
    var questionId = 'setup-question-' + Date.now();
    var question = node('p', 'setup-question'); question.id = questionId;
    var choices = node('div', 'setup-choices'); choices.setAttribute('role', 'listbox'); choices.setAttribute('aria-labelledby', questionId);
    var own = node('form', 'setup-own'), line = node('textarea', 'form-input'), say = button('Say'); say.type = 'submit';
    line.rows = 2; line.maxLength = 2000; line.setAttribute('aria-label', 'Say it in your own words'); line.placeholder = 'Or say it in your own words.';
    own.append(line, say);
    panes[1].append(said, question, choices, own);
    var reviewTitle = node('h2', '', 'What will be set'); reviewTitle.id = 'setup-review-' + Date.now();
    var reviewRows = node('tbody'), review = node('table', 'data-table setup-review'), head = node('thead'), headRow = node('tr');
    ['Setting', 'Value', ''].forEach(function (h) { var th = node('th', '', h); th.scope = 'col'; if (!h) th.setAttribute('aria-label', 'Change'); headRow.appendChild(th); });
    head.appendChild(headRow); review.append(head, reviewRows); review.setAttribute('aria-labelledby', reviewTitle.id);
    panes[2].append(reviewTitle, review);
    var body = node('div', 'setup-body'); body.append(steps, panes[0], panes[1], panes[2]);
    var skip = button('Skip Set-Up'), status = node('span', 'setup-status'), back = button('Back'), next = button('Next', 'btn btn-primary');
    status.setAttribute('role', 'status');
    var foot = node('div', 'setup-foot'); foot.append(skip, status, back, next);
    root.append(body, foot);

    /* ---- What is known and what was answered ------------------------------ */
    var offer = null, agent = '', tests = {}, asking = null, signed = '';
    var step = 0, at = 0, picked = null, answers = {}, from = {}, values = {};
    var busy = false, applied = false, opened = [];

    function note(text) { H.setText(status, text); status.title = text; }
    function chosen() { return offer ? offer.agents.filter(function (a) { return a.id === agent; })[0] : null; }
    function agentName() { var a = chosen(); return a ? a.name : 'the agent'; }
    function judgeName() { var a = chosen(); return a && a.judges ? a.name : offer.judgeName; }

    /* The questions that are asked, with the choices each has now. The one
       about what the agent asks is the chosen agent's own setting, and is
       left out for an agent that has none; the one about the background is
       left out when it is answered already or no agent can answer it. */
    function questions() {
      /* A product whose people all do one kind of work offers no uses, and
         the question is left out. */
      var qs = [];
      if (offer.uses && offer.uses.length) qs.push({ id: 'use', setting: 'Windows open at the start', ask: 'What will you use ' + productName() + ' for most?',
        choices: offer.uses.map(function (u) { return { id: u.id, label: u.label, value: u.opens }; }) });
      if (asking && asking.choices.length) qs.push({ id: 'ask', setting: 'Before ' + agentName() + ' acts', ask: 'How much should ' + agentName() + ' ask you before it acts?',
        choices: asking.choices.map(function (c) { return { id: c.id, label: c.label, value: c.label }; }) });
      /* A person with no workspace yet is offered a first one, made of a
         folder they choose (ApplySetup). One who has a workspace is asked
         nothing about them: there are as many as a person wants, and they
         are made and chosen in the Workspace window. */
      if (!offer.workspaces) qs.push({ id: 'folder', setting: 'First workspace', ask: 'A workspace is a folder your agents work in, and you can have as many as you like. Make your first one now?',
        choices: [{ id: 'pick', label: values.folder ? 'Make a workspace of ' + values.folder : 'Choose a Folder…', value: values.folder ? 'A workspace is made of ' + values.folder : '' }, { id: 'none', label: 'Not now', value: 'None yet' }] });
      var names = [];
      if (offer.personName) names.push({ id: 'keep', label: offer.personName + ', as now', value: 'No change, ' + offer.personName });
      names.push({ id: 'account', label: offer.accountName + ', from this computer’s account', value: offer.accountName });
      if (values.name) names.push({ id: 'typed', label: values.name, value: values.name });
      qs.push({ id: 'name', setting: 'What agents call you', ask: 'What should the agents call you? Choose one, or type another name below.', choices: names });
      if (!offer.judgeOn && judgeName()) qs.push({ id: 'judge', setting: 'Small questions in the background',
        ask: 'May ' + judgeName() + ' answer small questions in the background, such as a chat’s title and a suggested reply? At most ' + offer.judgeCalls + ' calls a day.',
        choices: [{ id: 'on', label: 'Yes', value: 'On, at most ' + offer.judgeCalls + ' calls a day' }, { id: 'off', label: 'Not now', value: 'Off' }] });
      return qs;
    }
    function choiceOf(q) { return q.choices.filter(function (c) { return c.id === answers[q.id]; })[0]; }
    function nextOpen(qs) { for (var i = 0; i < qs.length; i++) if (!choiceOf(qs[i])) return i; return -1; }
    function sayLine(who, text) { said.appendChild(node('p', who, text)); said.scrollTop = said.scrollHeight; }
    function answer(q, id, how) { answers[q.id] = id; from[q.id] = how; }

    /* ---- Step 1 ----------------------------------------------------------- */
    function test(id) {
      tests[id] = { state: 'testing' }; draw();
      return Promise.resolve().then(function () { return app().AgentCheck(id); }).then(function (info) {
        var ok = info && info.status === 'connected';
        tests[id] = { state: ok ? 'ok' : 'fail', why: ok ? '' : (info && (info.detail || info.statusWords)) || 'It did not answer.' };
        return ok && app().SetupAsking ? app().SetupAsking(id) : null;
      }, function (err) { tests[id] = { state: 'fail', why: H.message(err, 'sentence') }; return null; }).then(function (a) {
        if (disposed || id !== agent) return;
        asking = a; delete answers.ask;
        var t = tests[id];
        if (step === 0) note(t.state === 'fail' ? agentName() + ' does not answer. ' + t.why : '');
        draw();
      });
    }
    function choose(id) { agent = id; asking = null; delete answers.ask; note(''); test(id); }
    function drawAgents() {
      agentRows.replaceChildren.apply(agentRows, offer.agents.map(function (a) {
        var tr = node('tr'), pick = node('td'), name = node('td'), state = node('td'), act = node('td');
        var radio = node('input'); radio.type = 'radio'; radio.name = 'setup-agent-' + key; radio.id = 'setup-agent-' + a.id + '-' + key;
        radio.checked = a.id === agent; radio.disabled = !a.found || busy || applied;
        radio.addEventListener('change', function () { choose(a.id); });
        pick.appendChild(radio);
        var label = node('label', '', a.name); label.htmlFor = radio.id; name.appendChild(label);
        var t = a.id === agent ? tests[a.id] : null;
        var word = !a.found ? 'Not found' : !t ? 'Found' : t.state === 'testing' ? 'Testing…' : t.state === 'ok' ? 'Answers' : 'Does not answer';
        state.appendChild(node('span', 'badge' + (word === 'Answers' ? ' accent' : ''), word));
        if (!a.found && a.words) state.title = a.words;
        /* An agent that does not answer is tested again from its own row;
           Next waits until it answers. */
        if (t && t.state === 'fail') {
          var again = button('Test Again', 'btn btn-sm'); again.disabled = busy || applied;
          again.addEventListener('click', function () { note(''); test(a.id); });
          act.appendChild(again);
        }
        if (!a.found ? a.installPage : a.signIn) {
          var b = button(a.found ? 'Sign In…' : 'How to Install', 'btn btn-sm'); b.disabled = busy || applied;
          b.addEventListener('click', function () {
            if (!a.found) {
              Promise.resolve().then(function () { return app().OpenInstallPage(a.id); }).then(function () { note('The page that says how to install ' + a.name + ' is open in your browser.'); },
                function (err) { note(H.message(err, 'sentence')); });
              return;
            }
            note('Signing in to ' + a.name + '…');
            Promise.resolve().then(function () { return app().AgentSignIn(a.id); }).then(function () {
              signed = a.name;
              note('Finish signing in to ' + a.name + ' in your browser, then press Test Again.');
              if (a.id === agent) test(a.id);
            }, function (err) { note(H.message(err, 'sentence')); });
          });
          act.appendChild(b);
        }
        tr.append(pick, name, state, act);
        return tr;
      }));
    }

    /* ---- The connections ---------------------------------------------------
       Each has a pane made once, when the offer comes, and only its words
       change after that. A connection that has a token is tested at once,
       so its step says who is signed in. */
    var links = {};
    function makeConn(c) {
      var pane = node('div', 'setup-pane setup-conn'), head = node('div', 'setup-conn-head'), badge = node('span', 'badge');
      head.append(node('h2', '', 'Connect ' + c.label), badge);
      var list = node('ol', 'setup-conn-steps');
      ['Press Open ' + c.label + ' and sign in.'].concat(c.tokenSteps).forEach(function (t) { list.appendChild(node('li', '', t)); });
      var form = node('div', 'setup-conn-form');
      var la = node('label', '', 'Address'), address = node('input', 'form-input'), site = button('Open ' + c.label);
      address.type = 'url'; address.spellcheck = false; address.id = 'setup-address-' + c.id + '-' + key; la.htmlFor = address.id; address.value = c.address || c.defaultAddress || '';
      var lt = node('label', '', 'Token'), token = node('input', 'form-input'), save = button('Save and Test');
      token.type = 'password'; token.autocomplete = 'off'; token.spellcheck = false; token.id = 'setup-token-' + c.id + '-' + key; lt.htmlFor = token.id;
      form.append(la, address, site, lt, token, save);
      pane.append(head, list, form, node('p', 'setup-conn-kept', 'The token is kept in ' + (c.tokenStore || 'this computer’s credential store') + '. It is not shown again, and it is not sent to the agent.'));
      var l = links[c.id] = { pane: pane, badge: badge, address: address, token: token, site: site, save: save, is: 'none', has: !!c.hasToken, line: '', why: c.problem || '', saved: false };
      token.addEventListener('input', draw);
      site.addEventListener('click', function () {
        Promise.resolve().then(function () { return app().OpenConnectionSite(c.id, address.value); }).then(function () { note(c.label + ' is open in your browser.'); }, function (err) { note(H.message(err, 'sentence')); });
      });
      save.addEventListener('click', function () {
        var value = token.value.trim();
        l.is = 'testing'; note('Signing in to ' + c.label + '…'); draw();
        Promise.resolve().then(function () { return app().SetConnection(c.id, address.value, value); }).then(function () {
          token.value = ''; l.has = true; if (value) l.saved = true;
          return signIn(c);
        }, function (err) { l.is = l.line ? 'ok' : l.has ? 'fail' : 'none'; l.why = H.message(err, 'sentence'); if (!disposed) { sayConn(c); draw(); } });
      });
      return pane;
    }
    function signIn(c) {
      var l = links[c.id];
      l.is = 'testing'; draw();
      return Promise.resolve().then(function () { return app().TestConnection(c.id); }).then(function (words) {
        l.is = 'ok'; l.line = String(words || 'Connected to ' + c.label + '.'); l.why = '';
      }, function (err) { l.is = 'fail'; l.line = ''; l.why = H.message(err, 'sentence'); }).then(function () { if (!disposed) { sayConn(c); draw(); } });
    }
    /* What is known of a connection is said at the foot while its step is
       the one in hand. */
    function sayConn(c) { if (conns[step - 1] === c) { var l = links[c.id]; note(l.is === 'ok' ? l.line : l.why); } }
    function drawConn(c) {
      var l = links[c.id], testing = l.is === 'testing';
      var words = l.is === 'ok' ? l.line.replace(/\.$/, '') : testing ? 'Testing…' : l.is === 'fail' ? 'Not connected' : 'No token yet';
      H.setText(l.badge, words); l.badge.title = words; l.badge.classList.toggle('accent', l.is === 'ok');
      l.token.placeholder = l.has ? 'Paste another to replace it' : 'Paste the token here';
      l.address.disabled = l.token.disabled = l.site.disabled = testing || busy || applied;
      l.save.disabled = testing || busy || applied || !(l.token.value.trim() || l.has);
    }

    /* ---- The questions ---------------------------------------------------- */
    function drawQuestion() {
      var qs = questions(), q = qs[Math.min(at, qs.length - 1)];
      H.setText(question, q.ask);
      choices.replaceChildren.apply(choices, q.choices.map(function (c) {
        var row = node('div', 'md-row' + (picked === c.id ? ' active' : '')), b = node('button', 'md-item', c.label);
        b.type = 'button'; b.setAttribute('role', 'option'); b.setAttribute('aria-selected', String(picked === c.id)); b.disabled = busy;
        b.addEventListener('click', function () {
          if (q.id === 'folder' && c.id === 'pick') { pickFolder(); return; }
          picked = c.id; draw();
        });
        row.appendChild(b);
        return row;
      }));
      line.disabled = say.disabled = busy;
    }
    function pickFolder() {
      busy = true; draw();
      Promise.resolve().then(function () { return app().ChooseImportFolder(values.folder || offer.workspace || ''); }).then(function (path) {
        if (path) { values.folder = path; picked = 'pick'; }
      }, function (err) { note(H.message(err, 'sentence')); }).then(function () { busy = false; if (!disposed) draw(); });
    }
    function toQuestions() {
      var qs = questions();
      if (!said.childNodes.length) sayLine('guide', 'I am ' + agentName() + '. A few questions, and then I show you everything I would set before anything is set.');
      at = nextOpen(qs); if (at < 0) at = qs.length - 1;
      var c = choiceOf(qs[at]); picked = c ? c.id : null;
    }
    /* What the person said in their own words. The engine has the chosen
       agent read them, and matches words when it does not answer
       (ReadSetupWords). At the question about the name, words that answer
       nothing are the name. */
    function readOwn(text) {
      /* Every question is put to the reading, the answered ones too: what
         a person says now takes the place of what they chose before. */
      var current = questions(), asked = current[at];
      busy = true; note(agentName() + ' is reading that…'); draw();
      var input = current.map(function (q) { return { id: q.id, choices: q.id === 'use' || q.id === 'ask' ? q.choices.map(function (c) { return { id: c.id, label: c.label }; }) : [] }; });
      return Promise.resolve().then(function () { return app().ReadSetupWords(text, agent, input); }).then(function (reading) {
        var got = reading && reading.answers || {}, took = [];
        if (!Object.keys(got).length && asked.id === 'name' && text.length <= 80 && !/\n/.test(text)) got = { name: text };
        current.forEach(function (q) {
          var v = got[q.id]; if (!v) return;
          if (q.id === 'folder') { values.folder = v; v = 'pick'; }
          if (q.id === 'name') { values.name = v; v = 'typed'; }
          answer(q, v, 'What you said');
        });
        questions().forEach(function (q) { if (got[q.id] && choiceOf(q)) took.push(q.setting.toLowerCase() + ': ' + choiceOf(q).value); });
        note('');
        return took;
      }, function (err) { note(H.message(err, 'sentence')); return null; }).then(function (took) { busy = false; return took; });
    }

    /* ---- Step 3 ----------------------------------------------------------- */
    function drawReview() {
      var qs = questions();
      /* A row is its setting, its value, where the value came from, the
         step that changes it and, for a question, which one. A connection's
         row is not among what Set It Up sets, so its words stay as they are
         afterwards. */
      var rows = [['Agent', agentName(), 'Step 1' + (signed ? ' · Undo does not sign you out of ' + signed : ''), 0, -1, true]].concat(conns.map(function (c, i) {
        var l = links[c.id];
        return [c.label, l.is === 'ok' ? l.line.replace(/\.$/, '') : 'Not connected',
          l.is === 'ok' ? (l.saved ? 'Token saved in step ' + (i + 2) + ' · Undo does not remove it' : 'Connected already') : 'Settings › Connections connects it later', i + 1, -1, false];
      }), qs.map(function (q, i) {
        var c = choiceOf(q);
        return [q.setting, c ? c.value : '', (from[q.id] || '') + (q.id === 'judge' && answers.judge === 'on' ? ' · Undo turns this off' : ''), Q, i, true];
      }));
      reviewRows.replaceChildren.apply(reviewRows, rows.map(function (r) {
        var tr = node('tr'), value = node('td', '', r[1] + ' '), act = node('td');
        value.appendChild(node('span', 'from', '· ' + (applied && r[5] ? r[2].replace(/^[^·]*/, 'set ').trim() : r[2])));
        var b = button('Change', 'btn btn-sm'); b.disabled = busy || applied;
        b.addEventListener('click', function () {
          step = r[3]; enter();
          if (r[4] >= 0) { at = r[4]; var c = choiceOf(questions()[at]); picked = c ? c.id : null; }
          draw();
        });
        act.appendChild(b);
        tr.append(node('td', '', r[0]), value, act);
        return tr;
      }));
    }
    function setIt() {
      var c = { agent: agent, asking: answers.ask || '', folder: answers.folder === 'pick' ? values.folder : '', judge: answers.judge === 'on',
        setName: answers.name === 'account' || answers.name === 'typed', name: answers.name === 'typed' ? values.name : '' };
      var use = (offer.uses || []).filter(function (u) { return u.id === answers.use; })[0];
      busy = true; note('Setting it up…'); draw();
      return Promise.resolve().then(function () { return app().ApplySetup(c); }).then(function () {
        applied = true; opened = [];
        var before = P().state().open.slice();
        return (use ? use.windows : []).reduce(function (done, kind) {
          return done.then(function () { return window.dinahWorkbench.open(kind); }).then(function (k) { if (before.indexOf(k) < 0) opened.push(k); }, function () {});
        }, Promise.resolve()).then(function () {
          if (key) P().raise(key);
          note('Everything is set. Each can be changed in Settings.');
        });
      }, function (err) { note(H.message(err, 'sentence')); }).then(function () { busy = false; if (!disposed) draw(); });
    }
    function undoIt() {
      busy = true; note('Putting it back…'); draw();
      return Promise.resolve().then(function () { return app().UndoSetup(); }).then(function () {
        note('The settings are back as they were.' + (conns.some(function (c) { return links[c.id].saved; }) ? ' The tokens are still saved.' : ''));
      }, function (err) { note('Not everything could be put back: ' + H.message(err, 'sentence')); }).then(function () {
        opened.forEach(function (k) { try { if (P().state().open.indexOf(k) >= 0) P().close(k); } catch (err) { /* The window is gone already. */ } });
        opened = []; applied = false; busy = false;
        if (!disposed) draw();
      });
    }

    /* ---- The whole window ------------------------------------------------- */
    function draw() {
      if (!offer) return;
      panes.forEach(function (p, i) { p.hidden = i !== step; });
      Array.prototype.forEach.call(steps.children, function (li) {
        var n = Number(li.dataset.step);
        if (n === step) li.setAttribute('aria-current', 'step'); else li.removeAttribute('aria-current');
        li.toggleAttribute('data-done', n < step || applied);
      });
      back.disabled = step === 0 || applied || busy;
      H.setText(skip, applied ? 'Close' : 'Skip Set-Up');
      var t = tests[agent];
      /* Next waits for an agent that answers and for an answer to the
         question in hand. A connection's step can be passed, so Next waits
         there only while its token is tested. */
      H.setText(next, step < R ? 'Next' : applied ? 'Undo' : 'Set It Up');
      if (step === 0) { drawAgents(); next.disabled = busy || !t || t.state !== 'ok'; }
      else if (step < Q) { drawConn(conns[step - 1]); next.disabled = busy || links[conns[step - 1].id].is === 'testing'; }
      else if (step === Q) { drawQuestion(); next.disabled = busy || !picked; }
      else { drawReview(); next.disabled = busy; }
      next.classList.toggle('btn-primary', !(step === R && applied));
    }
    /* What a step needs when it becomes the one in hand. */
    function enter() {
      note('');
      if (step > 0 && step < Q) sayConn(conns[step - 1]);
      else if (step === Q) toQuestions();
    }
    next.addEventListener('click', function () {
      if (step < Q) { step++; enter(); }
      else if (step === Q) {
        var qs = questions(), q = qs[at], c = q.choices.filter(function (x) { return x.id === picked; })[0];
        sayLine('guide', q.ask); answer(q, picked, 'Your answer'); sayLine('you', c ? c.label : '');
        at = nextOpen(questions());
        if (at < 0) { step = R; note(''); } else picked = null;
      } else if (!applied) { setIt(); return; }
      else { undoIt(); return; }
      draw();
    });
    back.addEventListener('click', function () {
      var qs = questions(), c;
      note('');
      if (step === R) { step = Q; at = qs.length - 1; c = choiceOf(qs[at]); picked = c ? c.id : null; }
      else if (step === Q && at > 0) { at--; c = choiceOf(qs[at]); picked = c ? c.id : null; }
      else { step--; if (step > 0) sayConn(conns[step - 1]); }
      draw();
    });
    own.addEventListener('submit', function (e) {
      e.preventDefault();
      var text = line.value.trim(); if (!text || busy) return;
      line.value = '';
      sayLine('you', text);
      readOwn(text).then(function (took) {
        if (disposed) return;
        if (!took) { draw(); return; }
        if (!took.length) { sayLine('guide', 'I could not take an answer from that. Choose one above, or say it another way.'); draw(); return; }
        sayLine('guide', 'From that I take ' + took.join('; ') + '.');
        var qs = questions(), left = qs.filter(function (q) { return !choiceOf(q); }).length;
        at = nextOpen(qs);
        if (at < 0) { step = R; note(''); } else { picked = null; sayLine('guide', 'That leaves ' + (left === 1 ? 'one question.' : left + ' questions.')); }
        draw();
      });
    });
    line.addEventListener('keydown', function (e) { if (e.key === 'Enter' && !e.shiftKey && !e.isComposing) { e.preventDefault(); own.requestSubmit(); } });
    skip.addEventListener('click', function () { if (key) P().close(key); });

    note('Looking for the agents on this computer…');
    next.disabled = back.disabled = true;
    Promise.resolve().then(function () { return app().SetupOffer(); }).then(function (o) {
      if (disposed) return;
      offer = o; note('');
      conns = o.connections || []; Q = 1 + conns.length; R = Q + 1;
      listSteps();
      var made = conns.map(makeConn);
      made.forEach(function (pane) { body.insertBefore(pane, panes[1]); });
      panes.splice.apply(panes, [1, 0].concat(made));
      conns.forEach(function (c) { if (c.hasToken) signIn(c); });
      var first = o.agents.filter(function (a) { return a.id === o.default && a.found; })[0] || o.agents.filter(function (a) { return a.found; })[0];
      if (first) choose(first.id); else note('No agent was found. Install one, then open this again from Help.');
      draw();
    }, function (err) { note(H.message(err, 'sentence')); });

    return { focus: function () { next.focus(); }, destroy: function () { disposed = true; } };
  }

  /* The window floats in the middle of the room the docked windows leave, at
     the size the approved mockup has where the room allows it, and in front (Paul, 20 November 2026; it had
     been snapped from the top of that room to its bottom). The window layer
     asks where a window opens (its pudl:window-place event) and takes a
     place as fractions of that room, so the room is measured: by the box
     the layer keeps for measuring it, or the layer itself. The window opens
     here every time, whatever place it had before. */
  function centred() {
    var room = document.querySelector('[data-win-layer] > .win-inner') || document.querySelector('[data-win-layer]');
    var r = room ? room.getBoundingClientRect() : { width: 0, height: 0 };
    var w = r.width ? Math.min(1, WIDE / r.width) : 0.6, h = r.height ? Math.min(1, TALL / r.height) : 0.7;
    return { mode: 'floating', x: (1 - w) / 2, y: (1 - h) / 2, w: w, h: h };
  }
  document.addEventListener('pudl:window-place', function (e) { if (e.detail.key === KIND) e.detail.placement = centred(); }, true);
  function open() { return window.dinahWorkbench.open(KIND).then(function (key) { P().raise(key); return key; }); }
  window.dinahWorkbench.register(KIND, { pinnable: false, singleton: true, title: 'Set-Up', glyph: 'gear', init: init });
  window.dinahSetup = { open: open, opening: function () { return opening; } };

  document.addEventListener('DOMContentLoaded', function () {
    if (window.dinahCommands) window.dinahCommands.register({ id: 'setup', menu: 'help', after: 'commands', label: function () { return 'Set Up ' + productName() + '…'; }, run: function () { return open(); } });
    /* The window opens by itself once, when the saved windows are drawn. It
       says so to the engine first, so a second window of the product, or a
       start that follows at once, does not open it again. */
    var tried = false;
    function offerOnce() {
      if (tried) return; tried = true;
      setTimeout(function () {
        if (!app() || !app().SetupShown) return;
        Promise.resolve().then(function () { return app().SetupShown(); }).then(function (shown) {
          if (shown) return;
          opening = true;
          return Promise.resolve(app().NoteSetupShown()).then(open);
        }).catch(function () {});
      }, 1200);
    }
    document.addEventListener('dinah:ui-state-restored', offerOnce, { once: true });
    setTimeout(offerOnce, 6000);
  });
})();
