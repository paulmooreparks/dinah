// The deadline-check suite's child (e2e/deadline.js runs it and measures its
// exit): settle a wait, close the browser, and leave at once. With the
// deadline's timer left running, the process pays the full timeout after its
// work is done, which is what the suite measures.
const { launch } = require('./page');
(async () => {
  const t = await launch({ timeout: 1000 });
  await t.deadline('the child’s settling wait', Promise.resolve(7));
  await t.close();
  console.log('settled-and-closed');
})().catch(error => { console.error(error); process.exitCode = 1; });
