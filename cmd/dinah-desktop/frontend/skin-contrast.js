/* Inspect correlated paints at laid-out text positions without changing app focus. */
(function () {
  'use strict';
  const cache = new Map();
  const escape = value => String(value).replace(/[&<>"]/g, c => ({'&':'&amp;', '<':'&lt;', '>':'&gt;', '"':'&quot;'})[c]);
  function links() { return [...document.querySelectorAll('link[data-pudl-skin]')].map(link => ({id:link.dataset.pudlSkin, href:link.href})); }
  function sheets() { return [...document.querySelectorAll('link[rel="stylesheet"]')].map(link => link.href); }
  function markup() {
    return '<!doctype html><html data-theme="light" data-pudl-persistence="host"><head><meta charset="utf-8">' + sheets().map(href => '<link rel="stylesheet" href="' + escape(href) + '">').join('') + '<style>:root{forced-color-adjust:none}body{margin:0}.skin-audit-host{padding:32px;background:var(--surface)}.skin-audit-host .win{position:relative;width:400px;min-height:80px}.skin-audit-host *{transition:none!important;animation:none!important}</style></head><body><main class="skin-audit-host"></main></body></html>';
  }
  function split(value) {
    const result = []; let depth = 0, start = 0;
    [...value].forEach((c,i) => { if(c==='(')depth++;else if(c===')')depth--;else if(c===','&&!depth){result.push(value.slice(start,i).trim());start=i+1;} });
    result.push(value.slice(start).trim()); return result;
  }
  // CSSOM changes are confined to the disposable audit document.
  function interactionRules(rules) {
    for (const rule of rules) {
      // The fixed audit viewport evaluates palette paints independently of
      // the operating system's forced colors. The application is untouched.
      if (rule.media && rule.media.mediaText.includes('forced-colors')) {
        const media = rule.media.mediaText.replace(/\(forced-colors:\s*none\)/g, '(min-width: 0px)').replace(/\(forced-colors:\s*active\)/g, '(max-width: 0px)');
        if (media.includes('forced-colors')) throw Error('Unsupported forced-color audit condition.');
        rule.media.mediaText = media;
      }
      if (rule.selectorText) rule.selectorText = rule.selectorText.replace(/:(focus-visible|focus-within|focus|hover|active)\b/g, (_,name) => ':is(:'+name+',[data-audit-'+name+'])');
      if (rule.cssRules) interactionRules(rule.cssRules);
    }
  }
  function inspector(doc) {
    const view=doc.defaultView, host=doc.querySelector('main'), canvas=doc.createElement('canvas'), ctx=canvas.getContext('2d'), colorCache=new Map();
    const colorProbe=doc.createElement('span');doc.body.append(colorProbe);
    function rgba(value) {
      if(colorCache.has(value))return colorCache.get(value);
      ctx.clearRect(0,0,1,1);ctx.fillStyle=value;ctx.fillRect(0,0,1,1);
      const result=[...ctx.getImageData(0,0,1,1).data].map(n=>n/255);colorCache.set(value,result);return result;
    }
    function over(top,bottom) { const a=top[3]+bottom[3]*(1-top[3]);return [0,1,2].map(i=>a?(top[i]*top[3]+bottom[i]*bottom[3]*(1-top[3]))/a:0).concat(a); }
    function luminance(c) { return c.slice(0,3).reduce((sum,n,i)=>sum+(n<=.04045?n/12.92:((n+.055)/1.055)**2.4)*[.2126,.7152,.0722][i],0); }
    function ratio(a,b) { const x=luminance(a),y=luminance(b);return (Math.max(x,y)+.05)/(Math.min(x,y)+.05); }
    function mix(a,b,t,space) {
      const value='color-mix(in '+space+', '+a+' '+((1-t)*100)+'%, '+b+' '+(t*100)+'%)';
      if(colorCache.has(value))return colorCache.get(value);
      colorProbe.style.color=value;
      const result=rgba(view.getComputedStyle(colorProbe).color);colorCache.set(value,result);return result;
    }
    function gradient(source,w,h) {
      const match=/^(repeating-)?linear-gradient\((.*)\)$/.exec(source);
      if(!match)throw Error('Unsupported contrast paint: '+source);
      const parts=split(match[2]);let head='',angle=Math.PI,space;
      if(!/^(rgba?|color)\(/.test(parts[0]))head=parts.shift();
      if(head){
        const interpolation=/\bin (srgb|oklab)\b/.exec(head);if(interpolation)space=interpolation[1];
        const direction=head.replace(/\bin (srgb|oklab)\b/,'').trim();
        if(direction==='to bottom'||!direction)angle=Math.PI;
        else if(direction==='to top')angle=0;
        else if(direction==='to right')angle=Math.PI/2;
        else if(direction==='to left')angle=3*Math.PI/2;
        else if(/^-?[\d.]+deg$/.test(direction))angle=parseFloat(direction)*Math.PI/180;
        else throw Error('Unsupported gradient direction: '+head);
      }
      // CSS Color 4 retains sRGB interpolation when every stop is legacy RGB.
      space ||= parts.every(part=>/^rgba?\(/.test(part))?'srgb':'oklab';
      const axis=value=>Math.abs(value)<1e-8?0:value,dx=axis(Math.sin(angle)),dy=axis(-Math.cos(angle)),length=Math.abs(w*dx)+Math.abs(h*dy),stops=[];
      for(const part of parts){
        const found=/^((?:rgba?|color)\([^)]*\))(.*)$/.exec(part);if(!found)throw Error('Unsupported gradient stop: '+part);
        const positions=found[2].trim().split(/\s+/).filter(Boolean);
        if(!positions.length)stops.push({color:found[1],position:null});
        else positions.forEach(position=>{if(!/^-?[\d.]+(?:px|%)$/.test(position))throw Error('Unsupported gradient position: '+position);stops.push({color:found[1],position:parseFloat(position)*(position.endsWith('%')?length/100:1)});});
      }
      stops[0].position ??= 0;stops[stops.length-1].position ??= length;
      let last=0;
      for(let i=1;i<stops.length;i++){
        if(stops[i].position===null)continue;
        stops[i].position=Math.max(stops[last].position,stops[i].position);
        for(let j=last+1;j<i;j++)stops[j].position=stops[last].position+(stops[i].position-stops[last].position)*(j-last)/(i-last);
        last=i;
      }
      return (x,y)=>{
        let point=(x-w/2)*dx+(y-h/2)*dy+length/2;
        if(match[1]){const start=stops[0].position,period=stops.at(-1).position-start;if(period<=0)throw Error('Unsupported zero-period gradient.');point=((point-start)%period+period)%period+start;}
        if(point<=stops[0].position)return rgba(stops[0].color);
        for(let i=1;i<stops.length;i++)if(point<stops[i].position){const a=stops[i-1],b=stops[i];return mix(a.color,b.color,(point-a.position)/(b.position-a.position),space);}
        return rgba(stops.at(-1).color);
      };
    }
    function paint(el) {
      const css=view.getComputedStyle(el),rect=el.getBoundingClientRect(),surface=rgba(view.getComputedStyle(host).backgroundColor);
      const backgrounds=css.backgroundImage==='none'?[]:split(css.backgroundImage);
      const origins=split(css.backgroundOrigin),sizes=split(css.backgroundSize),positions=split(css.backgroundPosition);
      const layers=backgrounds.map((value,i)=>{
        const size=sizes[i%sizes.length],position=positions[i%positions.length];
        if(!['auto','auto auto','100% 100%'].includes(size)||!['0% 0%','0px 0px'].includes(position))throw Error('Unsupported positioned contrast image.');
        const origin=origins[i%origins.length];let left=0,top=0,w=rect.width,h=rect.height;
        if(origin!=='border-box'){left=parseFloat(css.borderLeftWidth);top=parseFloat(css.borderTopWidth);w-=left+parseFloat(css.borderRightWidth);h-=top+parseFloat(css.borderBottomWidth);}
        if(origin==='content-box'){left+=parseFloat(css.paddingLeft);top+=parseFloat(css.paddingTop);w-=parseFloat(css.paddingLeft)+parseFloat(css.paddingRight);h-=parseFloat(css.paddingTop)+parseFloat(css.paddingBottom);}
        const sample=gradient(value,w,h);return (x,y)=>sample(x-left,y-top);
      }).reverse();
      const base=over(rgba(css.backgroundColor),surface);
      return {css,rect,at:(x,y)=>layers.reduce((color,layer)=>over(layer(x,y),color),base)};
    }
    function inspect(label,records) {
      const failures=[];
      for(const [name,tag,classes] of [['button','button','btn'],['primary action','button','btn btn-primary'],['destructive action','button','btn btn-danger'],['input','input','form-input'],['active title','header','win-head']]){
        const container=doc.createElement('section');container.className='win active';const control=doc.createElement(tag);control.className=classes;
        const text=doc.createElement('span');text.textContent='Sample';if(name==='active title')text.className='win-title';
        const baseline=doc.createElement('i');baseline.style.cssText='display:inline-block;width:0;height:0;vertical-align:baseline';
        if(tag==='input')control.value='Sample';else{text.append(baseline);control.append(text);}container.append(control);host.append(container);
        for(const state of name==='active title'?['rest']:['rest','hover','pressed','focus']){
          ['hover','active','focus','focus-visible','focus-within'].forEach(value=>control.removeAttribute('data-audit-'+value));
          if(state==='hover'||state==='pressed')control.setAttribute('data-audit-hover','');
          if(state==='pressed')control.setAttribute('data-audit-active','');
          if(state==='focus'){control.setAttribute('data-audit-focus','');control.setAttribute('data-audit-focus-visible','');}
          const result=paint(control),foreground=rgba(result.css.color);let bounds;
          if(tag==='input'){
            const left=parseFloat(result.css.borderLeftWidth)+parseFloat(result.css.paddingLeft),top=parseFloat(result.css.borderTopWidth)+parseFloat(result.css.paddingTop);
            bounds={left,top,right:result.rect.width-left,bottom:result.rect.height-top};
          }else{const range=doc.createRange();range.selectNodeContents(text.firstChild);const r=range.getBoundingClientRect();bounds={left:r.left-result.rect.left,top:r.top-result.rect.top,right:r.right-result.rect.left,bottom:r.bottom-result.rect.top};}
          let minimum=Infinity,samples=0;
          if(tag==='input'){
            for(let y=Math.ceil(bounds.top-.5)+.5;y<bounds.bottom;y++)for(let x=Math.ceil(bounds.left-.5)+.5;x<bounds.right;x++){const bg=result.at(x,y);minimum=Math.min(minimum,ratio(over(foreground,bg),bg));samples++;}
          }else{
            const mask=doc.createElement('canvas'),left=Math.floor(bounds.left),top=Math.floor(bounds.top),font=view.getComputedStyle(text);
            mask.width=Math.ceil(bounds.right)-left+2;mask.height=Math.ceil(bounds.bottom)-top+2;
            const glyph=mask.getContext('2d');glyph.font=font.font;glyph.textBaseline='alphabetic';glyph.fillStyle='#000';glyph.fontKerning=font.fontKerning;
            if('letterSpacing'in glyph&&font.letterSpacing!=='normal')glyph.letterSpacing=font.letterSpacing;
            glyph.fillText('Sample',bounds.left-left,baseline.getBoundingClientRect().top-result.rect.top-top);
            const pixels=glyph.getImageData(0,0,mask.width,mask.height).data;
            for(let y=0;y<mask.height;y++)for(let x=0;x<mask.width;x++)if(pixels[(y*mask.width+x)*4+3]>=230){const bg=result.at(left+x+.5,top+y+.5);minimum=Math.min(minimum,ratio(over(foreground,bg),bg));samples++;}
          }
          if(!Number.isFinite(minimum))throw Error('The contrast control has no text bounds.');
          const item={label,name,state,minimum,samples,bounds,color:result.css.color,image:result.css.backgroundImage};records.push(item);
          if(minimum<4.5)failures.push(label+' '+name+' '+state+' text needs 4.5:1 ('+minimum.toFixed(2)+':1)');
          if(name==='input'){const edge=rgba(result.css.borderTopColor),inside=result.at(result.rect.width/2,parseFloat(result.css.borderTopWidth)+.5),n=ratio(over(edge,inside),inside);if(n<3)failures.push(label+' input '+state+' boundary needs 3:1 ('+n.toFixed(2)+':1)');}
        }
        container.remove();
      }
      return failures;
    }
    return inspect;
  }
  async function check(theme,records) {
    theme=JSON.parse(JSON.stringify(theme));
    const choices=dinahAppearance.snapshot().skins,available=pudlSkins.available(),key=JSON.stringify([theme,choices,sheets(),available]);
    const missing=choices.filter(skin=>skin.id!=='standard'&&!pudlSkins.available().includes(skin.id));

    if(!records&&cache.has(key))return cache.get(key);
    const job=(async()=>{
      const frame=document.createElement('iframe');frame.title='Skin contrast validation';frame.tabIndex=-1;frame.setAttribute('aria-hidden','true');frame.style.cssText='position:fixed;left:-12000px;top:0;width:1024px;height:768px;visibility:hidden;pointer-events:none';
      try{
        const loaded=new Promise((resolve,reject)=>{frame.onload=resolve;frame.onerror=()=>reject(Error('Skin contrast document could not load.'));});frame.srcdoc=markup();document.body.append(frame);await loaded;
        const doc=frame.contentDocument;if(doc.documentElement.clientWidth<=0)throw Error('The audit viewport has no width.');for(const sheet of doc.styleSheets)interactionRules(sheet.cssRules);
        doc.documentElement.style.cssText=document.documentElement.style.cssText;
        for(const [role,token]of [['interface','font'],['heading','font-display'],['reading','reading-font'],['monospace','mono']])doc.documentElement.style.setProperty('--'+token,dinahAppearance.font(theme.typography[role],role));
        const fontSample=doc.createElement('span');fontSample.textContent='Sample';doc.body.append(fontSample);await doc.fonts.ready;fontSample.remove();
        const inspect=inspector(doc),failures=missing.map(skin=>skin.label+' is unavailable for contrast checks.'),detail=records||[];
        for(const mode of ['light','dark']){doc.documentElement.dataset.theme=mode;Object.entries(theme.palettes[mode]).forEach(([token,value])=>doc.documentElement.style.setProperty('--'+token,value));
          for(const skin of choices.filter(skin=>!missing.includes(skin))){doc.documentElement.dataset.skin=skin.id;failures.push(...inspect(skin.label+' '+mode,detail));}
        }
        return failures;
      }finally{frame.remove();}
    })();
    if(!records){cache.set(key,job);if(cache.size>32)cache.delete(cache.keys().next().value);}
    try{return await job;}catch(error){cache.delete(key);throw error;}
  }
  window.dinahSkinContrast={check,links};
})();
