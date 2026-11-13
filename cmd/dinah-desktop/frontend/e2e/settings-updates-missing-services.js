// Unsupported service capabilities stay explicit in the headless page.
require('./lib/updates-cases')('missing-services').catch(error => { console.error(error); process.exitCode = 1; });
