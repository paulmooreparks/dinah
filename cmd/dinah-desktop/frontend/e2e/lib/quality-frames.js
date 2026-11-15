const fs = require('fs'), path = require('path');
const { OUT } = require('./page');
// Like decisions-answer-stability, retain a baseline and bounds from rAF.
// Also retain intervening frames while asynchronous bridge replies arrive.
async function measure(t, name, roots, action, ready) {
  const p = t.page;
  await p.evaluate(async roots => {
    await new Promise(requestAnimationFrame); await new Promise(requestAnimationFrame);
    function key(el) {
      if (el.id) return '#' + el.id;
      for (const attr of ['data-win', 'data-copy-id', 'data-notice-id', 'data-arriving']) {
        if (el.hasAttribute(attr)) return '[' + attr + '="' + el.getAttribute(attr) + '"]';
      }
      if (!el.parentElement) return el.tagName;
      return key(el.parentElement) + '>' + el.tagName.toLowerCase() + ':nth-child(' + ([...el.parentElement.children].indexOf(el) + 1) + ')';
    }
    function sample() {
      const result = {};
      const els = [document.body, ...document.body.querySelectorAll('*')];
      for (const e of els) {
        if (!e.checkVisibility({ checkOpacity: true, checkVisibilityCSS: true, contentVisibilityAuto: true })) continue;
        const r = e.getBoundingClientRect();
        if (!r.width || !r.height || r.bottom <= 0 || r.right <= 0 || r.top >= innerHeight || r.left >= innerWidth) continue;
        let clipped = false;
        for (let a = e.parentElement; a; a = a.parentElement) {
          const s = getComputedStyle(a), b = a.getBoundingClientRect();
          if (/(auto|scroll|hidden|clip)/.test(s.overflowY) && (r.bottom <= b.top || r.top >= b.bottom)) clipped = true;
          if (/(auto|scroll|hidden|clip)/.test(s.overflowX) && (r.right <= b.left || r.left >= b.right)) clipped = true;
        }
        if (!clipped) result[key(e)] = { box: [r.x,r.y,r.width,r.height], text: (e.getAttribute('aria-label') || e.innerText || '').slice(0,100) };
      }
      return result;
    }
    window.__qualityFrames = { before: sample(), frames: [], sample, active: true };
    function tick() { const q = __qualityFrames; if (q.active) { q.frames.push(q.sample()); requestAnimationFrame(tick); } }
    requestAnimationFrame(tick);
  }, roots);
  await action();
  if (ready) await ready();
  const result = await p.evaluate(async () => {
    const q = __qualityFrames; q.afterTransition = q.frames.length;
    for (let n=0;n<8;n++) await new Promise(requestAnimationFrame);
    q.active = false;
    return { scope:'all CSS-visible, viewport-intersecting page elements after overflow clipping', before:q.before, frames:q.frames, afterTransition:q.afterTransition };
  });
  result.name = name;
  result.changes = result.frames.map((frame, i) => {
    const previous = i ? result.frames[i-1] : result.before;
    return { frame:i+1, moved:Object.keys(frame).filter(k => previous[k] && frame[k].box.some((v,n) => Math.abs(v-previous[k].box[n]) >= 1)).map(k => ({key:k,from:previous[k].box,to:frame[k].box,text:frame[k].text})), added:Object.keys(frame).filter(k=>!previous[k]), removed:Object.keys(previous).filter(k=>!frame[k]) };
  });
  fs.mkdirSync(OUT,{recursive:true}); fs.writeFileSync(path.join(OUT,name+'.json'),JSON.stringify(result,null,2));
  console.log('MEASURE '+name+' '+JSON.stringify(result.changes.filter(c=>c.moved.length).map(c=>({frame:c.frame,moved:c.moved.length}))));
  t.ok(name+' captured eight frames after the observed transition',result.frames.length-result.afterTransition>=8);
  await t.shot(name);
  return result;
}
module.exports = { measure };
