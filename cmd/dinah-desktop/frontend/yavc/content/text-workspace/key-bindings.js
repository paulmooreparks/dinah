/* Copyright 2026 Paul Parks. SPDX-License-Identifier: Apache-2.0 */
(function () {
    'use strict';
    var sequence = 0;
    async function open(opts) {
        var U=window.yavcUtil, esc=U.esc, api, busy=false, draft=Object.assign(Object.create(null),opts.keys || {}), id='key-bindings-'+(++sequence);
        var labels=new Map(opts.commands.map(function (c) { return [c.id,c.label]; }));
        function commandOptions() {
            return opts.commands.map(function (c) {return '<option value="'+esc(c.id)+'">'+esc(c.label)+'</option>';}).join('');
        }
        function problem(message,field) {
            var notice=U.notice(api.body.querySelector('[data-bindings-errors]'),message,true);
            if(field){notice.id=id+'-error';field.setAttribute('aria-invalid','true');field.setAttribute('aria-describedby',notice.id);field.focus();}
        }
        function clearProblem(field) {field.removeAttribute('aria-invalid');field.removeAttribute('aria-describedby');U.notice(api.body.querySelector('[data-bindings-errors]'),'');}
        function render() {
            var keys=Array.from(new Set(Object.keys(opts.defaults).concat(Object.keys(draft)))).sort(), query=api.body.querySelector('[data-bindings-filter]').value.toLowerCase();
            api.body.querySelector('tbody').innerHTML=keys.map(function (key,index) {
                var value=Object.prototype.hasOwnProperty.call(draft,key)?draft[key]===null?'!disabled':draft[key]:'!default';
                var defaultLabel=labels.get(opts.defaults[key]) || opts.defaults[key] || 'Unassigned';
                var options='<option value="!default">Default: '+esc(defaultLabel)+'</option><option value="!disabled">Disabled</option>'+commandOptions();
                if(value!=='!disabled'&&value!=='!default'&&!labels.has(value))options+='<option value="'+esc(value)+'">Unknown command: '+esc(value)+'</option>';
                return '<tr'+((key+' '+opts.keyLabel(key)+' '+String(value)+' '+String(labels.get(value)||'')).toLowerCase().includes(query)?'':' hidden')+'><th scope="row"><label for="'+id+'-'+index+'">'+esc(opts.keyLabel(key))+'</label></th><td><span class="form-select-wrap"><select class="form-select" id="'+id+'-'+index+'" data-binding-key="'+esc(key)+'">'+options+'</select></span></td></tr>';
            }).join('');
            api.body.querySelectorAll('[data-binding-key]').forEach(function (select) {
                var key=select.dataset.bindingKey;
                select.value=Object.prototype.hasOwnProperty.call(draft,key)?draft[key]===null?'!disabled':draft[key]:'!default';
            });
        }
        async function save() {
            if(busy)return;
            if(!opts.current()){problem('The key bindings changed elsewhere. Close and reopen this dialog.');return;}
            for(var key of Object.keys(draft)){
                if(!opts.validKey(key) || draft[key]!==null&&!labels.has(draft[key])){problem('Choose a valid shortcut and command for '+key+'.');return;}
            }
            busy=true;api.dialog.querySelectorAll('button,input,select').forEach(function (b) {b.disabled=true;});
            try {
                var error=await opts.save(Object.assign({},draft));if(error)throw new Error(error);
                api.close('apply');
            }catch(err){if(api.dialog.open)problem('The key bindings were not saved: '+err.message);}
            finally{busy=false;if(api.dialog.open)api.dialog.querySelectorAll('button,input,select').forEach(function (b) {b.disabled=false;});}
        }
        return window.yavcDialog.open({title:opts.title+' Key Bindings',className:'ed-key-bindings',signal:opts.signal,
            body:'<p>Changes apply to '+esc(opts.title)+'. Choose Default to restore a shortcut, or Disabled to turn it off.</p>'+
                '<div class="form-group"><label class="form-label" for="'+id+'-filter">Find a Shortcut</label><input class="form-input" type="search" id="'+id+'-filter" data-bindings-filter></div>'+
                '<div class="ed-bindings-list"><table class="data-table"><thead><tr><th scope="col">Shortcut</th><th scope="col">Command</th></tr></thead><tbody></tbody></table></div>'+
                '<fieldset class="form-fieldset"><legend>Add a Shortcut</legend><div class="ed-binding-add"><div class="form-group"><label class="form-label" for="'+id+'-new-key">Shortcut</label><input class="form-input" id="'+id+'-new-key" data-new-key maxlength="100" placeholder="Mod-Shift-k"></div>'+
                '<div class="form-group"><label class="form-label" for="'+id+'-new-command">Command</label><span class="form-select-wrap"><select class="form-select" id="'+id+'-new-command" data-new-command><option value="">Choose a command</option><option value="!disabled">Disabled</option>'+commandOptions()+'</select></span></div><button type="button" class="btn" data-add-binding>Add</button></div><p class="muted">Use names such as Mod-s or Ctrl-Shift-k. Mod means Ctrl on Windows and Command on macOS.</p></fieldset>'+
                '<button type="button" class="btn" data-reset-bindings>Restore Defaults</button><div data-bindings-errors></div>',
            buttons:[{id:'cancel',label:'Cancel'},{id:'apply',label:'Apply',primary:true}],
            check:function(value){if(value==='apply'){save();return true;}},
            onOpen:function(value){
                api=value;render();
                api.body.querySelector('[data-bindings-filter]').addEventListener('input',render);
                api.body.querySelector('[data-new-key]').addEventListener('input',function(e){clearProblem(e.target);});
                api.body.querySelector('[data-new-command]').addEventListener('change',function(e){clearProblem(e.target);});
                api.body.addEventListener('change',function(e){if(!e.target.matches('[data-binding-key]'))return;clearProblem(e.target);var key=e.target.dataset.bindingKey,v=e.target.value;if(v==='!default')delete draft[key];else draft[key]=v==='!disabled'?null:v;});
                api.body.querySelector('[data-reset-bindings]').addEventListener('click',function(){draft=Object.create(null);render();});
                api.body.querySelector('[data-add-binding]').addEventListener('click',function(){
                    var keyField=api.body.querySelector('[data-new-key]'),command=api.body.querySelector('[data-new-command]'),key=keyField.value.trim();
                    keyField.removeAttribute('aria-invalid');keyField.removeAttribute('aria-describedby');command.removeAttribute('aria-invalid');command.removeAttribute('aria-describedby');
                    if(!opts.validKey(key)){problem('Enter a shortcut such as Mod-Shift-k.',keyField);return;}
                    if(!command.value){problem('Choose a command for this shortcut.',command);return;}
                    draft[key]=command.value==='!disabled'?null:command.value;
                    api.body.querySelector('[data-bindings-filter]').value='';keyField.value='';render();
                });
            }});
    }
    window.yavcKeyBindings=Object.freeze({open:open});
})();
