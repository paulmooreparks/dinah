/* Copyright 2026 Paul Parks. SPDX-License-Identifier: Apache-2.0 */
/* Host-owned overlays. Source, recovery and persisted review stay separate. */
(function(global){
    'use strict';
    function normalize(text){return text.replace(/\r\n?/g,'\n');}
    function boundary(text,at){return Number.isInteger(at)&&at>=0&&at<=text.length&&!(at>0&&at<text.length&&(text[at-1]==='\r'&&text[at]==='\n'||/[\uD800-\uDBFF]/.test(text[at-1])&&/[\uDC00-\uDFFF]/.test(text[at])));}
    function lfOffset(text,at){return at-(text.slice(0,at).match(/\r\n/g)||[]).length;}
    function rawOffset(text,at){var raw=0,n=0;while(n<at&&raw<text.length){raw+=text[raw]==='\r'&&text[raw+1]==='\n'?2:1;n++;}return raw;}
    function parse(snapshot,base){
        if(!snapshot||typeof snapshot.revision!=='string'||!snapshot.revision||snapshot.revision.length>4096||!Array.isArray(snapshot.items)||snapshot.items.length>256)throw new Error('Invalid host suggestion snapshot.');
        var text=snapshot.text===undefined&&base&&base.revision===snapshot.revision?base.text:snapshot.text;
        if(text===undefined)return null;
        if(typeof text!=='string'||text.length>4194304)throw new Error('Host suggestions need bounded source text.');
        var ids=new Set(),units=0;
        var items=snapshot.items.map(function(item){
            if(!item||typeof item.id!=='string'||!item.id||item.id.length>256||ids.has(item.id)||!boundary(text,item.start)||!boundary(text,item.end)||item.end<item.start)throw new Error('Invalid host suggestion range or identity.');
            if(item.stale!==undefined&&typeof item.stale!=='boolean'||item.edited!==undefined&&typeof item.edited!=='boolean')throw new Error('Invalid host suggestion status.');
            ids.add(item.id);units+=item.id.length;var out={id:item.id,from:lfOffset(text,item.start),to:lfOffset(text,item.end),stale:item.stale===true,edited:item.edited===true};
            [['before',65536],['after',65536],['author',256],['reason',4096],['group',256],['prefix',4096],['suffix',4096]].forEach(function(pair){var value=item[pair[0]];if(value===undefined&&pair[0]!=='before'&&pair[0]!=='after')value='';if(typeof value!=='string'||value.length>pair[1])throw new Error('Invalid host suggestion text.');units+=value.length;out[pair[0]]=value;});
            if(units>524288||!out.stale&&text.slice(item.start,item.end)!==out.before)throw new Error('Host suggestions do not match their source.');
            return out;
        });
        return {revision:snapshot.revision,baseRaw:text,items:items,journal:[]};
    }
    function mapItem(item,changes,doc){
        var touched=false;changes.iterChangedRanges(function(from,to){if(from<item.to&&to>item.from||from===to&&from>item.from&&from<item.to||item.from===item.to&&from<=item.from&&to>=item.to)touched=true;});
        var from=changes.mapPos(item.from,1),to=item.from===item.to?from:Math.max(from,changes.mapPos(item.to,-1));
        return Object.assign({},item,{from:from,to:to,stale:!!(item.stale||touched||doc&&doc.sliceString(from,to)!==normalize(item.before))});
    }
    function create(CM,o){
        var binding=null,serial=0,pendingSnapshot=null,latestRich=null,overrides=new Map(),busy=new Set(),closed=false;
        var set=CM.StateEffect.define();
        function alive(){return !closed&&!o.signal.aborted;}
        function view(){return o.view();}
        function data(){return view()&&view().state.field(field,false);}
        function current(){var owner=o.owner();return alive()&&binding&&binding.owner===owner&&binding.resource===(owner&&owner.path);}
        function key(item){return JSON.stringify([item.id,item.before,item.after,item.author,item.reason,item.stale,item.edited,overrides.get(item.id),busy.has(item.id),o.readOnly()]);}
        function visible(item){return Object.assign({},item,{after:overrides.has(item.id)?overrides.get(item.id):item.after,edited:item.edited||overrides.has(item.id),pending:busy.has(item.id),readOnly:o.readOnly()});}
        function widget(item,block){
            item=visible(item);var box=document.createElement(block?'div':'span');box.className='host-suggestion'+(item.stale?' host-suggestion-stale':'');box.contentEditable='false';box.dataset.hostSuggestion=item.id;box.setAttribute('role','note');
            var who=(item.author||'An agent')+(item.reason?': '+item.reason:'');box.title=who;
            var label=document.createElement('span');label.className='host-suggestion-label';label.textContent=(item.stale?'Stale suggestion by ':'Suggested by ')+(item.author||'an agent')+(item.edited?' (edited)':'');box.append(label);
            if(item.before){var removed=document.createElement('del');removed.className='host-suggestion-before';removed.textContent=item.before;box.append(removed);}
            var replacement=document.createElement('span');replacement.className='host-suggestion-new';replacement.textContent=item.after||'Delete the marked text';box.append(replacement);
            var actions=document.createElement('span');actions.className='host-suggestion-actions';
            [['accept','Accept','tick'],['reject','Reject','close'],['edit','Edit','pencil']].forEach(function(spec){
                var origin={binding:binding,serial:serial},button=document.createElement('button');button.type='button';button.className='btn btn-sm';button.dataset.suggestionAction=spec[0];button.setAttribute('aria-label',spec[1]+' suggestion by '+(item.author||'an agent'));button.disabled=busy.has(item.id)||spec[0]!=='reject'&&(item.stale||o.readOnly());
                var glyph=document.createElement('span');glyph.className='glyph';glyph.setAttribute('aria-hidden','true');glyph.style.setProperty('--glyph','var(--glyph-'+spec[2]+')');button.append(glyph,document.createTextNode(' '+spec[1]));
                button.addEventListener('mousedown',function(event){event.preventDefault();});button.addEventListener('click',function(event){event.preventDefault();event.stopPropagation();decide(item.id,spec[0],origin).catch(function(error){if(alive())o.say(error.message,true);});});actions.append(button);
            });box.append(actions);return box;
        }
        class SuggestionWidget extends CM.WidgetType{
            constructor(item){super();this.item=item;this.key=key(item);}
            eq(other){return this.key===other.key;}
            toDOM(){return widget(this.item,false);}
            ignoreEvent(){return true;}
        }
        function decorate(value){
            var marks=[];value.items.forEach(function(item){if(item.to>item.from)marks.push(CM.Decoration.mark({class:'host-suggestion-old'+(item.stale?' host-suggestion-stale':''),attributes:{title:(item.author||'An agent')+(item.reason?': '+item.reason:'')}}).range(item.from,item.to));marks.push(CM.Decoration.widget({widget:new SuggestionWidget(item),side:1}).range(item.to));});
            return Object.assign({},value,{decorations:CM.Decoration.set(marks,true)});
        }
        var field=CM.StateField.define({create:function(){return decorate({items:[],revision:null,baseRaw:null,journal:[]});},update:function(value,tr){
            var next=value;
            if(tr.docChanged){next=Object.assign({},value,{items:value.items.map(function(item){return mapItem(item,tr.changes,tr.newDoc);}),journal:value.baseRaw!==null&&value.journal.length<128?value.journal.concat(tr.changes):[],baseRaw:value.journal.length<128?value.baseRaw:null});}
            tr.effects.forEach(function(effect){if(effect.is(set))next=effect.value||next;});
            return next===value&&!tr.effects.some(function(effect){return effect.is(set);})?value:decorate(next);
        },provide:function(f){return CM.EditorView.decorations.from(f,function(value){return value.decorations;});}});
        function paint(){
            if(!current()||!view())return;
            var active=document.activeElement,box=active&&active.closest('[data-host-suggestion]'),ticket=serial,focus=box&&o.root.contains(box)?{id:box.dataset.hostSuggestion,action:active.dataset.suggestionAction,rich:!!box.closest('.ProseMirror')}:null;
            view().dispatch({effects:set.of(null)});
            if(focus)Promise.resolve().then(function(){
                if(!current()||serial!==ticket||active.isConnected||document.activeElement!==document.body)return;
                var replacement=Array.from(o.root.querySelectorAll('[data-host-suggestion]')).find(function(node){return node.dataset.hostSuggestion===focus.id&&!!node.closest('.ProseMirror')===focus.rich;});
                var button=replacement&&Array.from(replacement.querySelectorAll('button')).find(function(node){return node.dataset.suggestionAction===focus.action&&!node.disabled;});
                if(button)button.focus({preventScroll:true});else o.focus();
            });
        }
        function receive(snapshot,ticket){
            if(!current()||ticket!==serial)return;
            try{
                var parsed=parse(snapshot,binding.owner.resource),state=data();
                if(!parsed){pendingSnapshot=snapshot;return;}
                if(normalize(parsed.baseRaw)!==view().state.doc.toString()){
                    if(state.baseRaw===parsed.baseRaw&&state.revision===parsed.revision){state.journal.forEach(function(change){parsed.items=parsed.items.map(function(item){return mapItem(item,change,null);});});parsed.journal=state.journal;}
                    else{pendingSnapshot=snapshot;return;}
                }
                if(parsed.items.some(function(item){return item.to>view().state.doc.length;}))throw new Error('Host suggestions are outside this document.');
                pendingSnapshot=null;var ids=new Set(parsed.items.map(function(item){return item.id;}));overrides.forEach(function(_,id){var old=state.items.find(function(item){return item.id===id;}),next=parsed.items.find(function(item){return item.id===id;});if(!ids.has(id)||old&&next&&old.after!==next.after)overrides.delete(id);});
                view().dispatch({effects:set.of(parsed)});
            }catch(error){pendingSnapshot=null;o.say(error.message,true);}
        }
        function refresh(){
            if(!alive())return;var owner=o.owner(),resource=owner&&owner.path;
            if(!binding||binding.owner!==owner||binding.resource!==resource){
                if(binding){binding.controller.abort();if(binding.off)binding.off();}serial++;binding=null;pendingSnapshot=null;overrides.clear();busy.clear();latestRich=null;
                if(!owner||!resource||!view())return;
                var ticket=serial,controller=new AbortController();binding={owner:owner,resource:resource,controller:controller,off:null};
                view().dispatch({effects:set.of({items:[],revision:null,baseRaw:null,journal:[]})});
                try{var off=o.provider.subscribe(resource,function(snapshot){Promise.resolve().then(function(){receive(snapshot,ticket);});},{host:o.root,signal:controller.signal,current:function(){return current()&&serial===ticket;}});if(typeof off!=='function')throw new Error('Host suggestion subscriptions must return an unsubscribe function.');binding.off=off;}catch(error){o.say(error.message,true);}
            }
            if(pendingSnapshot){var pending=pendingSnapshot,ticket=serial;pendingSnapshot=null;Promise.resolve().then(function(){receive(pending,ticket);});}
            draw();
        }
        function draw(){
            var rich=o.rich(),state=data();if(!rich||!state)return;
            var text=view().state.doc.toString(),signature=JSON.stringify([state.revision,state.items.map(function(item){return [item.from,item.to,key(item)];})]);
            if(latestRich&&latestRich.rich===rich&&latestRich.text===text&&latestRich.signature===signature)return;
            latestRich={rich:rich,text:text,signature:signature};rich.suggest(text,state.items.map(visible));
        }
        async function decide(id,decision,origin){
            if(!['accept','reject','edit'].includes(decision))throw new Error('Unknown suggestion decision.');
            if(!current()||busy.has(id)||origin&&(origin.binding!==binding||origin.serial!==serial))return;
            var initiating=binding,ticket=serial,initial=data(),original=initial.items.find(function(value){return value.id===id;});if(!original)return;
            busy.add(id);paint();
            try{
                if(o.prepare)await o.prepare();
                if(!current()||binding!==initiating||ticket!==serial)return;
                var state=data(),item=state.items.find(function(value){return value.id===id;});
                if(!item||state.revision!==initial.revision||item.before!==original.before||item.after!==original.after)return;
                if(decision!=='reject'&&(item.stale||o.readOnly()))throw new Error('This suggestion cannot be accepted after its text changed.');
                var resource=initiating.resource,text=o.text(),revision=state.revision,before=item;
                function still(){return current()&&binding===initiating&&ticket===serial&&o.text()===text&&data().revision===revision&&data().items.find(function(value){return value.id===id;})===before;}
                if(decision==='edit'){
                    var replacement=await o.edit(visible(item).after,still);if(replacement==null||!still())return;if(replacement.length>65536)throw new Error('A replacement exceeds the suggestion limit.');overrides.set(id,replacement);return;
                }
                var request={resource:resource,revision:revision,id:id,decision:decision,text:text,start:rawOffset(text,item.from),end:rawOffset(text,item.to)};
                if(overrides.has(id))request.after=overrides.get(id);
                await o.provider.decide(request,{host:o.root,signal:initiating.controller.signal,current:still});
            }finally{if(binding===initiating&&ticket===serial){busy.delete(id);paint();}}
        }
        function dispose(){if(closed)return;closed=true;if(binding){binding.controller.abort();if(binding.off)binding.off();}binding=null;pendingSnapshot=null;overrides.clear();busy.clear();}
        o.signal.addEventListener('abort',dispose,{once:true});
        return {extension:field,refresh:refresh,update:function(update){if(update.docChanged||update.transactions.some(function(tr){return tr.effects.some(function(effect){return effect.is(set);});})){draw();o.changed();}},draw:draw,widget:widget,decide:decide,available:function(){return !!(current()&&data()&&data().items.length);},
            navigate:async function(direction){if(o.prepare)await o.prepare();if(!current())return;var items=data().items.slice().sort(function(a,b){return a.from-b.from;}),head=view().state.selection.main.head;if(!items.length)return;var item=direction>0?items.find(function(value){return value.from>head;})||items[0]:items.slice().reverse().find(function(value){return value.from<head;})||items[items.length-1];await o.navigate(item);},dispose:dispose};
    }
    var api=Object.freeze({parse:parse,mapItem:mapItem,create:create});
    if(typeof module==='object'&&module.exports)module.exports=api;else global.yavcHostSuggestions=api;
})(typeof window==='undefined'?null:window);
