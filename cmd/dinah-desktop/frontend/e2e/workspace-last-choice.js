const {launch,OUT}=require('./lib/page');
const fs=require('fs'),path=require('path');
(async()=>{
 for(const identity of ['path','id']){
  const t=await launch({ready:'workbench',args:identity,app:identity=>{
   const paths=['C:/Alpha','C:/Beta','C:/Gamma'];
   const records=paths.map((executionPath,i)=>({id:'ws_'+i,name:['Alpha','Beta','Gamma'][i],executionPath,primarySourceId:'root',sources:[{id:'root',kind:'local-folder',path:executionPath}]}));
   Object.assign(__state,{workspace:paths[0],recent:paths,workspaceId:identity==='id'?'ws_0':'',workspaceRecords:identity==='id'?records:[]});
   window.__workspacePending=[];window.__workspaceRequests=[];window.__workspaceActive=0;window.__workspaceMax=0;
   const select=target=>new Promise((resolve,reject)=>{
    __workspaceRequests.push(target);__workspaceActive++;__workspaceMax=Math.max(__workspaceMax,__workspaceActive);
    __workspacePending.push({target,finish:fail=>{__workspaceActive--;if(fail)return reject(Error('Invented workspace failure.'));const record=records.find(r=>(identity==='id'?r.id:r.executionPath)===target);__state.workspace=record.executionPath;__state.workspaceId=identity==='id'?record.id:'';resolve({...__state});}});
   });
   return identity==='id'?{State:async()=>({...__state}),SelectWorkspace:select,SetWorkspace:async()=>{throw Error('Saved workspaces must use their ID');}}:{State:async()=>({...__state}),SetWorkspace:select};
  }});
  const p=t.page,chooser=p.locator('#ws-select'),target=i=>identity==='id'?'ws_'+i:['C:/Alpha','C:/Beta','C:/Gamma'][i],evidence=[];
  const choose=i=>chooser.selectOption(['C:/Alpha','C:/Beta','C:/Gamma'][i]);
  const wait=length=>p.waitForFunction(length=>__workspaceRequests.length===length,length);
  const finish=fail=>p.evaluate(fail=>__workspacePending.shift().finish(fail),!!fail);
  const settled=i=>p.waitForFunction(value=>__state.workspace===value&&document.querySelector('#ws-select').value===value&&__workspaceActive===0,['C:/Alpha','C:/Beta','C:/Gamma'][i]);
  async function sample(label){const value=await p.evaluate(()=>({requests:__workspaceRequests.slice(),selected:document.querySelector('#ws-select').value,host:__state.workspace,maxActive:__workspaceMax}));evidence.push({label,...value});return value;}
  try{
   await chooser.locator('option[value="C:/Beta"]').waitFor({state:'attached'});
   await choose(1);await wait(1);await choose(0);
   await p.evaluate(()=>{window.__workspaceFrames=[];window.__watchWorkspace=true;const watch=()=>{if(!__watchWorkspace)return;__workspaceFrames.push(document.querySelector('#ws-select').value);requestAnimationFrame(watch);};watch();});
   await finish();await wait(2);
   t.ok(identity+' return choice is submitted after the earlier switch',await p.evaluate(target=>__workspaceRequests[1]===target,target(0)));
   t.ok(identity+' selector retains the last choice while its host call waits',await chooser.inputValue()==='C:/Alpha');
   await finish();await settled(0);await p.evaluate(()=>{__watchWorkspace=false;});
   t.ok(identity+' no frame paints the superseded workspace',await p.evaluate(()=>__workspaceFrames.every(value=>value==='C:/Alpha')));
   await sample('B then A');
   await choose(1);await wait(3);await choose(2);await choose(0);await finish();await wait(4);await finish();await settled(0);
   t.ok(identity+' an intermediate queued choice is replaced by the last one',await p.evaluate(target=>__workspaceRequests.length===4&&__workspaceRequests[3]===target,target(0)));
   await sample('B then C then A');
   await choose(1);await wait(5);await choose(0);await choose(1);await finish();await settled(1);
   t.ok(identity+' selecting the in-flight workspace again needs no duplicate call',await p.evaluate(()=>__workspaceRequests.length===5));
   await sample('B then A then B');
   await choose(2);await wait(6);await choose(0);await finish();await wait(7);await choose(1);await finish();await wait(8);await finish();await settled(1);
   t.ok(identity+' a newer choice during the next switch still wins',await p.evaluate(target=>__workspaceRequests[7]===target,target(1)));
   await sample('choice changes during the queued call');
   await choose(2);await wait(9);await finish(true);await settled(1);
   t.ok(identity+' a failed last choice restores the actual workspace and reports the failure',await p.locator('.toast-text').filter({hasText:'Invented workspace failure.'}).count()===1);
   await choose(0);await wait(10);await finish();await settled(0);
   t.ok(identity+' another selection works after a failure',await p.evaluate(()=>__workspaceMax===1));
   await sample('failure and subsequent selection');
   await choose(1);await wait(11);await choose(2);await finish();await wait(12);await finish(true);await settled(1);
   t.ok(identity+' a queued failure shows the last successful workspace',await chooser.inputValue()==='C:/Beta');
   await sample('queued failure after a superseded success');
   await choose(0);await wait(13);await p.locator('#status-ws-recent [data-ws="C:/Gamma"]').evaluate(el=>el.click());await finish();await wait(14);await finish();await settled(2);
   t.ok(identity+' recent-workspace actions use the same last-choice queue',await p.evaluate(target=>__workspaceRequests[13]===target,target(2)));
   fs.writeFileSync(path.join(OUT,'workspace-last-choice-'+identity+'.json'),JSON.stringify(evidence,null,2));
  }finally{await t.close();}
 }
})().catch(error=>{console.error(error);process.exitCode=1;});
