/* Copyright 2026 Paul Parks. SPDX-License-Identifier: Apache-2.0 */
/* Document starters and theme previews use the normal scoped renderer. */
(function(){
    'use strict';
    var count=0, LIMIT=256*1024;
    var templates=Object.freeze([
        Object.freeze({id:'blank',title:'Blank Document',description:'A clean page with the standard document defaults.',text:''}),
        Object.freeze({id:'report',title:'Report',description:'A report with a summary, findings and recommendations.',text:'---\ntitle: Report\ntheme: report\npage: a4\nmargins: 25mm\nfooter: "{page} of {pages}"\n---\n\n# Report\n\n## Summary\n\nDescribe the purpose and the main finding.\n\n## Findings\n\nRecord the evidence and what it means.\n\n## Recommendations\n\nDescribe the proposed action.\n'}),
        Object.freeze({id:'letter',title:'Letter',description:'A letter with sender, recipient, greeting and closing.',text:'---\ntitle: Letter\ntheme: letter\npage: letter\nmargins: 1in\n---\n\nSender name  \nSender address\n\nDate\n\nRecipient name  \nRecipient address\n\nDear Recipient,\n\nWrite your letter here.\n\nSincerely,\n\nSender name\n'}),
        Object.freeze({id:'manuscript',title:'Manuscript',description:'A double-spaced manuscript with running page numbers.',text:'---\ntitle: Manuscript\ntheme: manuscript\npage: a4\nmargins: 25mm\nheader: "{title}"\nfooter: "{page}"\nheader-first: none\n---\n\n# Manuscript\n\nAuthor name\n\n## Chapter One\n\nBegin the manuscript here.\n'}),
        Object.freeze({id:'meeting',title:'Meeting Notes',description:'An agenda, notes, decisions and action table.',text:'---\ntitle: Meeting Notes\ntheme: report\npage: a4\nmargins: 25mm\n---\n\n# Meeting Notes\n\n**Date:** \n\n**Attendees:** \n\n## Agenda\n\n- First topic\n\n## Notes\n\nRecord the discussion.\n\n## Decisions\n\nRecord the agreed decisions.\n\n## Actions\n\n| Action | Owner | Due |\n| --- | --- | --- |\n| Describe the next step | Name | Date |\n'})
    ]);
    function stock(id){var item=templates.find(function(t){return t.id===id;});if(!item)throw new Error('Choose a supported document template.');return item.text;}
    async function open(kind,opts){
        if(opts.signal.aborted||!opts.current())return null;
        var K=window.yavcMdEditor,P=await K.pages(),D=await K.styles(),S=window.yavcDocumentSettings,esc=window.yavcUtil.esc;
        if(opts.signal.aborted||!opts.current())return null;
        var theme=kind==='theme',choices=theme?S.themes:templates,id='document-gallery-'+(++count),api,controller,alive=true,revision=0,prepared=null,ready=false,timer=null,reading=null,lastProblem=null;
        var selected=theme?(opts.properties.theme||'plain'):'blank';
        var options=choices.map(function(item){return '<option value="'+item.id+'"'+(item.id===selected?' selected':'')+'>'+esc(item.title)+'</option>';}).join('');
        if(theme&&!choices.some(function(item){return item.id===selected;}))options='<option value="" selected>Choose a Theme</option>'+options;
        if(!theme&&opts.read)options+='<option value="file">From a Markdown File…</option>';
        var body='<div class="md-gallery-layout"><section class="md-gallery-controls"><div class="form-group"><label class="form-label" for="'+id+'-choice">'+(theme?'Theme':'Template')+'</label><span class="form-select-wrap"><select class="form-select" id="'+id+'-choice" data-gallery-choice>'+options+'</select></span><p class="form-help" data-gallery-description></p></div>';
        if(!theme&&opts.read)body+='<div data-gallery-file hidden>'+window.yavcDialog.pathField({id:id+'-file',label:'Template File',accept:'.md,.markdown',maxBytes:LIMIT,help:'Choose or upload a Markdown template. The copy uses its new location for relative references; absolute virtual paths keep shared assets.'})+'</div>';
        body+='<p class="form-help">'+(theme?'Document typography and styles can override the theme. Apply changes the whole document.':'Create opens an independent unsaved document. The template file is never changed.')+'</p><div data-gallery-error-ref></div></section><section class="md-gallery-sample" aria-label="Document Preview"><p class="form-label">Preview</p><div class="md-gallery-boundary"><div class="doc-page md-gallery-preview" data-gallery-preview></div></div></section></div>';
        function current(){return alive&&!opts.signal.aborted&&opts.current();}
        function clearProblems(){lastProblem=null;api.body.querySelectorAll('[data-gallery-choice],[data-file-path]').forEach(function(node){node.removeAttribute('aria-invalid');var refs=(node.getAttribute('aria-describedby')||'').split(/\s+/).filter(function(ref){return ref&&ref!==id+'-error';});if(refs.length)node.setAttribute('aria-describedby',refs.join(' '));else node.removeAttribute('aria-describedby');});window.yavcUtil.notice(api.body.querySelector('[data-gallery-error-ref]'),'');}
        function fileError(message){return Object.assign(new Error(message),{field:'file'});}
        function failure(error,focus){ready=false;prepared=null;lastProblem=error;if(alive){var notice=window.yavcUtil.notice(api.body.querySelector('[data-gallery-error-ref]'),error.message,true);notice.id=id+'-error';var field=error.field==='file'?api.body.querySelector('[data-file-path]'):error.field==='theme'?api.body.querySelector('[data-gallery-choice]'):null;if(field){field.setAttribute('aria-invalid','true');var refs=new Set((field.getAttribute('aria-describedby')||'').split(/\s+/).filter(Boolean));refs.add(notice.id);field.setAttribute('aria-describedby',Array.from(refs).join(' '));if(focus)field.focus();}}}
        async function draw(){
            clearTimeout(timer);if(reading)reading.abort();reading=new AbortController();var token=++revision;ready=false;prepared=null;
            if(!current())return;
            clearProblems();
            var choice=api.body.querySelector('[data-gallery-choice]').value,file=api.body.querySelector('[data-gallery-file]'),item=choices.find(function(t){return t.id===choice;});
            if(file)file.hidden=choice!=='file';
            api.body.querySelector('[data-gallery-description]').textContent=item?item.description:'';
            var surface=api.body.querySelector('[data-gallery-preview]');surface.replaceChildren();api.body.querySelector('.md-gallery-sample').setAttribute('aria-busy','true');
            try{
                var text,props,inline=[],stylesheet='';
                if(theme){props=Object.assign({},opts.properties,{theme:choice});S.themeCSS(props);text='# A clear document\n\nThe theme sets the default type and spacing. Document typography and styles retain their overrides.\n\n## A useful heading\n\nA second paragraph shows the space between paragraphs.\n';inline=opts.styles||[];stylesheet=opts.stylesheet||'';}
                else{
                    if(choice==='file'){
                        var path=api.body.querySelector('[data-file-path]').value.trim();
                        if(!/^(~\/|\/(?!\/))[^:\\?#\u0000-\u001f\u007f]+\.(md|markdown)$/i.test(path)||/\.slides\.md$/i.test(path))throw fileError('Choose an absolute virtual path to a Markdown document.');
                        try{text=await opts.read(path,{signal:reading.signal,maxBytes:LIMIT});}catch(error){throw fileError(error.message||'The template file could not be read.');}
                        if(!current()||token!==revision)return;
                        if(typeof text!=='string')throw fileError('The template file could not be read.');
                    }else text=stock(choice);
                    if(new TextEncoder().encode(text).byteLength>LIMIT)throw fileError('A document template must be no larger than 256 KiB.');
                    props=S.properties(opts.engine.front(text));S.typeCSS(props);inline=opts.engine.styles(text).map(function(block){return block.css;});
                }
                var rendered=opts.engine.preview(text);
                if((rendered.problems||[]).length)throw new Error('Fix the template warnings before creating a document.');
                var fragment=window.yavcHTML.fragment(rendered.blocks.map(function(b){return b.html;}).join(''));
                // A preview never starts embedded applets or loads author-supplied assets.
                fragment.querySelectorAll('img,iframe,video,audio,[data-applet],script,style').forEach(function(node){node.remove();});
                fragment.querySelectorAll('a').forEach(function(node){node.removeAttribute('href');});
                if(!text)fragment.append(document.createTextNode('Your document starts with a blank page.'));
                surface.append(fragment);
                await controller.update({identity:id,base:P.paper+';'+S.typeCSS(props),inline:inline,stylesheet:stylesheet});
                if(!current()||token!==revision)return;
                var problems=controller.state().problems;if(problems.length)throw new Error(problems.join(' '));
                ready=true;prepared=theme?choice:text;window.yavcUtil.notice(api.body.querySelector('[data-gallery-error-ref]'),'');
            }catch(error){if(current()&&token===revision)failure(error);}
            finally{if(alive&&token===revision)api.body.querySelector('.md-gallery-sample').removeAttribute('aria-busy');}
        }
        function abort(){if(api)api.close('cancel');}
        opts.signal.addEventListener('abort',abort);
        try{
            var result=await window.yavcDialog.open({title:theme?'Document Theme':'New from Template',className:'md-gallery-dialog',body:body,buttons:[{id:'cancel',label:'Cancel'},{id:'apply',label:theme?'Apply':'Create',primary:true}],
                onOpen:function(value){api=value;controller=D.attach({surfaces:[api.body.querySelector('[data-gallery-preview]')],boundaries:[api.body.querySelector('.md-gallery-boundary')],base:P.paper,read:opts.read});api.body.addEventListener('change',function(e){if(e.target.matches('[data-gallery-choice],[data-file-path]'))draw();});api.body.addEventListener('input',function(e){if(e.target.matches('[data-file-path]')){revision++;ready=false;clearProblems();clearTimeout(timer);timer=setTimeout(draw,200);}});if(!theme&&opts.read)window.yavcDialog.wirePaths(api,{signal:opts.signal,current:current});draw();},
                check:function(value){if(value!=='apply')return false;if(!current()){failure(new Error('The document changed. Close and reopen this dialog.'));return true;}if(!ready){failure(lastProblem||new Error('Choose a template or theme and wait for its preview.'),true);return true;}if(theme)opts.save({theme:prepared==='plain'?null:prepared},{});return false;}});
            return result&&result.value==='apply'?prepared:null;
        }finally{alive=false;revision++;clearTimeout(timer);if(reading)reading.abort();if(controller)controller.destroy();opts.signal.removeEventListener('abort',abort);}
    }
    window.yavcDocumentGallery=Object.freeze({theme:function(opts){return open('theme',opts);},template:function(opts){return open('template',opts);},stock:stock,templates:templates});
})();
