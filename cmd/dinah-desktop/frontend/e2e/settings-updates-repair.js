// This state uses only a stubbed bridge in the shared headless browser host.
require('./lib/updates-cases')('repair').catch(error => { console.error(error); process.exitCode = 1; });
