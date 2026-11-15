'use strict';
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const { spawn } = require('node:child_process');

exports.open = async function open() {
  if (!process.env.QUALITY_TYPED_BINARY) throw new Error('QUALITY_TYPED_BINARY must name the frozen tagged Go test binary');
  const artifact = fs.mkdtempSync(path.join(os.tmpdir(), 'typed-question-receiving-'));
  const ready = path.join(artifact, 'ready.txt');
  const output = fs.openSync(path.join(artifact, 'host.log'), 'w');
  const child = spawn(process.env.QUALITY_TYPED_BINARY, ['-test.run=^TestTypedQuestionAcceptanceServer$', '-test.timeout=150s', '-test.v'], {
    windowsHide: true, stdio: ['ignore', output, output],
    env: { ...process.env, QUALITY_TYPED_READY: ready, QUALITY_TYPED_ARTIFACT: artifact }
  });
  fs.closeSync(output);
  const exited = new Promise((resolve, reject) => { child.on('error', reject); child.on('exit', (code, signal) => resolve({code, signal})); });
  let url;
  try {
    url = await new Promise((resolve, reject) => {
      const timer = setTimeout(() => finish(new Error(`Host did not publish its ready address; evidence ${artifact}`)), 30000);
      const watch = fs.watch(artifact, () => read());
      function finish(err, value) { clearTimeout(timer); watch.close(); err ? reject(err) : resolve(value); }
      function read() { try { const value = fs.readFileSync(ready, 'utf8'); if (value) finish(null, value); } catch {} }
      child.once('exit', code => finish(new Error(`Host exited ${code}; evidence ${artifact}`)));
      read();
    });
  } catch (error) { child.kill(); await exited; throw error; }
  async function call(method, ...args) {
    const response = await fetch(url, { method: 'POST', body: JSON.stringify({method, args}), signal: AbortSignal.timeout(20000) });
    if (!response.ok) throw new Error(`${method}: ${response.status} ${await response.text()}`);
    return response.json();
  }
  let queued = [], scheduled = false;
  function answer(key, raw) {
    return new Promise((resolve, reject) => {
      queued.push({key, raw, resolve, reject});
      if (scheduled) return;
      scheduled = true;
      setImmediate(async () => {
        const batch = queued; queued = []; scheduled = false;
        try {
          if (batch.length === 1) batch[0].resolve(await call('answer', batch[0].key, batch[0].raw));
          else {
            const results = await call('concurrentAnswers', batch.map(x => [x.key, x.raw]));
            batch.forEach((item, i) => item.resolve(results[i]));
          }
        } catch (error) { batch.forEach(item => item.reject(error)); }
      });
    });
  }
  return {
    ask: request => call('ask', request), pending: view => call('pending', view), answer,
    answerBytes: (key, bytes) => call('answerBytes', key, bytes),
    answerLegacy: (key, id) => call('answerLegacy', key, id), result: key => call('result', key),
    snapshot: () => call('snapshot'), mutatePendingCopy: (key, patch) => call('mutatePendingCopy', key, patch),
    settlement: key => call('settlement', key), withdraw: (kind, key) => call('withdraw', kind, key),
    restart: () => call('restart'), callService: (name, input) => call('callService', name, input),
    observations: () => call('observations'),
    async close() {
      try { await call('close'); }
      finally {
        let timer;
        const result = await Promise.race([exited, new Promise(resolve => { timer = setTimeout(() => { child.kill(); resolve({code:null,signal:'watchdog'}); }, 10000); })]);
        clearTimeout(timer);
        if (result.code !== 0) throw new Error(`Tagged host did not finish cleanly (${JSON.stringify(result)}); evidence ${artifact}`);
      }
    }
  };
};
