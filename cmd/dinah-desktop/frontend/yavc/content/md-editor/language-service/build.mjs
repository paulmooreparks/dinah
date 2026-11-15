// Copyright 2026 Paul Parks. SPDX-License-Identifier: Apache-2.0
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { createRequire } from 'node:module';
import { createHash } from 'node:crypto';
const here = path.dirname(fileURLToPath(import.meta.url));
const dependencies = process.env.YAVC_LSP_BUILD_ROOT || here;
const require = createRequire(path.join(dependencies, 'package.json'));
const { build } = require('esbuild');
const check = process.argv.includes('--check');
const digest = bytes => createHash('sha256').update(bytes).digest('hex');
function output(name, bytes) {
    const target = path.resolve(here, name);
    if (check) { if (!fs.existsSync(target) || !Buffer.from(bytes).equals(fs.readFileSync(target))) throw new Error('Rebuild required: ' + name); }
    else fs.writeFileSync(target, bytes);
}
const manifest = { version: 1, inputs: {}, outputs: {}, packages: {} };
for (const name of ['build.mjs', 'package.json', '../vendor/codemirror-6.43/entry.js', 'canvas-schema.mjs', 'json-server.mjs', 'worker-entry.mjs', 'bridge.js', 'package-lock.json', 'licenses/vscode-l10n-LICENSE.txt']) {
    manifest.inputs[name] = digest(fs.readFileSync(path.resolve(here, name)));
}
for (const [entry, target, globalName] of [['../vendor/codemirror-6.43/entry.js', '../vendor/codemirror-6.43/codemirror.js', 'CM'], ['worker-entry.mjs', 'worker.js', undefined]]) {
    const result = await build({ absWorkingDir: here, entryPoints: [entry], bundle: true, format: 'iife', globalName, minify: true, legalComments: 'eof', platform: 'browser', target: ['es2020'], write: false, metafile: true, nodePaths: [path.join(dependencies, 'node_modules')] });
    const bytes = result.outputFiles[0].contents; output(target, bytes); manifest.outputs[target] = digest(bytes);
    const packages = new Set();
    for (const input of Object.keys(result.metafile.inputs)) {
        const marker = input.replaceAll('\\', '/').split('/node_modules/').pop();
        if (marker === input.replaceAll('\\', '/')) continue;
        const parts = marker.split('/'); packages.add(parts[0].startsWith('@') ? parts.slice(0, 2).join('/') : parts[0]);
    }
    const licenses = [];
    for (const name of [...packages].sort()) {
        const dir = path.join(dependencies, 'node_modules', name), info = JSON.parse(fs.readFileSync(path.join(dir, 'package.json')));
        manifest.packages[name] = { version: info.version, license: info.license };
        const license = fs.readdirSync(dir).find(f => /^licen[cs]e(\.txt|\.md)?$/i.test(f));
        const licensePath = license ? path.join(dir, license) : name === '@vscode/l10n' ? path.join(here, 'licenses/vscode-l10n-LICENSE.txt') : null;
        if (!licensePath) throw new Error('No license file for ' + name);
        licenses.push(name + ' ' + info.version + ' (' + info.license + ')\n\n' + fs.readFileSync(licensePath, 'utf8').replace(/\r\n/g, '\n').replace(/[ \t]+$/gm, '').trimEnd());
    }
    const licenseTarget = target === 'worker.js' ? 'WORKER-LICENSES.txt' : '../vendor/codemirror-6.43/LICENSE.txt', licenseBytes = Buffer.from(licenses.join('\n\n') + '\n');
    output(licenseTarget, licenseBytes); manifest.outputs[licenseTarget] = digest(licenseBytes);
}
// Code grammars (docs/language-packs-design.md, decision 1): one classic script for each language, built from CodeMirror's legacy-mode package. A legacy mode is a plain object that holds no CodeMirror of its own, so the script needs no copy of the editor's bundle and cannot be loaded into one it does not match. Each script only records its mode as window.yavcCodeModes[id]; apps/md-editor/languages.js is the one table that says which file or fence is which language.
const GRAMMARS = [['go', 'go', 'go']];
{
    const legacy = path.join(dependencies, 'node_modules', '@codemirror', 'legacy-modes'), info = JSON.parse(fs.readFileSync(path.join(legacy, 'package.json')));
    manifest.packages['@codemirror/legacy-modes'] = { version: info.version, license: info.license };
    for (const [id, file, name] of GRAMMARS) {
        const contents = `import { ${name} as mode } from '@codemirror/legacy-modes/mode/${file}';\n(window.yavcCodeModes = window.yavcCodeModes || {})[${JSON.stringify(id)}] = mode;\n`;
        const result = await build({ stdin: { contents, resolveDir: dependencies, sourcefile: 'grammar-' + id + '.js' }, bundle: true, format: 'iife', minify: true, legalComments: 'none', platform: 'browser', target: ['es2020'], write: false });
        const target = '../vendor/codemirror-6.43/lang/' + id + '.js', bytes = result.outputFiles[0].contents;
        fs.mkdirSync(path.dirname(path.resolve(here, target)), { recursive: true }); output(target, bytes); manifest.outputs[target] = digest(bytes);
    }
    const licenseFile = fs.readdirSync(legacy).find(f => /^licen[cs]e(\.txt|\.md)?$/i.test(f)), licenseTarget = '../vendor/codemirror-6.43/lang/LICENSE.txt';
    const licenseBytes = Buffer.from('@codemirror/legacy-modes ' + info.version + ' (' + info.license + ')\n\n' + fs.readFileSync(path.join(legacy, licenseFile), 'utf8').replace(/\r\n/g, '\n').replace(/[ \t]+$/gm, '').trimEnd() + '\n');
    output(licenseTarget, licenseBytes); manifest.outputs[licenseTarget] = digest(licenseBytes);
}
manifest.packages = Object.fromEntries(Object.entries(manifest.packages).sort(([a], [b]) => a.localeCompare(b)));
output('manifest.json', JSON.stringify(manifest, null, 2) + '\n');
console.log(check ? 'Language-service artifacts match their pinned sources.' : 'Built shared CodeMirror LSP client and browser JSON server.');
