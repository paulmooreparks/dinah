/* The chart adds team and group relationships to the shared Chats inventory. */
(function() {
  "use strict";
  var node = window.dinahHelpers.node, app = window.dinahHelpers.app;
  function init(root, opts) {
    var disposed = false, snapshot = null, relations = null, pending = null, again = false, geometry = "", elements = new Map(), folded = new Set();
    var workspace = "", find = "", readingState = false, failed = false, lineFrame = 0;
    root.classList.add("org-chart");
    var toolbar = node("div", "org-chart-toolbar");
    toolbar.setAttribute("role", "toolbar");
    toolbar.setAttribute("aria-label", "Org Chart");
    var scope = node("select", "form-select");
    scope.setAttribute("aria-label", "Workspace");
    var search = node("input", "form-input");
    search.type = "search";
    search.placeholder = "Find a Chat";
    search.setAttribute("aria-label", "Find a Chat");
    var refresh = node("button", "btn btn-sm", "Retry Loading");
    refresh.type = "button";
    refresh.disabled = true;
    refresh.hidden = true;
    var manage = node("button", "btn btn-sm", "Manage Chats…");
    manage.type = "button";
    var legend = node("div", "org-chart-legend text-muted");
    var lineLegend = node("span"), line = node("span", "org-chart-legend-line");
    lineLegend.append(line, document.createTextNode("Team"));
    var groupLegend = node("span"), groupLabel = node("span", "oc-group-label", "Group");
    groupLegend.append(groupLabel);
    legend.append(lineLegend, groupLegend);
    toolbar.append(scope, search, manage, refresh, legend);
    var scroll = node("div", "org-chart-scroll");
    scroll.tabIndex = 0;
    scroll.setAttribute("aria-label", "Org Chart");
    var canvas = node("div", "org-chart-canvas oc-canvas");
    scroll.append(canvas);
    var bar = node("div", "org-chart-statusbar"), status = node("div", "org-chart-statusbar-message org-chart-status", "Loading the chart."), count = node("span", "org-chart-statusbar-count");
    status.setAttribute("role", "status");
    status.setAttribute("aria-live", "polite");
    bar.append(status, count);
    root.append(toolbar, scroll, bar);
    var actionRoot = node("dialog", "dialog org-dialog org-action");
    root.append(actionRoot);
    var management = window.dinahOrgChartManagement.create(actionRoot, {
      changed: function(p) {
        opts.changed(p.toString());
      },
      refresh: load,
      state: address,
      openChat,
      success: function(result, title, kind) {
        status.replaceChildren(node("span", "glyph"));
        status.firstChild.style.setProperty("--glyph", "var(--glyph-check)");
        status.append(node("span", "", kind === "delete_chat" ? "Moved " + title + " to the Recycle Bin." : kind === "start_chat" ? "Started " + title + "." : title + ". " + (result.message || "The chart was updated.")));
        status.dataset.tone = "positive";
        status.title = status.textContent;
        if (kind === "delete_chat") {
          var link2 = node("a", "", "Open Recycle Bin");
          link2.href = viewHref({ kind: "chat-cleanup" }, "");
          link2.addEventListener("click", function(event) {
            event.preventDefault();
            window.dinahChatCleanup.open();
          });
          status.append(link2);
        }
      },
      returnFocus: function(id, control) {
        var box = elements.get(id);
        if (box) (control === "actions" ? box.box.querySelector(".org-chart-actions-button") || box.box : box.box).focus({ preventScroll: true });
        else scroll.focus({ preventScroll: true });
      },
      navigate: function(p, push) {
        var key = root.closest("[data-win]").dataset.win, url = new URL(location.href);
        url.searchParams.set("view." + key, p.toString());
        if (push) history.pushState(history.state, "", url);
        window.dinahWorkbench.setState(key, p.toString());
      }
    });
    function viewHref(item, state) {
      var url = new URL(location.href), key = window.dinahWorkbench.key(item), opened = new Set((url.searchParams.get("open") || "").split(",").filter(Boolean));
      opened.add(key);
      url.searchParams.set("open", Array.from(opened).join(","));
      if (state) url.searchParams.set("view." + key, state);
      return url.href;
    }
    function icon(name) {
      var el = node("span", "glyph");
      el.style.setProperty("--glyph", "var(--glyph-" + name + ")");
      el.setAttribute("aria-hidden", "true");
      return el;
    }
    function link(text, cls, item, open2) {
      var a = node("a", cls, text);
      a.href = viewHref(item);
      a.addEventListener("click", function(event) {
        if (event.button || event.ctrlKey || event.metaKey || event.shiftKey || event.altKey) return;
        event.preventDefault();
        Promise.resolve().then(open2).catch(window.dinahHost.fail);
      });
      return a;
    }
    function openChat(id) {
      var chat = snapshot.chats.find(function(c) {
        return c.chatId === id;
      });
      return chat && chat.head && window.dinahChats.open(chat.head);
    }
    function groupLink(group, cls) {
      if (group.resolution && group.resolution !== "resolved") {
        var text = node("span", cls, group.title);
        text.title = group.reason || "This group needs attention.";
        return text;
      }
      var id = group.id.replace(/^team:/, "");
      return link(group.title, cls, { kind: "bus-room", path: id }, function() {
        return window.dinahBus.openGroup(id);
      });
    }
    function showMenu(id, owner, detail) {
      management.menu(id, owner, detail || { add: function(rows) {
        var r = owner.getBoundingClientRect();
        window.pudlMenu.openAt(r.left, r.bottom, rows, { owner, for: owner });
      }, mark: function() {
      } });
    }
    manage.addEventListener("click", function() {
      showMenu("", manage);
    });
    function address() {
      var p = new URLSearchParams();
      if (workspace) p.set("workspace", workspace);
      if (find) p.set("q", find);
      Array.from(folded).sort().forEach(function(id) {
        p.append("fold", id);
      });
      management.write(p);
      return p.toString();
    }
    function readState(value) {
      var p = new URLSearchParams(value || "");
      workspace = p.get("workspace") || "";
      find = p.get("q") || "";
      folded = new Set(p.getAll("fold"));
      search.value = find;
      scope.value = workspace;
      management.read(p);
    }
    function changed() {
      if (!readingState) opts.changed(address());
      render();
    }
    function wanted(chat) {
      var normalize = function(value) {
        return value.toLowerCase().replace(/[\\/]+$/, "");
      };
      return (!workspace || !chat.workspace || normalize(chat.workspace) === normalize(workspace)) && (!find.trim() || chat.title.toLowerCase().includes(find.trim().toLowerCase()));
    }
    function actionButton(box, id, title) {
      var button = node("button", "btn btn-sm icon-btn org-chart-actions-button");
      button.type = "button";
      button.setAttribute("aria-label", "Actions for " + title);
      button.append(icon("more"));
      button.addEventListener("click", function() {
        showMenu(id, button);
      });
      box.append(button);
    }
    function chatBox(chat) {
      if (elements.has(chat.chatId)) return null;
      var box = node("article", "org-chart-chat oc-chat");
      box.dataset.orgChat = chat.chatId;
      box.tabIndex = 0;
      var title = node("span", "org-chart-title oc-chat-title");
      if (chat.resolution === "resolved" && chat.head) title = link("", "org-chart-title oc-chat-title", { kind: "chat-session", workspace: chat.workspace, path: chat.head }, function() {
        return openChat(chat.chatId);
      });
      var role = node("span", "oc-chat-role"), meta = node("span", "org-chart-meta oc-chat-role"), stateRow = node("div", "oc-chat-state"), state = node("span", "org-chart-state"), groups = node("div", "oc-chat-groups");
      stateRow.append(state);
      box.append(title, role, meta, stateRow, groups);
      elements.set(chat.chatId, { box, title, role, meta, state, groups });
      if (chat.resolution === "resolved") actionButton(box, chat.chatId, chat.title);
      box.addEventListener("pudl:contextmenu", function(event) {
        event.stopPropagation();
        if (chat.resolution !== "resolved") {
          event.preventDefault();
          return;
        }
        showMenu(chat.chatId, box, event.detail);
      });
      return box;
    }
    // A card says whom its chat reports to; how many it leads is on its team's head.
    function roleOf(chat) {
      var team = snapshot.teams.find(function(t) { return t.memberIds.includes(chat.chatId); });
      if (team) {
        var lead = snapshot.chats.find(function(c) { return c.chatId === team.leadId; });
        return "Reports to " + (lead && lead.title || "its lead");
      }
      return snapshot.teams.some(function(t) { return t.leadId === chat.chatId; }) ? "Reports to you" : "Free agent, reports to you";
    }
    function updateText() {
      snapshot.chats.forEach(function(chat) {
        var el = elements.get(chat.chatId);
        if (!el) return;
        el.title.textContent = chat.title || "Untitled Chat";
        el.title.title = el.title.textContent;
        el.role.textContent = roleOf(chat);
        el.role.title = el.role.textContent;
        el.meta.textContent = [chat.workspaceName || chat.workspace, chat.providerName || chat.provider || "Unknown program"].filter(Boolean).join(" / ");
        el.meta.title = el.meta.textContent;
        el.box.setAttribute("aria-label", el.title.textContent + ". " + el.role.textContent + ". " + el.meta.textContent + (chat.resolution !== "resolved" ? ". Needs attention. " + chat.reason : ""));
        el.state.replaceChildren(chat.resolution === "resolved" ? window.dinahChatState.badge(chat.state) : node("span", "org-chart-repair", chat.reason.includes("not in Chats") ? "⚠ Not in Chats" : "⚠ Needs attention"));
        el.state.title = chat.reason || "";
        var priorTime = el.state.parentElement.querySelector("time");
        if (priorTime) priorTime.remove();
        if (chat.updatedAt) {
          var updated = node("time", "oc-updated", window.dinahHelpers.fmtDate(chat.updatedAt));
          updated.dateTime = chat.updatedAt;
          updated.title = chat.updatedAt;
          el.state.parentElement.append(updated);
        }
        var button = el.box.querySelector(".org-chart-actions-button");
        if (button) button.setAttribute("aria-label", "Actions for " + el.title.textContent);
        if (!el.groups.childNodes.length) ordinaryGroups().filter(function(g) {
          return g.resolution === "resolved" && (g.ownerId === chat.chatId || g.memberIds.includes(chat.chatId));
        }).forEach(function(g) {
          var a = groupLink(g, "oc-group-label");
          a.dataset.orgGroupLabel = g.id;
          el.groups.append(a);
        });
      });
      canvas.querySelectorAll("[data-chat-link]").forEach(function(a) {
        var chat = snapshot.chats.find(function(c) {
          return c.chatId === a.dataset.chatLink;
        });
        if (chat) a.textContent = chat.title;
      });
      var inventoryIds = new Set(window.dinahChats.inventory().map(function(c) {
        return c.chatId || c.sessionId;
      })), total = snapshot.chats.filter(function(c) {
        return inventoryIds.has(c.chatId) && wanted(c);
      }).length;
      count.textContent = total + (total === 1 ? " chat" : " chats") + (folded.size ? ", some in folded teams" : "");
      canvas.dataset.generation = snapshot.generation;
    }
    function ordinaryGroups() {
      var channels = new Set(snapshot.teams.map(function(team) { return team.id.replace(/^team:/, ""); }));
      return snapshot.groups.filter(function(group) { return !channels.has(group.id); });
    }
    function rebuild() {
      var top = scroll.scrollTop, left = scroll.scrollLeft, focused = document.activeElement.closest("[data-org-chat]");
      var focusId = focused && focused.dataset.orgChat;
      var focusControl = document.activeElement.matches(".org-chart-actions-button") ? ".org-chart-actions-button" : document.activeElement.matches(".oc-chat-title") ? ".oc-chat-title" : "";
      elements.clear();
      canvas.replaceChildren();
      var personRow = node("div", "oc-person-row"), person = node("div", "oc-person");
      person.append(node("p", "oc-person-name", "You"), node("p", "oc-person-role", "In charge. Every chat at the top of a branch reports to you."));
      personRow.append(person);
      canvas.append(personRow);
      /* Each workspace is a box holding its branches, each drawn from the chat at
         its top, and its free agents (docs/design/team-hierarchy.md). A branch is
         in the box of its top chat's workspace. */
      var spacesRow = node("div", "oc-spaces"), spaces = new Map();
      function spaceOf(chat) {
        var key = chat && chat.workspace || "";
        if (!spaces.has(key)) spaces.set(key, { name: chat && chat.workspaceName || key || "Workspace not recorded", trees: [], free: [] });
        return spaces.get(key);
      }
      var renderedTeams = new Set(), parentTeams = new Map(), byLead = new Map(snapshot.teams.map(function(team) { return [team.leadId, team]; }));
      snapshot.teams.forEach(function(team) {
        team.memberIds.forEach(function(id) { if (byLead.has(id) && !parentTeams.has(id)) parentTeams.set(id, team); });
      });
      function hiddenByAncestor(team) {
        var visited = new Set();
        while (parentTeams.has(team.leadId) && !visited.has(team.id)) {
          visited.add(team.id); team = parentTeams.get(team.leadId);
          if (folded.has(team.id)) return true;
        }
        return false;
      }
      function hasWanted(team, visited) {
        if (visited.has(team.id)) return false;
        var next = new Set(visited); next.add(team.id);
        return [team.leadId].concat(team.memberIds).some(function(id) {
          var chat = snapshot.chats.find(function(chat) { return chat.chatId === id; });
          var child = id !== team.leadId && byLead.get(id);
          return chat && wanted(chat) || child && hasWanted(child, next);
        });
      }
      function teamSection(team) {
        if (renderedTeams.has(team.id) || !hasWanted(team, new Set())) return null;
        renderedTeams.add(team.id);
        var chats = [team.leadId].concat(team.memberIds).map(function(id) {
          return snapshot.chats.find(function(c) {
            return c.chatId === id;
          });
        }).filter(function(c) {
          return c && wanted(c);
        });
        var section = node("section", "oc-team");
        section.dataset.orgTeam = team.id;
        var head = node("div", "oc-team-head"), toggle = node("button", "card-toggle");
        toggle.type = "button";
        toggle.dataset.orgFold = team.id;
        toggle.setAttribute("aria-label", (folded.has(team.id) ? "Expand " : "Collapse ") + team.title);
        toggle.setAttribute("aria-expanded", String(!folded.has(team.id)));
        var membersId = "org-members-" + root.closest("[data-win]").dataset.win + "-" + team.id;
        toggle.setAttribute("aria-controls", membersId);
        head.append(toggle, groupLink(team, "oc-team-name"), node("span", "oc-team-summary", team.memberIds.length + (team.memberIds.length === 1 ? " member" : " members")));
        section.append(head);
        toggle.addEventListener("click", function() {
          if (folded.has(team.id)) folded.delete(team.id);
          else folded.add(team.id);
          changed();
          var next = canvas.querySelector('[data-org-fold="' + CSS.escape(team.id) + '"]');
          if (next) next.focus({ preventScroll: true });
        });
        var lead = chats.find(function(c) {
          return c.chatId === team.leadId;
        });
        if (lead) {
          var leadBox = chatBox(lead);
          if (leadBox) section.append(leadBox);
        }
        if (team.resolution !== "resolved" && team.reason) section.append(node("p", "org-chart-repair", team.reason));
        var members = node("ul", "oc-members");
        members.id = membersId;
        members.hidden = folded.has(team.id);
        /* Folded members stay out of the rendered chart, but remain in its count. */
        if (!members.hidden) team.memberIds.forEach(function(id) {
          var child = byLead.get(id), c = snapshot.chats.find(function(chat) { return chat.chatId === id; });
          var box = child ? teamSection(child) : c && wanted(c) ? chatBox(c) : null;
          if (box) {
            var li = node("li");
            li.append(box);
            members.append(li);
          }
        });
        section.append(members);
        return section;
      }
      function leadChat(team) { return snapshot.chats.find(function(c) { return c.chatId === team.leadId; }); }
      snapshot.teams.filter(function(team) { return !parentTeams.has(team.leadId); }).forEach(function(team) {
        var section = teamSection(team); if (section) spaceOf(leadChat(team)).trees.push(section);
      });
      snapshot.teams.forEach(function(team) {
        if (hiddenByAncestor(team)) return;
        if (!renderedTeams.has(team.id)) { var section = teamSection(team); if (section) spaceOf(leadChat(team)).trees.push(section); }
      });
      var teamIds = new Set(snapshot.teams.flatMap(function(t) {
        return [t.leadId].concat(t.memberIds);
      }));
      var free = snapshot.chats.filter(function(c) {
        return wanted(c) && !teamIds.has(c.chatId);
      });
      free.forEach(function(c) { spaceOf(c).free.push(c); });
      spaces.forEach(function(space) {
        var box = node("section", "oc-space");
        box.append(node("h3", "oc-ws-name", space.name));
        if (space.trees.length) {
          var trees = node("div", "oc-space-trees");
          space.trees.forEach(function(section) { trees.append(section); });
          box.append(trees);
        }
        if (space.free.length) {
          var freeBox = node("div", "oc-free"), boxes = node("div", "oc-ws-boxes");
          freeBox.append(node("h4", "oc-free-title", "Free agents"));
          space.free.forEach(function(c) { var cb = chatBox(c); if (cb) boxes.append(cb); });
          freeBox.append(boxes);
          box.append(freeBox);
        }
        spacesRow.append(box);
      });
      if (spacesRow.childNodes.length) canvas.append(spacesRow);
      var groups = ordinaryGroups().filter(function(g) {
        return g.memberIds.concat(g.ownerId || []).some(function(id) {
          return snapshot.chats.some(function(c) {
            return c.chatId === id && wanted(c);
          });
        });
      });
      if (groups.length) {
        canvas.append(node("h2", "oc-section", "Groups"), node("p", "oc-section-note", "Groups are shared chats with no reporting line. Each lists its members."));
        var list = node("div", "oc-wrap");
        groups.forEach(function(g) {
          var section = node("section", "org-chart-group oc-group");
          section.dataset.orgGroup = g.id;
          section.tabIndex = 0;
          section.addEventListener("pudl:contextmenu", function(event) {
            event.stopPropagation();
            event.detail.add([{ label: "Open Group Chat", disabled: g.resolution !== "resolved", reason: g.reason, run: function() {
              return window.dinahBus.openGroup(g.id);
            } }]);
            event.detail.mark(section);
          });
          section.append(groupLink(g, "oc-group-name"));
          var ids = Array.from(new Set(g.memberIds.concat(g.ownerId || []))), members = node("ul", "oc-group-members");
          ids.forEach(function(id) {
            var c = snapshot.chats.find(function(c2) {
              return c2.chatId === id;
            });
            if (!c) return;
            var li = node("li"), a = node("span", "", c.title);
            if (c.head && c.resolution === "resolved") a = link(c.title, "", { kind: "chat-session", workspace: c.workspace, path: c.head }, function() {
              return openChat(id);
            });
            a.dataset.chatLink = id;
            li.append(a);
            members.append(li);
          });
          section.append(node("p", "oc-group-count", ids.length + " members"), members);
          if (g.resolution !== "resolved") section.append(node("p", "org-chart-repair", g.reason || "This group needs attention."));
          var more = node("button", "btn btn-sm icon-btn org-chart-actions-button");
          more.type = "button";
          more.setAttribute("aria-label", "Actions for " + g.title);
          more.append(icon("more"));
          more.addEventListener("click", function() {
            var r = more.getBoundingClientRect();
            window.pudlMenu.openAt(r.left, r.bottom, [{ label: "Open Group Chat", disabled: g.resolution !== "resolved", reason: g.reason, run: function() {
              return window.dinahBus.openGroup(g.id);
            } }], { owner: more, for: section });
          });
          section.append(more);
          list.append(section);
        });
        canvas.append(list);
      }
      if (!elements.size) canvas.append(node("p", "oc-section-note org-chart-empty", "No chats match these filters."));
      updateText();
      drawLines();
      scroll.scrollTop = top;
      scroll.scrollLeft = left;
      if (focusId && elements.has(focusId)) {
        var restored = elements.get(focusId).box;
        (focusControl && restored.querySelector(focusControl) || restored).focus({ preventScroll: true });
      }
    }
    function drawLines() {
      if (disposed || !snapshot || !canvas.clientWidth) return;
      var previous = canvas.querySelector(".oc-lines");
      if (previous) previous.remove();
      var origin = canvas.getBoundingClientRect(), person = canvas.querySelector(".oc-person"), columns = Array.from(canvas.querySelectorAll(".oc-space-trees > .oc-team")), segments = [];
      /* A line runs from the person to the top of each branch. A branch in the
         first row is reached straight down from the rail; one in a later row,
         where the workspace boxes wrap, down the left edge, so that no line
         crosses the cards of another branch. */
      if (person && columns.length) {
        var p = person.getBoundingClientRect(), px = p.left - origin.left + p.width / 2, pb = p.bottom - origin.top;
        var rail = canvas.querySelector(".oc-spaces").getBoundingClientRect().top - origin.top - 20;
        var firstTop = Math.min.apply(null, columns.map(function(col) { return col.getBoundingClientRect().top; }));
        var gutter = Math.max(2, Math.min.apply(null, Array.from(canvas.querySelectorAll(".oc-space")).map(function(sp) { return sp.getBoundingClientRect().left; })) - origin.left - 5);
        columns.forEach(function(col) {
          var c = col.getBoundingClientRect(), x = c.left - origin.left + c.width / 2, y = c.top - origin.top;
          segments.push(c.top - firstTop < 4 ? [[px, pb], [px, rail], [x, rail], [x, y]] : [[px, pb], [px, rail], [gutter, rail], [gutter, y - 14], [x, y - 14], [x, y]]);
        });
      }
      /* The lines are plain paths over the workspace boxes, so they reach the
         top of each branch inside its box. They take no pointer events. */
      var ns = "http://www.w3.org/2000/svg", svg = document.createElementNS(ns, "svg");
      svg.setAttribute("width", String(canvas.scrollWidth));
      svg.setAttribute("height", String(canvas.scrollHeight));
      segments.forEach(function(points) {
        var path = document.createElementNS(ns, "path");
        path.setAttribute("d", points.map(function(pt, j) { return (j ? "L" : "M") + Math.round(pt[0]) + " " + Math.round(pt[1]); }).join(" "));
        svg.append(path);
      });
      svg.classList.add("oc-lines", "org-chart-lines");
      svg.setAttribute("aria-hidden", "true");
      var old = canvas.querySelector(".oc-lines");
      if (old) old.replaceWith(svg);
      else canvas.prepend(svg);
    }
    function render() {
      if (disposed || !snapshot || !scroll.clientWidth) return;
      var next = JSON.stringify([workspace, find, Array.from(folded), snapshot.teams, snapshot.groups, snapshot.chats.map(function(c) {
        return [c.chatId, c.workspace, c.resolution, wanted(c)];
      })]);
      if (next !== geometry) {
        geometry = next;
        rebuild();
      } else updateText();
    }
    function workspaceOptions() {
      var values = new Map(snapshot.chats.map(function(c) {
        return [c.workspace, c.workspaceName || c.workspace];
      }));
      values.delete("");
      if (workspace && !values.has(workspace)) values.set(workspace, workspace);
      var choices = [["", "All Workspaces"]].concat(Array.from(values).sort(function(a, b) {
        return a[1].localeCompare(b[1]);
      }));
      if (JSON.stringify(choices) !== scope.dataset.choices) {
        scope.replaceChildren();
        choices.forEach(function(pair) {
          var o = node("option", "", pair[1]);
          o.value = pair[0];
          scope.append(o);
        });
        scope.dataset.choices = JSON.stringify(choices);
      }
      scope.value = workspace;
    }
    function syncInventory() {
      if (!relations || disposed) return;
      snapshot = window.dinahChatInventory.chart(relations, window.dinahChats.inventory());
      workspaceOptions();
      render();
    }
    function load(refreshInventory) {
      if (disposed) return Promise.resolve();
      if (pending) {
        again = true;
        return pending;
      }
      refresh.disabled = true;
      pending = Promise.resolve().then(function() {
        if (refreshInventory === true) return window.dinahChats.refreshInventory();
      }).then(function() {
        if (!app() || !app().OrgChart) throw new Error("The chart is not available in this build.");
        return app().OrgChart();
      }).then(function(view) {
        if (disposed) return;
        if (!view || typeof view.generation !== "string" || !Array.isArray(view.chats) || !Array.isArray(view.teams) || !Array.isArray(view.groups)) throw new Error("The chart could not be read.");
        relations = view;
        failed = false;
        refresh.hidden = true;
        if (!status.dataset.tone) status.textContent = "";
        syncInventory();
      }).catch(function(error) {
        if (!disposed) {
          failed = true;
          refresh.disabled = false;
          refresh.hidden = false;
          status.textContent = window.dinahHelpers.message(error, "sentence");
          delete status.dataset.tone;
        }
      }).finally(function() {
        pending = null;
        if (again && !disposed) {
          again = false;
          return load();
        }
      });
      return pending;
    }
    scope.addEventListener("change", function() {
      workspace = scope.value;
      changed();
    });
    search.addEventListener("input", function() {
      find = search.value;
      changed();
    });
    refresh.addEventListener("click", function() {
      load(true);
    });
    document.addEventListener("dinah:chats", syncInventory);
    document.addEventListener("dinah:execution-state", syncInventory);
    var stops = ["team:state", "bus:changed", "chat:removed", "chat:restored", "workspace:changed"].map(function(event) {
      return window.dinahHost.on(event, load);
    });
    stops.push(window.dinahHost.onResync(load));
    var resize = new ResizeObserver(function() {
      cancelAnimationFrame(lineFrame);
      lineFrame = requestAnimationFrame(function() {
        render();
        drawLines();
      });
    });
    resize.observe(scroll);
    var theme = new MutationObserver(drawLines);
    theme.observe(document.documentElement, { attributes: true, attributeFilter: ["data-theme", "data-skin", "style"] });
    readState(opts.state);
    load(true);
    return { state: address, setState: function(value) {
      readingState = true;
      readState(value);
      render();
      readingState = false;
    }, commands: function() {
      return [{ label: "Manage Chats…", run: function() {
        manage.click();
      } }, { label: "Retry Loading", disabled: !failed, reason: "The chart updates automatically.", run: load }];
    }, destroy: function() {
      disposed = true;
      management.destroy();
      stops.forEach(function(stop) {
        stop();
      });
      document.removeEventListener("dinah:chats", syncInventory);
      document.removeEventListener("dinah:execution-state", syncInventory);
      resize.disconnect();
      theme.disconnect();
      cancelAnimationFrame(lineFrame);
    } };
  }
  function open() {
    return window.dinahWorkbench.open("org-chart");
  }
  window.dinahWorkbench.register("org-chart", { applet: { id: "dinah.org-chart", title: "Org Chart", glyph: "subagents", workspace: false }, launcher: { run: open }, singleton: true, title: "Org Chart", glyph: "subagents", mode: "maximized", init });
  window.dinahCommands.register({ id: "org-chart", menu: "view", label: "Org Chart", glyph: "subagents", run: open });
})();
