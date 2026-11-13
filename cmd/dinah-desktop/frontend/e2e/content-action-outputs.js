// Real packaged applets and Dinah routing, with in-memory drafts and a
// headless browser. No native bridge, filesystem writes or OS input.
const { launch } = require('./lib/page');
const fs = require('fs');
const path = require('path');
(async () => {
  const t = await launch({ goto: false, timeout: 60000, state: {
    workspace: 'C:/Fixture', workspaceId: 'w', recent: ['C:/Fixture'],
    workspaceRecords: [{ id: 'w', name: 'Fixture', executionPath: 'C:/Fixture', primarySourceId: 's', sources: [{ id: 's', kind: 'local-folder', path: 'C:/Fixture' }] }]
  }, app: () => {
    let next = 0;
    const drafts = {}, files = {}, copy = v => JSON.parse(JSON.stringify(v));
    const fresh = (workspaceId, title = 'Untitled', text = '', origin) => {
      const id = (++next).toString(16).padStart(32, '0');
      return drafts[id] = { id, title, text, revision: '1', origin: origin || { kind: 'new', workspaceId },
        actions: ['update', 'save-as', 'discard'], suggestions: [], spans: {}, changed: false, documentType: '' };
    };
    window.__answers = {};
    return {
      NewEmptyDraft: async ws => copy(fresh(ws)),
      GetDraft: async id => { if (!drafts[id]) throw Error('Unknown fixture draft'); return copy(drafts[id]); },
      RetitleDraft: async (id, rev, title) => { drafts[id].title = title; return copy(drafts[id]); },
      UpdateDraft: async (id, rev, text) => { const d = drafts[id]; d.text = text; d.revision = String(+d.revision + 1); return copy(d); },
      SaveDraftAs: async (id, rev, ref) => { if (files[ref.path]) throw Error('Existing fixture file'); const d = drafts[id];
        d.origin = Object.assign({ kind: 'local-folder' }, ref); d.title = ref.path.split('/').pop(); files[ref.path] = d; return copy(d); },
      OpenDraft: async ref => copy(files[ref.path] || (files[ref.path] = fresh(ref.workspaceId, ref.path.split('/').pop(),
        ref.path.endsWith('.canvas') ? '{"nodes":[],"edges":[]}' : '# Fixture\n\nText\n', Object.assign({ kind: 'local-folder' }, ref)))),
      CompareDraft: async () => ({ moved: false }), SaveDraft: async id => copy(drafts[id]),
      AnswerPage: async (id, result, error) => { window.__answers[id] = { result, error }; },
      ShortcodeCatalog: async () => [], DraftFiles: async () => [], DocumentComments: async () => [],
      ReadWorkspaceMountText: async () => ({ found: false, text: '' }),
      ResolveWorkspaceResourceMount: async ref => ({ path: '/workspaces/Fixture/' + ref.path })
    };
  }});
  const records = [];
  const warnings = [];
  t.page.on('console', msg => { if (msg.type() === 'warning' || msg.type() === 'error') warnings.push(msg.text()); });
  try {
    await t.context.addInitScript(() => {
      window.__outputRejections = [];
      window.addEventListener('unhandledrejection', event => {
        const value = event.reason;
        /* Closing a Diagrams window ends its language session, and the package's bridge rejects the
           initialization still pending with { code: -32800, message: 'Language session ended.' }, which
           nothing in the package handles (reported to yavc.app Development, 14 November 2026). That one
           rejection is recorded and taken here, so this suite says whether the host's answers match
           their schemas; any other error still fails it. Remove this when the package handles it. */
        if (value && value.code === -32800) event.preventDefault();
        window.__outputRejections.push({ text: String(value), message: value && value.message, stack: value && value.stack,
          value: (() => { try { return JSON.stringify(value); } catch (_) { return ''; } })() });
      });
    });
    // Slides uses secure-context APIs. Fulfil HTTPS locally without a server
    // or certificate exception, using the same assets as the shared launcher.
    await t.context.route('https://dinah.test/**', route => {
      const name = new URL(route.request().url()).pathname;
      const override = process.env.DINAH_CONTENT_HOST_ROOT && /^\/(content-host|content-windows)\.js$/.test(name);
      const file = path.join(override ? process.env.DINAH_CONTENT_HOST_ROOT : path.join(__dirname, '..'), name);
      return fs.existsSync(file) && fs.statSync(file).isFile() ? route.fulfill({ path: file }) : route.fulfill({ status: 404, body: 'Not found' });
    });
    await t.page.goto('https://dinah.test/index.html');
    await t.waitReady();
    await t.page.evaluate(async () => {
      await dinahContentHost.install();
      window.__outputValidator = (await import('/pudl/vendor/cfworker-json-schema-4.1.1/index.js')).Validator;
    });
    const legacy = await t.page.evaluate(() => {
      const v = { id: 'legacy-draft', title: 'Old.canvas', origin: { kind: 'new' } };
      const names = dinahUIState.get('content-resources', {});
      names['Old.canvas (not saved)'] = v.id;
      dinahUIState.set('content-resources', names);
      const resource = dinahContentHost.resourceFor(v);
      const kept = dinahContentHost.draftOf('Old.canvas (not saved)') === v.id;
      dinahContentHost.forget(resource);
      return kept && resource === 'Old.canvas (not saved)';
    });
    t.ok('legacy recovery resource identities remain resolvable', legacy);
    let serial = 0;
    async function check(app, op, input, extra = {}) {
      const id = 'output-' + (++serial), ref = 'yavc.' + app + '/' + op + '@1';
      const manifest = JSON.parse(fs.readFileSync(path.join(__dirname, '../yavc/content', app, 'actions.json')));
      const action = manifest.actions.find(a => a.ref === ref);
      if (!action) throw Error('Action not declared: ' + ref);
      await t.page.evaluate(({ id, ref, app, input, action }) => {
        __emit('page:ask', { id, op: 'content', args: { app, action: ref, input, live: !!action.live, handles: action.handles || '', workspaceId: 'w', workspace: 'C:/Fixture' } });
      }, { id, ref, app, input, action });
      await t.page.waitForFunction(id => !!__answers[id], id);
      const answer = await t.page.evaluate(id => __answers[id], id);
      if (answer.error) answer.detail = await t.page.locator('[data-applet-state="error"]').allTextContents();
      const result = answer.result ? JSON.parse(answer.result) : null;
      const validation = await t.page.evaluate(({ schema, output }) => new __outputValidator(schema).validate(output), { schema: action.outputSchema, output: result && result.output });
      records.push({ ref, input, result, error: answer.error, detail: answer.detail, validation });
      t.ok(ref + ' output matches the packaged schema', !answer.error && validation.valid, JSON.stringify(records[records.length - 1]));
      if (extra.kind) t.ok(ref + ' retains requested ' + extra.kind, result && result.output.kind === extra.kind, JSON.stringify(result));
      return result;
    }
    await check('diagrams', 'new', { kind: 'canvas' }, { kind: 'canvas' });
    await check('diagrams', 'new', { kind: 'canvas' }, { kind: 'canvas' });
    await check('diagrams', 'new', { kind: 'mermaid', text: 'not valid Mermaid' }, { kind: 'mermaid' });
    await check('diagrams', 'new', { kind: 'd2', text: 'a -> b' }, { kind: 'd2' });
    await check('diagrams', 'new', { text: 'a -> b' }, { kind: 'd2' });
    await check('diagrams', 'save', { path: 'saved.d2' });
    await check('diagrams', 'open-file', { path: 'sample.canvas' });
    await check('diagrams', 'open', { path: 'sample.canvas' });
    // Exercise the actual host route independently of agentCall's New branch.
    const routed = await t.deadline('routed New', t.page.evaluate(async () => {
      const root = document.querySelector('[data-applet="diagrams"]');
      return pudlHost.documentWindows.open(root, 'diagrams', { operation: 'new', input: { kind: 'canvas', text: '{"nodes":[],"edges":[]}' } });
    }));
    const schema = JSON.parse(fs.readFileSync(path.join(__dirname, '../yavc/content/diagrams/actions.json'))).actions.find(a => a.ref === 'yavc.diagrams/new@1').outputSchema;
    const valid = await t.page.evaluate(({ schema, output }) => new __outputValidator(schema).validate(output), { schema, output: routed.output });
    records.push({ ref: 'yavc.diagrams/new@1', route: 'documentWindows.open', result: routed, validation: valid });
    t.ok('routed New retains its envelope, kind and drawing fields', valid.valid && routed.output.kind === 'canvas', JSON.stringify(routed));
    await check('diagrams', 'close', {});
    await check('docs', 'new', { text: '# Fixture\n\nBody\n' });
    await check('docs', 'new-template', { template: 'report' });
    await check('docs', 'save', { path: 'saved.md' });
    await check('docs', 'open-file', { path: 'sample.md' });
    await check('docs', 'open', { path: 'sample.md' });
    await check('docs', 'close', {});
    // Slides declares no new, new-template or open-file action. Its handled
    // present action opens a document window; it does not start a native show.
    await check('slides', 'present', { path: 'sample.slides.md' });
    await check('slides', 'save', { path: 'saved.slides.md' });
    await check('slides', 'close', {});
  } finally {
    const out = process.argv[3] || path.join(require('os').tmpdir(), 'dinah-content-action-outputs');
    fs.mkdirSync(out, { recursive: true });
    fs.writeFileSync(path.join(out, 'outputs.json'), JSON.stringify(records, null, 2));
    fs.writeFileSync(path.join(out, 'console.json'), JSON.stringify(warnings, null, 2));
    fs.writeFileSync(path.join(out, 'rejections.json'), JSON.stringify(await t.page.evaluate(() => window.__outputRejections || []), null, 2));
    await t.close();
  }
})().catch(err => { console.error(err); process.exitCode = 1; });
