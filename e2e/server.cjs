// Each browser run owns its upload directory, including legacy site pointers.
// Never mutate the repository's development media while exercising real uploads.
const fs=require('node:fs');
const os=require('node:os');
const path=require('node:path');
const {spawn,spawnSync}=require('node:child_process');
const root=path.resolve(__dirname,'..');
const sandbox=fs.mkdtempSync(path.join(os.tmpdir(),'degosalon-browser-'));
process.on('exit',()=>fs.rmSync(sandbox,{recursive:true,force:true}));
fs.cpSync(path.join(root,'templates'),path.join(sandbox,'templates'),{recursive:true});
fs.symlinkSync(path.join(root,'static'),path.join(sandbox,'static'),'dir');
const binary=path.join(sandbox,'app');
const built=spawnSync('go',['build','-o',binary,'.'],{cwd:root,stdio:'inherit'});
if(built.status!==0)process.exit(built.status || 1);
const app=spawn(binary,[],{cwd:sandbox,env:process.env,stdio:'inherit'});
for(const signal of ['SIGTERM','SIGINT'])process.on(signal,()=>app.kill(signal));
app.on('error',error=>{console.error(error.message);process.exit(1);});
app.on('exit',(code,signal)=>process.exit(code ?? (signal ? 0 : 1)));
