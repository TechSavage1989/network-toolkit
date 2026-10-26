// Rebuild the offline editor libraries from verified npm package directories.
// Usage: node scripts/build_editor_assets.cjs /path/to/prismjs/package /path/to/prettier/package
'use strict';
const fs=require('node:fs'),path=require('node:path'),vm=require('node:vm'),crypto=require('node:crypto');
const [prism,prettier]=process.argv.slice(2);
if(!prism||!prettier)throw Error('Provide extracted prismjs and prettier package directories.');
const output=path.resolve(__dirname,'../web/assets/editor-vendor');fs.mkdirSync(output,{recursive:true});
const metadata=require(path.resolve(prism,'components.json')),getLoader=require(path.resolve(prism,'dependencies.js'));
const ids=Object.keys(metadata.languages).filter(id=>id!=='meta'),order=getLoader(metadata,ids).getIds();
const banner='/* Prism 1.30.0 - MIT. See prism-LICENSE.txt. Offline grammars, no plugins or autoloading. */\n';
const core='var Prism = {manual:true, disableWorkerMessageHandler:true};\n'+fs.readFileSync(path.join(prism,'components/prism-core.min.js'),'utf8');
const bundle=banner+core+'\n'+order.map(id=>fs.readFileSync(path.join(prism,'components/prism-'+id+'.min.js'),'utf8')).join('\n');
fs.writeFileSync(path.join(output,'prism.js'),bundle);
const context={};context.self=context;vm.createContext(context);vm.runInContext(bundle,context);
const languages=ids.filter(id=>context.Prism.languages[id]).map(id=>{const m=metadata.languages[id];return {id,label:typeof m==='string'?m:m.title,aliases:[m.alias||[]].flat()};});
fs.writeFileSync(path.join(output,'languages.js'),'/* Generated from Prism 1.30.0 components.json; MIT. */\nglobalThis.NTKEditorLanguageData = '+JSON.stringify(languages)+';\n');
fs.copyFileSync(path.join(prism,'LICENSE'),path.join(output,'prism-LICENSE.txt'));
fs.copyFileSync(path.join(prettier,'LICENSE'),path.join(output,'prettier-LICENSE.txt'));
fs.copyFileSync(path.join(prettier,'THIRD-PARTY-NOTICES.md'),path.join(output,'prettier-NOTICES.txt'));
fs.copyFileSync(path.join(prettier,'standalone.js'),path.join(output,'prettier.js'));
for(const id of ['babel','estree','typescript','html','postcss','markdown','yaml','graphql'])fs.copyFileSync(path.join(prettier,'plugins/'+id+'.js'),path.join(output,'prettier-'+id+'.js'));
const packages=[prism,prettier].map(directory=>JSON.parse(fs.readFileSync(path.join(directory,'../provenance.json'),'utf8')));
const files=Object.fromEntries(fs.readdirSync(output).filter(name=>name!=='SOURCE.json').sort().map(name=>[name,crypto.createHash('sha256').update(fs.readFileSync(path.join(output,name))).digest('hex')]));
fs.writeFileSync(path.join(output,'SOURCE.json'),JSON.stringify({packages,grammars:languages.length,reference:'https://github.com/notepad-plus-plus/npp-usermanual/blob/master/content/docs/programing-languages.md',files},null,2)+'\n');
console.log('Bundled '+languages.length+' offline language grammars and Prettier '+JSON.parse(fs.readFileSync(path.join(prettier,'package.json'))).version+'.');
