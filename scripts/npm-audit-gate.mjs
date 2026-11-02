import { readFileSync, writeFileSync, mkdirSync } from 'node:fs';
import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { spawnSync } from 'node:child_process';

const here = dirname(fileURLToPath(import.meta.url));
const e2e = resolve(here, '../cmd/dinah-desktop/frontend/e2e');

// Resolve dependencies from the locked tree, including hoisted packages.
function dependencyPath(packages, from, name) {
  let parent = from;
  while (true) {
    const candidate = (parent ? parent + '/' : '') + 'node_modules/' + name;
    if (packages[candidate]) return candidate;
    if (!parent) return null;
    const cut = parent.lastIndexOf('/node_modules/');
    parent = cut < 0 ? '' : parent.slice(0, cut);
  }
}

function rootsFor(packages, target) {
  const root = packages[''];
  const roots = [];
  for (const name of Object.keys({ ...root.dependencies, ...root.devDependencies, ...root.optionalDependencies })) {
    const seen = new Set();
    const pending = [dependencyPath(packages, '', name)];
    while (pending.length) {
      const path = pending.pop();
      if (!path || seen.has(path)) continue;
      seen.add(path);
      if (path === target) { roots.push(name); break; }
      const pkg = packages[path];
      for (const dep of Object.keys({ ...pkg.dependencies, ...pkg.optionalDependencies, ...pkg.peerDependencies })) {
        pending.push(dependencyPath(packages, path, dep));
      }
    }
  }
  return roots;
}

export function assess(report, lock, policy, now = new Date()) {
  const failures = [];
  if (report.auditReportVersion !== 2 || report.error || !report.vulnerabilities || !report.metadata?.vulnerabilities || !lock.packages?.['']) {
    return ['Missing or invalid npm audit report or lockfile.'];
  }
  for (const rule of policy.exceptions) {
    if (!rule.approved || !/^\d{4}-\d{2}-\d{2}$/.test(rule.expires) || now >= new Date(rule.expires + 'T00:00:00Z')) {
      failures.push('Audit exception is unapproved or expired: ' + rule.advisory);
    }
    if (lock.packages['node_modules/eslint']?.version !== rule.rootVersion ||
        lock.packages[''].devDependencies?.eslint !== rule.rootVersion) {
      failures.push('Audit exception requires exact development dependency eslint ' + rule.rootVersion);
    }
    const matches = Object.entries(lock.packages).filter(([path]) => path === 'node_modules/' + rule.package || path.endsWith('/node_modules/' + rule.package));
    if (!matches.length || matches.some(([path, pkg]) => !rule.paths.includes(path) || pkg.version !== rule.version || !pkg.dev || JSON.stringify(rootsFor(lock.packages, path)) !== '["eslint"]')) {
      failures.push('Audit exception package version or dependency path changed: ' + rule.package);
    }
  }
  const vulnerabilities = report.vulnerabilities;
  for (const [name, vulnerability] of Object.entries(vulnerabilities)) {
    const seen = new Set();
    let leaves = 0;
    const visit = (key) => {
      if (seen.has(key)) return true;
      seen.add(key);
      const entry = vulnerabilities[key];
      if (!entry || !Array.isArray(entry.via) || !entry.via.length || !entry.nodes?.length) return false;
      for (const node of entry.nodes) {
        if (!lock.packages[node] || JSON.stringify(rootsFor(lock.packages, node)) !== '["eslint"]') return false;
      }
      for (const via of entry.via) {
        if (typeof via === 'string') {
          if (!visit(via)) return false;
        } else {
          leaves++;
          const allowed = policy.exceptions.some(rule => rule.approved &&
            now < new Date(rule.expires + 'T00:00:00Z') &&
            via.url === 'https://github.com/advisories/' + rule.advisory &&
            via.name === rule.package && key === rule.package &&
            entry.nodes.every(node => rule.paths.includes(node) && lock.packages[node]?.version === rule.version));
          if (!allowed) return false;
        }
      }
      return true;
    };
    if (!visit(name) || !leaves) failures.push('Unapproved npm finding: ' + name);
  }
  if (Object.keys(vulnerabilities).length !== report.metadata.vulnerabilities.total) {
    failures.push('Audit finding count does not match its summary.');
  }
  return failures;
}

if (process.argv[1] && resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  try {
    const result = spawnSync(process.platform === 'win32' ? 'npm.cmd' : 'npm', ['audit', '--json'], {
      cwd: e2e, encoding: 'utf8', shell: process.platform === 'win32', maxBuffer: 16 * 1024 * 1024,
    });
    if (process.argv[2]) {
      const output = resolve(process.argv[2]);
      mkdirSync(dirname(output), { recursive: true });
      writeFileSync(output, result.stdout || '');
    }
    if (result.error || ![0, 1].includes(result.status)) throw new Error('npm audit did not complete: ' + (result.stderr || result.error || result.status));
    const failures = assess(JSON.parse(result.stdout),
      JSON.parse(readFileSync(resolve(e2e, 'package-lock.json'), 'utf8')),
      JSON.parse(readFileSync(resolve(here, 'npm-audit-exceptions.json'), 'utf8')));
    if (failures.length) throw new Error(failures.join('\n'));
    console.log('npm audit passed with only approved, unexpired dependency paths.');
  } catch (error) {
    console.error(error.message);
    process.exitCode = 1;
  }
}
