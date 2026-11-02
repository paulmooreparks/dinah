import test from 'node:test';
import assert from 'node:assert/strict';
import { assess } from './npm-audit-gate.mjs';

const now = new Date('2026-11-02T00:00:00Z');
function fixture() {
  return {
    report: { auditReportVersion: 2, metadata: { vulnerabilities: { total: 2 } }, vulnerabilities: {
      minimatch: { nodes: ['node_modules/minimatch'], via: [{ name: 'minimatch', url: 'https://github.com/advisories/GHSA-g3hv-75wg-m249' }] },
      eslint: { nodes: ['node_modules/eslint'], via: ['minimatch'] },
    } },
    lock: { packages: {
      '': { devDependencies: { eslint: '9.39.5' } },
      'node_modules/eslint': { version: '9.39.5', dependencies: { minimatch: '^3.1.5' }, dev: true },
      'node_modules/minimatch': { version: '3.1.5', dev: true },
    } },
    policy: { exceptions: [{ advisory: 'GHSA-g3hv-75wg-m249', package: 'minimatch', version: '3.1.5', rootVersion: '9.39.5', paths: ['node_modules/minimatch'], expires: '2026-12-02', approved: true }] },
  };
}
test('only the approved ESLint path and its inherited findings pass', () => {
  const { report, lock, policy } = fixture();
  assert.deepEqual(assess(report, lock, policy, now), []);
});
test('another root reaching the same hoisted package fails', () => {
  const { report, lock, policy } = fixture();
  lock.packages[''].dependencies = { other: '1.0.0' };
  lock.packages['node_modules/other'] = { dependencies: { minimatch: '*' } };
  assert.ok(assess(report, lock, policy, now).length);
});
test('the same advisory on a different installed path fails', () => {
  const { report, lock, policy } = fixture();
  report.vulnerabilities.minimatch.nodes = ['node_modules/eslint/node_modules/minimatch'];
  lock.packages['node_modules/eslint/node_modules/minimatch'] = {};
  assert.ok(assess(report, lock, policy, now).length);
});
test('another advisory on the allowed package fails', () => {
  const { report, lock, policy } = fixture();
  report.vulnerabilities.minimatch.via.push({ name: 'minimatch', url: 'https://github.com/advisories/GHSA-other' });
  assert.ok(assess(report, lock, policy, now).length);
});
test('another minimatch or ESLint version fails even before the next advisory', () => {
  const { report, lock, policy } = fixture();
  lock.packages['node_modules/minimatch'].version = '10.2.6';
  assert.ok(assess(report, lock, policy, now).length);
  lock.packages['node_modules/minimatch'].version = '3.1.5';
  lock.packages['node_modules/eslint'].version = '10.14.0';
  assert.ok(assess(report, lock, policy, now).length);
});
test('expiry and pending approval fail, including with a clean audit', () => {
  const { report, lock, policy } = fixture();
  assert.ok(assess(report, lock, policy, new Date('2026-12-02T00:00:00Z')).length);
  policy.exceptions[0].approved = false;
  assert.ok(assess(report, lock, policy, now).length);
  report.vulnerabilities = {};
  report.metadata.vulnerabilities.total = 0;
  assert.ok(assess(report, lock, policy, now).length);
});
test('missing reports and dangling inherited findings fail closed', () => {
  const { report, lock, policy } = fixture();
  assert.ok(assess({}, lock, policy, now).length);
  report.vulnerabilities.eslint.via = ['missing'];
  assert.ok(assess(report, lock, policy, now).length);
});
