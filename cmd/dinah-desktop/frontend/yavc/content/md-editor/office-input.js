/* Copyright 2026 Paul Parks. SPDX-License-Identifier: Apache-2.0 */
/* Office selection stays with the host picker and bounded byte authority. */
(function(){
    'use strict';
    var HERE=new URL('./',document.currentScript.src).href,MAX=8*1024*1024;
    function current(o){return !(o.signal&&o.signal.aborted)&&(!o.current||o.current());}
    async function readComputer(file,o){
        if(!file.name.toLowerCase().endsWith(o.extension))throw new Error('Choose a '+o.extension+' file.');
        if(file.size>MAX)throw new Error('Choose an Office file no larger than 8 MiB.');
        var buffer=await file.arrayBuffer();if(!current(o))return null;
        if(buffer.byteLength>MAX)throw new Error('The Office file exceeds the import limit.');
        return {buffer:buffer,name:file.name};
    }
    async function pick(o){
        if(!current(o))return null;
        if(!o.browser||!o.fs)return new Promise(function(resolve,reject){
            var input=document.createElement('input'),done=false;input.type='file';input.accept=o.extension;input.hidden=true;o.root.appendChild(input);
            function finish(error,result){if(done)return;done=true;if(o.signal)o.signal.removeEventListener('abort',cancel);input.remove();if(error)reject(error);else resolve(result);}
            function cancel(){finish(null,null);}
            input.addEventListener('cancel',cancel,{once:true});if(o.signal)o.signal.addEventListener('abort',cancel,{once:true});
            input.addEventListener('change',function(){var file=input.files&&input.files[0];if(!file){cancel();return;}readComputer(file,o).then(function(result){finish(null,result);},function(error){finish(error);});},{once:true});
            input.click();
        });
        var reader=null;if(o.bytes){await yavcUtil.loadScript(new URL('../content-kit/byte-resources.js',HERE).href);reader=window.yavcByteResources.create(o.bytes);}
        if(!current(o))return null;
        var selected=null;
        await o.browser.pick(o.fs,{mode:'open',title:o.title,host:o.root,computer:true,accept:[o.extension],signal:o.signal,
            check:function(node){return node.name.toLowerCase().endsWith(o.extension)?null:'Choose a '+o.extension+' file.';},
            commit:async function(chosen){
                if(!current(o))return false;
                if(chosen.file){selected=await readComputer(chosen.file,o);return;}
                if(!reader)throw new Error('This host does not offer bounded Office file reads.');
                var read=await reader.read(chosen.path,MAX,{signal:o.signal,current:function(){return current(o);}});
                selected={buffer:read.bytes.buffer,name:read.name||chosen.path.split('/').pop()};
            }});
        return current(o)?selected:null;
    }
    async function report(o){
        if(!current(o))return false;var esc=yavcUtil.esc;
        var result=await yavcDialog.open({title:o.title,host:o.root,signal:o.signal,className:'md-import-report',body:'<p>'+esc(o.summary)+'</p><ul>'+o.messages.map(function(message){return '<li>'+esc(message)+'</li>';}).join('')+'</ul>',buttons:[{id:'cancel',label:'Cancel'},{id:'import',label:'Create Native Copy',primary:true}]});
        return result.value==='import'&&current(o);
    }
    window.yavcOfficeInput=Object.freeze({pick:pick,report:report});
})();
