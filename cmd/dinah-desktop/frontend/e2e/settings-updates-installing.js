// This state uses only a stubbed bridge in the shared headless browser host.
require('./lib/updates-cases')('installing').catch(error => { console.error(error); process.exitCode = 1; });
