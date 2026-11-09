/* Settings › Connections (docs/design/source-kinds.md). A build that
   connects to services with an address and a token registers each one
   (internal/connections), and this section shows them: the address, whether
   a token is saved, and Save, Test and Forget. The token itself is never
   shown or read back. The tab stays hidden while the build has no
   connection, as Dinah's own build has none. */
(function () {
  'use strict';
  var SECTION = 'connections';
  var items = [], loading = null;

  function app() { return window.dinahHelpers.app(); }
  function $(id) { return document.getElementById(id); }
  function message(error) { return window.dinahHelpers.message(error); }
  function node(tag, className, text) {
    var el = document.createElement(tag);
    if (className) el.className = className;
    if (text !== undefined) el.textContent = text;
    return el;
  }
  function available() { var a = app(); return !!(a && a.Connections); }

  function load() {
    if (!available()) { items = []; draw(); return Promise.resolve(); }
    if (loading) return loading;
    loading = Promise.resolve().then(function () { return app().Connections(); }).then(function (list) {
      items = list || [];
    }).catch(function () { items = []; }).then(function () { loading = null; draw(); });
    return loading;
  }

  function status(el, text, failed) { el.textContent = text || ''; el.classList.toggle('danger', !!failed); }

  function card(item) {
    var section = node('section', 'card vstack gap-3'), titleId = 'connection-' + item.id + '-title';
    section.setAttribute('aria-labelledby', titleId); section.dataset.connection = item.id;
    section.appendChild(node('h3', 'card-title', item.label)).id = titleId;
    if (item.help) section.appendChild(node('p', 'form-help', item.help));
    if (item.problem) section.appendChild(node('p', 'form-help danger', 'The saved settings could not be read: ' + item.problem));
    var addressGroup = node('div', 'form-group'), addressLabel = node('label', 'form-label', 'Address'), address = node('input', 'form-input');
    address.type = 'url'; address.id = 'connection-' + item.id + '-address'; addressLabel.htmlFor = address.id; address.autocomplete = 'off'; address.spellcheck = false;
    address.value = item.address || ''; address.placeholder = item.defaultAddress || 'https://';
    addressGroup.append(addressLabel, address);
    var tokenGroup = node('div', 'form-group'), tokenLabel = node('label', 'form-label', 'Token'), token = node('input', 'form-input');
    token.type = 'password'; token.id = 'connection-' + item.id + '-token'; tokenLabel.htmlFor = token.id; token.autocomplete = 'off'; token.spellcheck = false;
    token.placeholder = item.hasToken ? 'A token is saved; paste one to replace it' : 'Paste a personal access token';
    tokenGroup.append(tokenLabel, token);
    tokenGroup.appendChild(node('p', 'form-help', item.hasToken ? 'Your token is kept in the system keychain and never shown.' : 'No token is saved yet.'));
    var actions = node('div', 'settings-actions'), line = node('p', 'settings-status'); line.setAttribute('role', 'status');
    var save = node('button', 'btn btn-sm btn-primary', 'Save'); save.type = 'button';
    actions.appendChild(save);
    var test = null;
    if (item.testable) { test = node('button', 'btn btn-sm', 'Test Connection'); test.type = 'button'; test.disabled = !(item.address && item.hasToken); actions.appendChild(test); }
    var forget = null;
    if (item.address || item.hasToken) { forget = node('button', 'btn btn-sm', 'Forget'); forget.type = 'button'; actions.appendChild(forget); }
    function busy(on) { [save, test, forget, address, token].forEach(function (el) { if (el) el.disabled = on; }); }
    save.addEventListener('click', function () {
      busy(true); status(line, 'Saving…');
      Promise.resolve().then(function () { return app().SetConnection(item.id, address.value, token.value); }).then(function () {
        token.value = ''; status(line, 'Saved.');
        return load();
      }).catch(function (error) { busy(false); status(line, message(error), true); });
    });
    if (test) test.addEventListener('click', function () {
      busy(true); status(line, 'Testing the connection…');
      Promise.resolve().then(function () { return app().TestConnection(item.id); }).then(function (result) {
        busy(false); status(line, result || 'The connection works.');
      }).catch(function (error) { busy(false); status(line, message(error), true); });
    });
    if (forget) forget.addEventListener('click', function () {
      window.dinahHost.ask('Forget This Connection?', 'The saved address and token for ' + item.label + ' are removed from this computer. Sources that use the connection stay attached and work again once you connect.', 'Forget', true).then(function (yes) {
        if (!yes) return;
        busy(true);
        return Promise.resolve().then(function () { return app().RemoveConnection(item.id); }).then(load);
      }).catch(function (error) { busy(false); status(line, message(error), true); });
    });
    section.append(addressGroup, tokenGroup, actions, line);
    return section;
  }

  function draw() {
    var host = $('connections-list'), tab = document.querySelector('[data-settings-section="' + CSS.escape(SECTION) + '"]');
    if (tab) tab.hidden = !items.length;
    if (!host) return;
    host.textContent = '';
    if (!items.length) { host.appendChild(node('p', 'form-help', 'This build connects to no service with a token.')); return; }
    items.forEach(function (item) { host.appendChild(card(item)); });
  }

  window.dinahSettingsSections.register(SECTION, { state: function () { return ''; }, shown: function () { load(); } });
  // The tab is shown or hidden as soon as the page knows its connections.
  if (document.readyState === 'loading') document.addEventListener('DOMContentLoaded', function () { load(); });
  else load();
  window.dinahConnectionsSettings = { reload: load };
})();
