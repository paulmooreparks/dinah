// Build Clear Chime from its reviewed palette using the existing contrast solver.
const fs = require('node:fs');
const vm = require('node:vm');
const path = require('node:path');
const root = path.resolve(__dirname, '../../..');
const source = fs.readFileSync(path.join(__dirname, 'make-themes.mjs'), 'utf8');
const solver = source.slice(source.indexOf('const rgb ='), source.indexOf('const S ='));
const libraryPath = path.join(root, 'cmd/dinah-desktop/frontend/theme-data.js');
const context = {window:{}, console};
vm.runInNewContext(fs.readFileSync(libraryPath,'utf8'),context);
const doc = JSON.parse(JSON.stringify(context.window.dinahThemeBuiltins.mineral));
doc.name = 'Clear Chime';
Object.assign(doc.palettes.light, {
 bg:'#dce7e3',surface:'#faf8f1','surface-alt':'#e8eee8',text:'#243c3c','text-muted':'#526967',border:'#829590',
 accent:'#005b5d','accent-hover':'#004c4f','on-accent':'#ffffff','tb-bg':'#164c4e','tb-fg':'#fffaf0',
 light:'#fffdf5',shade:'#153d3b',lit:'48%',depth:'16%',positive:'#287145',warn:'#825b0b','on-warn':'#ffffff',danger:'#b03538','danger-hover':'#90282d'
});
Object.assign(doc.palettes.dark, {
 bg:'#1b2225',surface:'#282f32','surface-alt':'#343e41',text:'#eeefeb','text-muted':'#bdc7c6',border:'#839390',
 accent:'#83c9c2','accent-hover':'#a4d9d3','on-accent':'#192f31','tb-bg':'#263b3e','tb-fg':'#f3f2eb',
 light:'#f0f4f2',shade:'#101416',lit:'9%',depth:'50%',positive:'#9ed7ab',warn:'#f0cc79','on-warn':'#26332c',danger:'#ffa5a0','danger-hover':'#ffc0ba'
});
for(const mode of ['light','dark']){
 const p=doc.palettes[mode];
 for(const [key,role] of Object.entries({'dinah-title':'text','syntax-heading':'text','syntax-comment':'text-muted','syntax-link':'accent','syntax-name':'accent','syntax-string':'positive','syntax-number':'warn','syntax-attr':'warn','syntax-error':'danger'}))p[key]=p[role];
 for(const key of Object.keys(p))if(key.startsWith('category-'))delete p[key];
 const scope={p,mode,console};
 vm.runInNewContext(solver+'console.log(mode, settle(p, mode));',scope);
}
fs.writeFileSync(path.join(root,'build/clear-chime/candidate.json'),JSON.stringify(doc,null,2)+'\n');
