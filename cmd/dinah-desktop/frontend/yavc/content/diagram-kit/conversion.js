/* Copyright 2026 Paul Parks. SPDX-License-Identifier: Apache-2.0 */
/* Conversions create independent copies and return explicit loss reports. */
(function (root) {
    'use strict';
    var J = typeof module === 'object' && module.exports ? require('./jcanvas.js') : root.yavcCanvas;
    var shapes = {rectangle:'rectangle',square:'rectangle',circle:'oval',oval:'oval',diamond:'diamond',parallelogram:'parallelogram',document:'document',cylinder:'cylinder',queue:'queue',person:'person',cloud:'cloud',hexagon:'hexagon',package:'package',text:'text'};
    function checked(doc) { var got=J.read(J.write(doc)),fault=got.problems.find(function(p){return !p.warning;});if(!got.doc||fault)throw new Error(fault?fault.message:'The conversion is not a valid canvas.');return got.doc; }
    function toCanvas(compiled, outline) {
        if(!compiled || !outline || (compiled.problems||[]).length || (outline.problems||[]).length)throw new Error('Fix the D2 source before converting it.');
        var warnings=['Automatic layout rules, themes, imports, comments and board definitions remain in the original D2 file. The copy retains node positions and connections, reroutes lines as elbows and treats labels as Markdown.'],known=new Map((outline.shapes||[]).map(function(s){return [s.id,s];}));
        var parents=new Set((outline.shapes||[]).map(function(s){return s.parent;}).filter(Boolean));
        var nodes=(compiled.shapes||[]).map(function(box){
            var source=known.get(box.id)||{},type=parents.has(box.id)?'group':'text',shape=shapes[source.shape||box.type];
            var node={id:box.id,type:type,x:Math.round(box.x),y:Math.round(box.y),width:Math.round(box.w),height:Math.round(box.h)};
            var label=typeof source.label==='string'?source.label:box.id;
            if(type==='group')node.label=label;else {node.text=label;node.yavc={shape:shape||'rectangle'};}
            if(!shape&&type==='text')warnings.push(box.id+': '+(source.shape||box.type)+' becomes a rectangle.');
            var style=source.style||{},y=node.yavc||(node.yavc={});
            [['fill','fillColor'],['stroke','strokeColor']].forEach(function(pair){if(/^#[\da-f]{3}(?:[\da-f]{3})?$/i.test(style[pair[0]]||''))y[pair[1]]=style[pair[0]];});
            if(+style['stroke-width']>=1&&+style['stroke-width']<=8&&Number.isInteger(+style['stroke-width']))y.lineWidth=+style['stroke-width'];
            if(+style['stroke-dash']>0)y.line=+style['stroke-dash']<=3?'dotted':'dashed';
            if(+style['border-radius']>0&&type==='text'&&y.shape==='rectangle')y.shape='rounded';
            if(source.icon||source.link||source.tooltip||Object.keys(style).some(function(key){return !['fill','stroke','stroke-width','stroke-dash','border-radius'].includes(key);}))warnings.push(box.id+': icons, links, tooltips or unsupported style properties remain in D2.');
            return node;
        });
        var ids=new Set(nodes.map(function(n){return n.id;}));
        var edges=(outline.connections||[]).map(function(c,i){
            if(!ids.has(c.from)||!ids.has(c.to))throw new Error('A D2 connection has an endpoint outside the compiled board.');
            var edge={id:'edge-'+(i+1),fromNode:c.from,toNode:c.to,fromEnd:c.arrows&&c.arrows.startsWith('<')?'arrow':'none',toEnd:c.arrows&&c.arrows.endsWith('>')?'arrow':'none',label:c.label||'',yavc:{route:'elbow'}};
            if(c.source||c.target||Object.keys(c.style||{}).length)warnings.push('Connection '+edge.id+': custom arrowheads and styling use Canvas defaults.');
            return edge;
        });
        return {kind:'canvas',text:J.write(checked({nodes:nodes,edges:edges})),warnings:Array.from(new Set(warnings))};
    }
    function labels(doc) { return doc.nodes.map(function(n){return n.type==='text'?n.text:n.type==='file'?n.file+(n.subpath||''):n.type==='link'?n.url:n.label||n.id;}).concat(doc.edges.map(function(e){return e.label||'';})); }
    function toD2(input, quoted) {
        var doc=checked(input),values=labels(doc),warnings=['D2 chooses new positions and routes. Canvas coordinates, dimensions, grouping, Markdown formatting, text alignment and unknown extension properties remain in the original file.'],names=new Map(doc.nodes.map(function(n,i){return [n.id,'item'+(i+1)];}));
        if(!Array.isArray(quoted)||quoted.length!==values.length||quoted.some(function(v){return typeof v!=='string';}))throw new Error('The D2 label formatter returned an invalid result.');
        var lines=['direction: right',''];
        doc.nodes.forEach(function(n,i){
            var y=n.yavc||{},shape=y.shape||'rectangle',d2shape=shape==='rounded'?'rectangle':shape;
            if(n.type!=='text')warnings.push(n.id+': '+n.type+' content becomes a labeled shape.');
            if(n.color)warnings.push(n.id+': the coordinated Canvas color becomes an outline color.');
            lines.push(names.get(n.id)+': '+quoted[i]+' {','  shape: '+(J.SHAPES.includes(d2shape)?d2shape:'rectangle'));
            if(shape==='rounded')lines.push('  style.border-radius: 8');
            var color=y.strokeColor||n.color,fill=y.fillColor;
            function hex(c){return /^[1-6]$/.test(c||'')?J.PRESETS.light[Number(c)-1]:c;}
            if(color)lines.push('  style.stroke: '+JSON.stringify(hex(color)));if(fill)lines.push('  style.fill: '+JSON.stringify(hex(fill)));
            if(y.lineWidth)lines.push('  style.stroke-width: '+y.lineWidth);
            if(y.line==='dashed'||y.line==='dotted')lines.push('  style.stroke-dash: '+(y.line==='dotted'?2:5));
            lines.push('}','');
        });
        doc.edges.forEach(function(e,i){
            var y=e.yavc||{},from=y.fromHead?y.fromHead!=='none':e.fromEnd==='arrow',to=y.toHead?y.toHead!=='none':e.toEnd!=='none';
            var arrow=from?to?'<->':'<-':to?'->':'--';
            lines.push(names.get(e.fromNode)+' '+arrow+' '+names.get(e.toNode)+': '+quoted[doc.nodes.length+i]);
            if(Object.keys(y).length||e.color)warnings.push(e.id+': endpoint anchors, custom arrowhead shapes and styling use D2 defaults.');
        });
        return {kind:'d2',text:lines.join('\n')+'\n',warnings:Array.from(new Set(warnings))};
    }
    var api=Object.freeze({toCanvas:toCanvas,toD2:toD2,labels:labels});
    if(typeof module==='object'&&module.exports)module.exports=api;else root.yavcDiagramConversion=api;
})(typeof window!=='undefined'?window:this);
