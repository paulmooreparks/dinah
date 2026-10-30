/* Signed extensions contribute isolated views and declarative commands. */
(function () {
  'use strict';
  const app = window.dinahHelpers.app,
    bench = window.dinahWorkbench,
    frames = new Set(),
    launched = new Set();
  let catalog = [],
    generation = 0;
  const element = (tag, text, cls) => window.dinahHelpers.node(tag, cls, text);
  const button = (label, run, primary) => {
    const el = element('button', label, 'btn btn-sm' + (primary ? ' btn-primary' : ''));
    el.type = 'button';
    el.addEventListener('click', () =>
      Promise.resolve()
        .then(run)
        .catch(error => window.dinahHost.fail(error))
    );
    return el;
  };
  const uuid = () => crypto.randomUUID().replace(/-/g, '');
  function appearance() {
    const style = getComputedStyle(document.documentElement),
      tokens = {};
    const names = (window.dinahAppearance?.keys || [])
      .map(k => '--' + k)
      .concat(['--font', '--font-display', '--mono', '--font-mono', '--reading-font', '--chat-size', '--editor-size']);
    for (const key of names) tokens[key] = style.getPropertyValue(key).trim();
    return {
      version: 1,
      mode: document.documentElement.dataset.theme === 'dark' ? 'dark' : 'light',
      direction: document.documentElement.dir || 'ltr',
      identity: window.dinahAppearance?.snapshot().state.active || '',
      tokens
    };
  }
  /* Frames of one extension that show the same workspace share one map of JSON
     values, kept here while any frame of the extension lives and never persisted.
     A frame writes a key with shared-set; the extension's other frames in that
     workspace receive a 'shared' event, and a new frame gets the map with its
     init message. */
  const shared = new Map(),
    sharedKey = /^[a-z0-9][-a-z0-9_.]{0,63}$/,
    SHARED_VALUE_LIMIT = 64 * 1024,
    SHARED_KEYS_LIMIT = 64;
  function sharedMap(ext, ws, create) {
    let byWorkspace = shared.get(ext);
    if (!byWorkspace) {
      if (!create) return null;
      byWorkspace = new Map();
      shared.set(ext, byWorkspace);
    }
    let map = byWorkspace.get(ws);
    if (!map && create) {
      map = new Map();
      byWorkspace.set(ws, map);
    }
    return map || null;
  }
  function sharedSnapshot(ext, ws) {
    return Object.fromEntries(sharedMap(ext, ws, false) || []);
  }
  function frameOf(ext, viewID, ws) {
    for (const f of frames) if (f.extension() === ext && f.view() === viewID && f.workspace() === ws) return f;
    return null;
  }
  /* Opens a declared view. A view already open in the workspace is raised and
     given the state instead of being opened again, unless options.reuse is
     false. A pane is one per view; a view with follows takes the selected
     workspace as a pane does, so its window key has no workspace, and a second
     one (a pinned window, or one asked for on purpose) gets a key of its own. */
  function openView(manifest, view, nextState, options) {
    const reuse = !options || options.reuse !== false,
      ws = options?.workspace || window.dinahHost.workspaceId(),
      follows = !!(view.pane || view.follows);
    let existing = null;
    for (const f of frames)
      if (f.extension() === manifest.id && f.view() === view.id && (view.pane || (reuse && f.workspace() === ws))) {
        existing = f;
        break;
      }
    if (existing) return bench.open(existing.item, nextState);
    const item = { kind: 'extension-view', workspace: follows ? '' : ws, path: JSON.stringify([manifest.id, view.id]), title: view.title },
      key = bench.key(item);
    if ([...frames].some(f => f.key === key)) item.path = JSON.stringify([manifest.id, view.id, uuid()]);
    return bench.open(item, nextState).then(key => {
      if (view.pane) pudlWindows.dock(key, 'left');
      return key;
    });
  }
  /* The open-diff payload, checked as extension.DiffSpec.Validate in the SDK checks it. */
  const diffActionID = /^[-a-z0-9]{1,64}$/,
    diffGlyph = /^[a-z0-9-]{0,40}$/,
    diffLanguage = /^[a-z0-9+#.-]{0,40}$/,
    controlCharacter = /[\u0000-\u001f\u007f]/,
    DIFF_SIDE_LIMIT = 2 * 1024 * 1024;
  const plainText = (value, limit) => typeof value === 'string' && value.length <= limit && !controlCharacter.test(value);
  function diffSpec(payload) {
    if (!payload || typeof payload !== 'object') throw Error('A diff specification is required.');
    if (!plainText(payload.key, 200) || !payload.key.trim()) throw Error('A diff needs a key of at most 200 characters.');
    if (!plainText(payload.title, 200) || !payload.title.trim()) throw Error('A diff needs a title of at most 200 characters.');
    const detail = payload.detail ?? '',
      path = payload.path ?? '',
      language = payload.language ?? '',
      before = payload.before ?? '',
      after = payload.after ?? '',
      mode = payload.mode ?? '',
      actions = payload.actions ?? [];
    if (!plainText(detail, 500) || !plainText(path, 1024) || typeof language !== 'string' || !diffLanguage.test(language))
      throw Error('Invalid diff detail, path or language.');
    if (typeof before !== 'string' || typeof after !== 'string' || before.length > DIFF_SIDE_LIMIT || after.length > DIFF_SIDE_LIMIT)
      throw Error('A diff side exceeds 2 MiB.');
    if (mode !== '' && mode !== 'inline' && mode !== 'split') throw Error('Diff mode must be inline or split.');
    if (!Array.isArray(actions) || actions.length > 20) throw Error('A diff may declare at most 20 actions.');
    const seen = new Set();
    const list = actions.map(a => {
      if (!a || typeof a !== 'object' || typeof a.id !== 'string' || !diffActionID.test(a.id) || seen.has(a.id))
        throw Error('Invalid or repeated diff action.');
      seen.add(a.id);
      if (!plainText(a.label, 100) || !a.label.trim() || typeof (a.glyph ?? '') !== 'string' || !diffGlyph.test(a.glyph ?? ''))
        throw Error('A diff action needs a label of at most 100 characters.');
      if (!['file', 'hunk', 'line'].includes(a.scope)) throw Error('A diff action must apply to a file, hunk or line.');
      return { id: a.id, label: a.label, glyph: a.glyph || '', scope: a.scope, primary: !!a.primary, danger: !!a.danger };
    });
    return {
      key: payload.key,
      title: payload.title,
      detail,
      path,
      language,
      before,
      after,
      mode,
      readOnly: !!payload.readOnly,
      actions: list
    };
  }
  /* The file types an extension's views declare, as Settings › Extensions lists them, and a warning for
     those of their file extensions that open somewhere else: in a part of Dinah, which comes first, or in
     an extension enabled before this one. */
  const listOf = items => (items.length < 2 ? items.join('') : items.slice(0, -1).join(', ') + ' and ' + items.at(-1));
  function fileTypeLines(record) {
    const lines = [];
    for (const type of record.fileTypes || []) {
      lines.push(element('p', 'Opens ' + type.mediaType + ' (' + type.extensions.join(', ') + ') in ' + type.viewTitle + '.', 'form-help'));
      const by = new Map();
      for (const e of type.elsewhere || []) {
        const key = e.why + '\n' + e.by;
        if (!by.has(key)) by.set(key, { ...e, files: [] });
        by.get(key).files.push(e.fileExtension);
      }
      for (const e of by.values()) {
        const line = element('p', undefined, 'extension-conflict'),
          glyph = element('span', undefined, 'glyph'),
          files = listOf(e.files) + ' files';
        glyph.setAttribute('aria-hidden', 'true');
        glyph.style.setProperty('--glyph', 'var(--glyph-warning)');
        const text =
          e.why === 'type'
            ? files + ' do not open here, because Dinah knows them as ' + e.by + '.'
            : e.why === 'earlier'
              ? files + ' open in ' + e.by + ' instead, because it was enabled first.'
              : files + ' open in ' + e.by + ' instead, because Dinah opens them itself.';
        line.append(glyph, document.createTextNode(' ' + text));
        lines.push(line);
      }
    }
    return lines;
  }
  async function reload() {
    if (!app()?.Extensions) return [];
    const serial = ++generation,
      next = await app().Extensions();
    if (serial !== generation) return catalog;
    catalog = next;
    for (const id of launched) bench.unregisterLauncher(id);
    launched.clear();
    for (const record of catalog) {
      if (!record.enabled || record.error) continue;
      const m = record.manifest;
      for (const view of m.views) {
        const id = 'extension-' + m.id + '-' + view.id;
        bench.registerLauncher(id, {
          title: view.title,
          glyph: view.glyph || 'app',
          description: m.description,
          applet: m.id,
          run: () => openView(m, view)
        });
        launched.add(id);
      }
    }
    for (const frame of frames) frame.catalogChanged();
    document.dispatchEvent(new Event('dinah:extensions-change'));
    return catalog;
  }
  bench.register('extensions', {
    singleton: true,
    title: 'Extensions',
    glyph: 'app',
    launcher: { description: 'Install and manage signed native extensions.', run: () => bench.open('extensions') },
    init(root) {
      const toolbar = element('div', undefined, 'toolbar'),
        source = element('input'),
        status = element('p', '', 'form-help'),
        list = element('div', undefined, 'extension-library');
      source.className = 'form-input';
      source.placeholder = 'Signed folder, ZIP or HTTPS/SSH Git URL';
      source.setAttribute('aria-label', 'Extension source');
      status.setAttribute('role', 'status');
      let disposed = false,
        busy = false;
      async function job(fn) {
        if (busy) return;
        busy = true;
        status.textContent = 'Working…';
        try {
          await fn();
          status.textContent = '';
        } catch (error) {
          status.textContent = String(error.message || error);
        } finally {
          busy = false;
        }
      }
      async function install() {
        if (!source.value.trim()) return;
        const preview = await app().PreviewExtension(source.value.trim()),
          m = preview.manifest;
        const text =
          m.title +
          ' ' +
          m.version +
          ' by ' +
          m.publisher.name +
          '\nPublisher key SHA-256: ' +
          preview.fingerprint +
          '\nPermissions: ' +
          m.permissions.join(', ') +
          '\nNative code runs under your account. Host permissions do not sandbox the native process.';
        if (!(await dinahHost.ask('Install This Extension?', text, 'Trust and Install', true))) return;
        await app().InstallExtension(source.value.trim(), { fingerprint: preview.fingerprint, signature: m.signature });
        await reload();
        draw();
      }
      toolbar.append(
        source,
        button('Choose Folder…', () =>
          job(async () => {
            source.value = (await app().ChooseExtensionSource('folder')) || source.value;
          })
        ),
        button('Choose ZIP…', () =>
          job(async () => {
            source.value = (await app().ChooseExtensionSource('zip')) || source.value;
          })
        ),
        button('Install…', () => job(install), true),
        button('Refresh', () =>
          job(async () => {
            await reload();
            draw();
          })
        )
      );
      function draw() {
        if (disposed) return;
        list.replaceChildren();
        if (!catalog.length) list.append(element('p', 'No extensions installed. Choose a signed package to begin.', 'muted'));
        for (const record of catalog) {
          const m = record.manifest,
            card = element('section', undefined, 'card'),
            actions = element('div', undefined, 'hstack gap-2');
          card.append(
            element('h3', m.title + ' ' + m.version, 'card-title'),
            element('p', m.publisher.name + ' · ' + (record.enabled ? 'Enabled' : 'Disabled')),
            element('p', m.description),
            element('p', 'Permissions: ' + m.permissions.join(', '), 'form-help')
          );
          card.append(...fileTypeLines(record));
          if (record.error) card.append(element('p', record.error, 'extension-error'));
          if (record.enabled && !record.error)
            for (const view of m.views) actions.append(button('Open ' + view.title, () => openView(m, view)));
          actions.append(
            button(record.enabled ? 'Disable' : 'Enable', () =>
              job(async () => {
                await app().EnableExtension(m.id, !record.enabled);
                await reload();
                draw();
              })
            ),
            button('Update…', () =>
              job(async () => {
                source.value = record.origin;
                await install();
              })
            ),
            button('Remove…', () =>
              job(async () => {
                if (
                  await dinahHost.ask(
                    'Remove ' + m.title + '?',
                    'Remove this extension and all of its extension data? Workspace files remain.',
                    'Remove',
                    true
                  )
                ) {
                  await app().RemoveExtension(m.id);
                  await reload();
                  draw();
                }
              })
            )
          );
          const review = element('label', undefined, 'check'),
            check = element('input');
          check.type = 'checkbox';
          check.checked = !!record.reviewCommands;
          check.onchange = () =>
            job(async () => {
              await app().SetExtensionCommandReview(m.id, check.checked);
              await reload();
              draw();
            });
          review.append(check, document.createTextNode('Show Me First: review each external command'));
          card.append(review, actions);
          list.append(card);
        }
      }
      root.classList.add('extension-manager');
      root.append(toolbar, status, list);
      const changed = () => draw();
      document.addEventListener('dinah:extensions-change', changed);
      const ready = reload()
        .then(draw)
        .catch(error => {
          status.textContent = String(error.message || error);
        });
      return {
        ready,
        destroy() {
          disposed = true;
          document.removeEventListener('dinah:extensions-change', changed);
        }
      };
    }
  });
  bench.register('extension-view', {
    title: 'Extension',
    glyph: 'app',
    start: 'open',
    init(root, opts, item) {
      root.classList.add('extension-window');
      const status = element('p', 'Loading extension…', 'form-help'),
        iframe = element('iframe');
      iframe.className = 'extension-frame';
      iframe.setAttribute('sandbox', 'allow-scripts allow-forms');
      iframe.setAttribute('referrerpolicy', 'no-referrer');
      iframe.style.visibility = 'hidden';
      root.append(status);
      let disposed = false,
        port,
        loads = 0,
        metadata,
        view,
        state = opts.state || '',
        workspace = '',
        pinned = '',
        focusToken = '',
        lastID = 0,
        commands = [],
        loadTimer,
        workspaceSerial = 0;
      const running = new Map(),
        address = JSON.parse(item.path),
        key = root.closest('[data-win]').dataset.win;
      const send = (event, data) => {
        if (port && !disposed) port.postMessage({ version: 1, event, data });
      };
      function stop() {
        clearTimeout(loadTimer);
        port?.close();
        port = null;
        focusToken = '';
        for (const [id, ws] of running)
          app()
            .CancelExtensionCall(metadata?.id || '', ws, id)
            .catch(() => {});
        running.clear();
        commands = [];
      }
      /* A following window's pin is kept with the UI state under the window's key, so a restored window
         keeps the workspace it was pinned to. */
      const PINS = 'dinah-extension-pins';
      function savedPin() {
        try {
          return (window.dinahUIState?.get(PINS, {}) || {})[key] || '';
        } catch (error) {
          return '';
        }
      }
      function savePin(value) {
        try {
          const all = window.dinahUIState?.get(PINS, {}) || {};
          if (value) all[key] = value;
          else delete all[key];
          window.dinahUIState?.set(PINS, all);
        } catch (error) {
          /* A pin is a preference; the window still works without it. */
        }
      }
      function follows() {
        return !!(view?.pane || (view?.follows && !pinned));
      }
      /* The workspace a frame shows: the selected one for a pane or an unpinned following window, the
         pinned one, or the one the window opened with. */
      async function setWorkspace() {
        const serial = ++workspaceSerial;
        const ws = await app().ExtensionWorkspace(view?.pane ? '' : view?.follows ? pinned : item.workspace || '');
        if (disposed || serial !== workspaceSerial) return;
        const previous = workspace;
        workspace = ws.id;
        if (view?.follows) {
          ws.pinned = !!pinned;
          pudlWindows.retitle(key, view.title, ws.name || '');
        }
        send('workspace', ws);
        if (previous && previous !== ws.id)
          for (const [name, value] of sharedMap(address[0], ws.id, false) || []) send('shared', { key: name, value, from: '' });
        return ws;
      }
      function togglePin() {
        pinned = pinned ? '' : workspace;
        savePin(pinned);
        window.pudlCommands?.changed();
        document.dispatchEvent(new Event('dinah:commands-change'));
        return setWorkspace();
      }
      async function receive(message) {
        if (disposed || message?.version !== 1) return;
        if (message.method === 'ready') {
          clearTimeout(loadTimer);
          iframe.style.visibility = '';
          status.hidden = true;
          return;
        }
        if (message.method === 'focus') {
          if (document.activeElement !== iframe) return;
          focusToken = '';
          pudlWindows.raise(root.closest('[data-win]').dataset.win);
          return;
        }
        const id = message.id;
        if (!Number.isSafeInteger(id) || id <= lastID) return;
        lastID = id;
        let result;
        try {
          switch (message.method) {
            case 'call': {
              if (!metadata.actions.some(a => a.name === message.action)) throw Error('This extension action is not declared.');
              const input = JSON.stringify(message.input || {});
              if (input.length > 4 * 1024 * 1024) throw Error('Extension input exceeds limit.');
              const requestID = uuid(),
                ws = workspace;
              running.set(requestID, ws);
              try {
                result = await app().ExtensionCall(metadata.id, message.action, ws, input, requestID);
              } finally {
                running.delete(requestID);
              }
              break;
            }
            case 'commands': {
              if (!Array.isArray(message.commands) || message.commands.length > 60) throw Error('Invalid extension commands.');
              const seen = new Set();
              commands = message.commands.map(c => {
                if (
                  !c ||
                  typeof c.id !== 'string' ||
                  !/^[-a-z0-9]{1,64}$/.test(c.id) ||
                  c.id === 'pin-workspace' ||
                  seen.has(c.id) ||
                  typeof c.label !== 'string' ||
                  c.label.length > 100
                )
                  throw Error('Invalid extension command descriptor.');
                seen.add(c.id);
                return {
                  id: c.id,
                  label: c.label,
                  disabled: !!c.disabled,
                  glyph: /^[a-z0-9-]{1,40}$/.test(c.glyph || '') ? c.glyph : '',
                  shortcut: typeof c.shortcut === 'string' ? c.shortcut.slice(0, 40) : '',
                  danger: !!c.danger
                };
              });
              window.pudlCommands?.changed();
              document.dispatchEvent(new Event('dinah:commands-change'));
              result = { updated: true };
              break;
            }
            case 'navigate':
              if (typeof message.state !== 'string' || message.state.length > 8192) throw Error('Invalid extension navigation state.');
              state = message.state;
              opts.changed(state);
              result = { updated: true };
              break;
            case 'shared-set': {
              if (typeof message.key !== 'string' || !sharedKey.test(message.key)) throw Error('Invalid shared state key.');
              const value = message.value === undefined ? null : message.value,
                encoded = JSON.stringify(value);
              if (encoded === undefined || encoded.length > SHARED_VALUE_LIMIT) throw Error('Shared state value exceeds 64 KiB.');
              const map = sharedMap(metadata.id, workspace, true);
              if (value === null) map.delete(message.key);
              else {
                if (!map.has(message.key) && map.size >= SHARED_KEYS_LIMIT) throw Error('Shared state holds at most 64 keys.');
                map.set(message.key, JSON.parse(encoded));
              }
              for (const f of frames)
                if (f !== holder && f.extension() === metadata.id && f.live() && f.workspace() === workspace)
                  f.send('shared', { key: message.key, value: value === null ? null : JSON.parse(encoded), from: view.id });
              result = { updated: true };
              break;
            }
            case 'host-command':
              if (message.command === 'focus-menubar') {
                if (typeof message.focusToken !== 'string' || !message.focusToken || message.focusToken.length > 100)
                  throw Error('Invalid focus excursion.');
                focusToken = message.focusToken;
                result = { focused: !!window.pudlMenubar?.focus({ returnTo: iframe }) };
                if (!result.focused) focusToken = '';
              } else if (typeof message.command === 'string' && message.command.startsWith('open-view/')) {
                const target = metadata.views.find(v => v.id === message.command.slice(10));
                if (!target) throw Error('This extension view is not declared.');
                const payload = message.payload;
                result = {
                  window: await openView(metadata, target, state, {
                    reuse: !(payload && typeof payload === 'object' && payload.reuse === false),
                    workspace
                  })
                };
              } else if (message.command === 'open-diff') {
                const spec = diffSpec(message.payload);
                if (typeof window.dinahDiff?.open !== 'function') throw Error("The Editor's diff view is not available");
                const own = spec.key;
                await window.dinahDiff.open({
                  ...spec,
                  key: metadata.id + '/' + own,
                  onAction: (id, where) => {
                    if (!spec.actions.some(a => a.id === id)) return;
                    let place = null;
                    try {
                      place = where === undefined ? null : JSON.parse(JSON.stringify(where));
                    } catch (error) {
                      place = null;
                    }
                    send('command', { id: 'diff/' + id, diff: own, where: place });
                  }
                });
                result = { opened: true, diff: own };
              } else if (message.command === 'toggle-fullscreen') {
                result = { ran: !!(await window.dinahNativeWindow?.toggle()) };
              } else if (message.command === 'keyboard-shortcuts') {
                result = { ran: dinahCommands.run('keyboard-shortcuts') };
              } else throw Error('Host command is not available to this frame.');
              break;
            default:
              throw Error('Unsupported extension message.');
          }
          if (port && !disposed) port.postMessage({ version: 1, reply: id, result });
        } catch (error) {
          if (port && !disposed) port.postMessage({ version: 1, reply: id, error: String(error.message || error) });
        }
      }
      const returnFocus = event => {
        if (
          event.detail.target === iframe &&
          focusToken &&
          iframe.isConnected &&
          iframe.getBoundingClientRect().height > 0 &&
          document.activeElement === iframe
        ) {
          send('focus-return', { token: focusToken });
          focusToken = '';
        }
      };
      const prepareNativeFocus = event => {
        const destination = event.type === 'focusout' ? event.relatedTarget : event.target;
        if (!port || disposed || !destination?.closest?.('[data-native-chrome-slot]')) return;
        if (event.type === 'focusout' ? event.target !== iframe : document.activeElement !== iframe) return;
        focusToken = uuid();
        send('focus-prepare', { token: focusToken });
      };
      const appearanceChanged = () => send('appearance', appearance());
      const workspaceChanged = () => {
        if (follows())
          setWorkspace().catch(error => {
            status.textContent = String(error.message || error);
            status.hidden = false;
          });
      };
      /* What the host knows of this frame for its siblings: its address, the workspace it shows (or is
         about to show), and a way to send it events. */
      const holder = {
        key,
        item,
        extension: () => address[0],
        view: () => address[1],
        workspace: () => workspace || (follows() ? window.dinahHost.workspaceId() : item.workspace || ''),
        live: () => !!port && !disposed,
        send,
        catalogChanged() {
          if (
            metadata &&
            !catalog.some(
              record =>
                record.enabled && !record.error && record.manifest.id === metadata.id && record.manifest.version === metadata.version
            )
          ) {
            stop();
            iframe.remove();
            status.hidden = false;
            status.textContent = 'This extension was disabled, removed or updated. Close and reopen its window.';
          }
        }
      };
      frames.add(holder);
      const ready = (async () => {
        await window.dinahAppearance?.ready;
        if (!catalog.length) await reload();
        if (disposed) return;
        const record = catalog.find(record => record.enabled && !record.error && record.manifest.id === address[0]);
        if (!record) throw Error('This extension is unavailable. Open Extensions to enable or install it.');
        metadata = record.manifest;
        view = metadata.views.find(v => v.id === address[1]);
        if (!view) throw Error('This extension view is no longer declared.');
        iframe.title = view.title;
        const title = root.closest('.win').querySelector('.win-title');
        title.textContent = view.title;
        root.closest('.win').dataset.winGlyph = view.glyph || 'app';
        pinned = view.follows ? savedPin() : '';
        const ws = await setWorkspace();
        if (disposed) return;
        iframe.addEventListener('load', () => {
          if (disposed) return;
          if (++loads !== 1) {
            stop();
            status.hidden = false;
            status.textContent = 'Extension navigation closed its host connection. Close and reopen the window.';
            return;
          }
          const channel = new MessageChannel();
          port = channel.port1;
          port.onmessage = e => receive(e.data);
          port.start();
          loadTimer = setTimeout(() => {
            if (!disposed) {
              stop();
              status.hidden = false;
              status.textContent = 'The extension did not initialize its frame bridge.';
            }
          }, 10000);
          iframe.contentWindow.postMessage(
            {
              type: 'dinah-extension-init',
              version: 1,
              appearance: appearance(),
              workspace: ws,
              state,
              shared: sharedSnapshot(metadata.id, workspace)
            },
            '*',
            [channel.port2]
          );
        });
        iframe.src =
          '/extension-assets/' +
          encodeURIComponent(metadata.id) +
          '/' +
          encodeURIComponent(metadata.version) +
          '/' +
          view.page.split('/').map(encodeURIComponent).join('/');
        root.append(iframe);
      })().catch(error => {
        if (!disposed) status.textContent = String(error.message || error);
      });
      document.addEventListener('pudl:menubar-return-focus', returnFocus);
      document.addEventListener('pudl:native-return-focus', returnFocus);
      document.addEventListener('pointerdown', prepareNativeFocus, true);
      document.addEventListener('focusout', prepareNativeFocus, true);
      document.addEventListener('dinah:appearance-change', appearanceChanged);
      document.addEventListener('dinah:workspace-selected', workspaceChanged);
      return {
        ready,
        state: () => state,
        setState(next) {
          state = next || '';
          send('state', state);
        },
        commands() {
          const own = commands.map(c => ({ ...c, run: () => send('command', { id: c.id }) }));
          if (view?.follows) own.unshift({ id: 'pin-workspace', label: 'Pin to This Workspace', checked: !!pinned, run: togglePin });
          return own;
        },
        destroy() {
          disposed = true;
          stop();
          frames.delete(holder);
          if (![...frames].some(f => f.extension() === address[0])) shared.delete(address[0]);
          document.removeEventListener('pudl:menubar-return-focus', returnFocus);
          document.removeEventListener('pudl:native-return-focus', returnFocus);
          document.removeEventListener('pointerdown', prepareNativeFocus, true);
          document.removeEventListener('focusout', prepareNativeFocus, true);
          document.removeEventListener('dinah:appearance-change', appearanceChanged);
          document.removeEventListener('dinah:workspace-selected', workspaceChanged);
          iframe.remove();
        }
      };
    }
  });
  bench.register('extension-terminal', {
    title: 'Prepared Command',
    glyph: 'script',
    init(root, opts, item) {
      const prepared = JSON.parse(item.path),
        heading = element('p', 'Review the prepared command. It has not run.'),
        command = element('pre', JSON.stringify([prepared.command.program, ...prepared.command.args]), 'code'),
        folder = element('p', prepared.command.cwd, 'form-help'),
        screen = element('div');
      let terminal;
      const run = button(
        'Run in Terminal',
        () => {
          run.disabled = true;
          terminal = window.dinahShellTerminal.init(screen, opts, {
            ...item,
            shell: 'command',
            preparedId: prepared.ticket,
            instanceId: prepared.ticket
          });
          return terminal.ready;
        },
        true
      );
      root.append(heading, folder, command, run, screen);
      return {
        destroy() {
          terminal?.destroy();
        }
      };
    }
  });
  if (window.runtime) {
    window.dinahHost.on('extensions:error', message => dinahHost.fail(Error(message)));
    window.dinahHost.on('extensions:changed', () => reload().catch(dinahHost.fail));
    window.dinahHost.onResync(() => reload());
    window.dinahHost.on('extension:terminal', prepared =>
      bench.open({ kind: 'extension-terminal', path: JSON.stringify(prepared) }).catch(dinahHost.fail)
    );
  }
  /* Opens a file in the extension view that Go's resolver chose for it (document_links.go): the view
     opens in the file's workspace with the file in its state, and a window of the view already open
     there is raised and given the state. */
  async function openFile(link) {
    if (!link || typeof link.state !== 'string' || !link.resource?.workspaceId) throw Error('That file cannot open in an extension.');
    let record = catalog.find(r => r.enabled && !r.error && r.manifest.id === link.extension);
    if (!record) {
      await reload();
      record = catalog.find(r => r.enabled && !r.error && r.manifest.id === link.extension);
    }
    const view = record?.manifest.views.find(v => v.id === link.extensionView && !v.pane && !v.follows);
    if (!view) throw Error('The extension that opens this file is unavailable. Open Extensions to enable or install it.');
    return openView(record.manifest, view, link.state, { workspace: link.resource.workspaceId });
  }
  window.dinahExtensions = { reload, openFile, catalog: () => catalog.slice() };
  const reviews = new Map();
  if (window.runtime) {
    // The keys of reviews already shown, kept after they end, so one that a reread
    // returns after the page saw it end is not shown again.
    const reviewed = new Set();
    const reviewAsked = request => {
      if (reviewed.has(request.key)) return;
      reviewed.add(request.key);
      if (reviewed.size > 1000) reviewed.delete(reviewed.values().next().value);
      const dialog = element('dialog'),
        form = element('form'),
        title = element('h2', 'Review ' + request.input.program + ' command'),
        help = element('p', request.input.explanation),
        folder = element('p', request.input.cwd, 'form-help'),
        label = element('label', 'Arguments (JSON array)'),
        args = element('textarea'),
        error = element('p', '', 'extension-error'),
        actions = element('div', undefined, 'toolbar');
      args.className = 'form-input';
      args.rows = 10;
      args.value = JSON.stringify(request.input.args, null, 2);
      label.append(args);
      const respond = async decision => {
        try {
          await app().AnswerExtensionCommand(request.key, decision, args.value);
        } catch (e) {
          error.textContent = String(e.message || e);
        }
      };
      actions.append(
        button('Cancel', () => respond('cancel')),
        button('Prepare in Terminal', () => respond('terminal')),
        button('Run', () => respond('run'), true)
      );
      form.append(title, help, folder, label);
      if (request.input.stdin) form.append(element('pre', request.input.stdin, 'code'));
      form.append(error, actions);
      form.onsubmit = e => e.preventDefault();
      dialog.oncancel = e => {
        e.preventDefault();
        respond('cancel');
      };
      dialog.append(form);
      document.body.append(dialog);
      reviews.set(request.key, dialog);
      dialog.showModal();
    };
    const reviewEnded = key => {
      reviewed.add(key);
      const dialog = reviews.get(key);
      dialog?.close();
      dialog?.remove();
      reviews.delete(key);
    };
    window.dinahHost.on('extension:review', reviewAsked);
    window.dinahHost.on('extension:review-ended', reviewEnded);
    // After a resync the engine's pending reviews are read again: a new one is shown, and a
    // shown one that is no longer pending ended unseen.
    window.dinahHost.onResync(() => {
      if (!app().PendingExtensionReviews) return null;
      return Promise.resolve(app().PendingExtensionReviews()).then(list => {
        list = list || [];
        const pending = new Set(list.map(request => request.key));
        Array.from(reviews.keys()).forEach(key => {
          if (!pending.has(key)) reviewEnded(key);
        });
        list.forEach(reviewAsked);
      });
    });
  }
  reload().catch(error => {
    console.warn('Extension catalog:', String(error.message || error));
  });
})();
