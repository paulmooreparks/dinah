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
manifest.packages = Object.fromEntries(Object.entries(manifest.packages).sort(([a], [b]) => a.localeCompare(b)));
output('manifest.json', JSON.stringify(manifest, null, 2) + '\n');
console.log(check ? 'Language-service artifacts match their pinned sources.' : 'Built shared CodeMirror LSP client and browser JSON server.');
