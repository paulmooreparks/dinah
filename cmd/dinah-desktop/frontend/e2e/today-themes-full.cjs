// This audit is opt-in. run.ps1 discovers only .js suite entry points.
require('fs').mkdirSync(require('./lib/page').OUT,{recursive:true});
require('./lib/today-themes-run')({only:process.env.DINAH_TODAY_SURFACE})
 .catch(error=>{console.error(error);process.exitCode=1;});
