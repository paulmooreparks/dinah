// The default audit samples the main states in two contrasting skins.
require('./lib/today-themes-run')({
 skins:['standard','glass'],
 states:{
  decisions:['decisions-context','broadcast-confirm'],
  notification:['pause-overdue'],
  settings:['set-aside']
 }
}).catch(error=>{console.error(error);process.exitCode=1;});
