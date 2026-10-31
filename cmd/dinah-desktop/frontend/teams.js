/* Teams compose the pinned PUDL tree, disclosures, meters and notices.
   The bus owns relations, state reasons, usage and relay outcomes. */
(function () {
  'use strict';
  var H = window.dinahHelpers, node = H.node, serial = 0, snapshots = new Map();
  function publish(v) {
    var id = chatId(v && v.lead); if (!id) return;
    var old = snapshots.get(id);
    if (old && old.stateGen === v.stateGen && old.stateVer >= v.stateVer) return;
    snapshots.set(id, v);
    document.dispatchEvent(new CustomEvent('dinah:team-state', { detail: v }));
  }
  window.dinahHost.on('team:state', publish);
  function api() { return H.app(); }
  function host() { return window.dinahHost; }
  function chatId(c) { return c && (c.chat || c.bus) || ''; }
  function address(item, state) {
    var key = window.dinahWorkbench.key(item), url = new URL(location.href);
    var open = (url.searchParams.get('open') || '').split(',').filter(Boolean);
    if (open.indexOf(key) < 0) open.push(key);
    url.searchParams.set('open', open.join(',')); url.searchParams.set('top', key);
    if (state) url.searchParams.set('view.' + key, JSON.stringify(state));
    return url.href;
  }
  function groupAddress(group, message) { return address({ kind: 'bus-room', path: group }, { message: message || 0 }); }
  function groupLink(group, message, mention) {
    return window.dinahBus.openGroup(group, { message: message || 0, mention: mention || null });
  }
  function stateBadge(m) { return window.dinahChatState.badge(m, { reason: false, compact: true, className: 'team-state' }); }
  function disclosure(label, body, id) {
    var b = node('button', 'btn btn-sm disclosure', label);
    b.type = 'button'; body.id = id; body.hidden = true;
    b.setAttribute('aria-controls', id); b.setAttribute('aria-expanded', 'false');
    b.addEventListener('click', function () { body.hidden = !body.hidden; b.setAttribute('aria-expanded', String(!body.hidden)); });
    return b;
  }
  function usageWords(u) {
    if (!u) return 'Usage unavailable';
    if (Array.isArray(u)) return u.length ? u.map(usageWords).join('; ') : 'Usage unavailable';
    var count = u.today !== undefined ? u.today : u.used;
    var gaps = u.todayUnknown !== undefined ? u.todayUnknown : u.unknown;
    var words = typeof count === 'number' && u.unit && (count > 0 || !gaps) ? count.toLocaleString() + ' ' + u.unit + (u.today !== undefined ? ' reported today' : ' reported this period') : 'Usage unavailable';
    if (gaps) words += '; ' + gaps + (gaps === 1 ? ' turn unmeasured' : ' turns unmeasured');
    else if (u.complete === false) words += '; incomplete';
    return words;
  }
  function budgetLines(target, budget, prefix) {
    target.replaceChildren();
    if (!budget || !(budget.rows || []).length) return;
    target.appendChild(node('p', 'form-help', 'Allowance period: ' + new Date(budget.periodStart).toLocaleString() + ' to ' + new Date(budget.periodEnd).toLocaleString() + '.'));
    budget.rows.forEach(function (r, i) {
      var row = node('div', 'team-budget-row vstack gap-1');
      var label = node('span', 'form-label', r.name || r.provider); label.id = prefix + '-budget-' + i;
      row.appendChild(label);
      var text = (r.used === 0 && r.unknown ? 'Period usage unavailable; ' + r.unknown + (r.unknown === 1 ? ' turn unmeasured' : ' turns unmeasured') : r.used.toLocaleString() + ' ' + r.unit + ' reported this period') + '. ' + usageWords(r) + '.';
      if (r.remaining !== undefined && r.complete) text += ' ' + r.remaining.toLocaleString() + ' remaining.';
      if (!r.complete) text += ' Remaining usage is incomplete.';
      if (r.over) text += ' Allowance spent; work can continue.';
      if (r.hasAllowance === true) {
        text += ' Allowance: ' + r.allowance.toLocaleString() + ' ' + r.unit + '.';
        // A zero allowance has no percentage to draw.
        if (r.allowance > 0 && !(r.used === 0 && r.unknown)) {
          var meter = node('meter', 'meter'); meter.min = 0; meter.max = r.allowance; meter.value = r.used;
          meter.setAttribute('aria-labelledby', label.id); row.appendChild(meter);
        }
      }
      if (r.money) text += ' ' + H.money(r.money) + '.';
      var words = node('span', 'form-help', text); words.id = label.id + '-words';
      row.appendChild(words); var bar = row.querySelector('meter'); if (bar) bar.setAttribute('aria-describedby', words.id);
      if (r.over) row.appendChild(node('span', 'badge warn', 'Allowance Spent'));
      target.appendChild(row);
    });
  }
  // Folding changes only presentation. Original rows retain their message and
  // tool data, which the session export reads even when the rows are hidden.
  function foldTraffic(activity, group) {
    var log = activity.querySelector('.run-log:not(.subagent-log)');
    if (!log) return;
    var runs = [], run = null;
    Array.from(log.children).forEach(function (li) {
      if (li.classList.contains('team-traffic-fold')) return;
      var mark = li._teamTraffic || li._delivered;
      var team = !!(group && mark && mark.room === group && mark.id);
      li.classList.toggle('team-traffic-original', team);
      if (!team) { run = null; return; }
      if (!run) { run = { first: li, count: 0, message: mark.id, last: mark.id }; runs.push(run); }
      run.count++; run.last = mark.id;
    });
    var retained = new Set();
    runs.forEach(function (r) {
      var fold = r.first.previousElementSibling;
      if (!fold || !fold.classList.contains('team-traffic-fold')) {
        fold = node('li', 'team-traffic-fold');
        var a = node('a');
        a.addEventListener('click', function (e) { e.preventDefault(); groupLink(fold.dataset.group, Number(fold.dataset.first)).catch(host().fail); });
        fold.appendChild(a); r.first.before(fold);
      }
      retained.add(fold); fold.dataset.group = group; fold.dataset.first = r.message; fold.dataset.last = r.last;
      var link = fold.firstChild; link.href = groupAddress(group, r.message);
      var words = r.count + (r.count === 1 ? ' team message; open the team chat' : ' team messages; open the team chat');
      if (link.textContent !== words) link.textContent = words;
    });
    log.querySelectorAll(':scope > .team-traffic-fold').forEach(function (li) { if (!retained.has(li)) li.remove(); });
  }
  function mount(o) {
    var prefix = 'team-' + (++serial), id = '', current = null, lead = null, generation = 0, disposed = false, infoLeadSignature, relationEpoch = 0;
    var session = null, relationKnown = true, relationError = '', waitsForOpen = false;
    var section = node('section', 'chat-team'), content = node('div', 'team-content vstack gap-2');
    section.hidden = true;
    var toggle = disclosure('Team', content, prefix + '-content');
    section.append(toggle, content);
    o.root.querySelector('.chat-output').before(section);
    var tree = node('ul', 'tree team-tree'); tree.setAttribute('aria-label', 'Team members');
    var retiredTree = node('ul', 'tree team-tree'); retiredTree.setAttribute('aria-label', 'Retired members');
    var retiredBox = node('div', 'team-retired');
    var retiredToggle = disclosure('Retired', retiredTree, prefix + '-retired');
    retiredBox.append(retiredToggle, retiredTree);
    var budget = node('div', 'team-budget');
    var budgetButton = node('button', 'btn btn-sm', 'Team Allowances'); budgetButton.type = 'button';
    budgetButton.addEventListener('click', function () { editBudget(id, current).catch(host().fail); });
    content.append(tree, retiredBox, budget, budgetButton);
    var memberNote = node('div', 'team-member-note form-help'); memberNote.hidden = true;
    o.root.querySelector('[data-view-id="composer"]').appendChild(memberNote);
    var rows = new Map(), expanded = new Map();
    var activity = o.root.querySelector('[data-view-id="activity"]'), foldFrame = 0;
    function observeTraffic() { trafficObserver.observe(activity, { childList: true, subtree: true }); }
    function refreshTraffic() {
      foldFrame = 0;
      if ((!current || !current.group) && !activity.querySelector('.team-traffic-fold')) return;
      trafficObserver.disconnect();
      o.keepPlace(function () { foldTraffic(activity, current && current.group); });
      observeTraffic();
    }
    var trafficObserver = new MutationObserver(function () { if (!foldFrame) foldFrame = requestAnimationFrame(refreshTraffic); });
    observeTraffic();
    function member() { return !!lead; }
    function setLead(value, force) {
      value = value || null;
      if (!value && session && (session.loading || session.restoring) && !force) {
        waitsForOpen = true;
        if (!lead) relationKnown = false;
        renderMember(); return;
      }
      relationKnown = true; relationError = ''; waitsForOpen = false;
      if (JSON.stringify(value) === JSON.stringify(lead)) { renderMember(); return; }
      lead = value; renderMember();
    }
    function renderMember() {
      o.root.toggleAttribute('data-team-member', member());
      o.root.toggleAttribute('data-team-pending', !!id && !relationKnown);
      memberNote.hidden = !member() && relationKnown && !relationError; memberNote.replaceChildren();
      memberNote.tabIndex = -1;
      if (lead) {
        var room = window.dinahBus && (window.dinahBus.overview().rooms || []).find(function (r) { return r.name === lead.group; });
        memberNote.appendChild(node('span', null, 'Speak to this member in ' + (room ? room.title : lead.title + "'s team") + '. '));
        var a = node('a', null, 'Open Team Chat'); a.href = groupAddress(lead.group);
        a.addEventListener('click', function (e) {
          e.preventDefault(); groupLink(lead.group, 0, { chat: id, title: o.title() }).catch(host().fail);
        }); memberNote.appendChild(a);
      }
      if (relationError || !relationKnown) {
        memberNote.appendChild(node('span', null, (lead ? ' ' : '') + (relationError || 'Checking team membership…')));
        if (relationError) { var retry = node('button', 'btn btn-sm', 'Retry'); retry.type = 'button'; retry.addEventListener('click', function () { read(); }); memberNote.appendChild(retry); }
      }
      if (lead && memberNote.isConnected && o.root.querySelector('.composer-row').contains(document.activeElement)) memberNote.querySelector('a').focus();
      if (o.changed) o.changed(lead);
    }
    function drawMembers(into, members, group, includeRetired) {
      var wanted = new Set();
      (members || []).forEach(function (m) {
        var key = m.bus || m.chat; wanted.add(key);
        var held = rows.get(key);
        if (!held) {
          var li = node('li'); li.dataset.teamMember = key;
          var a = node('a', 'team-person'); a.href = '#';
          var title = node('span', 'team-name'), status = node('span', 'team-badge'), why = node('span', 'team-reason');
          var meta = node('span', 'team-meta'), usage = node('span', 'team-usage');
          a.append(title, status, why, meta, usage);
          var children = node('ul'), assignment = node('div', 'team-assignment');
          li.append(a, children, assignment);
          held = { li: li, a: a, title: title, status: status, why: why, meta: meta, usage: usage, children: children, assignment: assignment, member: m };
          a.addEventListener('click', function (e) {
            if (e.target.closest('.tree-toggle')) return;
            e.preventDefault();
            if (held.member.chat) Promise.resolve(o.openChat(held.member.chat)).catch(host().fail);
          });
          a.addEventListener('pudl:tree-toggle', function (e) { expanded.set(key, !!e.detail.open); });
          rows.set(key, held);
        }
        held.member = m;
        held.title.textContent = m.title;
        var ref = m.chat && window.dinahChats && window.dinahChats.ref(m.chat);
        if (ref) held.a.href = address({ kind: 'chat-session', workspace: ref.cwd, path: m.chat });
        held.status.replaceChildren(stateBadge(m));
        if (m.counts) held.status.appendChild(window.dinahChatState.badge(m.counts, { compact: true }));
        var liveState = window.dinahChatState.read(m);
        held.why.textContent = (liveState.exec && liveState.exec.why || '') + (!m.chat ? ' Open this chat in its owning window.' : '');
        held.meta.textContent = [m.workspace, window.dinahProviderName ? window.dinahProviderName(m.provider) : m.provider, m.model].filter(Boolean).join(' · ');
        held.usage.textContent = usageWords(m.usage);
        [held.why, held.meta, held.usage].forEach(function (line) { line.title = line.textContent; line.setAttribute('aria-label', line.textContent); });
        held.a.title = [m.title, window.dinahChatState.describe(m).label, held.why.textContent, held.meta.textContent, held.usage.textContent].filter(Boolean).join('. ');
        held.a.setAttribute('aria-label', held.a.title);
        held.a.setAttribute('aria-disabled', String(!m.chat));
        if (m.task) {
          if (!held.task) {
            held.task = node('a', 'team-task');
            held.task.addEventListener('click', function (e) { e.preventDefault(); var latest = held.member; groupLink(latest.taskGroup || current.group, latest.taskMessage).catch(host().fail); });
          }
          if (held.task.textContent !== m.task) held.task.textContent = m.task;
          held.task.href = groupAddress(m.taskGroup || group, m.taskMessage);
          if (!held.task.parentElement) held.assignment.appendChild(held.task);
        } else if (held.task) held.task.remove();
        if ((m.members || []).length) {
          if (held.children.parentElement !== held.li) held.a.after(held.children);
          held.a.setAttribute('aria-expanded', String(expanded.get(key) !== false));
          drawMembers(held.children, includeRetired ? m.members : m.members.filter(function (child) { return child.life !== 'retired'; }), m.group || group, includeRetired);
        } else {
          held.children.remove(); held.a.removeAttribute('aria-expanded');
        }
        // Existing rows are not replaced as usage and state change.
        if (held.li.parentElement !== into) into.appendChild(held.li);
      });
      Array.from(into.children).forEach(function (li) { if (!wanted.has(li.dataset.teamMember)) li.remove(); });
    }
    function draw(v, again) {
      if (disposed || !v || chatId(v.lead) !== id) return;
      if (!window.dinahStateOrder.accept('team:state', id, v, again)) return;
      current = v; publish(v);
      var members = v.members || [], activeMembers = members.filter(function (m) { return m.life !== 'retired'; }), retired = [];
      (function collect(list) { list.forEach(function (m) { if (m.life === 'retired') retired.push(m); else collect(m.members || []); }); })(members);
      o.keepPlace(function () {
        section.hidden = !v.group && !members.length;
        toggle.textContent = 'Team (' + members.length + ')';
        var aggregate = section.querySelector('.team-aggregate');
        if (!aggregate) { aggregate = node('span', 'team-aggregate'); toggle.after(aggregate); }
        aggregate.replaceChildren(window.dinahChatState.badge(v));
        drawMembers(tree, activeMembers, v.group); drawMembers(retiredTree, retired, v.group, true);
        retiredBox.hidden = !retired.length;
        budgetLines(budget, v.budget, prefix);
        if (window.pudlTree) { window.pudlTree.enhance(tree); window.pudlTree.enhance(retiredTree); }
      });
      refreshTraffic(); header();
    }
    function header() {
      var win = o.root.closest('.win'), statuses = win && win.querySelector('.win-statuses');
      if (!statuses) return;
      var chip = statuses.querySelector('.team-stuck-chip');
      if (!chip) { chip = node('span', 'win-status team-stuck-chip'); statuses.appendChild(chip); }
      chip.hidden = !current || !current.counts || (!current.group && !(current.members || []).length);
      chip.replaceChildren();
      if (current && current.counts) chip.appendChild(window.dinahChatState.badge(current));
    }
    function read(force) {
      if (!id || !api().ChatLead) { relationKnown = true; renderMember(); return Promise.resolve(); }
      var epoch = ++generation, relation = relationEpoch;
      var membership = Promise.resolve().then(function () { return api().ChatLead(id); }).then(function (fresh) {
        if (!disposed && epoch === generation && relation === relationEpoch) setLead(fresh, force);
      }).catch(function (error) {
        if (!disposed && epoch === generation) { relationError = 'Team membership could not be checked. '; renderMember(); host().fail(error); }
      });
      var team = api().Team ? Promise.all([api().Team(id), window.dinahBus ? window.dinahBus.load() : Promise.resolve()]).then(function (all) {
        if (!disposed && epoch === generation) draw(all[0], true);
      }).catch(function (error) { if (!disposed) host().fail(error); }) : Promise.resolve();
      return Promise.all([membership, team]);
    }
    function executionChanged() { if (current) draw(current, true); }
    document.addEventListener('dinah:execution-state', executionChanged);
    var stopTeam = host().on('team:state', function (v) {
      if (chatId(v.lead) === id) draw(v, false);
      // Membership changes can affect a nested lead's own parent.
      if (api().ChatLead && id) {
        var epoch = generation, relation = relationEpoch;
        Promise.resolve(api().ChatLead(id)).then(function (fresh) { if (!disposed && epoch === generation && relation === relationEpoch) setLead(fresh); }).catch(host().fail);
      }
    });
    var stopResync = host().onResync(read);
    return {
      update: function (s) {
        session = s;
        var next = s && s.id || '';
        if (next !== id) { id = next; infoLeadSignature = JSON.stringify(s && s.info.lead); relationEpoch++; lead = s && s.info.lead || null; relationKnown = !next || !!lead || !api().ChatLead; relationError = ''; waitsForOpen = false; current = null; renderMember(); read(); }
        else if (s && Object.prototype.hasOwnProperty.call(s.info, 'lead') && JSON.stringify(s.info.lead) !== infoLeadSignature) { infoLeadSignature = JSON.stringify(s.info.lead); relationEpoch++; if (s.info.lead || !api().ChatLead) setLead(s.info.lead); else read(); }
        if (waitsForOpen && s && !s.loading && !s.restoring) { waitsForOpen = false; read(); }
        header();
      },
      isMember: member,
      isPending: function () { return !!id && !relationKnown; },
      focus: function () { (memberNote.querySelector('a, button') || memberNote).focus(); },
      openTeam: function () { return lead ? groupLink(lead.group, 0, { chat: id, title: o.title() }) : Promise.resolve(); },
      commands: function () {
        if (!id || !api().SetTeamLead) return [];
        return [{ id: 'change-team-lead', label: member() ? 'Change Team Lead…' : 'Make a Team Member…', run: function () { return chooseLead(id, lead).then(read); } },
          { id: 'leave-team', label: 'Make an Ordinary Chat', hidden: !member(), run: function () { return Promise.resolve(api().ClearTeamLead(id)).then(function () { return read(true); }); } },
          { id: 'open-team', label: 'Open Team Chat', hidden: !member(), run: function () { return groupLink(lead.group); } }];
      },
      destroy: function () { trafficObserver.disconnect(); if (foldFrame) cancelAnimationFrame(foldFrame); disposed = true; generation++; document.removeEventListener('dinah:execution-state', executionChanged); if (stopTeam) stopTeam(); if (stopResync) stopResync(); section.remove(); memberNote.remove(); }
    };
  }
  function notice(target, title, text, tone) {
    target.className = 'notice ' + tone; target.hidden = false;
    var content = node('div', 'notice-content');
    content.append(node('p', 'notice-title', title), node('p', 'notice-body', text));
    target.replaceChildren(content);
  }
  function choiceDialog(title, label, choices, submit) {
    return new Promise(function (resolve) {
      var dlg = node('dialog', 'dialog team-dialog'), form = node('form'); form.noValidate = true;
      var heading = node('h2', 'dialog-title', title), body = node('div', 'dialog-body vstack gap-3');
      heading.id = 'team-dialog-title-' + (++serial); dlg.setAttribute('aria-labelledby', heading.id);
      var field = node('label', 'form-label', label), wrap = node('span', 'form-select-wrap'), select = node('select', 'form-select');
      field.htmlFor = 'team-choice-' + (++serial); select.id = field.htmlFor;
      choices.forEach(function (c) { select.appendChild(new Option(c.title, c.value)); });
      wrap.appendChild(select); body.append(field, wrap);
      var status = node('div'); status.hidden = true; body.appendChild(status);
      var actions = node('div', 'dialog-actions'), cancel = node('button', 'btn', 'Cancel'), go = node('button', 'btn btn-primary', title);
      cancel.type = 'button'; go.type = 'submit'; go.disabled = !choices.length;
      cancel.addEventListener('click', function () { dlg.close(); });
      form.addEventListener('submit', function (e) {
        e.preventDefault(); go.disabled = true;
        Promise.resolve().then(function () { return submit(select.value); }).then(function (result) {
          if (result && result.leftOut && result.leftOut.length) {
            notice(status, 'Some Chats Kept Their Lead', result.leftOut.map(function (m) { return m.title + ': ' + m.why; }).join(' '), 'info');
            go.remove(); cancel.textContent = 'Done';
          } else dlg.close();
        }).catch(function (error) { notice(status, 'The Team Could Not Be Changed', H.message(error, 'sentence'), 'danger'); }).finally(function () { go.disabled = false; });
      });
      dlg.addEventListener('close', function () { dlg.remove(); resolve(); });
      actions.append(cancel, go); form.append(heading, body, actions); dlg.appendChild(form); document.body.appendChild(dlg); dlg.showModal();
    });
  }
  function chooseLead(member, lead) {
    return window.dinahBus.load().then(function () {
      var chats = [], seen = new Set();
      (window.dinahBus.overview().workspaces || []).forEach(function (w) { (w.chats || []).forEach(function (c) {
        var id = c.chat;
        if (id && id !== member && !seen.has(id)) { seen.add(id); chats.push({ value: id, title: c.title + ' (' + (w.label || w.name) + ')' }); }
      }); });
      return choiceDialog('Choose Team Lead', 'Lead', chats, function (id) { return api().SetTeamLead(member, id); });
    });
  }
  function convertGroup(room) {
    var choices = (room.members || []).filter(function (m) { return m.state !== 'person' && m.chat; }).map(function (m) { return { value: m.chat, title: m.title }; });
    return choiceDialog('Make This Group a Team', 'Lead', choices, function (lead) { return api().MakeGroupATeam(room.name, lead); });
  }
  function editBudget(id, team) {
    // Budget rows carry each provider's unit; the page never converts them.
    return Promise.resolve(team && team.budget && team.budget.providers || []).then(function (providers) {
      return new Promise(function (resolve) {
        var dlg = node('dialog', 'dialog team-budget-dialog'), form = node('form');
        var body = node('div', 'dialog-body vstack gap-3'), inputs = [];
        body.appendChild(node('p', 'form-help', 'Weekly allowances warn the lead when spent. They do not stop work. Leave an allowance empty for none.'));
        (providers || []).forEach(function (p) {
          var row = (team && team.budget && team.budget.rows || []).find(function (r) { return r.provider === p.provider; });
          var label = node('label', 'form-label', p.name + ' (' + p.unit + ')');
          var input = node('input', 'form-input'); input.type = 'number'; input.min = '0'; input.step = '1'; input.placeholder = 'No allowance'; input.id = 'team-allowance-' + (++serial); label.htmlFor = input.id;
          if (row && row.hasAllowance === true) input.value = row.allowance;
          body.append(label, input);
          if ((team.budget.unmeasured || []).includes(p.provider)) body.appendChild(node('p', 'form-help', 'This provider does not report request usage. Its turns remain unmeasured.'));
          inputs.push({ provider: p.provider, input: input });
        });
        var error = node('div', 'notice danger'); error.hidden = true; body.appendChild(error);
        var actions = node('div', 'dialog-actions'), cancel = node('button', 'btn', 'Cancel'), save = node('button', 'btn btn-primary', 'Save'); cancel.type = 'button'; save.type = 'submit';
        cancel.addEventListener('click', function () { dlg.close(); });
        form.addEventListener('submit', function (e) {
          e.preventDefault(); if (!form.reportValidity()) return;
          var rows = inputs.filter(function (r) { return r.input.value !== ''; }).map(function (r) { return { provider: r.provider, allowance: Number(r.input.value) }; });
          save.disabled = true;
          Promise.resolve(api().SetTeamBudget(id, rows)).then(function () { dlg.close(); }).catch(function (err) { notice(error, 'The Allowances Could Not Be Saved', H.message(err, 'sentence'), 'danger'); }).finally(function () { save.disabled = false; });
        });
        var heading = node('h2', 'dialog-title', 'Team Allowances'); heading.id = 'team-dialog-title-' + (++serial); dlg.setAttribute('aria-labelledby', heading.id);
        dlg.addEventListener('close', function () { dlg.remove(); resolve(); }); actions.append(cancel, save); form.append(heading, body, actions); dlg.appendChild(form); document.body.appendChild(dlg); dlg.showModal();
      });
    });
  }
  window.dinahTeams = { mount: mount, convertGroup: convertGroup, groupLink: groupLink, aggregate: function (id) { var state = snapshots.get(id); return state && state.counts && (state.group || (state.members || []).length) ? state.counts : null; } };
})();
