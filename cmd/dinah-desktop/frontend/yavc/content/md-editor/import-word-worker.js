/* Copyright 2026 Paul Parks. SPDX-License-Identifier: Apache-2.0 */
/* A disposable, bounded semantic DOCX conversion worker. */
'use strict';
importScripts('vendor/mammoth-1.13.0/office.js','import-images.js');
var mammoth=self.yavcOfficeLibraries.mammoth,JSZip=self.yavcOfficeLibraries.JSZip,XMLParser=self.yavcOfficeLibraries.DOMParser;
var PART_MAX=8*1024*1024,TOTAL_MAX=32*1024*1024,IMAGE_MAX=128*1024,HTML_MAX=2*1024*1024;
function readPart(entry,budget){return new Promise(function(resolve,reject){
    var chunks=[],size=0,done=false,stream=entry.internalStream('uint8array');
    stream.on('data',function(chunk){if(done)return;size+=chunk.length;budget.used+=chunk.length;if(size>PART_MAX||budget.used>TOTAL_MAX){done=true;stream.pause();reject(new Error('The expanded Office package exceeds the import limit.'));return;}chunks.push(chunk);})
        .on('error',function(err){if(!done){done=true;reject(err);}}).on('end',function(){if(done)return;done=true;var bytes=new Uint8Array(size),at=0;chunks.forEach(function(chunk){bytes.set(chunk,at);at+=chunk.length;});resolve(bytes);}).resume();
});}
self.onmessage=async function(event){try{
    var buffer=event.data;if(!(buffer instanceof ArrayBuffer)||buffer.byteLength>8*1024*1024)throw new Error('Choose a DOCX file no larger than 8 MiB.');
    var zip=await JSZip.loadAsync(buffer),entries=Object.values(zip.files);if(entries.length>1024)throw new Error('The Office package has too many entries.');
    if(!zip.file('word/document.xml')||!zip.file('[Content_Types].xml'))throw new Error('This package is not a Word DOCX document.');
    var bounded=new JSZip(),budget={used:0},report=[],features=new Set(),nodeCount=0,wordType=false;
    for(var entry of entries){
        var path=entry.unsafeOriginalName||entry.name;
        if(path.length>512||/^[\/\\]|[\\:\x00-\x1f]/.test(path)||path.split('/').some(function(part){return part==='..'||part==='.';}))throw new Error('The Office package contains an invalid part path.');
        if(entry.dir)continue;
        var bytes=await readPart(entry,budget);
        if(/\.(xml|rels)$/i.test(path)||path==='[Content_Types].xml'){
            var part=new TextDecoder('utf-8',{fatal:true}).decode(bytes);
            if(/<!DOCTYPE|<!ENTITY/i.test(part))throw new Error('Office XML declarations with entities are not supported.');
            var strict=function(message){throw new Error('Malformed Office XML: '+String(message).slice(0,200));};
            var doc=new XMLParser({errorHandler:{warning:strict,error:strict,fatalError:strict}}).parseFromString(part,'application/xml');
            if(!doc.documentElement)throw new Error('An Office XML part is empty.');
            if(path==='[Content_Types].xml')wordType=Array.from(doc.getElementsByTagNameNS('http://schemas.openxmlformats.org/package/2006/content-types','Override')).some(function(node){return node.getAttribute('PartName')==='/word/document.xml'&&node.getAttribute('ContentType')==='application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml';});
            var stack=[{node:doc.documentElement,depth:1}];
            while(stack.length){var item=stack.pop();if(++nodeCount>100000||item.depth>64)throw new Error('The Office XML exceeds the structural import limit.');var node=item.node;
                if(node.namespaceURI==='http://schemas.openxmlformats.org/wordprocessingml/2006/main')features.add(node.localName);
                if(node.namespaceURI==='http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing'&&node.localName==='anchor')features.add('floating-anchor');
                for(var child=node.firstChild;child;child=child.nextSibling)if(child.nodeType===1)stack.push({node:child,depth:item.depth+1});
            }
        }
        bounded.file(entry.name,bytes);
    }
    if(!wordType)throw new Error('This package is not a supported Word DOCX document type.');
    function detected(names,message){if(names.some(function(name){return features.has(name);}))report.push(message);}
    detected(['ins','del','moveFrom','moveTo'],'Tracked changes are flattened to the converter’s visible text; their review history is not imported.');
    detected(['commentRangeStart','commentReference','comments'],'Word comments are not imported. Keep the original file to review them.');
    detected(['headerReference','footerReference'],'Headers and footers are not imported.');
    detected(['drawing','pict','txbxContent','floating-anchor'],'Floating positions, drawing geometry and text-box placement are not retained.');
    detected(['fldChar','instrText'],'Word fields become their displayed text and do not update as fields.');
    detected(['altChunk','object'],'Embedded documents and objects are not imported.');
    detected(['gridSpan','vMerge'],'Merged tables remain sanitized HTML source blocks rather than native editable tables.');
    var pictures=0,omitted=0,imageBytes=0;
    var result=await mammoth.convertToHtml({arrayBuffer:await bounded.generateAsync({type:'arraybuffer',compression:'STORE'})},{
        includeEmbeddedStyleMap:false,externalFileAccess:false,styleMap:['u => u','strike => s'],
        convertImage:mammoth.images.imgElement(async function(image){
            var encoded=await image.read('base64'),binary=atob(encoded),bytes=Uint8Array.from(binary,function(char){return char.charCodeAt(0);});
            if(bytes.length>IMAGE_MAX||imageBytes+bytes.length>IMAGE_MAX||!yavcImportImages.accepts(bytes,image.contentType)){omitted++;return {alt:'[Image omitted during import]'};}
            imageBytes+=bytes.length;pictures++;return {src:'data:'+image.contentType+';base64,'+encoded};
        })
    });
    if(result.value.length>HTML_MAX)throw new Error('The converted document exceeds the import limit.');
    (result.messages||[]).forEach(function(message){if(report.length<100)report.push(String(message.message).slice(0,1000));});
    if(omitted)report.push(omitted+' image(s) were omitted because their type or combined size exceeds the 128 KiB native-image budget.');
    self.postMessage({html:result.value,report:report,pictures:pictures,omitted:omitted});
}catch(err){self.postMessage({error:String(err.message||err)});}finally{self.close();}};
