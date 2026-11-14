const { launch } = require('./lib/page');
(async () => {
 const t = await launch({ready:'workbench',init:()=>{window.__personAnswers={};},app:()=>({UpdateState:async()=>({ready:true,pending:false,working:0}),AnswerPage:async(id,result,err)=>{window.__personAnswers[id]={result:result?JSON.parse(result):null,err};}})});
 try {
  const p=t.page;
  const key=await p.evaluate(async()=>{dinahWorkbench.register('person-fixture',{title:'Fixture',init:root=>{pudlCommands.register(root,[{id:'read-fixture',label:'Read',run:()=>{window.__personRead=true;}}]);return {};}});return dinahWorkbench.open({kind:'person-fixture',workspace:'C:/Fixture',path:'one'});});
  await p.evaluate(key=>dinahWorkbench.ready(key),key);
  const ask=(op,command)=>p.evaluate(async({op,command,key})=>{const id='person-'+Math.random();__emit('page:ask',{id,op,args:{workspace:'C:/Fixture',window:key,command}});while(!__personAnswers[id])await new Promise(resolve=>setTimeout(resolve,10));return __personAnswers[id];},{op,command,key});
  const listed=await ask('commands');
  t.ok('application restart is absent from the agent command inventory',!listed.err&&!listed.result.commands.some(c=>['pause-and-restart','restart-to-update','restart-now'].includes(c.id)));
  for(const command of ['pause-and-restart','restart-to-update','restart-now']) {const r=await ask('run',command);t.ok(command+' is refused even when guessed',!!r.err&&r.err.includes('left to the person'));}
  const r=await ask('run','read-fixture');t.ok('documented ordinary commands remain usable',!r.err&&await p.evaluate(()=>__personRead));
  t.ok('no restart method reached the App',!(await t.calls()).some(c=>['PauseAndRestart','RestartToUpdate','CancelRestart'].includes(c[0])));
 }finally{await t.close();}
})().catch(error=>{console.error(error);process.exitCode=1;});
