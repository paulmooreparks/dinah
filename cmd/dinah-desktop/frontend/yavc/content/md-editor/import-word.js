/* Copyright 2026 Paul Parks. SPDX-License-Identifier: Apache-2.0 */
/* DOCX import creates native source and a report; it never saves a file. */
(function(){
    'use strict';
    var HERE=new URL('./',document.currentScript.src).href,MAX=256*1024;
    function current(options){if(options.signal&&options.signal.aborted||options.current&&!options.current())throw new DOMException('Import cancelled.','AbortError');}
    function convert(buffer,options){return new Promise(function(resolve,reject){
        current(options);var worker=new Worker(HERE+'import-word-worker.js'),done=false;
        function finish(error,result){if(done)return;done=true;clearTimeout(timer);if(options.signal)options.signal.removeEventListener('abort',cancel);worker.terminate();if(error)reject(error);else resolve(result);}
        function cancel(){finish(new DOMException('Import cancelled.','AbortError'));}
        var timer=setTimeout(function(){finish(new Error('Word import exceeded its 30-second time limit.'));},30000);
        if(options.signal)options.signal.addEventListener('abort',cancel,{once:true});
        worker.onmessage=function(event){var result=event.data;if(!result||result.error)finish(new Error(result&&result.error||'The Word importer returned no result.'));else finish(null,result);};
        worker.onerror=function(){finish(new Error('The Word import worker could not complete.'));};
        worker.postMessage(buffer.slice(0));
    });}
    function sourceBlock(template,element,source){var block=document.createElement('div');block.className='pm-source';block.setAttribute('data-kind','import');block.setAttribute('data-src',source);block.setAttribute('data-html',element.outerHTML);element.replaceWith(block);return block;}
    function escapeImage(value){return String(value||'').replace(/[\\\[\]]/g,'\\$&').replace(/[\r\n]/g,' ');}
    async function docx(buffer,options){
        options=options||{};current(options);var result=await convert(buffer,options);current(options);
        var K=options.kit;await K.pageView();var engine=await yavcMdEngine();current(options);
        var template=document.createElement('template');template.innerHTML=window.yavcHTML.clean(result.html);
        var root=template.content,stack=Array.from(root.childNodes).map(function(node){return {node:node,depth:1};}),count=0;
        while(stack.length){var item=stack.pop();if(++count>20000||item.depth>64)throw new Error('The converted document is too complex to import safely.');Array.from(item.node.childNodes).forEach(function(node){stack.push({node:node,depth:item.depth+1});});}
        var notes=Array.from(root.querySelectorAll('li[id^="footnote-"],li[id^="endnote-"]')),labels=new Map(),notesCount=0;
        notes.forEach(function(note,index){labels.set(note.id,'import-'+(index+1));});
        root.querySelectorAll('a[href^="#footnote-"],a[href^="#endnote-"]').forEach(function(link){
            var label=labels.get(link.getAttribute('href').slice(1));if(!label)return;
            var ref=document.createElement('sup');ref.className='doc-footnote-ref';ref.setAttribute('data-footnote-ref',label);ref.textContent=String(++notesCount);
            if(link.parentElement&&link.parentElement.tagName==='SUP'&&link.parentElement.childNodes.length===1)link.parentElement.replaceWith(ref);else link.replaceWith(ref);
        });
        notes.forEach(function(note){
            note.querySelectorAll('a[href^="#footnote-ref-"],a[href^="#endnote-ref-"]').forEach(function(back){back.remove();});
            var aside=document.createElement('aside');aside.className='doc-footnote';aside.setAttribute('data-footnote',labels.get(note.id));
            var body=document.createElement('div');body.className='doc-footnote-body';while(note.firstChild)body.appendChild(note.firstChild);aside.appendChild(body);root.appendChild(aside);note.remove();
        });
        root.querySelectorAll('ol:empty,ul:empty').forEach(function(list){list.remove();});
        root.querySelectorAll('img').forEach(function(image){
            if(!root.contains(image))return;
            var src=image.getAttribute('src');if(!src){image.replaceWith(document.createTextNode(image.alt||'[Image omitted during import]'));return;}
            var holder=image.closest('td,th');
            if(holder){result.report.push('An image in a table remains in an HTML source block.');return;}
            var paragraph=image.closest('p');
            if(paragraph&&(paragraph.textContent.trim()||paragraph.querySelectorAll('img').length>1)){sourceBlock(template,paragraph,paragraph.outerHTML);result.report.push('Inline images remain in an HTML source block.');}
            else sourceBlock(template,paragraph||image,'!['+escapeImage(image.alt)+']('+src+')');
        });
        root.querySelectorAll('table').forEach(function(table){
            var cells=Array.from(table.querySelectorAll('td,th'));
            if(table.querySelector('[colspan]:not([colspan="1"]),[rowspan]:not([rowspan="1"]),table,img,ol,ul')||cells.some(function(cell){return cell.querySelectorAll('p').length>1;})||Array.from(table.rows).some(function(row){return row.cells.length!==table.rows[0].cells.length;})){sourceBlock(template,table,table.outerHTML);result.report.push('A complex table remains in an HTML source block.');}
            else cells.forEach(function(cell){cell.querySelectorAll('p').forEach(function(paragraph){paragraph.replaceWith.apply(paragraph,Array.from(paragraph.childNodes));});});
        });
        var node=await window.yavcPageView.fromHTML(template.innerHTML),text=engine.markdown(node);current(options);
        if(new TextEncoder().encode(text).length>MAX)throw new Error('The imported Markdown exceeds the 256 KiB native-document limit. The original file is unchanged.');
        return {text:text,pictures:result.pictures,notes:notes.length,report:['Word page layout, fonts and section formatting become native document defaults.'].concat(result.report)};
    }
    window.yavcWordImport=Object.freeze({docx:docx});
})();
