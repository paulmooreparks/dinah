/* The chart adds relationships to the Chats panel's current inventory. */
(function () {
  'use strict';
  function project(relations, rows) {
    var aliases = new Map(), source = new Map((relations.chats || []).map(function (chat) { return [chat.chatId, chat]; }));
    (relations.chats || []).forEach(function (chat) { aliases.set(chat.chatId, chat.head || chat.chatId); });
    var chats = rows.map(function (row) {
      return { chatId: row.sessionId, head: row.sessionId, title: row.title, workspace: row.cwd || '', workspaceName: row.workspaceName || '', provider: row.provider || '', providerName: row.providerName || '', state: row.state, resolution: 'resolved', reason: '' };
    });
    var present = new Set(chats.map(function (chat) { return chat.chatId; }));
    function reference(id) {
      if (!id) return '';
      var resolved = present.has(id) ? id : aliases.get(id) || id;
      if (!present.has(resolved)) {
        var prior = source.get(id);
        chats.push({ chatId: resolved, head: '', title: prior && prior.title || 'Chat not in Chats', workspace: prior && prior.workspace || '', workspaceName: prior && prior.workspaceName || '', provider: prior && prior.provider || '', providerName: prior && prior.providerName || '', state: prior && prior.state || {}, resolution: 'repair', reason: 'This chat is referenced by a team or group but is not in Chats.' });
        present.add(resolved);
      }
      return resolved;
    }
    var teams = (relations.teams || []).map(function (team) { return Object.assign({}, team, { leadId: reference(team.leadId), memberIds: team.memberIds.map(reference) }); });
    var groups = (relations.groups || []).map(function (group) { return Object.assign({}, group, { ownerId: reference(group.ownerId), memberIds: group.memberIds.map(reference) }); });
    return Object.assign({}, relations, { chats: chats, teams: teams, groups: groups });
  }
  window.dinahChatInventory = { chart: project };
}());
