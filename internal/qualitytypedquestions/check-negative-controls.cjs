'use strict';
const fs = require('node:fs');
const path = require('node:path');
const os = require('node:os');
const { spawnSync } = require('node:child_process');
const output = fs.mkdtempSync(path.join(os.tmpdir(), 'typed-question-negative-controls-'));
const cases = [
  ['wrong-unicode-count', 'text counts Unicode'],
  ['reordered-selection', 'multiple answers validate'],
  ['consumed-invalid-answer', 'strict answer shapes'],
  ['duplicate-settlement', 'views share one question'],
  ['erased-question-obligation', 'service calls cannot answer']
];
let failures = 0;
for (const [variant, pattern] of cases) {
  const result = spawnSync(process.execPath, ['--test', '--test-concurrency=1', `--test-name-pattern=${pattern}`, path.join(__dirname, 'acceptance.test.cjs')], {
    windowsHide:true, encoding:'utf8', timeout:45000,
    env:{...process.env, QUALITY_TYPED_NEGATIVE:variant, QUALITY_TYPED_QUESTION_BINDING:path.join(__dirname,'negative-binding.cjs')}
  });
  fs.writeFileSync(path.join(output, `${variant}.json`), JSON.stringify(result, null, 2));
  if (result.status === 0 || !`${result.stdout}${result.stderr}`.includes('ERR_ASSERTION')) failures++;
}
console.log(JSON.stringify({detected:cases.length-failures, expected:cases.length, artifactDirectory:output}));
process.exitCode = failures ? 1 : 0;
