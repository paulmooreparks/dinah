/* Copyright 2026 Paul Parks. SPDX-License-Identifier: Apache-2.0 */
/* Persisted review UI. The workspace supplies captures and guarded edits. */
(function(){
    'use strict';
    var count=0,names={addition:'Insertion',deletion:'Deletion',replacement:'Replacement',comment:'Comment',highlight:'Highlight'},glyphs={addition:'plus',deletion:'minus',replacement:'pencil',comment:'comments',highlight:'quote'};
    function create(o){
        var manager=null;
        function guard(snapshot,selection){
            var controller=new AbortController();function check(){if(!snapshot.current()||selection&&snapshot.selectionCurrent&&!snapshot.selectionCurrent())controller.abort();}
            function abort(){controller.abort();}if(o.signal){o.signal.addEventListener('abort',abort,{once:true});if(o.signal.aborted)abort();}var unwatch=o.watch?o.watch(check):function(){};check();
            return {signal:controller.signal,release:function(){unwatch();if(o.signal)o.signal.removeEventListener('abort',abort);}};
        }
        async function propose(kind){
            var snapshot=await o.capture(true);if(!snapshot||!snapshot.writable)return false;
            var catalog=snapshot.engine.review(snapshot.text),from=snapshot.from,to=snapshot.to,body='',result;
            if((kind==='deletion'||kind==='replacement'||kind==='highlight')&&from===to)throw new Error('Select the text to review first.');
            if(kind==='addition')from=to;
            function prepare(value){if(!snapshot.current()||snapshot.selectionCurrent&&!snapshot.selectionCurrent())throw new Error('The document or selection changed.');return snapshot.engine.proposeReview({text:snapshot.text,revision:catalog.revision,from:from,to:to,kind:kind,body:value});}
            if(kind==='deletion'||kind==='highlight'){result=prepare('');}
            else{
                var lifetime=guard(snapshot,true);
                try{body=await yavcDialog.askText(kind==='comment'?'Add Comment':'Suggest '+(kind==='addition'?'Insertion':'Replacement'),kind==='comment'?'Comment':'Proposed Text',kind==='comment'&&snapshot.author?snapshot.author+': ':'',{
                    multiline:true,signal:lifetime.signal,okLabel:kind==='comment'?'Add Comment':'Add Suggestion',
                    text:kind==='addition'?'The insertion goes at the caret or after the selection.':kind==='replacement'?'The selected source remains as the original text until the suggestion is accepted.':'The comment stays in the source until it is cleared. Export omits comments from a clean copy.',
                    help:kind==='comment'?'The author signature is ordinary editable comment text.':'Markdown formatting is supported in suggested text. Keep review delimiters inside code examples.',
                    validate:function(value){if(!value.trim())return 'Enter some text.';try{result=prepare(value);return null;}catch(err){return err.message;}}
                });}finally{lifetime.release();}
                if(body===null)return false;
            }
            if(!snapshot.current()||snapshot.selectionCurrent&&!snapshot.selectionCurrent())return false;
            await snapshot.apply(result);return true;
        }
        async function all(decision){
            var snapshot=await o.capture(false);if(!snapshot||!snapshot.writable)return false;var catalog=snapshot.engine.review(snapshot.text);if(!catalog.marks.length){o.say('There are no review marks to resolve.');return false;}
            var lifetime=guard(snapshot,false),accepted;
            try{accepted=await yavcDialog.confirm((decision==='accept'?'Accept':'Reject')+' All Changes?',decision==='accept'?'Keep insertions and replacement text, remove deletions, and clear comments and review highlights. Undo restores the annotations.':'Remove insertions, restore original replacement text, and clear comments and review highlights. Undo restores the annotations.',{okLabel:decision==='accept'?'Accept All':'Reject All',signal:lifetime.signal});}finally{lifetime.release();}
            if(!accepted||!snapshot.current())return false;
            await snapshot.apply(snapshot.engine.resolveReview({text:snapshot.text,revision:catalog.revision,id:'all',decision:decision}));return true;
        }
        async function browse(raw){
            if(manager){manager.dialog.focus();return;}
            var snapshot=await o.capture(false);if(!snapshot)return;var catalog=snapshot.engine.review(snapshot.text),selected=-1,pending=false,api,disposed=false,refreshTicket=0,unwatch=function(){},id='md-review-'+(++count);
            function current(){return !disposed&&snapshot&&snapshot.current();}
            function chosen(){return catalog.marks[selected]||null;}
            function status(message){if(api)yavcUtil.notice(api.body.querySelector('[data-review-error]'),message,true);}
            function busy(value){pending=value;if(api)api.dialog.setAttribute('aria-busy',String(value));details();}
            function details(){if(!api)return;var mark=chosen(),before=api.body.querySelector('[data-review-before]'),after=api.body.querySelector('[data-review-after]');
                before.hidden=!mark||!mark.before;after.hidden=!mark||!mark.after;before.querySelector('div').textContent=mark?mark.before:'';after.querySelector('div').textContent=mark?mark.after:'';
                api.body.querySelector('[data-review-location]').textContent=mark?names[mark.kind]+' on line '+mark.line:'No annotation selected.';
                var edit=api.body.querySelector('[data-review-edit]');edit.disabled=pending||!mark||!snapshot.writable||!['addition','replacement','comment'].includes(mark.kind);
                api.body.querySelector('[data-review-source]').disabled=pending||!mark;
                api.dialog.querySelector('button[value="accept"]').disabled=pending||!mark||!snapshot.writable;
                api.dialog.querySelector('button[value="reject"]').disabled=pending||!mark||!snapshot.writable;
            }
            function choose(index,scroll){selected=index;var list=api.body.querySelector('[role="listbox"]');list.querySelectorAll('[role="option"]').forEach(function(row){var on=Number(row.dataset.index)===selected;row.setAttribute('aria-selected',String(on));if(on){list.setAttribute('aria-activedescendant',row.id);if(scroll)row.scrollIntoView({block:'nearest'});}});if(!chosen())list.removeAttribute('aria-activedescendant');details();}
            function rows(){if(!api)return;var list=api.body.querySelector('[role="listbox"]'),query=api.body.querySelector('input[type="search"]').value.trim().toLowerCase();list.replaceChildren();var visible=[];
                catalog.marks.forEach(function(mark,index){if(query&&!(names[mark.kind]+' '+mark.before+' '+mark.after).toLowerCase().includes(query))return;visible.push(index);var row=document.createElement('div');row.className='listbox-row';row.dataset.value=String(index);row.id=id+'-row-'+index;row.dataset.index=String(index);row.setAttribute('role','option');row.setAttribute('aria-selected','false');var glyph=document.createElement('span');glyph.className='glyph';glyph.style.setProperty('--glyph','var(--glyph-'+glyphs[mark.kind]+')');glyph.setAttribute('aria-hidden','true');var label=document.createElement('span');label.className='listbox-title';label.textContent=names[mark.kind]+': '+(mark.after||mark.before||'(empty)').replace(/\s+/g,' ').slice(0,180);var location=document.createElement('span');location.className='listbox-detail';location.textContent='Line '+mark.line;label.prepend(glyph,document.createTextNode(' '));row.append(label,location);list.appendChild(row);});
                if(selected>=0&&!visible.includes(selected))selected=visible.length?visible[0]:-1;
                api.body.querySelector('[data-review-empty]').hidden=visible.length>0;
                api.body.querySelector('[data-review-summary]').textContent=catalog.marks.length+' annotation(s)'+(catalog.problems.length?'; '+catalog.problems.length+' malformed mark(s) remain in Text.':'.');choose(selected,false);pudlListbox.enhance(list.parentNode);pudlListbox.refresh(list);
            }
            async function refresh(preferred){var ticket=++refreshTicket;busy(true);try{var next=await o.capture(false);if(disposed||ticket!==refreshTicket||!next||!next.current())return;snapshot=next;catalog=snapshot.engine.review(snapshot.text);selected=Math.min(preferred==null?selected:preferred,catalog.marks.length-1);rows();}catch(err){if(!disposed)status(err.message);}finally{if(!disposed&&ticket===refreshTicket)busy(false);}}
            async function resolve(decision){var mark=chosen();if(!mark||pending)return;if(!current()){status('The document changed. Review the refreshed list before applying a decision.');await refresh();return;}var index=selected;busy(true);try{await snapshot.apply(snapshot.engine.resolveReview({text:snapshot.text,revision:catalog.revision,id:mark.id,decision:decision}));await refresh(index);}catch(err){status(err.message);}finally{busy(false);}}
            async function edit(){
                var mark=chosen();if(!mark||pending||!current())return;
                var captured=snapshot,revision=catalog.revision,lifetime=guard(captured,false),result;busy(true);
                try{
                    var value=await yavcDialog.askText('Edit '+names[mark.kind],mark.kind==='comment'?'Comment':'Proposed Text',mark.kind==='comment'?mark.before:mark.after,{
                        multiline:true,signal:lifetime.signal,okLabel:'Apply',
                        validate:function(value){try{if(!captured.current())return 'The document changed.';result=captured.engine.editReview({text:captured.text,revision:revision,id:mark.id,body:value});return null;}catch(err){return err.message;}}
                    });
                    if(value!==null&&captured.current()){await captured.apply(result);await refresh(selected);}
                }catch(err){if(err.name!=='AbortError')status(err.message);}
                finally{lifetime.release();busy(false);if(!disposed&&!captured.current())await refresh(selected);}
            }
            try{await yavcDialog.open({title:'Review Changes',className:'md-review-dialog',signal:o.signal,
                body:'<p data-review-summary></p><div data-review-error></div><div class="md-review-layout"><section class="md-review-selection"><label class="form-label" for="'+id+'-search">Find an Annotation</label><input class="form-input" type="search" id="'+id+'-search"><div class="listbox md-review-list" data-listbox="single" data-listbox-rows="10" role="listbox" aria-label="Annotations"></div><p class="form-help" data-review-empty hidden>No matching annotations.</p></section><section class="md-review-details" aria-label="Annotation Details"><p data-review-location></p><section data-review-before><h4>Original Text or Annotation</h4><div class="md-review-copy"></div></section><section data-review-after><h4>Proposed Text</h4><div class="md-review-copy"></div></section><div class="md-review-tools"><button type="button" class="btn" data-review-edit>Edit…</button><button type="button" class="btn" data-review-source>Show in Text</button></div></section></div>',
                buttons:[{id:'cancel',label:'Close'},{id:'reject',label:'Reject',submit:true},{id:'accept',label:'Accept',primary:true}],
                check:function(value){if(value==='accept'||value==='reject'){resolve(value);return true;}return false;},
                onOpen:function(value){api=value;manager=api;var matches=raw?catalog.marks.map((mark,index)=>mark.raw===raw?index:-1).filter(index=>index>=0):[];selected=matches.length===1?matches[0]:raw?-1:0;rows();if(matches.length>1)status('This annotation occurs more than once. Select the occurrence by its source line before applying a decision.');
                    api.body.querySelector('input[type="search"]').addEventListener('input',rows);
                    var list=api.body.querySelector('[role="listbox"]');list.addEventListener('pudl:listbox-change',function(){var value=pudlListbox.value(list);choose(value===null?-1:Number(value),false);});
                    api.body.querySelector('[data-review-edit]').addEventListener('click',edit);
                    api.body.querySelector('[data-review-source]').addEventListener('click',async function(){var mark=chosen();if(!mark||!current())return;api.close('source');await o.navigate(mark);});
                    unwatch=o.watch?o.watch(function(){if(!pending&&!current())refresh();}):function(){};
                }});
            }finally{disposed=true;unwatch();manager=null;}
        }
        return Object.freeze({propose:propose,browse:browse,all:all});
    }
    async function project(o){
        var catalog=o.engine.review(o.text);if(!catalog.marks.length&&!catalog.problems.length)return o.text;
        try{var prepared=o.engine.exportPreview?o.engine:o.engine.withMacros(o.text,'');prepared.exportPreview(o.text);return o.text;}catch(err){if(!/review projection/.test(err.message))throw err;}
        if(catalog.problems.length)throw new Error('Repair malformed review marks in Text before exporting.');
        if(o.policy!=null){if(!['accepted','original'].includes(o.policy))throw new Error('Choose accepted or original reviewPolicy.');return o.engine.resolveReview({text:o.text,revision:catalog.revision,id:'all',decision:o.policy==='accepted'?'accept':'reject'}).text;}
        if(o.interactive===false)throw new Error('This document has unresolved review marks. Supply reviewPolicy as accepted or original.');
        var choice=await yavcDialog.askText('Export Reviewed Copy','Version','accepted',{choices:[['accepted','Accepted version'],['original','Original version']],signal:o.signal,okLabel:'Continue',text:'The exported copy omits comments and review highlights. Your source and its annotations stay unchanged.',validate:function(){return o.current()?null:'The document changed. Cancel and export again.';}});
        if(choice===null||!o.current())return null;
        return o.engine.resolveReview({text:o.text,revision:catalog.revision,id:'all',decision:choice==='accepted'?'accept':'reject'}).text;
    }
    window.yavcReviewTools=Object.freeze({create:create,project:project});
})();
