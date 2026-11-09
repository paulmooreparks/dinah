// Copyright 2026 Paul Parks. SPDX-License-Identifier: Apache-2.0
import fs from 'node:fs/promises';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { createRequire } from 'node:module';
import { execFileSync } from 'node:child_process';
import crypto from 'node:crypto';
const here=path.dirname(fileURLToPath(import.meta.url)),root=process.env.YAVC_OFFICE_BUILD_ROOT;
if(!root||!path.isAbsolute(root))throw new Error('Set YAVC_OFFICE_BUILD_ROOT to an isolated absolute build directory.');
await fs.mkdir(root,{recursive:true});
for(const name of ['package.json','package-lock.json','entry.cjs'])await fs.copyFile(path.join(here,name),path.join(root,name));
if(process.platform==='win32')execFileSync(process.env.ComSpec||'cmd.exe',['/d','/s','/c','npm ci --ignore-scripts --no-fund'],{cwd:root,stdio:'inherit'});
else execFileSync('npm',['ci','--ignore-scripts','--no-fund'],{cwd:root,stdio:'inherit'});
const require=createRequire(path.join(root,'package.json')),esbuild=require('esbuild');
const built=await esbuild.build({entryPoints:[path.join(root,'entry.cjs')],bundle:true,platform:'browser',format:'iife',target:['es2020'],minify:true,legalComments:'eof',write:false});
const output=built.outputFiles[0].contents,destination=path.join(here,'../vendor/mammoth-1.13.0/office.js');
if(process.argv.includes('--check')){if(!Buffer.from(output).equals(await fs.readFile(destination)))throw new Error('The Office import bundle differs from pinned inputs.');}
else await fs.writeFile(destination,output);
const hash=bytes=>crypto.createHash('sha256').update(bytes).digest('hex'),inputs={};
for(const name of ['package.json','package-lock.json','entry.cjs','build.mjs'])inputs[name]=hash(await fs.readFile(path.join(here,name)));
const manifest=JSON.stringify({inputs:inputs,bundle:hash(output)},null,2)+'\n',manifestPath=path.join(here,'manifest.json');
if(process.argv.includes('--check')){if(manifest!==await fs.readFile(manifestPath,'utf8'))throw new Error('The Office import provenance manifest differs.');}
else await fs.writeFile(manifestPath,manifest);
console.log('Office import bundle SHA256 '+crypto.createHash('sha256').update(output).digest('hex'));
