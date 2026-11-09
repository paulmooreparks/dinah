/* Copyright 2026 Paul Parks. SPDX-License-Identifier: Apache-2.0 */
/* Header-only size guards use PNG IHDR, GIF logical screen dimensions and
   JPEG T.81 frame headers. Decoding remains the browser's responsibility. */
(function(global){
    'use strict';
    function accepts(bytes,type){
        if(!(bytes instanceof Uint8Array)||bytes.length>128*1024)return false;
        var width=0,height=0,view=new DataView(bytes.buffer,bytes.byteOffset,bytes.byteLength);
        if(type==='image/png'&&bytes.length>=24&&[137,80,78,71,13,10,26,10].every(function(n,i){return bytes[i]===n;})&&view.getUint32(8)===13&&view.getUint32(12)===0x49484452){width=view.getUint32(16);height=view.getUint32(20);}
        if(type==='image/gif'&&bytes.length>=10&&/^GIF8[79]a$/.test(String.fromCharCode.apply(null,bytes.subarray(0,6)))){width=view.getUint16(6,true);height=view.getUint16(8,true);}
        if(type==='image/jpeg'&&bytes.length>=4&&bytes[0]===255&&bytes[1]===216){
            for(var at=2;at+4<=bytes.length;){
                if(bytes[at++]!==255)break;while(bytes[at]===255)at++;var marker=bytes[at++];
                if(marker===217||marker===218)break;if(marker===1||marker>=208&&marker<=215)continue;
                if(at+2>bytes.length)break;var length=view.getUint16(at);if(length<2||at+length>bytes.length)break;
                if(marker>=192&&marker<=207&&marker!==196&&marker!==200&&marker!==204&&length>=8&&length===8+3*bytes[at+7]){height=view.getUint16(at+3);width=view.getUint16(at+5);break;}at+=length;
            }
        }
        return width>0&&height>0&&width<=16384&&height<=16384&&width*height<=16*1024*1024;
    }
    function dataURL(bytes,type){if(!accepts(bytes,type))return null;var chunks=[];for(var at=0;at<bytes.length;at+=8192)chunks.push(String.fromCharCode.apply(null,bytes.subarray(at,at+8192)));return 'data:'+type+';base64,'+btoa(chunks.join(''));}
    var api=Object.freeze({accepts:accepts,dataURL:dataURL,maxBytes:128*1024});
    if(typeof module==='object'&&module.exports)module.exports=api;else global.yavcImportImages=api;
})(typeof self==='undefined'?null:self);
