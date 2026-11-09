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
  // A member's state is the standard chip, as wide as its own glyph and word.
  function stateBadge(m) { return window.dinahChatState.badge(m, { labelled: true, className: 'team-state' }); }
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
    var signature = JSON.stringify([budget, prefix]);
    if (target._budgetSignature === signature) return;
    target._budgetSignature = signature;
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
  /* The Team Report's tree of a team's members, nested teams under their
     leads. Each row opens its member's chat and, with detailed, says what
     it is doing, where and on what it runs, its usage, its counts and its
     last assignment. Rows are kept and updated in place as the team
     changes. */
  function memberTree(opts) {
    var rows = new Map(), expanded = new Map();
    // A tree's rows share one map at every depth, so none is dropped from
    // it when a branch is drawn (keep).
    function draw(into, members, group, includeRetired) {
      window.dinahHelpers.reconcile(into, members || [], { rows: rows, keep: true, key: function (m) { return m.bus || m.chat; }, create: function (m, key) {
          var li = node('li'); li.dataset.teamMember = key;
          var a = node('a', 'team-person'); a.href = '#';
          var title = node('span', 'team-name'), status = node('span', 'team-badge');
          a.append(title, status);
          var held = { node: li, li: li, a: a, title: title, status: status, children: node('ul'), member: m };
          if (opts.detailed) {
            held.why = node('span', 'team-reason'); held.meta = node('span', 'team-meta'); held.usage = node('span', 'team-usage');
            a.append(held.why, held.meta, held.usage);
            held.assignment = node('div', 'team-assignment');
          }
          li.append(a, held.children);
          if (held.assignment) li.appendChild(held.assignment);
          a.addEventListener('click', function (e) {
            if (e.target.closest('.tree-toggle')) return;
            e.preventDefault();
            if (held.member.chat) Promise.resolve(opts.openChat(held.member.chat)).catch(host().fail);
          });
          a.addEventListener('pudl:tree-toggle', function (e) { expanded.set(key, !!e.detail.open); });
          return held;
        }, update: function (held, m) {
        var key = held.li.dataset.teamMember;
        held.member = m;
        var ref = m.chat && window.dinahChats && window.dinahChats.ref(m.chat);
        var href = ref ? address({ kind: 'chat-session', workspace: ref.cwd, path: m.chat }) : '#';
        var signature = JSON.stringify([Object.assign({}, m, { members: undefined }), window.dinahChatState.read(m), window.dinahChatState.describe(m), href, group,
          window.dinahProviderName ? window.dinahProviderName(m.provider) : m.provider]);
        if (held.signature !== signature) {
          if (held.title.textContent !== m.title) held.title.textContent = m.title;
          if (held.a.href !== href) held.a.href = href;
          held.status.replaceChildren(stateBadge(m));
          var words = [m.title, window.dinahChatState.describe(m).label];
          if (opts.detailed) {
            if (m.counts) held.status.appendChild(window.dinahChatState.badge(m.counts, { compact: true }));
            var liveState = window.dinahChatState.read(m);
            held.why.textContent = (liveState.exec && liveState.exec.why || '') + (!m.chat ? ' Open this chat in its owning window.' : '');
            held.meta.textContent = [m.workspace, window.dinahProviderName ? window.dinahProviderName(m.provider) : m.provider, m.model].filter(Boolean).join(' · ');
            held.usage.textContent = usageWords(m.usage);
            [held.why, held.meta, held.usage].forEach(function (line) { line.title = line.textContent; line.setAttribute('aria-label', line.textContent); });
            words.push(held.why.textContent, held.meta.textContent, held.usage.textContent);
            if (m.task) {
              if (!held.task) {
                held.task = node('a', 'team-task');
                held.task.addEventListener('click', function (e) { e.preventDefault(); var latest = held.member; groupLink(latest.taskGroup || opts.group(), latest.taskMessage).catch(host().fail); });
              }
              if (held.task.textContent !== m.task) held.task.textContent = m.task;
              held.task.href = groupAddress(m.taskGroup || group, m.taskMessage);
              if (!held.task.parentElement) held.assignment.appendChild(held.task);
            } else if (held.task) held.task.remove();
          }
          held.a.title = words.filter(Boolean).join('. ');
          held.a.setAttribute('aria-label', held.a.title);
          held.a.setAttribute('aria-disabled', String(!m.chat));
          held.signature = signature;
          }
        if ((m.members || []).length) {
          if (held.children.parentElement !== held.li) held.a.after(held.children);
          var open = String(expanded.get(key) !== false);
          if (held.a.getAttribute('aria-expanded') !== open) held.a.setAttribute('aria-expanded', open);
          draw(held.children, includeRetired ? m.members : m.members.filter(function (child) { return child.life !== 'retired'; }), m.group || group, includeRetired);
        } else {
          held.children.remove(); held.a.removeAttribute('aria-expanded');
        }
        // Existing rows are not replaced as usage and state change.
      } });
    }
    return { draw: draw };
  }

  function mount(o) {
    var prefix = 'team-' + (++serial), id = '', current = null, lead = null, generation = 0, disposed = false, infoLeadSignature, relationEpoch = 0;
    var session = null, relationKnown = true, relationError = '', waitsForOpen = false;
    var section = node('section', 'chat-team');
    section.hidden = true;
    /* The lead's direct reports are the Members dropdown, the one a team's
       group chat has too (bus-applets.js): a row for each, its name and
       standard state chip, which opens its chat. Each member that leads a
       team of its own has its own Members. Rows are kept and updated in
       place, so a state change leaves an open dropdown and its focus. */
    var content = node('nav', 'menu-panel members-menu team-members'); content.setAttribute('popover', ''); content.setAttribute('aria-label', 'Members');
    content.id = prefix + '-members';
    var toggle = node('button', 'btn btn-sm menu-btn team-toggle'); toggle.type = 'button'; toggle.hidden = true;
    toggle.id = content.id + '-button'; toggle.setAttribute('popovertarget', content.id);
    var toggleWords = H.toolbarLabel(toggle, 'account', 'Members');
    var membersMenu = node('div', 'menu'); membersMenu.append(toggle, content);
    var openTeamButton = node('button', 'btn btn-sm team-open'); openTeamButton.type = 'button'; openTeamButton.hidden = true;
    H.toolbarLabel(openTeamButton, 'open', 'Open Team');
    /* Open Team opens a member's lead's team group, or the team group a
       lead that is no one's member leads. A chat that is both, a lead
       within another's team, has Open Own Team for the team it leads. Each
       opens the group's stable key through the same navigation as every
       group link, which keeps every chat's draft. */
    function ownGroup() { return (current && current.group) || ''; }
    function teamGroup() { return (lead && lead.group) || ownGroup(); }
    function navigateTeam() { var g = teamGroup(); return g ? groupLink(g) : Promise.resolve(); }
    function navigateOwnTeam() { var g = ownGroup(); return g ? groupLink(g) : Promise.resolve(); }
    var ownTeamButton = node('button', 'btn btn-sm team-open-own'); ownTeamButton.type = 'button'; ownTeamButton.hidden = true;
    H.toolbarLabel(ownTeamButton, 'open', 'Open Own Team');
    ownTeamButton.addEventListener('click', function () { navigateOwnTeam().catch(host().fail); });
    function syncOpen() {
      openTeamButton.hidden = !teamGroup();
      ownTeamButton.hidden = !(lead && lead.group && ownGroup() && ownGroup() !== lead.group);
    }
    openTeamButton.addEventListener('click', function () { navigateTeam().catch(host().fail); });
    // The bar is also the chat's top bar. A chat that keeps its Last Read
    // here (o.lastRead) keeps the bar mounted at one height, so the
    // transcript never moves when there comes to be something unread.
    var lastRead = o.lastRead || null;
    function showSection() { section.hidden = !lastRead && openTeamButton.hidden && ownTeamButton.hidden && toggle.hidden; }
    /* The bar's controls are a folding PUDL toolbar, as the group's is: Last
       Read first and longest, then Members, then the team's navigation. */
    var bar = node('div', 'toolbar chat-team-toolbar');
    bar.setAttribute('role', 'toolbar'); bar.setAttribute('aria-label', 'Chat'); bar.setAttribute('data-toolbar-fold', ''); bar.setAttribute('data-toolbar-captions', 'none');
    function toolbarGroup(name, glyph, priority) {
      var g = node('div', 'toolbar-group'); g.setAttribute('role', 'group'); g.setAttribute('aria-label', name);
      g.setAttribute('data-toolbar-group', name); g.setAttribute('data-toolbar-glyph', glyph); g.setAttribute('data-toolbar-priority', String(priority));
      return g;
    }
    var readingGroup = toolbarGroup('Reading', 'history', 30), membersGroup = toolbarGroup('Members', 'account', 20), teamGroupBar = toolbarGroup('Team', 'open', 10);
    if (lastRead) readingGroup.appendChild(lastRead);
    membersGroup.appendChild(membersMenu);
    teamGroupBar.append(openTeamButton, ownTeamButton);
    bar.append(readingGroup, membersGroup, teamGroupBar);
    section.appendChild(bar);
    o.root.querySelector('.chat-output').before(section);
    var memberList = node('div', 'member-list'); memberList.setAttribute('role', 'list'); memberList.setAttribute('aria-label', 'Members');
    var retiredLabel = node('p', 'md-section-label members-section', 'Retired');
    var retiredList = node('div', 'member-list'); retiredList.setAttribute('role', 'list'); retiredList.setAttribute('aria-label', 'Retired members');
    content.append(memberList, retiredLabel, retiredList);
    var leadRows = new Map();
    /* A row opens the chat of the member it was made for, read as it is when
       pressed; a state change only redraws its words and chip. */
    // The active and retired lists share leadRows, so a member moving
    // between them keeps its row (keep); drawMembersMenu drops the rest.
    function drawRows(into, members) {
      return window.dinahHelpers.reconcile(into, members, { rows: leadRows, keep: true, key: function (m) { return m.bus || m.chat; }, lost: function () { toggle.focus(); }, create: function (m, key) {
          var row = node('div', 'member-row lead-member'); row.setAttribute('role', 'listitem'); row.dataset.teamMember = key;
          var open = node('button', 'menu-action member-open'); open.type = 'button';
          var name = node('span', 'member-name'), slot = node('span', 'member-state-slot');
          // The chat glyph the taskbar draws for a chat window, before the name.
          var mark = node('span', 'glyph member-glyph'); mark.setAttribute('aria-hidden', 'true'); mark.style.setProperty('--glyph', 'var(--glyph-comments)');
          open.append(mark, name, slot); row.appendChild(open);
          var held = { node: row, row: row, open: open, name: name, slot: slot, member: m };
          held.open.addEventListener('click', function () {
            var latest = held.member;
            if (content.matches(':popover-open')) content.hidePopover();
            if (latest.chat) Promise.resolve(o.openChat(latest.chat)).catch(host().fail);
          });
          return held;
        }, update: function (held, m) {
        held.member = m;
        var signature = JSON.stringify([m.title, m.chat, m.life, window.dinahChatState.read(m), window.dinahChatState.describe(m)]);
        if (held.signature === signature) return;
        held.signature = signature;
        if (held.name.textContent !== m.title) held.name.textContent = m.title;
        var label = window.dinahChatState.describe(m).label;
        held.open.setAttribute('aria-label', 'Open ' + m.title + ', ' + label);
        held.open.title = m.chat ? 'Open ' + m.title + '. ' + label : 'Open this chat in its owning window. ' + label;
        held.open.setAttribute('aria-disabled', String(!m.chat));
        held.slot.replaceChildren(stateBadge(m));
        held.row.classList.toggle('member-ended', m.life === 'retired' || m.life === 'closed');
      } }).map(function (held) { return held.row; });
    }
    function drawMembersMenu(active, retired) {
      var shown = drawRows(memberList, active).concat(drawRows(retiredList, retired));
      retiredLabel.hidden = retiredList.hidden = !retired.length;
      leadRows.forEach(function (held, key) {
        if (shown.indexOf(held.row) >= 0) return;
        if (held.row.contains(document.activeElement)) toggle.focus();
        held.row.remove(); leadRows.delete(key);
      });
    }
    var memberNote = node('div', 'team-member-note form-help'); memberNote.hidden = true;
    o.root.querySelector('[data-view-id="composer"]').appendChild(memberNote);
    var activity = o.root.querySelector('[data-view-id="activity"]'), foldFrame = 0;
    function observeTraffic() { trafficObserver.observe(activity, { childList: true, subtree: true }); }
    function refreshTraffic() {
      foldFrame = 0;
      if ((!current || !current.group) && !activity.querySelector('.team-traffic-fold')) return;
      trafficObserver.disconnect();
      o.keepPlace(function () { foldTraffic(activity, current && current.group); });
      observeTraffic();
    }
    /* The fold groups the log's rows by the team message each came from,
       so only a change to the rows themselves (one added, removed or
       marked) can change it. Text streaming into a row, which is most of
       what changes in a working chat, is left alone; refolding at every
       chunk re-laid the transcript out for each one. */
    var trafficObserver = new MutationObserver(function (records) {
      function log(el) { return el && el.nodeType === 1 && el.classList && el.classList.contains('run-log'); }
      var rows = records.some(function (r) {
        var t = r.target;
        if (t === activity || log(t) || log(t.parentNode)) return true;
        // The whole log is swapped when a reconnect's replay takes the saved copy's place.
        return Array.prototype.some.call(r.addedNodes, log) || Array.prototype.some.call(r.removedNodes, log);
      });
      if (rows && !foldFrame) foldFrame = requestAnimationFrame(refreshTraffic);
    });
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
      syncOpen();
      toggle.hidden = !current || (!current.group && !(current.members || []).length);
      showSection();
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
    function draw(v, again) {
      if (disposed || !v || chatId(v.lead) !== id) return;
      if (!window.dinahStateOrder.accept('team:state', id, v, again)) return;
      current = v; publish(v);
      // The lead's direct reports; a nested team's members are in its own lead's Members.
      var members = v.members || [], activeMembers = members.filter(function (m) { return m.life !== 'retired'; }), retired = members.filter(function (m) { return m.life === 'retired'; });
      o.keepPlace(function () {
        toggle.hidden = !v.group && !members.length;
        syncOpen();
        showSection();
        if (toggleWords.textContent !== 'Members (' + activeMembers.length + ')') toggleWords.textContent = 'Members (' + activeMembers.length + ')';
        drawMembersMenu(activeMembers, retired);
      });
      refreshTraffic();
    }
    /* A lead's title bar shows its own chat's state alone; the team's state
       as a whole is its group window's (bus-applets.js). */
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
      },
      isMember: member,
      isPending: function () { return !!id && !relationKnown; },
      focus: function () { (memberNote.querySelector('a, button') || memberNote).focus(); },
      openTeam: function () { return lead ? groupLink(lead.group, 0, { chat: id, title: o.title() }) : Promise.resolve(); },
      commands: function () {
        if (!id) return [];
        var commands = [{ id: 'open-team', label: 'Open Team', hidden: !teamGroup(), run: navigateTeam },
          { id: 'open-own-team', label: 'Open Own Team', hidden: ownTeamButton.hidden, run: navigateOwnTeam }];
        if (api().SetTeamLead) commands.unshift(
          { id: 'change-team-lead', label: member() ? 'Change Team Lead…' : 'Make a Team Member…', run: function () { return chooseLead(id, lead).then(read); } },
          { id: 'leave-team', label: 'Make an Ordinary Chat', hidden: !member(), run: function () { return Promise.resolve(api().ClearTeamLead(id)).then(function () { return read(true); }); } });
        return commands;
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
  /* The Team Report, a popover a team group's window opens from its
     toolbar (bus-applets.js): every member in detail, as memberTree draws
     them, then the team's allowances and their use. leadChat() is the
     lead's chat, read as the report opens, which it then follows live
     until it closes. Each team group window has one; its button is the
     popover's invoker. */
  function reportPanel(leadChat, openChat) {
    var panel = node('div', 'popover team-report');
    panel.id = 'team-report-' + (++serial);
    panel.setAttribute('popover', ''); panel.setAttribute('role', 'dialog'); panel.setAttribute('aria-label', 'Team Report');
    var heading = node('h2', 'team-report-title', 'Team Report'), note = node('p', 'form-help', 'Reading the team…');
    var list = node('ul', 'tree team-tree'), retiredList = node('ul', 'tree team-tree'), retiredHeading = node('h3', 'team-report-section', 'Retired');
    list.setAttribute('aria-label', 'Team members'); retiredList.setAttribute('aria-label', 'Retired members');
    var allowances = node('h3', 'team-report-section', 'Allowances'), budget = node('div', 'team-budget');
    panel.append(heading, note, list, retiredHeading, retiredList, allowances, budget);
    var lead = '', state = null, stop = null, tree = memberTree({ detailed: true, openChat: openChat, group: function () { return state && state.group; } });
    function draw(v) {
      if (!v || chatId(v.lead) !== lead) return;
      state = v; if (!note.hidden) note.hidden = true;
      var members = v.members || [], retired = [];
      (function collect(l) { l.forEach(function (m) { if (m.life === 'retired') retired.push(m); else collect(m.members || []); }); })(members);
      tree.draw(list, members.filter(function (m) { return m.life !== 'retired'; }), v.group);
      tree.draw(retiredList, retired, v.group, true);
      if (retiredHeading.hidden !== !retired.length) retiredHeading.hidden = !retired.length;
      if (retiredList.hidden !== !retired.length) retiredList.hidden = !retired.length;
      budgetLines(budget, v.budget, panel.id);
      if (window.pudlTree) { window.pudlTree.enhance(list); window.pudlTree.enhance(retiredList); }
    }
    panel.addEventListener('beforetoggle', function (e) {
      if (e.newState !== 'open') { if (stop) stop(); stop = null; return; }
      lead = leadChat();
      note.textContent = 'Reading the team…'; note.hidden = false;
      if (!stop) stop = host().on('team:state', draw);
      if (lead && api().Team) Promise.resolve(api().Team(lead)).then(draw).catch(function (error) { note.textContent = H.message(error, 'sentence'); note.hidden = false; });
    });
    return panel;
  }
  /* The allowance editor for the team leadChat leads, with its state now. */
  function editAllowances(leadChat) {
    return Promise.resolve(api().Team(leadChat)).then(function (v) { return editBudget(leadChat, v); });
  }
  window.dinahTeams = { mount: mount, convertGroup: convertGroup, groupLink: groupLink, reportPanel: reportPanel, editAllowances: editAllowances, aggregate: function (id) { var state = snapshots.get(id); return state && state.counts && (state.group || (state.members || []).length) ? state.counts : null; } };
})();
