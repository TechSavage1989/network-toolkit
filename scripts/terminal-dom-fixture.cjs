// Small DOM fixture for exercising the real console-control renderer in Node.
'use strict';
class Element {
 constructor(tag='div',attrs={}){this.tagName=tag.toUpperCase();this.attrs={...attrs};this.children=[];this.parentElement=null;this.disabled=false;this.value='';this.checked=false;this.style={};this.listeners={};this.dataset=new Proxy({}, {get:(_,k)=>this.attrs['data-'+k.replace(/[A-Z]/g,c=>'-'+c.toLowerCase())],set:(_,k,v)=>{this.attrs['data-'+k.replace(/[A-Z]/g,c=>'-'+c.toLowerCase())]=String(v);return true;}});this.classList={contains:n=>this.className.split(/\s+/).includes(n),add:(...ns)=>{this.className=[...new Set([...this.className.split(/\s+/).filter(Boolean),...ns])].join(' ');},remove:(...ns)=>{this.className=this.className.split(/\s+/).filter(n=>!ns.includes(n)).join(' ');},toggle:(n,force)=>{const has=this.classList.contains(n),next=force===undefined?!has:force;next?this.classList.add(n):this.classList.remove(n);return next;}};}
 get id(){return this.attrs.id||'';}set id(v){this.attrs.id=v;}
 get className(){return this.attrs.class||'';}set className(v){this.attrs.class=v;}
 get textContent(){return this.tagName==='#TEXT'?this.text||'':this.children.map(c=>c.textContent).join('');}set textContent(text){this.children=[];const child=new Element('#text');child.text=text;this.append(child);}
 get innerHTML(){return this.children.map(c=>c.outerHTML).join('');}set innerHTML(html){this.children=[];for(const child of parse(html))this.append(child);}
 get outerHTML(){if(this.tagName==='#TEXT')return this.text||'';return '<'+this.tagName.toLowerCase()+Object.entries(this.attrs).map(([k,v])=>' '+k+'="'+v+'"').join('')+'>'+this.innerHTML+'</'+this.tagName.toLowerCase()+'>';}
 append(...nodes){for(const node of nodes){node.remove();node.parentElement=this;this.children.push(node);}}
 after(node){const p=this.parentElement;node.remove();node.parentElement=p;p.children.splice(p.children.indexOf(this)+1,0,node);}
 before(node){const p=this.parentElement;node.remove();node.parentElement=p;p.children.splice(p.children.indexOf(this),0,node);}
 remove(){if(this.parentElement){const p=this.parentElement;p.children.splice(p.children.indexOf(this),1);this.parentElement=null;}}
 insertAdjacentHTML(where,html){const nodes=parse(html);if(where==='beforebegin'){for(const node of nodes)this.before(node);}else if(where==='afterbegin'){for(const node of nodes.reverse()){node.parentElement=this;this.children.unshift(node);}}else this.append(...nodes);}
 setAttribute(k,v){this.attrs[k]=String(v);}getAttribute(k){return this.attrs[k]??null;}
 hasAttribute(k){return k in this.attrs;}removeAttribute(k){delete this.attrs[k];}
 contains(node){for(let at=node;at;at=at.parentElement)if(at===this)return true;return false;}
 get isConnected(){let at=this;while(at.parentElement)at=at.parentElement;return at.tagName==='DOCUMENT';}
 get firstElementChild(){return this.children.find(child=>child.tagName!=='#TEXT')||null;}
 matches(selector){if(selector.includes(','))return selector.split(',').some(s=>this.matches(s.trim()));const parts=selector.trim().split(/\s+/);if(parts.length>1){if(!this.matches(parts.pop()))return false;let parent=this.parentElement;while(parent){if(parent.matches(parts.join(' ')))return true;parent=parent.parentElement;}return false;}return (selector.match(/(?:^[\w-]+|#[\w-]+|\.[\w-]+|\[[^\]]+\])/g)||[]).every(s=>{if(s[0]==='#')return this.id===s.slice(1);if(s[0]==='.')return this.classList.contains(s.slice(1));if(s[0]==='['){const m=s.match(/^\[([^=\]]+)(?:="([^"]*)")?\]$/);return m&&m[1] in this.attrs&&(m[2]===undefined||this.attrs[m[1]]===m[2]);}return this.tagName.toLowerCase()===s.toLowerCase();});}
 querySelectorAll(selector){const found=[];for(const child of this.children){if(child.tagName!=='#TEXT'&&child.matches(selector))found.push(child);found.push(...child.querySelectorAll(selector));}return found;}
 querySelector(selector){return this.querySelectorAll(selector)[0]||null;}getElementById(id){return this.querySelector('#'+id);}
 closest(selector){let node=this;while(node){if(node.matches(selector))return node;node=node.parentElement;}return null;}
 addEventListener(type,fn){(this.listeners[type]||=[]).push(fn);}removeEventListener(){}
 emit(type,target,properties={}){const event={target,preventDefault(){},stopPropagation(){},stopImmediatePropagation(){},...properties};for(const fn of this.listeners[type]||[])fn(event);}
 setCustomValidity(message){this.validationMessage=message;}
 focus(){}getBoundingClientRect(){return {left:0,right:100,top:0,bottom:30};}
 select(){this.selectionStart=0;this.selectionEnd=this.value.length;}
 setSelectionRange(start,end){this.selectionStart=start;this.selectionEnd=end;}
}
function parse(html){const fragment=new Element('fragment'),stack=[fragment];for(const token of html.match(/<[^>]*>|[^<]+/g)||[]){if(token.startsWith('</')){stack.pop();continue;}if(token.startsWith('<')){const tag=token.match(/^<([\w-]+)/)?.[1];if(!tag)continue;const attrs={};for(const m of token.slice(tag.length+1,-1).matchAll(/([^\s=]+)(?:="([^"]*)"|'([^']*)'|=([^\s]+))?/g))attrs[m[1]]=m[2]??m[3]??m[4]??'';const node=new Element(tag,attrs);stack.at(-1).append(node);if(!/^(input|br|hr|meta|link|img)$/i.test(tag)&&!token.endsWith('/>'))stack.push(node);}else{const node=new Element('#text');node.text=token;stack.at(-1).append(node);}}return [...fragment.children];}
function document(html){const doc=new Element('document');doc.innerHTML=html;doc.createElement=tag=>new Element(tag);return doc;}
module.exports={Element,document};
