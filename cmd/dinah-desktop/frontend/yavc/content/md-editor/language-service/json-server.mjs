// Copyright 2026 Paul Parks. SPDX-License-Identifier: Apache-2.0
// A browser-local LSP 3.17 subset. No schema or document is fetched from a URL.
import { getLanguageService } from 'vscode-json-languageservice';
import { TextDocument } from 'vscode-languageserver-textdocument';
import { canvasSchema } from './canvas-schema.mjs';
import { getCSSLanguageService } from 'vscode-css-languageservice';
import { getLanguageService as getHTMLLanguageService } from 'vscode-html-languageservice';

export function createServer(send) {
    const documents = new Map(), requests = new Map(), timers = new Map();
    const service = getLanguageService({ schemaRequestService: async () => { throw new Error('External schemas are unavailable. Only bundled schemas are supported.'); } });
    service.configure({ validate: true, allowComments: false, schemas: [{ uri: canvasSchema.$id, fileMatch: ['*.canvas'], schema: canvasSchema }] });
    const css = getCSSLanguageService(), html = getHTMLLanguageService();
    const providers = {
        json: { service, parse: doc => service.parseJSONDocument(doc), validate: (doc, parsed) => service.doValidation(doc, parsed, { comments: 'error', trailingCommas: 'error' }) },
        css: { service: css, parse: doc => css.parseStylesheet(doc), validate: (doc, parsed) => css.doValidation(doc, parsed) },
        // The HTML service exposes completion, hover and formatting, but no
        // general validation API. Do not present an empty report as validation.
        html: { service: html, parse: doc => html.parseHTMLDocument(doc) }
    };
    let initialized = false, shutdown = false;
    const response = (id, result) => send({ jsonrpc: '2.0', id, result: result == null ? null : result });
    const error = (id, code, message) => send({ jsonrpc: '2.0', id, error: { code, message } });
    const notify = (method, params) => send({ jsonrpc: '2.0', method, params });
    function diagnose(uri, record) {
        clearTimeout(timers.get(uri));
        timers.set(uri, setTimeout(async () => {
            timers.delete(uri);
            try {
                const provider = providers[record.doc.languageId];
                if (!provider.validate) return;
                const diagnostics = await provider.validate(record.doc, provider.parse(record.doc));
                if (documents.get(uri) === record) notify('textDocument/publishDiagnostics', { uri, version: record.doc.version, diagnostics });
            } catch { /* A later change or request can retry schema validation. */ }
        }, 120));
    }
    async function receive(message) {
        const m = typeof message === 'string' ? JSON.parse(message) : message;
        if (!m || m.jsonrpc !== '2.0' || typeof m.method !== 'string') return;
        const p = m.params || {};
        if (m.method === '$/cancelRequest') { const r = requests.get(p.id); if (r) r.cancelled = true; return; }
        if (m.method === 'initialize') {
            initialized = true;
            response(m.id, { capabilities: { positionEncoding: 'utf-16', textDocumentSync: { openClose: true, change: 2 }, completionProvider: { triggerCharacters: ['"', ':', '<', ' ', '.', '#'] }, hoverProvider: true, documentFormattingProvider: true }, serverInfo: { name: 'yavc-browser-languages', version: '1.1.0' } }); return;
        }
        if (m.method === 'shutdown') { shutdown = true; response(m.id, null); return; }
        if (!initialized || shutdown) { if (m.id != null) error(m.id, -32002, 'The language service is not initialized.'); return; }
        if (m.method === 'initialized' || m.method === 'exit') return;
        if (m.method === 'textDocument/didOpen') {
            const d = p.textDocument;
            const kind = d && (d.languageId === undefined ? 'json' : d.languageId);
            if (!d || typeof d.uri !== 'string' || typeof d.text !== 'string' || d.text.length > 2097152 || !Object.hasOwn(providers,kind) || !Number.isSafeInteger(d.version) || documents.size >= 64) return;
            const record = { doc: TextDocument.create(d.uri, kind, d.version, d.text) };
            documents.set(d.uri, record); diagnose(d.uri, record); return;
        }
        const uri = p.textDocument && p.textDocument.uri, record = documents.get(uri);
        if (m.method === 'textDocument/didClose') {
            documents.delete(uri); clearTimeout(timers.get(uri)); timers.delete(uri); return;
        }
        if (m.method === 'textDocument/didChange') {
            if (!record || p.textDocument.version <= record.doc.version || !Array.isArray(p.contentChanges)) return;
            // TextDocument.update mutates its input. Use a new object so pending
            // requests and validations retain their original revision.
            const next = TextDocument.update(TextDocument.create(uri, record.doc.languageId, record.doc.version, record.doc.getText()), p.contentChanges, p.textDocument.version);
            if (next.getText().length > 2097152) { documents.delete(uri); clearTimeout(timers.get(uri)); return; }
            const updated = { doc: next }; documents.set(uri, updated); diagnose(uri, updated); return;
        }
        if (m.id == null) return;
        if (!record) { error(m.id, -32801, 'The document is no longer open.'); return; }
        if (requests.size >= 32) { error(m.id, -32000, 'Too many pending language requests.'); return; }
        const request = { cancelled: false }; requests.set(m.id, request);
        try {
            const provider = providers[record.doc.languageId], parsed = provider.parse(record.doc), language = provider.service;
            let result;
            if (m.method === 'textDocument/completion') result = await language.doComplete(record.doc, p.position, parsed);
            else if (m.method === 'textDocument/hover') result = await language.doHover(record.doc, p.position, parsed);
            else if (m.method === 'textDocument/formatting') result = language.format(record.doc, undefined, { tabSize: Math.max(1, Math.min(8, p.options && p.options.tabSize || 2)), insertSpaces: !(p.options && p.options.insertSpaces === false) });
            else { error(m.id, -32601, 'Unsupported language request.'); return; }
            if (request.cancelled) error(m.id, -32800, 'Request cancelled.');
            else if (documents.get(uri) !== record) error(m.id, -32801, 'The document changed before the request completed.');
            else response(m.id, result);
        } catch { error(m.id, -32603, 'The language request could not be completed.'); }
        finally { requests.delete(m.id); }
    }
    return { receive, dispose() { for (const timer of timers.values()) clearTimeout(timer); timers.clear(); documents.clear(); requests.clear(); } };
}
