/* The Org Chart for a reporting tree of any depth (docs/design/team-hierarchy.md).
   Every chat reports to one lead or to the person. A chat with reports leads a
   team and has a team room with its direct reports; a chat that reports to the
   person and has no reports is a free agent. Group chats are drawn as labels on
   the cards and impose no reporting line. The chats are invented and nothing is
   saved. */
(function () {
  'use strict';
  const spaces = { harbor: 'Harbor', orchard: 'Orchard' };
  const groups = { leads: 'Team Leads', review: 'Review Exchange' };
  const chats = [
    { id: 'harbor-lead', title: 'Harbor Lead', room: 'Harbor Team', space: 'harbor', program: 'Claude', state: 'idle', groups: ['leads'] },
    { id: 'g05-lead', title: 'Gap 05 Lead', room: 'Gap 05 Team', space: 'harbor', program: 'Codex', state: 'working', lead: 'harbor-lead' },
    { id: 'g05-author', title: 'Gap 05 Author', space: 'harbor', program: 'Claude', state: 'working', lead: 'g05-lead' },
    { id: 'g05-reviewer', title: 'Gap 05 Reviewer', space: 'harbor', program: 'Codex', state: 'idle', lead: 'g05-lead', groups: ['review'] },
    { id: 'g08-lead', title: 'Gap 08 Lead', room: 'Gap 08 Team', space: 'harbor', program: 'Claude', state: 'idle', lead: 'harbor-lead' },
    { id: 'g08-author', title: 'Gap 08 Author', space: 'harbor', program: 'Claude', state: 'closed', lead: 'g08-lead' },
    { id: 'g08-reviewer', title: 'Gap 08 Reviewer', space: 'harbor', program: 'Codex', state: 'closed', lead: 'g08-lead' },
    { id: 'notes', title: 'Research Notes', space: 'harbor', program: 'Claude', state: 'closed' },
    { id: 'g15-lead', title: 'Gap 15 Lead', room: 'Gap 15 Team', space: 'orchard', program: 'Codex', state: 'idle', groups: ['leads'] },
    { id: 'g15-author', title: 'Gap 15 Author', space: 'orchard', program: 'Claude', state: 'idle', lead: 'g15-lead' },
    { id: 'g15-reviewer', title: 'Gap 15 Reviewer', space: 'orchard', program: 'Claude', state: 'closed', lead: 'g15-lead', groups: ['review'] },
    { id: 'hosting', title: 'Site Hosting', space: 'orchard', program: 'Codex', state: 'closed' }
  ];
  const byId = new Map(chats.map(c => [c.id, c]));
  const logs = new Map(), rooms = new Map(), folded = new Set();
  const $ = id => document.getElementById(id);
  const canvas = $('canvas'), scroll = $('scroll'), statusEl = $('status'), menu = $('menu');
  let filter = { space: '', text: '' }, menuFor = null, windowFor = null;

  function node(tag, cls, text) { const el = document.createElement(tag); if (cls) el.className = cls; if (text !== undefined) el.textContent = text; return el; }
  const reports = id => chats.filter(c => c.lead === id);
  const leads = c => reports(c.id).length > 0;
  // A lead's team room is a group chat with a title of its own, as today.
  const roomName = c => c.room || c.title + ' Team';
  function descendants(id, out = new Set()) { reports(id).forEach(r => { out.add(r.id); descendants(r.id, out); }); return out; }
  function wanted(c) {
    if (filter.space && c.space !== filter.space) return false;
    return !filter.text || c.title.toLowerCase().includes(filter.text) || c.program.toLowerCase().includes(filter.text);
  }
  function branchWanted(c) { return wanted(c) || reports(c.id).some(branchWanted); }
  function say(text, tone) {
    statusEl.replaceChildren(node('span', '', text));
    if (tone) statusEl.dataset.tone = tone; else delete statusEl.dataset.tone;
  }

  // The team's head says how many a lead leads, so the card says only whom it reports to.
  function role(c) {
    if (c.lead) return 'Reports to ' + byId.get(c.lead).title;
    return leads(c) ? 'Reports to you' : 'Free agent, reports to you';
  }
  function badge(state) {
    const b = node('span', 'badge', { working: 'Working', idle: 'Idle', closed: 'Closed' }[state]);
    if (state === 'working') b.dataset.state = 'working';
    return b;
  }
  function card(c) {
    const box = node('article', 'org-chart-chat oc-chat');
    box.dataset.orgChat = c.id; box.tabIndex = 0;
    const title = node('a', 'org-chart-title oc-chat-title', c.title);
    title.href = '#'; title.addEventListener('click', e => { e.preventDefault(); openChat(c.id); });
    const stateRow = node('div', 'oc-chat-state'); stateRow.append(badge(c.state), node('span', 'oc-updated', 'Today'));
    const labels = node('div', 'oc-chat-groups');
    (c.groups || []).forEach(g => labels.append(node('span', 'oc-group-label', groups[g])));
    const more = node('button', 'btn btn-sm icon-btn org-chart-actions-button');
    more.type = 'button'; more.setAttribute('aria-label', 'Actions for ' + c.title);
    const glyph = node('span', 'glyph'); glyph.style.setProperty('--glyph', 'var(--glyph-more)'); glyph.setAttribute('aria-hidden', 'true');
    more.append(glyph); more.addEventListener('click', () => openMenu(c, more));
    box.append(title, node('span', 'oc-chat-role', role(c)), node('span', 'org-chart-meta oc-chat-role', spaces[c.space] + ' / ' + c.program), stateRow, labels, more);
    box.addEventListener('contextmenu', e => { e.preventDefault(); openMenu(c, more); });
    return box;
  }
  function team(c) {
    const section = node('section', 'oc-team'); section.dataset.orgTeam = c.id;
    const head = node('div', 'oc-team-head'), toggle = node('button', 'card-toggle');
    toggle.type = 'button'; toggle.setAttribute('aria-expanded', String(!folded.has(c.id)));
    toggle.setAttribute('aria-label', (folded.has(c.id) ? 'Expand ' : 'Collapse ') + roomName(c));
    toggle.addEventListener('click', () => { folded.has(c.id) ? folded.delete(c.id) : folded.add(c.id); render(); canvas.querySelector('[data-org-team="' + c.id + '"] .card-toggle').focus(); });
    const name = node('a', 'oc-team-name', roomName(c)); name.href = '#';
    name.addEventListener('click', e => { e.preventDefault(); openRoom(c.id); });
    const n = reports(c.id).length;
    head.append(toggle, name, node('span', 'oc-team-summary', n + (n === 1 ? ' member' : ' members') + (folded.has(c.id) ? ', folded' : '')));
    section.append(head, card(c));
    const list = node('ul', 'oc-members'); list.hidden = folded.has(c.id);
    if (!list.hidden) reports(c.id).filter(branchWanted).forEach(r => { const li = node('li'); li.append(leads(r) ? team(r) : card(r)); list.append(li); });
    section.append(list);
    return section;
  }
  function render() {
    canvas.replaceChildren();
    const personRow = node('div', 'oc-person-row'), person = node('div', 'oc-person');
    person.append(node('p', 'oc-person-name', 'You'), node('p', 'oc-person-role', 'In charge. Every chat at the top of a branch reports to you.'));
    personRow.append(person); canvas.append(personRow);
    const row = node('div', 'oc-spaces');
    Object.keys(spaces).forEach(space => {
      const tops = chats.filter(c => c.space === space && !c.lead);
      const trees = tops.filter(c => leads(c) && branchWanted(c)), free = tops.filter(c => !leads(c) && wanted(c));
      if (!trees.length && !free.length) return;
      const box = node('section', 'oc-space'); box.dataset.space = space;
      box.append(node('h3', 'oc-ws-name', spaces[space]));
      const holder = node('div', 'oc-space-trees'); trees.forEach(c => holder.append(team(c)));
      if (trees.length) box.append(holder);
      if (free.length) {
        const f = node('div', 'oc-free'), list = node('div', 'oc-ws-boxes');
        f.append(node('h4', '', 'Free agents')); free.forEach(c => list.append(card(c))); f.append(list); box.append(f);
      }
      row.append(box);
    });
    canvas.append(row);
    const count = chats.filter(wanted).length;
    $('count').textContent = count + (count === 1 ? ' chat' : ' chats') + (folded.size ? ', some in folded teams' : '');
    requestAnimationFrame(lines);
  }
  /* The lines from the person to the top of each branch. Within a team the
     shipped chart draws them with its own rules. */
  function lines() {
    canvas.querySelector('.oc-lines')?.remove();
    const origin = canvas.getBoundingClientRect(), person = canvas.querySelector('.oc-person');
    const tops = Array.from(canvas.querySelectorAll('.oc-space-trees > .oc-team'));
    if (!person || !tops.length) return;
    const p = person.getBoundingClientRect(), px = p.left - origin.left + p.width / 2, pb = p.bottom - origin.top;
    const rail = canvas.querySelector('.oc-spaces').getBoundingClientRect().top - origin.top - 20;
    const svg = document.createElementNS('http://www.w3.org/2000/svg', 'svg');
    svg.setAttribute('class', 'oc-lines'); svg.setAttribute('aria-hidden', 'true');
    svg.setAttribute('width', canvas.scrollWidth); svg.setAttribute('height', canvas.scrollHeight);
    // A branch in the first row is reached straight down from the rail. One in a
    // later row, where the workspace boxes stack on a narrow window, is reached
    // down the left edge, so the line never crosses the cards of another branch.
    const firstTop = Math.min(...tops.map(t => t.getBoundingClientRect().top));
    const gutter = Math.max(2, Math.min(...Array.from(canvas.querySelectorAll('.oc-space')).map(sp => sp.getBoundingClientRect().left)) - origin.left - 5);
    tops.forEach(box => {
      const b = box.getBoundingClientRect(), x = b.left - origin.left + b.width / 2, y = b.top - origin.top;
      const path = document.createElementNS('http://www.w3.org/2000/svg', 'path');
      path.setAttribute('d', b.top - firstTop < 4 ? `M${px} ${pb} V${rail} H${x} V${y}` : `M${px} ${pb} V${rail} H${gutter} V${y - 14} H${x} V${y}`);
      svg.append(path);
    });
    canvas.prepend(svg);
  }

  function openMenu(c, owner) {
    menuFor = c; menu.replaceChildren(node('h2', '', c.title));
    const item = (label, run) => { const b = node('button', '', label); b.type = 'button'; b.setAttribute('role', 'menuitem'); b.addEventListener('click', () => { closeMenu(); run(); }); menu.append(b); };
    item('Open Chat', () => openChat(c.id));
    if (leads(c)) item('Open Team Room', () => openRoom(c.id));
    menu.append(node('hr'));
    item('Reports To…', () => openReports(c));
    menu.hidden = false;
    const r = owner.getBoundingClientRect();
    menu.style.left = Math.max(8, Math.min(innerWidth - 238, r.right - 230)) + 'px';
    menu.style.top = Math.min(innerHeight - menu.offsetHeight - 8, r.bottom + 4) + 'px';
    menu.querySelector('button').focus();
  }
  function closeMenu() { menu.hidden = true; menuFor = null; }
  document.addEventListener('click', e => { if (!menu.hidden && !menu.contains(e.target) && !e.target.closest('.org-chart-actions-button')) closeMenu(); });
  document.addEventListener('keydown', e => { if (e.key === 'Escape' && !menu.hidden) closeMenu(); });

  let reportsFor = null;
  function openReports(c) {
    reportsFor = c;
    $('reports-title').textContent = 'Who Does ' + c.title + ' Report To?';
    const select = $('reports-to'), mine = descendants(c.id);
    select.replaceChildren();
    const you = node('option', '', 'You'); you.value = ''; select.append(you);
    chats.filter(o => o.id !== c.id && !mine.has(o.id)).forEach(o => { const opt = node('option', '', o.title + ' (' + spaces[o.space] + ')'); opt.value = o.id; select.append(opt); });
    select.value = c.lead || '';
    describeChange();
    $('reports').showModal(); select.focus();
  }
  function describeChange() {
    const c = reportsFor, to = $('reports-to').value, from = c.lead || '';
    const n = reports(c.id).length, keeps = n ? ' Its own ' + n + (n === 1 ? ' report goes' : ' reports go') + ' with it.' : '';
    let text;
    if (to === from) text = 'This is who it reports to now.';
    else {
      const leave = from ? 'It leaves ' + roomName(byId.get(from)) : 'It stops reporting to you';
      const join = to ? ' and joins ' + roomName(byId.get(to)) + '.' : (n ? ' and reports to you.' : ' and becomes a free agent that reports to you.');
      text = leave + join + keeps + ' Its group chats do not change.';
    }
    $('reports-change').textContent = text;
    $('reports-go').disabled = to === from;
  }
  $('reports-to').addEventListener('change', describeChange);
  $('reports').addEventListener('close', () => {
    if ($('reports').returnValue !== 'change' || !reportsFor) return;
    const c = reportsFor, to = $('reports-to').value || undefined;
    c.lead = to;
    if (to) c.space = byId.get(to).space;
    descendants(c.id).forEach(id => { byId.get(id).space = c.space; });
    render();
    say(c.title + ' now reports to ' + (to ? byId.get(to).title : 'you') + '.', 'positive');
  });

  /* The two stand-in windows: a chat, with its input box, and a team room. */
  function msg(cls, label, text, copy) {
    const m = node('div', 'msg' + (cls ? ' ' + cls : '') + (copy ? ' mock-copy' : ''));
    m.append(node('span', 'msg-label', label), node('p', '', text));
    return m;
  }
  function logOf(id) {
    if (!logs.has(id)) logs.set(id, [['', byId.get(id).program, 'Ready. This is an invented conversation.']]);
    return logs.get(id);
  }
  function roomOf(id) {
    if (!rooms.has(id)) rooms.set(id, [['', byId.get(id).title, 'Team, the work for this week is in the brief. Report here when your part is done.']]);
    return rooms.get(id);
  }
  function show(kind, id) {
    windowFor = { kind, id };
    const c = byId.get(id), w = $('window'); w.hidden = false; requestAnimationFrame(lines);
    const log = $('window-log'); log.replaceChildren();
    if (kind === 'chat') {
      $('window-title').textContent = c.title;
      $('window-sub').textContent = role(c) + ' · ' + spaces[c.space] + ' / ' + c.program;
      logOf(id).forEach(([cls, label, text, copy]) => log.append(msg(cls, label, text, copy)));
      $('composer').hidden = false;
      const lead = c.lead && byId.get(c.lead);
      $('copy-note').textContent = lead ? 'Copied to ' + roomName(lead) + ', with the answer.' : 'This chat reports to you, so nothing is copied.';
      $('copy-glyph').hidden = !lead;
    } else {
      $('window-title').textContent = roomName(c);
      $('window-sub').textContent = c.title + ' and its direct reports: ' + reports(id).map(r => r.title).join(', ') + '.';
      roomOf(id).forEach(([cls, label, text, copy]) => log.append(msg(cls, label, text, copy)));
      $('composer').hidden = true;
    }
    log.scrollTop = log.scrollHeight;
    $('composer-text').focus();
  }
  const openChat = id => show('chat', id);
  const openRoom = id => show('room', id);
  $('window-close').addEventListener('click', () => { $('window').hidden = true; windowFor = null; requestAnimationFrame(lines); });
  $('composer').addEventListener('submit', e => {
    e.preventDefault();
    const text = $('composer-text').value.trim();
    if (!text || !windowFor || windowFor.kind !== 'chat') return;
    const c = byId.get(windowFor.id), answer = 'Understood. I will do that and report back.';
    logOf(c.id).push(['msg-you', 'You', text], ['', c.program, answer]);
    if (c.lead) {
      const lead = byId.get(c.lead);
      roomOf(lead.id).push(['', 'Copy: you to ' + c.title, text, true], ['', 'Copy: ' + c.title + ' to you', answer, true]);
      say('Copied to ' + roomName(lead) + ': your message and the answer.', 'positive');
    } else say('Sent to ' + c.title + '.', 'positive');
    $('composer-text').value = '';
    show('chat', c.id);
  });

  $('scope').addEventListener('change', e => { filter.space = e.target.value; render(); });
  $('find').addEventListener('input', e => { filter.text = e.target.value.trim().toLowerCase(); render(); });
  $('theme').addEventListener('click', () => {
    const dark = document.documentElement.dataset.theme !== 'dark';
    document.documentElement.dataset.theme = dark ? 'dark' : 'light';
    $('theme').textContent = dark ? 'Light Theme' : 'Dark Theme';
    requestAnimationFrame(lines);
  });
  addEventListener('resize', () => requestAnimationFrame(lines));
  scroll.addEventListener('scroll', () => {});
  render();
  window.__orgChart = { chats, render, openChat, openRoom, openReports: id => openReports(byId.get(id)) };
  document.documentElement.dataset.mockReady = 'true';
}());
