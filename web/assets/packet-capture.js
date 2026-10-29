(function(root){
'use strict';
const model={capabilities:null,captures:[],selected:'',newCapture:false,page:0,limit:300,windowStart:0,scrollTop:0,overview:null,overviewSignature:"",windowSignature:"",layoutSignature:"",selectedIndex:-1,pendingIndex:null,insightsOpen:true,guideOpen:false,appearanceOpen:false,granularDragging:false,overviewDragging:false,detailMapRange:"",suppressMapClick:false,mapExpectedTop:-1,overviewMapExpectedTop:-1,windowToken:0,sort:'number',direction:'asc',expression:'',query:'',protocol:'any',view:'troubleshooting',goal:'general',report:null,reportStamp:'',detail:null,paused:false,follow:true,severityFilter:'any',typeFilter:'any',statFilter:'',newestIndex:-1,inspectPointer:false,inspectFocus:false,busy:false,refreshQueued:false,refreshQueuedForce:false,refreshPromise:null,timer:null,sequence:0,loaded:[],total:0,interfaces:[],interfacesInitialized:false,interfacesLoading:false,interfacesPromise:null,startPending:false,range:null,detailRequest:0,disclosures:{setup:true,advanced:false,display:false,glossary:false},setupManual:false};
// Cache a few neighboring decoded windows; the packet bar and inspectors never
// need to be recreated just because the virtual row window advances.
const packetWindowCache=new Map();
const packetWindowInFlight=new Map();
const packetCacheLimit=9;
let lastOverviewFetch=0;
let overviewInFlight=null;
const detailDefaults={width:360,height:260,hidden:false,dock:'auto'};
// User-editable packet palette and map settings are saved with the regular app state.
// Validate all restored values before using them in CSS or the rendering geometry.
const packetPaletteDefaults=Object.freeze({
 error:'#ff0000',warn:'#ffff00',note:'#0000ff',other:'#b8c7d6',
 tcp:'#a7c6e9',udp:'#b6d9df',dns:'#b8a5d9',dhcp:'#e1c691',arp:'#e1c691',
 icmp:'#8dcfce',icmpv6:'#8dcfce',tls:'#beb8dc',http:'#a7d9af',ssh:'#beb8dc',
 mdns:'#bea6d5',smb:'#aacabb',ntp:'#b8ccc0',wireguard:'#96aedb',lldp:'#b7d5bf',cdp:'#c1d2b3'
});
const legacyEventColors=Object.freeze({error:'#be3f48',warn:'#dfa344',note:'#66b9cf'});
const packetPaletteGroups=[
 ['Events',[['error','Error'],['warn','Warning'],['note','Note']]],
 ['Protocols',[['tcp','TCP'],['udp','UDP'],['dns','DNS'],['dhcp','DHCP'],['arp','ARP'],['icmp','ICMP'],['icmpv6','ICMPv6'],['tls','TLS'],['http','HTTP'],['ssh','SSH'],['mdns','mDNS'],['smb','SMB'],['ntp','NTP'],['wireguard','WireGuard'],['lldp','LLDP'],['cdp','CDP'],['other','Other / unknown']]]
];
const packetMapDefaults=Object.freeze({lineHeight:1,overviewScale:'fit',overviewWidth:12,detailWidth:15,rowStyle:'marker',mapColors:'mixed',showOverview:true,showDetail:true});
const packetChoice=(value,allowed,fallback)=>allowed.includes(value)?value:fallback;
function defaultMapSettings(){return {...packetMapDefaults,colors:{...packetPaletteDefaults}};}
function normalizeMapSettings(value){
 const v=value&&typeof value==='object'&&!Array.isArray(value)?value:{},colors=v.colors&&typeof v.colors==='object'&&!Array.isArray(v.colors)?v.colors:{};
 return {
  lineHeight:packetChoice(v.lineHeight,[1,2,3,4],packetMapDefaults.lineHeight),
  overviewScale:packetChoice(v.overviewScale,['fit',1,2,3,4],packetMapDefaults.overviewScale),
  overviewWidth:packetChoice(v.overviewWidth,[8,12,16,20,24],packetMapDefaults.overviewWidth),
  detailWidth:packetChoice(v.detailWidth,[10,15,18,24,32],packetMapDefaults.detailWidth),
  rowStyle:packetChoice(v.rowStyle,['off','marker','subtle','strong','full'],packetMapDefaults.rowStyle),
  mapColors:packetChoice(v.mapColors,['mixed','protocols','alerts'],packetMapDefaults.mapColors),
  showOverview:v.showOverview!==false,
  showDetail:v.showDetail!==false,
  colors:Object.fromEntries(Object.entries(packetPaletteDefaults).map(([key,defaultColor])=>{const stored=typeof colors[key]==='string'&&/^#[0-9a-f]{6}$/i.test(colors[key])?colors[key].toLowerCase():defaultColor;return [key,legacyEventColors[key]===stored?defaultColor:stored];}))
 };
}
function mapSettings(){if(!state.packetMapSettings)state.packetMapSettings=defaultMapSettings();return state.packetMapSettings;}
function mapLineHeight(){return mapSettings().lineHeight;}
function overviewLineHeight(){const scale=mapSettings().overviewScale;return scale==='fit'?0:scale;}
function contrastText(color){const h=color.slice(1),channels=[0,2,4].map(i=>parseInt(h.slice(i,i+2),16)/255).map(x=>x<=.04045?x/12.92:((x+.055)/1.055)**2.4);return .2126*channels[0]+.7152*channels[1]+.0722*channels[2]>.179?'#101820':'#ffffff';}
function applyPacketAppearance(){
 const workspace=node('packetCaptureWorkspace');if(!workspace)return;
 const settings=mapSettings(),style=workspace.style;
 const put=(name,value)=>{if(typeof style.setProperty==='function')style.setProperty(name,value);else style[name]=value;};
 for(const [key,value] of Object.entries(settings.colors))put('--packet-color-'+key,value);
 for(const [key,value] of Object.entries(settings.colors))put('--packet-contrast-'+key,contrastText(value));
 put('--packet-map-line-height',settings.lineHeight+'px');put('--packet-map-overview-line-height',(overviewLineHeight()||1)+'px');put('--packet-map-overview-width',settings.overviewWidth+'px');put('--packet-map-detail-width',settings.detailWidth+'px');
 for(const row of ['off','marker','subtle','strong','full'])workspace.classList.toggle('packet-row-mode-'+row,row===settings.rowStyle);
 for(const mode of ['mixed','protocols','alerts'])workspace.classList.toggle('packet-map-mode-'+mode,mode===settings.mapColors);
 const overview=node('packetOverviewMap'),fine=node('packetDetailMap');
 if(overview)overview.hidden=!settings.showOverview;
 if(fine)fine.hidden=!settings.showDetail;
}
let packetColorDialog=null;
function packetColorControl(key,label,value){const picker=root.NetworkToolkitColorPicker;return picker?picker.buttonHTML({id:'packet-color-'+key,label:label+' packet color',value}).replace('<button ','<button data-packet-color="'+key+'" '):'<input type="color" data-packet-color="'+key+'" aria-label="'+label+' packet color" value="'+value+'">';}
function openPacketColor(button){const key=button.dataset.packetColor,picker=root.NetworkToolkitColorPicker;if(!picker||!Object.hasOwn(packetPaletteDefaults,key))return;
 const preview=value=>{const style=node('packetCaptureWorkspace')?.style;if(!style)return;for(const [name,v]of [['--packet-color-'+key,value],['--packet-contrast-'+key,contrastText(value)]]){if(style.setProperty)style.setProperty(name,v);else style[name]=v;}};
 packetColorDialog=picker.open({title:'Packet '+key+' color',value:mapSettings().colors[key],defaultValue:packetPaletteDefaults[key],allowAlpha:false,
  onPreview:preview,onCancel:()=>{packetColorDialog=null;applyPacketAppearance();},onApply:value=>{packetColorDialog=null;updateAppearanceSetting({dataset:{packetColor:key},value});const current=document.querySelector('button[data-packet-color="'+key+'"]');if(current)picker.syncButton(current,mapSettings().colors[key]);}});
}
function appearancePanelHTML(){const s=mapSettings();return '<aside id="packetAppearanceSettings" class="packet-appearance-settings" '+(!model.appearanceOpen?'hidden':'')+' aria-label="Packet color and map appearance settings"><header><strong>Packet colors & maps</strong><button type="button" class="iconbtn" data-packet-action="close-appearance" aria-label="Close packet appearance settings" title="Close">×</button></header><div class="packet-appearance-scroll"><div class="packet-appearance-controls">'+
 '<label>Packet row coloring<select id="packetRowStyle" data-packet-appearance="rowStyle">'+options([['off','Off'],['marker','Color marker only'],['subtle','Subtle row tint'],['strong','Strong row tint'],['full','Full strength (100% color)']],s.rowStyle)+'</select></label>'+
 '<label>Map colors<select id="packetMapColorMode" data-packet-appearance="mapColors">'+options([['mixed','Protocol + alerts'],['protocols','Protocols only'],['alerts','Alerts only']],s.mapColors)+'</select></label>'+
 '<label class="packet-map-visible-toggle"><input type="checkbox" data-packet-visibility="showOverview" '+(s.showOverview?'checked':'')+'> Show full map</label>'+'<label class="packet-map-visible-toggle"><input type="checkbox" data-packet-visibility="showDetail" '+(s.showDetail?'checked':'')+'> Show fine map</label>'+'<label>Full map width<select id="packetOverviewWidth" data-packet-appearance="overviewWidth">'+options([[8,'8 px'],[12,'12 px'],[16,'16 px'],[20,'20 px'],[24,'24 px']],s.overviewWidth)+'</select></label>'+
 '<label>Full map pixels per packet<select id="packetOverviewScale" data-packet-appearance="overviewScale">'+options([['fit','Fit entire capture (default)'],[1,'1 px / packet'],[2,'2 px / packet'],[3,'3 px / packet'],[4,'4 px / packet']],s.overviewScale)+'</select></label>'+
 '<label>Detail map width<select id="packetDetailWidth" data-packet-appearance="detailWidth">'+options([[10,'10 px'],[15,'15 px'],[18,'18 px'],[24,'24 px'],[32,'32 px']],s.detailWidth)+'</select></label>'+
 '<label>Detail pixels per packet<select id="packetMapLineHeight" data-packet-appearance="lineHeight">'+options([[1,'1 px (default)'],[2,'2 px'],[3,'3 px'],[4,'4 px']],s.lineHeight)+'</select></label></div>'+
 '<p class="packet-appearance-hint">Both maps support exact packet lines at 1-4 px per packet. Full map Fit mode compresses the entire capture for quick navigation. Alert colors take priority unless you select Protocols only.</p>'+
 packetPaletteGroups.map(([group,colors])=>'<section class="packet-appearance-group"><h3>'+group+'</h3><div class="packet-appearance-palette">'+colors.map(([key,label])=>'<label>'+packetColorControl(key,label,s.colors[key])+'<span>'+label+'</span></label>').join('')+'</div></section>').join('')+
 '</div><footer><button class="btn small ghost" type="button" data-packet-action="reset-appearance">Restore default colors & maps</button><span>Saved automatically</span></footer></aside>';}
function toggleAppearance(open){model.appearanceOpen=open===undefined?!model.appearanceOpen:!!open;if(!model.appearanceOpen)packetColorDialog?.close();const panel=node('packetAppearanceSettings');if(panel)panel.hidden=!model.appearanceOpen;const button=document.querySelector('[data-packet-action="toggle-appearance"]');if(button)button.setAttribute('aria-expanded',String(model.appearanceOpen));if(model.appearanceOpen){model.guideOpen=false;const guide=node('packetQuickGuide');if(guide)guide.hidden=true;const toggle=document.querySelector('[data-packet-action="toggle-guide"]');if(toggle)toggle.setAttribute('aria-expanded','false');}}
function updateAppearanceSetting(element){
 const settings=mapSettings(),key=element.dataset?.packetColor,field=element.dataset?.packetAppearance,visibility=element.dataset?.packetVisibility;
 if(visibility&&['showOverview','showDetail'].includes(visibility)){
  settings[visibility]=element.checked;
 }else if(key&&Object.hasOwn(packetPaletteDefaults,key)){
  if(!/^#[0-9a-f]{6}$/i.test(element.value))return;settings.colors[key]=element.value.toLowerCase();
 }else if(field&&Object.hasOwn(packetMapDefaults,field)){
  const value=['lineHeight','overviewWidth','detailWidth'].includes(field)||field==='overviewScale'&&element.value!=='fit'?Number(element.value):element.value;
  const clean=normalizeMapSettings({...settings,[field]:value});settings[field]=clean[field];
 }else return;
 applyPacketAppearance();
 if(visibility){updateMapViewport();requestPacketOverview();}
 if(field==='overviewScale'){updateOverviewScale(true);updateMapViewport();}
 if(field==='lineHeight'){
  const lane=node('packetDetailMap');if(lane){lane._mapRange='';const content=node('packetDetailMapContent');if(content)content.style.height=Math.max(1,model.total*mapLineHeight())+'px';updateMapViewport();renderPacketDetailMap(true);}
 }
 persistDetails();
}
let detailObserver,observedAnalysis,packetMenu,packetMenuContext;
const clamp=(value,min,max)=>Math.max(min,Math.min(max,value));
function mergePreferences(saved){const graph=saved?.packetGraphPreferences;state.packetGraphPreferences={...state.packetGraphPreferences,height:Number.isFinite(Number(graph?.height))?clamp(Number(graph.height),100,600):230,collapsed:graph?.collapsed===true,zoom:Number.isFinite(Number(graph?.zoom))&&Number(graph.zoom)>0?clamp(Number(graph.zoom),1e-18,1e24):1};const source=saved?.packetDetails,value=source&&typeof source==='object'&&!Array.isArray(source)?source:{};state.packetDetails={width:Number.isInteger(value.width)?clamp(value.width,260,700):detailDefaults.width,height:Number.isInteger(value.height)?clamp(value.height,80,650):detailDefaults.height,hidden:value.hidden===true,dock:['auto','right','bottom'].includes(value.dock)?value.dock:'auto'};state.packetMapSettings=normalizeMapSettings(saved?.packetMapSettings);layoutDetails();applyPacketAppearance();}
function preferences(){if(!state.packetDetails)mergePreferences({});return state.packetDetails;}
function persistDetails(){if(typeof saveState==='function')saveState();}
function disclosureKey(details){return details?.id==='packetCaptureSetup'?'setup':details?.id==='packetAdvancedCapture'?'advanced':details?.classList.contains('packet-display-options')?'display':details?.classList.contains('packet-glossary')?'glossary':'';}
function rememberDisclosures(){for(const details of document.querySelectorAll('#packetCaptureWorkspace details')){const key=disclosureKey(details);if(key)model.disclosures[key]=details.hasAttribute('open');}}
function restoreDisclosures(container=node('packetCaptureWorkspace')){if(!container)return;for(const details of container.querySelectorAll('details')){const key=disclosureKey(details);if(key){if(model.disclosures[key])details.setAttribute('open','');else details.removeAttribute('open');}}}
function layoutPacketConfiguration(){
 const region=document.querySelector('.packet-configuration-region');if(!region)return;const put=(key,value)=>{if(region.style[key]!==value)region.style[key]=value;};
 const formsOpen=!!node('packetCaptureSetup')?.hasAttribute('open')||!!region.querySelector('.packet-display-options')?.hasAttribute('open'),compact=(Number.isFinite(root.innerWidth)&&root.innerWidth>0&&root.innerWidth<=760)||(Number.isFinite(root.innerHeight)&&root.innerHeight>0&&root.innerHeight<=720),review=model.view==='packets'&&!!model.selected;
 const workspace=node('packetCaptureWorkspace'),available=Number(workspace?.clientHeight)||workspace?.getBoundingClientRect?.().height||600;
 if(formsOpen&&review&&compact){const height=Math.max(32,Math.min(160,Math.floor(available*.42)));put('maxHeight','42%');put('minHeight',height+'px');put('flexBasis',height+'px');return;}
 const cap=!formsOpen&&review&&compact?32:!formsOpen&&review&&state.packetGraphPreferences?.collapsed&&Number.isFinite(model.graphCollapseConfigurationHeight)?model.graphCollapseConfigurationHeight:null;
 if(cap===null){for(const key of ['maxHeight','minHeight','flexBasis'])put(key,'');return;}
 const height=Math.round(clamp(cap,32,Math.max(32,available*.42)));
 // Keep setup and filtering scrollable while the released plot space goes to
 // results. A formerly compressed region still gets one usable control row.
 put('maxHeight',height+'px');put('minHeight','32px');put('flexBasis',height+'px');
}
function layoutDetails(){
 layoutPacketConfiguration();const area=node('packetCaptureAnalysis');if(!area)return;if(model.view!=='packets'||!node('packetInspector')){area.style.gridTemplateColumns='';area.style.gridTemplateRows='';return;}
 const pref=preferences(),bounds=area.getBoundingClientRect(),width=bounds.width||area.clientWidth||1000,height=bounds.height||area.clientHeight||400,bottom=pref.dock==='bottom'||(pref.dock==='auto'&&width<((model.graphHold||model.liveDetails)?760:1070));
 const graphVisible=node('packetGraphDock')&&!node('packetGraphDock').hidden,listMinimum=graphVisible?Math.ceil(packetListOverhead()+146):96;
 const minimum=bottom?Math.min(80,Math.max(40,height-listMinimum)):260,maximum=bottom?Math.max(minimum,Math.min(650,height*.65,height-listMinimum)):Math.max(260,Math.min(700,width-326));
 const collapsedHeight=bottom&&state.packetGraphPreferences?.collapsed&&Number.isFinite(model.graphCollapseInspectorHeight)?model.graphCollapseInspectorHeight:null,requested=collapsedHeight===null?pref[bottom?'height':'width']:Math.min(pref.height,collapsedHeight),dimension=Math.round(clamp(requested,minimum,maximum));
 area.classList.toggle('packet-details-bottom',bottom);area.classList.toggle('packet-details-hidden',pref.hidden);
 area.style.gridTemplateColumns=bottom?'minmax(0,1fr)':'minmax(0,1fr) 6px '+(pref.hidden?30:dimension)+'px';area.style.gridTemplateRows=bottom?'minmax(0,1fr) 6px '+(pref.hidden?30:dimension)+'px':'minmax(0,1fr)';
 const divider=node('packetDetailsDivider');if(divider){divider.setAttribute('aria-orientation',bottom?'horizontal':'vertical');divider.setAttribute('aria-valuenow',pref.hidden?'0':String(dimension));divider.setAttribute('aria-valuemin',String(pref.hidden?0:Math.round(minimum)));divider.setAttribute('aria-valuemax',String(Math.round(maximum)));divider.setAttribute('aria-label',pref.hidden?'Show packet details':'Resize or hide packet details');divider.setAttribute('aria-valuetext',pref.hidden?'Collapsed':dimension+' pixels');divider.title=pref.hidden?'Double-click or press Enter or Space to show packet details.':'Drag to resize. Double-click or press Enter or Space to hide details. Arrow keys resize; Home resets.';}
 const inspector=node('packetInspector'),rail=node('packetDetailsRail');if(inspector)inspector.hidden=pref.hidden;if(rail)rail.hidden=!pref.hidden;
 for(const button of area.querySelectorAll('[data-packet-action="toggle-details"],[data-packet-action="hide-details"]'))root.NetworkToolkitPanelControls?.set(button,{name:'packet details',edge:bottom?'bottom':'right',expanded:!pref.hidden});
 if(rail)root.NetworkToolkitPanelControls?.bindReveal?.(rail,{collapsed:()=>preferences().hidden,button:()=>rail.querySelector('[data-packet-action="toggle-details"]')});
 if(graphVisible)layoutPacketGraph();
 if(!detailObserver&&typeof ResizeObserver==='function')detailObserver=new ResizeObserver(layoutDetails);if(detailObserver&&observedAnalysis!==area){if(observedAnalysis)detailObserver.unobserve(observedAnalysis);observedAnalysis=area;detailObserver.observe(area);}
}
function dockSelector(){return '<label class="sr-only" for="packetDetailsDock">Details position</label><select id="packetDetailsDock" aria-label="Details position" title="Details position">'+options([['auto','Auto'],['right','Right'],['bottom','Bottom']],preferences().dock)+'</select>';}
function toggleDetails(hidden,moveFocus=true){const pref=preferences();pref.hidden=hidden===undefined?!pref.hidden:hidden;layoutDetails();persistDetails();if(moveFocus)(pref.hidden?node('packetDetailsRail')?.querySelector('button'):node('packetInspector')?.querySelector('[data-packet-action="hide-details"]'))?.focus?.({preventScroll:true});}
function resizeDetails(event,divider){
 if(event.button!==0||preferences().hidden)return;event.preventDefault();event.stopPropagation();divider.focus?.({preventScroll:true});const area=node('packetCaptureAnalysis'),bottom=area.classList.contains('packet-details-bottom'),name=bottom?'height':'width',start=bottom?event.clientY:event.clientX,initial=Number(divider.getAttribute('aria-valuenow')),pointer=event.pointerId;
 divider.classList.add('dragging');document.body.classList.add(bottom?'resizing-rows':'resizing-columns');
 const move=next=>{if(pointer!==undefined&&next.pointerId!==undefined&&next.pointerId!==pointer)return;const amount=clamp(initial-((bottom?next.clientY:next.clientX)-start),Number(divider.getAttribute('aria-valuemin')),Number(divider.getAttribute('aria-valuemax')));preferences()[name]=Math.round(clamp(amount,bottom?80:260,bottom?650:700));if(bottom&&state.packetGraphPreferences?.collapsed)model.graphCollapseInspectorHeight=preferences().height;layoutDetails();};
 const finish=()=>{for(const type of ['pointerup','pointercancel','blur'])root.removeEventListener(type,finish);root.removeEventListener('pointermove',move);divider.classList.remove('dragging');document.body.classList.remove('resizing-rows','resizing-columns');persistDetails();};root.addEventListener('pointermove',move);for(const type of ['pointerup','pointercancel','blur'])root.addEventListener(type,finish);
}
const protocols=['any','TCP','UDP','ICMP','ICMPv6','ARP','DNS','DHCP','HTTP','TLS','SSH','LLDP','CDP'];
const dhcpFilter='dhcp || udp.port == 67 || udp.port == 68 || udp.port == 546 || udp.port == 547';
const quickFilters=[['dns','Name lookups (DNS)','Shows recognized DNS records. Cached answers and encrypted DNS may not appear as ordinary DNS packets.'],[dhcpFilter,'Address assignment (DHCP)','Shows decoded DHCPv4 plus UDP ports 67, 68, 546 and 547. DHCPv6 and short or undecoded packets appear as UDP; a port match alone does not confirm DHCP.'],['arp','Local address lookup (ARP)','Shows local IPv4 address lookups. IPv6 neighbor and router discovery uses ICMPv6 under Ping / network messages.'],['icmp || icmpv6','Ping / network messages','Includes ping, delivery messages and IPv6 neighbor discovery. Not every network control message indicates a fault.'],['tcp.flags.syn','Connection attempts','Shows TCP connection requests and replies to those requests. Inspect the exchange before deciding whether setup completed.'],['tcp.flags.rst','Connection resets','Shows TCP resets. A reset can reject or end a connection and does not identify the cause on its own.'],['tls','Encrypted traffic (TLS)','Shows TLS record headers recognized by the decoder. Encrypted contents are not decrypted; QUIC over UDP may remain labeled UDP.'],['udp.port == 5353','Multicast DNS (mDNS)','Local discovery queries and replies on UDP 5353.'],['tcp.port == 445','File sharing (SMB)','SMB traffic using TCP 445. Encryption and negotiation details depend on captured bytes.'],['udp.port == 123','Clock sync (NTP)','Time synchronization packets using UDP 123.'],['tcp.port == 22','SSH','Encrypted SSH sessions using TCP 22.'],['udp.port == 51820','VPN / WireGuard port','Traffic using the common WireGuard UDP port 51820. This does not confirm the application protocol.'],['expert','Decoder notes','Shows packets with decoder notes, such as incomplete data. A decoder note does not necessarily indicate a network fault.']];
const tasks={general:{label:'General troubleshooting',hint:'Start capture, reproduce the problem once, then stop and review the findings.',filter:''},website:{label:'Website or application will not connect',hint:'Start before opening the website or retrying the app. Reproduce the failure, then stop. DNS and connection setup can help narrow where it stops.',filter:'dns || tcp || tls || http || udp.port == 443'},dns:{label:'A website or device name does not resolve (DNS)',hint:'Start capture, then repeat the name lookup. Cached answers or encrypted DNS may not appear as ordinary DNS packets.',filter:'dns'},dhcp:{label:'Device cannot get an IP address (DHCP)',hint:'Start on the affected adapter before an address request or renewal. Record full packets with All protocols; short packets may not decode DHCPv4. Existing leases, static addresses or IPv6 automatic configuration (SLAAC) may produce no DHCP. DHCPv6 stays UDP here. This tool does not renew addresses.',filter:dhcpFilter},device:{label:'Cannot reach a device',hint:'Start capture, then ping the device or try the connection once. Local address lookup and replies show what was observed.',filter:'arp || icmp || icmpv6 || tcp'},slow:{label:'Slow or intermittent connections',hint:'Record while the slowdown happens. Repeated connection attempts and delivery messages can suggest what to check next; a short capture may miss an intermittent problem.',filter:'tcp || dns || icmp || icmpv6'}};
// Short, actionable guidance lives in an overlay. It never occupies permanent
// vertical space in the packet list. Filter expressions use Network Toolkit's
// supported syntax, not Wireshark's broader display-filter dialect.
const packetTroubleCases=[
 {title:'A device cannot obtain an address',look:'DHCP Discover, Offer, Request and ACK/NAK. Check whether the exchange reaches a reply.',meaning:'No Offer seen may mean the response was missed, blocked or not captured. NAK means an address request was rejected.',filter:dhcpFilter},
 {title:'Names fail to resolve',look:'DNS query and its matching response. Check name, requested record and response status.',meaning:'NXDOMAIN means the requested name was not found; SERVFAIL means the resolver could not complete it. No reply in this capture is not proof of a server failure.',filter:'dns'},
 {title:'Connection refused or never opens',look:'TCP SYN, SYN/ACK, ACK and RST packets around the connection attempt.',meaning:'SYN without a visible reply may be a path, host or capture-point issue. RST explicitly rejects or closes a connection but may be normal.',filter:'tcp.flags.syn || tcp.flags.rst'},
 {title:'Slow or intermittent communication',look:'Repeated TCP payloads, resets, long gaps or ICMP delivery errors. Compare both directions.',meaning:'Repeated packets may suggest retransmission, but duplicate capture observations can look similar. One trace cannot prove packet loss.',filter:'tcp || icmp || icmpv6'},
 {title:'A local device or gateway is unreachable',look:'ARP requests and replies, ICMP echo or unreachable messages, and ICMPv6 neighbor discovery.',meaning:'Unanswered ARP can suggest reachability or VLAN issues, but traffic may not be visible at your capture point. IPv6 does not use ARP.',filter:'arp || icmp || icmpv6'},
 {title:'Website or TLS handshake fails',look:'DNS, TCP connection attempts, TLS handshake or UDP 443 traffic.',meaning:'A successful TCP handshake does not prove HTTPS works. TLS is encrypted, and HTTP/3 over QUIC may only decode as UDP.',filter:'dns || tcp || tls || udp.port == 443'},
 {title:'VPN tunnel traffic is suspect',look:'UDP exchanges on expected VPN ports, such as WireGuard 51820 or IPsec 500/4500.',meaning:'Traffic on a port does not confirm a working tunnel. Custom VPN ports and encapsulated traffic may need separate inspection.',filter:'udp.port == 51820 || udp.port == 500 || udp.port == 4500'},
 {title:'Device discovery or file sharing is missing',look:'Multicast DNS (5353), local discovery, or TCP SMB (445) packets.',meaning:'Service discovery often depends on the local segment. Seeing discovery traffic does not prove the destination service is available.',filter:'udp.port == 5353 || tcp.port == 445'}
];
function packetGuideHTML(){return '<aside id="packetQuickGuide" class="packet-quick-guide" '+(!model.guideOpen?'hidden':'')+' aria-label="Packet troubleshooting reference"><header><strong>What packets should I look for?</strong><button type="button" class="iconbtn" data-packet-action="close-guide" aria-label="Close packet guidance" title="Close">×</button></header><p class="packet-guide-intro">Choose the problem, then use <strong>Show packets</strong>. Inspect both directions and correlate with the time of the failure.</p><div class="packet-guide-scroll">'+packetTroubleCases.map(item=>'<section class="packet-guide-case"><div class="packet-guide-title"><strong>'+esc(item.title)+'</strong><button class="btn small ghost" data-packet-filter="'+esc(item.filter)+'" title="Filter the packet list">Show packets</button></div><p><b>Look for:</b> '+esc(item.look)+'</p><p><b>Interpret:</b> '+esc(item.meaning)+'</p></section>').join('')+'</div><p class="packet-guide-caution">Map colors are customizable under Colors & maps. Alerts or protocols can be highlighted independently. Each line in the detailed strip is one packet (1 pixel by default). These are clues, not confirmed faults. A capture only shows traffic visible on the chosen adapter. The Troubleshooting tab gives a broader review.</p></aside>';}
const active=s=>['starting','running','stopping'].includes(s?.state),node=id=>document.getElementById(id),visible=()=>state.activePage==='capture'&&!!node('packetCaptureWorkspace');
const bytes=n=>n<1024?n+' B':n<1048576?(n/1024).toFixed(1)+' KiB':(n/1048576).toFixed(1)+' MiB';
function packetEpoch(value){
 if(root.NetworkToolkitResultTime)return root.NetworkToolkitResultTime.epoch(value);
 if(value==null||value==='')return NaN;const epoch=typeof value==='number'?value:Date.parse(value),date=new Date(epoch);
 return Number.isFinite(epoch)&&date.getUTCFullYear()>1?epoch:NaN;
}
function packetLocalTime(value,options){
 if(root.NetworkToolkitResultTime)return root.NetworkToolkitResultTime.format(value,options);
 const epoch=packetEpoch(value);return Number.isFinite(epoch)?new Date(epoch).toLocaleString(): 'Not recorded';
}
function packetClock(value){
 if(root.NetworkToolkitResultTime)return packetLocalTime(value,{timezone:false});
 const epoch=packetEpoch(value);if(!Number.isFinite(epoch))return 'Not recorded';
 const date=new Date(epoch),pad=(n,width=2)=>String(n).padStart(width,'0');
 return pad(date.getFullYear(),4)+'-'+pad(date.getMonth()+1)+'-'+pad(date.getDate())+' '+pad(date.getHours())+':'+pad(date.getMinutes())+':'+pad(date.getSeconds())+'.'+pad(date.getMilliseconds(),3);
}
function packetTimeCell(packet){
 const recorded=Number.isFinite(packetEpoch(packet.time)),full=packetLocalTime(packet.time),elapsed=Number.isFinite(Number(packet.relative))?Number(packet.relative).toFixed(6)+' s':'Not recorded';
 const title='Recorded: '+full+(recorded?' | Original timestamp: '+packet.time:'')+' | Elapsed since first captured frame: '+elapsed;
 return '<td class="packet-local-time" title="'+esc(title)+'"'+(recorded?' data-copy-value="'+esc(full+' | '+packet.time+' | Elapsed '+elapsed)+'"':' data-no-value-copy')+'><time'+(recorded?' datetime="'+esc(packet.time)+'"':'')+'>'+esc(packetClock(packet.time))+'</time><span class="packet-elapsed"> +'+esc(elapsed)+'</span></td>';
}
function packetBucketMetrics(bucket){
 const start=packetEpoch(bucket?.time),end=packetEpoch(bucket?.endTime),interval=Number(bucket?.intervalSeconds),packets=Number(bucket?.packets),size=Number(bucket?.bytes);
 const measured=Number.isFinite(start)&&Number.isFinite(end)&&end>=start&&Number.isFinite(interval)&&interval>0&&Math.abs((end-start)/1000-interval)<.002;
 return {start,end,interval,packets:Number.isFinite(packets)&&packets>=0?packets:0,bytes:Number.isFinite(size)&&size>=0?size:0,measured};
}
function packetNumber(value,digits=2){return Number.isFinite(Number(value))?Number(value).toLocaleString(undefined,{maximumFractionDigits:digits}):'Not measured';}
function packetBucketText(bucket){
 const m=packetBucketMetrics(bucket);
 return packetLocalTime(bucket?.time)+' to '+packetLocalTime(bucket?.endTime)+' | '+packetNumber(m.packets,0)+' packets | '+bytes(m.bytes)+(m.measured?' | '+packetNumber(m.packets/m.interval)+' packets/s | '+packetNumber(m.bytes*8/m.interval)+' bit/s':' | Rate not measured');
}
function packetZone(){return root.NetworkToolkitResultTime?.zoneLabel?.()||'local';}
function packetTimelineState(){
 const key=model.selected;
 if(model.timelineState?.key!==key){hidePacketGraphPopup();model.timelineState={key,index:null,time:null,pinned:false,pin:null,hover:null,scrollLeft:0,follow:true};}
 return model.timelineState;
}
function packetBucketKey(bucket){return JSON.stringify([bucket?.time||'',bucket?.endTime||'',bucket?.packetNumber||0]);}
function packetTimelineIndex(statistics,selection){
 const rows=Array.isArray(statistics?.timeline)?statistics.timeline:[];
 if(selection.pinned&&selection.pin){selection.index=rows.findIndex(bucket=>packetBucketKey(bucket)===packetBucketKey(selection.pin));if(selection.index<0)selection.index=null;}
 return Number.isInteger(selection.index)&&selection.index>=0&&selection.index<rows.length?selection.index:null;
}
function packetTimelineValues(bucket){
 const m=packetBucketMetrics(bucket),item=(label,value,raw)=>'<div><dt>'+esc(label)+'</dt><dd'+(raw?' data-copy-value="'+esc(value+' | '+raw)+'"':'')+'>'+(raw?'<time datetime="'+esc(raw)+'" title="'+esc(raw)+'">'+esc(value)+'</time>':esc(value))+'</dd></div>',zone=packetZone();
 return '<dl class="packet-timeline-values">'+item('Start ('+zone+')',packetLocalTime(bucket.time),bucket.time)+item('End ('+zone+')',packetLocalTime(bucket.endTime),bucket.endTime)+item('Packets',packetNumber(m.packets,0))+item('Bytes',bytes(m.bytes))+item('Observed interval',Number.isFinite(m.start)&&Number.isFinite(m.end)?packetNumber(Math.max(0,m.end-m.start)/1000,6)+' s':'Not recorded')+item('Packet rate',m.measured?packetNumber(m.packets/m.interval)+' packets/s':'Not measured')+item('Traffic rate',m.measured?packetNumber(m.bytes*8/m.interval)+' bit/s':'Not measured')+(bucket.severities?item('Recorded clues',Object.entries(bucket.severities).filter(([,count])=>Number(count)>0).map(([level,count])=>packetNumber(count,0)+' '+({normal:'without a clue',error:'decoder errors',warn:'warnings',note:'informational notes'}[level]||level)).join(', ')):'')+(model.statFilter==='traffic'?item('Aggregate traffic reference',(()=>{const summary=model.summaryStatistics||model.statistics||{},duration=Number(summary.duration),value=Number(summary.timedBytes);return duration>0&&Number.isFinite(value)?packetNumber(value*8/duration)+' bit/s across the observed span':'Not measured';})()):'')+(model.statFilter==='average'?item('Aggregate average reference',(()=>{const summary=model.summaryStatistics||model.statistics||{},count=Object.values(summary.protocols||{}).reduce((sum,value)=>sum+Number(value||0),0);return count?packetAverageBytes(Number(summary.bytes||0)/count)+' per contributing packet':'Not measured';})()):'')+'</dl>';
}
function packetTimelineDetails(statistics,index){
 const selection=packetTimelineState(),bucket=selection.pinned?selection.pin:statistics?.timeline?.[index];
 if(!bucket)return '<p class="packet-timeline-prompt">Hover over a traffic bar for its recorded time and values. Click or press Enter to keep it selected. Arrow keys move between intervals; Escape clears the selection.</p>';
 const exact=selection.pinned&&packetTimelineIndex(statistics,selection)!==null;
 return '<div class="packet-timeline-selection"><strong>'+(selection.pinned?'Selected interval':'Traffic interval')+'</strong><button type="button" class="btn small ghost" data-packet-timeline-clear>Clear selection</button></div>'+(selection.pinned&&!exact?'<p class="packet-timeline-retained">The selected interval is retained below. The current graph has regrouped or no longer includes these exact interval boundaries.</p>':'')+packetTimelineValues(bucket)+(selection.pinned?'<p class="packet-timeline-correlation">Values are retained from the selection. Rows are linked by recorded time within this interval.</p>':'');
}
function packetGraphPreferences(){
 const timeline=root.NetworkToolkitResultTimeline;if(timeline?.preferences)return timeline.preferences();
 return {mode:state.graphPreferences?.mode==='condensed'?'condensed':'linear'};
}
function setPacketGraphMode(mode){
 const value={mode:mode==='condensed'?'condensed':'linear'};
 if(root.NetworkToolkitResultTimeline?.setPreferences)root.NetworkToolkitResultTimeline.setPreferences(value);else{state.graphPreferences=value;persistDetails();}
 refreshPacketGraph();void refresh(true);
}
function packetGraphZoom(){return clamp(Number(state.packetGraphPreferences?.zoom)||1,1e-18,1e24);}
function packetBucketSeverity(bucket){const counts=bucket?.severities||{};return ['error','warn','note'].find(level=>Number(counts[level])>0)||(model.severityFilter!=='any'&&model.severityFilter!=='normal'?model.severityFilter:'normal');}
const packetStatisticLabels={packets:'Displayed packets',bytes:'Displayed bytes',duration:'Observed capture span',expert:'Packets with expert notes',average:'Average packet size',traffic:'Observed average traffic',dnsrequests:'Name lookup requests',dnsreplies:'Name lookup replies',tcpstarts:'TCP connection attempts',tcpresets:'Connection resets',icmperrors:'Delivery / hop-limit messages'};
function packetStatisticSummary(statistics){
 const s=statistics||{},total=Object.values(s.protocols||{}).reduce((n,count)=>n+Math.max(0,Number(count)||0),0),duration=Number(s.duration)||0,timedBytes=Number(s.timedBytes),measured=duration>0&&Number.isFinite(timedBytes)&&Number.isFinite(packetEpoch(s.started))&&Number.isFinite(packetEpoch(s.finished));
 const values={packets:packetNumber(total,0),bytes:bytes(Number(s.bytes)||0),duration:duration.toFixed(3)+' s',expert:packetNumber(s.expert||0,0),average:total?packetAverageBytes(Number(s.bytes)/total):'Not measured',traffic:measured?packetNumber(timedBytes*8/duration)+' bit/s':'Not measured'};
 return '<div id="packetGraphSummary" class="packet-graph-summary" role="group" aria-label="Packet statistics">'+['packets','bytes','duration','expert','average','traffic'].map(key=>[key,packetStatisticLabels[key]]).map(([key,label])=>'<button type="button" class="packet-statistic'+(model.statFilter===key?' active':'')+'" data-packet-stat="'+key+'" aria-pressed="'+(model.statFilter===key)+'" title="'+esc(packetStatisticExplanation(key))+'"><strong>'+esc(values[key])+'</strong><span>'+label+'</span></button>').join('')+'</div>';
}
function packetStatisticExplanation(key){return {packets:'Shows all packets within the other active filters.',bytes:'Shows packets contributing nonzero displayed bytes.',duration:'Shows packets with recorded times that contribute to the observed span.',expert:'Shows packets with recorded decoder or capture notes; notes are not necessarily faults.',average:'Shows every contributing packet. The displayed average is an aggregate reference, not an individual packet.',traffic:'Shows timed packets contributing bytes to the aggregate observed average traffic rate.',dnsrequests:'Shows assessed DNS requests contributing to this report, within the other display filters.',dnsreplies:'Shows assessed DNS replies contributing to this report, within the other display filters.',tcpstarts:'The report counts distinct connection attempts. Shows their captured initial SYN contributors; retries may contribute more than one packet per attempt.',tcpresets:'Shows captured TCP reset packets; a reset alone does not establish a fault.',icmperrors:'Shows assessed ICMP delivery and hop-limit messages; hop-limit messages may be expected during a trace route.'}[key]||'';}
function packetBaseScope(statistics=model.statistics){
 const rows=statistics?.timeline||[],history=root.NetworkToolkitGraphHistory,all=history?.preferences?.().allHistory&&packetGraphZoom()===1,first=(all?model.graphOverview?.start||statistics?.started:statistics?.started||model.graphOverview?.start)||rows[0]?.time,last=(all?model.graphOverview?.end||statistics?.finished:statistics?.finished||model.graphOverview?.end)||rows.at(-1)?.endTime;
 const base=model.graphBaseScope||history?.range?.([{time:first},{time:last}],history.preferences(),model.graphHeldEnd||'');if(model.graphBaseSpan&&history.instant(base?.end)!==null)return {...base,start:history.timestamp(history.instant(base.end)-BigInt(model.graphBaseSpan))};return base||{start:first,end:last,from:packetEpoch(first),to:packetEpoch(last)};
}
function packetPlotScope(statistics=model.statistics){const history=root.NetworkToolkitGraphHistory,scope=packetBaseScope(statistics),visible=model.graphWindow?root.NetworkToolkitGraphNavigator.scope(model.graphWindow.start,model.graphWindow.end):history?.viewport?.(scope,packetGraphZoom(),model.graphViewEnd||'')||scope;return {...visible,legacyCondensed:true};}
function packetZoomControls(){return '<div class="packet-zoom-controls graph-inline-zoom" role="group" aria-label="Packet graph zoom"><button type="button" class="btn small ghost" data-packet-zoom="out" aria-label="Zoom packet graph out" title="Zoom out" '+(packetGraphZoom()<=.25?'disabled':'')+'>−</button><span data-packet-zoom-value data-packet-zoom-factor="'+packetGraphZoom()+'" title="'+esc(packetIntervalLabel(packetPlotScope())+(packetGraphPreferences().mode==='condensed'?' · nonlinear time':''))+'">'+esc(root.NetworkToolkitGraphHistory?.spanLabel?.(packetPlotScope())||'—')+'</span><button type="button" class="btn small ghost" data-packet-zoom="in" aria-label="Zoom packet graph in" title="Zoom in" '+(packetGraphZoom()>=16?'disabled':'')+'>+</button><button type="button" class="btn small ghost" data-packet-zoom="reset" aria-label="Fit all retained packet history" title="Fit all retained history">Fit</button></div>';}
function packetIntervalLabel(scope){const history=root.NetworkToolkitGraphHistory,a=history?.instant?.(scope.start),b=history?.instant?.(scope.end);if(a==null||b==null)return 'Time unavailable';const ms=Number(b-a)/1000000;for(const [unit,size]of [['d',86400000],['h',3600000],['min',60000],['s',1000],['ms',1],['µs',.001],['ns',.000001]])if(ms>=size)return history.scaleLabel(ms/size)+' '+unit+' visible';return 'Single instant';}
function packetBucketContained(bucket,scope){const history=root.NetworkToolkitGraphHistory;return !history||history.contains(bucket.time,scope)&&history.contains(bucket.endTime,scope);}

function packetFollowHTML(){const following=!model.graphHold&&packetTimelineState().follow;return '<button type="button" class="btn small ghost" data-packet-graph-latest data-packet-graph-follow data-follow-state="'+(model.graphHold?'held':following?'following':'paused')+'" aria-pressed="'+following+'" title="'+(model.graphHold?'Selection held. ':'')+'Follow the newest matching packet in graph and list without opening Details">Follow latest</button>';}
function packetBucketGeometry(bucket,index,rows,statistics=model.statistics){
 const history=root.NetworkToolkitGraphHistory,scope=packetPlotScope(statistics),mode=packetGraphPreferences().mode,left=history?.fraction?.(bucket.time,scope,mode),right=history?.fraction?.(bucket.endTime,scope,mode);
 if(Number.isFinite(left)&&Number.isFinite(right)){const x=clamp(left*600,0,600),end=clamp(right*600,0,600);return end===x?{x:Math.min(598,Math.max(0,x-1)),width:2,visible:right>=0&&left<=1}:{x,width:Math.max(.001,end-x),visible:right>=0&&left<=1};}
 return {x:index*600/Math.max(1,rows.length),width:600/Math.max(1,rows.length),visible:true};
}
function packetTrafficHTML(statistics){
 const rows=Array.isArray(statistics.timeline)?statistics.timeline:[],selection=packetTimelineState(),selected=packetTimelineIndex(statistics,selection),mode=packetGraphPreferences().mode,scope=packetPlotScope(statistics),visible=rows.filter(bucket=>packetBucketContained(bucket,scope)),scale=root.NetworkToolkitGraphHistory.valueScale(visible.map(bucket=>packetBucketMetrics(bucket).packets),{whole:true}),max=scale.maximum??0,format=root.NetworkToolkitGraphHistory.scaleLabel;
 return '<section class="packet-traffic-section" data-packet-timeline data-packet-graph-mode-value="'+mode+'" data-packet-y-lower="'+scale.lower+'" data-packet-y-upper="'+scale.upper+'" data-packet-y-minimum="'+(scale.minimum??'')+'" data-packet-y-maximum="'+(scale.maximum??'')+'"><div class="packet-traffic-heading"><label class="packet-mode-control"><span class="sr-only">Graph view</span><select data-packet-graph-mode aria-label="Packet graph view">'+options([['linear','Linear'],['condensed','Condensed']],mode)+'</select></label>'+(root.NetworkToolkitGraphHistory?.controlHTML?.()||'')+packetZoomControls()+'</div><div class="packet-graph-scroll" data-packet-graph-scroll tabindex="0" role="region" aria-label="Packet traffic timeline; drag to pan time"><div class="packet-y-axis" aria-label="Visible packet-count scale"><span class="packet-y-upper">'+(scale.empty?'—':format(scale.upper))+'</span><span class="packet-y-lower">'+(scale.empty?'—':format(scale.lower))+'</span></div><svg class="packet-timeline" viewBox="0 0 600 110" preserveAspectRatio="none" role="group" aria-label="Packets in recorded time intervals">'+rows.map((bucket,i)=>{const m=packetBucketMetrics(bucket),height=(m.packets-scale.lower)/(scale.upper-scale.lower)*85,level=packetBucketSeverity(bucket),geometry=packetBucketGeometry(bucket,i,rows,statistics);if(!geometry.visible||!packetBucketContained(bucket,scope))return '';return '<g data-packet-bucket="'+i+'" data-packet-severity="'+level+'" tabindex="'+(i===(selected??0)?'0':'-1')+'" role="button" aria-pressed="'+(selection.pinned&&i===selected)+'" aria-label="'+esc(packetBucketText(bucket))+'" class="packet-bucket-'+level+(model.statFilter&&m.packets?' packet-stat-matched':'')+(selection.pinned&&i===selected?' selected':'')+'"><rect class="packet-timeline-bar" x="'+(geometry.x+Math.min(1,geometry.width/8))+'" y="'+(95-height)+'" width="'+Math.max(.001,geometry.width-Math.min(2,geometry.width/4))+'" height="'+height+'"/><rect class="packet-timeline-hit" x="'+geometry.x+'" y="0" width="'+Math.max(2,geometry.width)+'" height="100"/></g>';}).join('')+'</svg></div><div class="packet-time-ticks" data-packet-visible-axis data-packet-axis-scope="visible" aria-label="Displayed time axis">'+(root.NetworkToolkitGraphHistory?.axisHTML?.(scope,600,mode)||'')+'</div>'+packetNavigatorHTML()+'<p class="packet-timeline-aggregation" title="'+esc(packetIntervalLabel(scope)+' · '+(mode==='condensed'?'Smooth nonlinear compressed time. Older intervals grow gradually; original frames remain available. ':'')+'Counts use the labeled visible scale. Click an interval to hold its contributors in entry details. '+(statistics.graphNote||''))+'">'+esc(packetIntervalLabel(scope))+' · '+(mode==='condensed'?'Condensed · nonlinear time':'Recorded time')+' · '+esc(packetZone())+' · Peak '+packetNumber(max,0)+' / interval'+(Number(statistics.untimedPackets)>0?' · '+packetNumber(statistics.untimedPackets,0)+' untimed (not plotted)':'')+(rows.length?'':' · No timed packets')+(model.graphZoomPending?' · Updating range…':'')+'</p></section>';
}

function packetNavigatorScope(){return {start:model.graphOverview?.start||model.statistics?.started||'',end:model.graphOverview?.end||model.statistics?.finished||''};}
function packetNavigatorHTML(){const nav=root.NetworkToolkitGraphNavigator;if(!nav)return '';const full=packetNavigatorScope(),source=model.graphOverview||model.statistics,rows=model.graphOverview?.bins||model.statistics?.timeline||[];
 if(model.graphNavigatorSource!==source){model.graphNavigatorSource=source;model.graphNavigatorPreview=nav.geometry(full,rows.map(b=>({time:b.time,endTime:b.endTime,value:Number(b.packets)||0,level:packetBucketSeverity(b)})),{bars:true});}
 return nav.html('packet:'+model.selected,full,packetPlotScope(),model.graphNavigatorPreview,{label:'Full capture',note:'Bounded packet counts in linear time intervals; the extent includes filtered-out history.'});}
function attachPacketNavigator(){for(const nav of document.querySelectorAll('[data-graph-navigator]'))if(nav.dataset.graphNavigator==='packet:'+model.selected)root.NetworkToolkitGraphNavigator.attach(nav,{full:packetNavigatorScope(),view:packetPlotScope(),change:navigatePacketWindow,begin:()=>{model.graphNavigating=true;model.inspectPointer=false;hidePacketGraphPopup();},end:()=>{model.graphNavigating=false;}});updatePacketViewContext();}
function updatePacketViewContext(){const h=root.NetworkToolkitGraphHistory,scope=packetPlotScope(),stamp=model.detail?.packet?.time,bucket=model.intervalSelection?.bucket,outside=model.graphHold&&(bucket?!h.contains(bucket.time,scope)||!h.contains(bucket.endTime,scope):stamp&&!h.contains(stamp,scope));for(const note of document.querySelectorAll('[data-packet-view-context]'))note.textContent=outside?'The held selection is outside the current graph interval. Its recorded details remain selected.':'';}
async function refreshPacketNavigator(){
 const selected=model.selected,capture=model.captures.find(c=>c.id===selected),query=parameters().toString(),key=[selected,capture?.revision,capture?.count,query].join('|');if(!selected||model.graphOverviewKey===key)return;
 const value=await request('/'+selected+'/graph-overview?'+query);if(model.selected!==selected||query!==parameters().toString())return;
 // Revision and filter keyed; dragging does not invalidate the overview.
 if(value&&Array.isArray(value.bins)){model.graphOverview=value;model.graphOverviewKey=key;}
}
function navigatePacketWindow(scope){
 const h=root.NetworkToolkitGraphHistory,a=h.instant(scope.start),b=h.instant(scope.end),base=packetBaseScope(),span=model.graphBaseSpan?BigInt(model.graphBaseSpan):h.instant(base.end)-h.instant(base.start);if(a===null||b===null||b<=a)return;
 model.graphBaseSpan=String(span>0n?span:1000000000n);state.packetGraphPreferences={...state.packetGraphPreferences,zoom:Number(BigInt(model.graphBaseSpan))/Number(b-a)};model.graphWindow={start:scope.start,end:scope.end};model.graphViewEnd=scope.end;packetTimelineState().follow=false;model.follow=false;refreshPacketViewport();
}

function packetExactInstant(stamp){
 const epoch=packetEpoch(stamp);if(!Number.isFinite(epoch))return null;
 // The source timestamps retain nanoseconds. Preserve the sub-millisecond
 // remainder when associating a packet with an interval boundary.
 const fraction=typeof stamp==='string'?stamp.match(/\.(\d{1,9})(?:Z|[+-]\d{2}:\d{2})$/i)?.[1]:null;
 return BigInt(Math.trunc(epoch))*1000000n+BigInt(fraction?fraction.padEnd(9,'0').slice(3):'0');
}
function packetIntervalContains(bucket,stamp,last=false){last=last||bucket?.endInclusive===true;
 const start=packetExactInstant(bucket?.time),end=packetExactInstant(bucket?.endTime),time=packetExactInstant(stamp);
 return time!==null&&start!==null&&end!==null&&time>=start&&(time<end||last&&time===end);
}
function packetBucketForPacket(packet){
 const rows=model.statistics?.timeline||[];return rows.findIndex((bucket,index)=>packetIntervalContains(bucket,packet?.time,index===rows.length-1));
}
function updatePacketGraphRows(){
 const selection=packetTimelineState(),rows=model.statistics?.timeline||[],hover=Number.isInteger(selection.hover)?rows[selection.hover]:null,last=rows.at(-1);
 for(const row of document.querySelectorAll('#packetRows [data-packet-number]')){
  const stamp=row.dataset.packetTime,pinned=selection.pinned&&selection.pin&&packetIntervalContains(selection.pin,stamp,selection.pin._includesEnd===true);
  row.classList.toggle('packet-graph-hovered',!!hover&&packetIntervalContains(hover,stamp,hover===last));
  row.classList.toggle('packet-graph-pinned',!!pinned);const chosen=Number(row.dataset.packetNumber)===(model.heldPacketNumber||model.detail?.packet?.number);row.classList.toggle('selected',chosen);row.setAttribute('aria-selected',String(chosen));
 }
}
function updatePacketTimeline(){
 refreshPacketGraphAxis(); const statistics=model.statistics;if(!statistics)return;for(const control of document.querySelectorAll('[data-packet-timeline-clear]'))control.disabled=!model.graphHold&&!model.liveDetails;const selection=packetTimelineState(),index=packetTimelineIndex(statistics,selection),panel=node('packetTimelineDetails');
 if(panel)panel.innerHTML=packetTimelineDetails(statistics,selection.hover??index);
 for(const point of document.querySelectorAll('[data-packet-bucket]')){const i=Number(point.dataset.packetBucket),chosen=selection.pinned?index:model.liveDetails?packetBucketForPacket(model.detail?.packet):null;point.classList.toggle('selected',i===chosen);point.classList.toggle('packet-bucket-hovered',i===selection.hover);point.setAttribute('aria-pressed',String(i===(selection.pinned?index:model.liveDetails?packetBucketForPacket(model.detail?.packet):null)));point.tabIndex=i===(index??0)?0:-1;}
 updatePacketGraphRows();
}
function selectPacketTimeline(index,pin=false){
 const rows=model.statistics?.timeline||[];if(!Number.isInteger(index)||index<0||index>=rows.length)return;
 const selection=packetTimelineState();if(pin){selection.index=index;const stamp=packetEpoch(rows[index].time);selection.time=Number.isFinite(stamp)?stamp:null;selection.pinned=true;selection.pin={...rows[index],...(rows[index].severities?{severities:{...rows[index].severities}}:{}),_includesEnd:rows[index].endInclusive===true||index===rows.length-1};}else selection.hover=index;
 updatePacketTimeline();if(pin)void holdPacketBucket(rows[index],index);
}
function clearPacketGraphSelection(){const selection=packetTimelineState();model.graphSelectionVersion=(model.graphSelectionVersion||0)+1;model.graphHold=false;model.graphHeldEnd='';model.heldPacketNumber=null;model.intervalSelection=null;model.liveDetails=false;model.detail=null;model.inspectPointer=false;model.inspectFocus=false;model.keyboardInspection=false;model.selectedIndex=-1;selection.index=null;selection.time=null;selection.pinned=false;selection.pin=null;selection.hover=null;hidePacketGraphPopup();updatePacketTimeline();renderDetail();}
function packetSelectionControls(){return '<div class="packet-selection-controls"><p class="hint" data-packet-view-context></p><p class="hint">'+(model.graphHold?'Selection held. New packets continue recording. Filters and time ranges may hide this selection; its recorded details remain held. Choose Follow latest to follow again.':model.liveDetails?'Following the latest matching packet.':'Choose a packet or graph interval to inspect.')+'</p><button type="button" class="btn small ghost" data-packet-timeline-clear '+(model.graphHold||model.liveDetails?'':'disabled')+'>Clear selection</button></div>';}
function packetHeldLocate(){if(!model.graphHold||!model.heldPacketNumber||packetInspectionPaused(false))return null;const source=packetSourceSignature();if(model.holdLocatedSource===source||model.windowSignature.startsWith(source+'|')&&model.loaded.some(p=>p.number===model.heldPacketNumber))return null;model.holdLocatedSource=source;return {packetNumber:model.heldPacketNumber,selectionVersion:model.graphSelectionVersion};}
function packetGraphParameters(){
 const params=parameters(),pref=root.NetworkToolkitGraphHistory?.preferences?.(),mode=packetGraphPreferences().mode;if(model.graphWindow||model.graphBaseSpan||packetGraphZoom()>16||packetGraphZoom()<.25){const scope=packetPlotScope();if(root.NetworkToolkitGraphHistory.instant(scope.start)===null||root.NetworkToolkitGraphHistory.instant(scope.end)===null)return params;params.set('graph_mode',mode);params.set('graph_start',scope.start);params.set('graph_end',scope.end);if(pref?.recentMs)params.set('graph_recent_ms',String(pref.recentMs));return params;}if(mode!=='condensed'&&!pref?.durationMs&&(!pref||pref.anchor==='latest')&&!model.graphHeldEnd&&packetGraphZoom()===1&&!model.graphViewEnd)return params;params.set('graph_mode',mode);if(pref?.durationMs)params.set('graph_duration_ms',String(pref.durationMs));if(pref?.recentMs)params.set('graph_recent_ms',String(pref.recentMs));if(pref?.anchor==='range')params.set('graph_start',pref.start);const end=pref?.anchor!=='latest'&&pref?pref.end:model.graphHeldEnd;if(end)params.set('graph_end',end);
 if(packetGraphZoom()!==1)params.set('graph_zoom',String(packetGraphZoom()));if(model.graphViewEnd&&model.graphBaseScope){params.set('graph_start',model.graphBaseScope.start);params.set('graph_end',model.graphBaseScope.end);params.delete('graph_duration_ms');params.set('graph_view_end',model.graphViewEnd);}return params;
}

async function holdPacketBucket(bucket,index){
 const version=model.graphSelectionVersion=(model.graphSelectionVersion||0)+1;model.graphHold=true;model.liveDetails=false;model.graphHeldEnd=model.statistics?.graphFinished||model.statistics?.finished||'';packetTimelineState().follow=false;updatePacketTimeline();if(model.view!=='packets'){setView('packets');await refresh(true);if(version!==model.graphSelectionVersion)return;}
 if(bucket.packetNumber){await refreshPacketWindow(model.windowStart,{...bucket,packetNumber:bucket.packetNumber,selectionVersion:version});if(version!==model.graphSelectionVersion)return;await selectPacket(bucket.packetNumber);ensurePacketRowVisible(bucket.packetNumber);return;}
 model.heldPacketNumber=null;model.detail=null;model.intervalSelection={capture:model.selected,bucket:JSON.parse(JSON.stringify({...bucket,endInclusive:bucket.endInclusive===true||index===model.statistics?.timeline?.length-1})),query:parameters().toString(),rows:[],total:null,offset:0,loading:true};
 preferences().hidden=false;renderDetail();await Promise.all([loadPacketInterval(0),refreshPacketWindow(model.windowStart,{...bucket,endInclusive:bucket.endInclusive||index===model.statistics?.timeline?.length-1,selectionLocate:true,selectionVersion:version})]);
}
async function loadPacketInterval(offset){
 const selected=model.intervalSelection;if(!selected)return;selected.loading=true;const params=new URLSearchParams(selected.query);params.set('interval_start',selected.bucket.time);params.set('interval_end',selected.bucket.endTime);params.set('interval_inclusive',selected.bucket.endInclusive?'1':'0');params.set('offset',String(Math.max(0,offset)));params.set('limit','50');
 try{const value=await request('/'+selected.capture+'/interval?'+params);if(model.intervalSelection!==selected)return;selected.rows=Array.isArray(value.packets)?value.packets:[];selected.total=Number(value.total)||0;selected.offset=Number(value.offset)||0;selected.error='';}catch(error){if(model.intervalSelection!==selected)return;selected.error=error.message;}finally{if(model.intervalSelection===selected){selected.loading=false;renderDetail();}}
}
function renderPacketInterval(area){
 const selected=model.intervalSelection;if(!selected)return false;const b=selected.bucket,stamp=JSON.stringify([selected.bucket,selected.offset,selected.total,selected.loading,selected.error,selected.rows,root.NetworkToolkitResultTime?.preferences?.()]);if(area.dataset.packetDetail===stamp)return true;
 area.innerHTML='<div class="packet-detail-heading"><button type="button" class="iconbtn" data-packet-action="hide-details" aria-controls="packetInspector"></button><strong>Recorded traffic interval</strong>'+dockSelector()+'</div><div id="packetDetailScroll" class="packet-detail-scroll" tabindex="0">'+packetSelectionControls()+'<p class="hint">Aggregate interval snapshot. Counts and rates describe contributing packets, not a single response. The current graph may have regrouped; this selected summary remains a snapshot.</p><div class="packet-selected-interval-details">'+packetTimelineValues(b)+'</div>'+'<p class="hint">Select a contributing packet for its complete decoded fields and captured bytes.</p>'+(selected.loading?'<p role="status">Loading contributing packets...</p>':'')+(selected.error?'<p role="alert">'+esc(selected.error)+'</p>':'')+'<div class="result-graph-contributors">'+selected.rows.map(p=>'<button type="button" class="btn small ghost" data-packet-contributor="'+p.number+'">#'+p.number+' · '+esc(packetLocalTime(p.time))+' · '+esc(p.protocol)+' · '+p.length+' bytes</button>').join('')+'</div>'+(selected.total===0&&!selected.loading?'<p>No contributing packets are currently retained for this selected interval and its original display filters. The selected summary remains a snapshot.</p>':'')+'<div class="packet-interval-pages"><button type="button" class="btn small ghost" data-packet-interval-offset="'+Math.max(0,selected.offset-50)+'" '+(selected.offset===0||selected.loading?'disabled':'')+'>Previous</button><span>'+Math.min(selected.offset+1,selected.total||0)+'-'+Math.min(selected.offset+selected.rows.length,selected.total||0)+' of '+(selected.total??'...')+'</span><button type="button" class="btn small ghost" data-packet-interval-offset="'+(selected.offset+50)+'" '+(selected.loading||selected.offset+50>=selected.total?'disabled':'')+'>Next</button></div></div>';
 area.dataset.packetDetail=stamp;layoutDetails();updatePacketViewContext();return true;
}
async function resumePacketLatest(){
 if(root.NetworkToolkitGraphHistory?.preferences?.().allHistory&&packetGraphZoom()===1)model.graphBaseSpan='';
 model.graphViewEnd='';model.graphBaseScope=null;model.graphWindow=null; model.graphHold=false;model.graphSelectionVersion=(model.graphSelectionVersion||0)+1;model.graphHeldEnd='';model.heldPacketNumber=null;model.liveDetails=false;model.follow=true;if(node('packetFollow'))node('packetFollow').checked=true;
 const selection=packetTimelineState();selection.pinned=false;selection.pin=null;selection.hover=null;selection.follow=true;model.inspectPointer=false;model.inspectFocus=false;model.keyboardInspection=false;hidePacketGraphPopup();restorePacketGraphScroll();updatePacketTimeline();
 if(root.NetworkToolkitGraphHistory?.preferences?.().anchor!=='latest'&&root.NetworkToolkitGraphHistory)root.NetworkToolkitGraphHistory.setPreferences({anchor:'latest'});
 await refresh(true);await followPacketLatest(true);restorePacketGraphScroll();renderDetail();
}
function hidePacketGraphPopup(){node('packetGraphPopup')?.remove();for(const point of document.querySelectorAll('[aria-describedby="packetGraphPopup"]'))point.removeAttribute('aria-describedby');}
function showPacketGraphPopup(index,event,anchor){
 const bucket=model.statistics?.timeline?.[index];if(!bucket)return;let popup=node('packetGraphPopup');
 if(!popup){popup=document.createElement('aside');popup.id='packetGraphPopup';popup.className='packet-graph-popup';popup.setAttribute('role','tooltip');document.body.append(popup);}
 popup.innerHTML='<strong>Traffic interval</strong>'+packetTimelineValues(bucket);anchor?.setAttribute?.('aria-describedby','packetGraphPopup');
 const bounds=anchor?.getBoundingClientRect?.()||{left:8,top:8,bottom:20},x=Number.isFinite(event?.clientX)?event.clientX:bounds.left,y=Number.isFinite(event?.clientY)?event.clientY:bounds.bottom;
 const rect=popup.getBoundingClientRect?.()||{width:320,height:180},vw=root.innerWidth||800,vh=root.innerHeight||600,pw=Math.min(rect.width||320,vw-16),ph=Math.min(rect.height||180,vh-16);
 popup.style.left=Math.max(8,Math.min(vw-pw-8,x+14))+'px';popup.style.top=Math.max(8,Math.min(vh-ph-8,y+14))+'px';
}
function packetInstantString(value){
 let second=value/1000000000n,fraction=value%1000000000n;if(fraction<0){second--;fraction+=1000000000n;}
 try{return new Date(Number(second*1000n)).toISOString().replace(/\.\d{3}Z$/,'.'+String(fraction).padStart(9,'0')+'Z');}catch(_){return null;}
}
function packetVisibleTimeRange(scroll=document.querySelector('[data-packet-graph-scroll]'),statistics=model.statistics){const scope=packetPlotScope(statistics);return Number.isFinite(packetEpoch(scope.start))&&Number.isFinite(packetEpoch(scope.end))?{start:scope.start,end:scope.end,startRatio:0,endRatio:1}:null;}
function refreshPacketGraphAxis(){
 for(const section of document.querySelectorAll('[data-packet-timeline]')){const scroll=section.querySelector('[data-packet-graph-scroll]'),axis=section.querySelector('[data-packet-visible-axis]');if(!scroll||!axis)continue;const ticks=root.NetworkToolkitGraphHistory?.axisHTML?.(packetPlotScope(),Math.max(1,(scroll.querySelector('svg')?.getBoundingClientRect?.().width||scroll.clientWidth)-2),packetGraphPreferences().mode)||'';if(axis.innerHTML!==ticks)axis.innerHTML=ticks;}
 for(const button of document.querySelectorAll('[data-packet-graph-follow]')){const following=!model.graphHold&&packetTimelineState().follow;button.setAttribute('aria-pressed',String(following));button.dataset.followState=model.graphHold?'held':following?'following':'paused';button.title=(model.graphHold?'Selection held. ':following?'Following latest. ':'Time view paused. ')+'Follow the newest matching packet in graph and list without opening Details';}
}
function rememberPacketGraphScroll(){refreshPacketGraphAxis();}
function restorePacketGraphScroll(){refreshPacketGraphAxis();}
let packetViewportBusy=false,packetViewportQueued=false;
function packetViewportParameters(){const p=parameters(),scope=packetPlotScope(),pref=root.NetworkToolkitGraphHistory.preferences();p.set('graph_mode',packetGraphPreferences().mode);p.set('graph_start',scope.start);p.set('graph_end',scope.end);p.set('graph_recent_ms',String(pref.recentMs));if(model.graphOverview?.revision)p.set('graph_revision',String(model.graphOverview.revision));return p;}
async function requestPacketViewport(){
 if(packetViewportBusy){packetViewportQueued=true;return;}packetViewportBusy=true;
 try{do{packetViewportQueued=false;const selected=model.selected,query=packetViewportParameters().toString(),value=await request('/'+selected+'/graph?'+query);
  if(selected!==model.selected||query!==packetViewportParameters().toString())continue;
  if(Array.isArray(value.timeline)){model.statistics={...model.statistics,...value};model.graphZoomPending=false;if(model.view==='statistics')renderStatistics(model.statistics,true);else refreshPacketGraph();}
 }while(packetViewportQueued&&visible());}catch(e){if(visible())error(e.message);}finally{packetViewportBusy=false;}
}
function refreshPacketViewport(){model.graphZoomPending=true;if(model.view==='statistics')renderStatistics(model.statistics,true);else refreshPacketGraph();void requestPacketViewport();}
function panPacketGraph(amount){const h=root.NetworkToolkitGraphHistory,scope=packetPlotScope(),a=h?.instant?.(scope.start),b=h?.instant?.(scope.end);if(a==null||b==null)return;const shift=BigInt(Math.round(Number(b-a)*amount));navigatePacketWindow({start:h.timestamp(a+shift),end:h.timestamp(b+shift)});}
function zoomPacketGraph(action,{anchorShare,scope:anchorScope}={}){
 const h=root.NetworkToolkitGraphHistory,old=packetGraphZoom();if(action==='reset'){root.NetworkToolkitGraphNavigator.fit();return 1;}const next=h.zoomLevel(action==='in'?Math.min(Math.max(16,old),old*2):action==='out'?Math.max(.25,old/2):action,1e-18,1e24);if(next===old)return next;
 if(model.graphWindow||Number.isFinite(anchorShare)){const scope=anchorScope||packetPlotScope(),end=Number.isFinite(anchorShare)?h.anchoredEnd(scope,old,next,anchorShare,packetGraphPreferences().mode):scope.end,length=BigInt(Math.max(1,Math.round(Number(h.instant(scope.end)-h.instant(scope.start))*old/next)));model.graphBaseScope={...packetBaseScope()};navigatePacketWindow({start:h.timestamp(h.instant(end)-length),end});}
 else{state.packetGraphPreferences={...state.packetGraphPreferences,zoom:next};persistDetails();refreshPacketViewport();}return packetGraphZoom();
}


function renderStatistics(statistics,force=false){
 const area=node('packetCaptureAnalysis');if(!area)return;const previous=area.querySelector('.packet-summary-scroll'),scroll=previous?.scrollTop||0,focused=document.activeElement,point=focused?.closest?.('[data-packet-bucket]'),inside=point&&area.contains(point);rememberPacketGraphScroll();
 if(previous&&packetInspectionPaused(false)&&!force){model.pendingStatistics=statistics;return;}
 const selection=packetTimelineState();model.statistics=statistics;model.pendingStatistics=null;const selected=packetTimelineIndex(statistics,selection),markup=statisticsHTML(statistics);
 if(area._ntStatisticsMarkup===markup&&previous){updatePacketTimeline();return;}
 selection.hover=null;hidePacketGraphPopup();
 area._ntStatisticsMarkup=markup;const holder=area.cloneNode?.(false)||document.createElement('div');holder.innerHTML=markup;if(!root.NetworkToolkitGraphNavigator?.preserve?.(area,holder))area.innerHTML=markup;const reader=area.querySelector('.packet-summary-scroll');if(reader)reader.scrollTop=scroll;restorePacketGraphScroll();updatePacketTimeline();attachPacketNavigator();
 if(inside)area.querySelector('[data-packet-bucket="'+(selected??Math.min(Number(point.dataset.packetBucket),Math.max(0,(statistics.timeline||[]).length-1)))+'"]')?.focus?.({preventScroll:true});
}
function packetAverageBytes(value){const number=Number(value);return Number.isFinite(number)?number<1024?packetNumber(number,2)+' B':bytes(number):'Not measured';}
function packetGraphHeight(){const value=Number(state.packetGraphPreferences?.height);return Number.isFinite(value)?clamp(value,100,600):230;}
function packetListOverhead(){
 const panel=node('packetCaptureAnalysis')?.querySelector('.packet-list-panel'),table=node('packetTableScroll');
 const panelHeight=panel?.getBoundingClientRect?.().height,tableHeight=table?.getBoundingClientRect?.().height;
 return Number.isFinite(panelHeight)&&Number.isFinite(tableHeight)&&panelHeight>0&&tableHeight>0?Math.max(0,panelHeight-tableHeight):70;
}
function packetGraphMinimum(){
 const dock=node('packetGraphDock'),reader=node('packetGraphReader'),plot=reader?.querySelector('svg.packet-timeline'),frame=dock?.getBoundingClientRect?.(),body=reader?.getBoundingClientRect?.(),svg=plot?.getBoundingClientRect?.(),height=Number(reader?.clientHeight);
 if(!frame||!body||!svg||!Number.isFinite(frame.height)||frame.height<=0||!Number.isFinite(body.top)||!Number.isFinite(svg.top)||!Number.isFinite(svg.height)||svg.height<=0||!Number.isFinite(height)||height<0)return 100;
 // The saved height describes the pane. Its actual display must also fit the
 // permanent statistics, current wrapped controls and at least 90px of SVG.
 // Add the reader's scrollTop back to recover the natural plot offset.
 const chrome=Math.max(0,frame.height-height);
 return clamp(Math.ceil(chrome+(root.innerHeight>0&&root.innerHeight<=720?149:161)),146,600);
}
function layoutPacketGraph(){
 layoutPacketConfiguration();const dock=node('packetGraphDock'),area=node('packetCaptureAnalysis'),workspace=node('packetCaptureWorkspace');if(!dock||!area)return;
 const show=model.view==='packets'&&!!model.selected&&!dock.hidden;workspace?.classList.toggle('packet-has-traffic-graph',show);
 if(!show){area.style.minHeight='';dock.style.maxHeight='';dock.style.minHeight='';return;}
 const bounds=area.getBoundingClientRect(),bottom=preferences().dock==='bottom'||preferences().dock==='auto'&&(bounds.width||area.clientWidth||1000)<((model.graphHold||model.liveDetails)?760:1070);
 // Reserve actual row-list chrome as well as readable rows. The old fixed
 // panel reserve ignored wrapped toolbar/footer content on narrow windows.
 const collapsed=state.packetGraphPreferences?.collapsed===true,listHeight=collapsed&&Number.isFinite(model.graphCollapseListHeight)?Math.max(140,model.graphCollapseListHeight):140;
 const minimum=Math.ceil(packetListOverhead()+listHeight+(bottom?(preferences().hidden?36:86):0));
 if(state.packetGraphPreferences?.collapsed){dock.style.minHeight='0px';dock.style.maxHeight='none';dock.style.flexBasis='auto';area.style.minHeight=Math.ceil(minimum)+'px';return;}
 const graphMinimum=packetGraphMinimum(),available=Math.floor((dock.getBoundingClientRect?.().height||packetGraphHeight())+(bounds.height||area.clientHeight||minimum)),maximum=Math.max(graphMinimum,Math.min(600,available-minimum));
 area.style.minHeight=Math.ceil(minimum)+'px';dock.style.minHeight=graphMinimum+'px';dock.style.maxHeight=Math.round(maximum)+'px';dock.style.flexBasis=Math.round(clamp(packetGraphHeight(),graphMinimum,maximum))+'px';
 const divider=node('packetGraphDivider');if(divider){divider.setAttribute('aria-valuenow',String(Math.round(dock.getBoundingClientRect?.().height||packetGraphHeight())));divider.setAttribute('aria-valuemin',String(graphMinimum));divider.setAttribute('aria-valuemax',String(Math.round(maximum)));}
}
function refreshPacketGraph(){
 const dock=node('packetGraphDock');if(!dock)return;
 const show=model.view==='packets'&&!!model.selected;dock.hidden=!show;if(!show){dock.innerHTML='';dock._ntGraphMarkup='';layoutPacketGraph();return;}
 const stats=model.statistics||{timeline:[]},collapsed=state.packetGraphPreferences?.collapsed===true,markup=packetStatisticSummary(model.summaryStatistics||stats)+'<div class="packet-graph-heading"><strong>Traffic graph</strong>'+packetFollowHTML()+'<button type="button" class="btn small ghost" data-packet-graph-collapse aria-expanded="'+!collapsed+'" aria-controls="packetGraphReader">'+(collapsed?'Expand graph':'Collapse graph')+'</button></div>'+(collapsed?'':'<div id="packetGraphReader" class="packet-graph-reader" tabindex="0" role="region" aria-label="Packet graph and associated information">'+packetTrafficHTML(stats)+'</div><div id="packetGraphDivider" class="packet-graph-divider" role="separator" tabindex="0" aria-orientation="horizontal" aria-label="Resize packet graph and packet list" title="Drag to resize graph area. Arrow keys resize; Home resets."></div>'),old=node('packetGraphReader'),top=old?.scrollTop||0;
 rememberPacketGraphScroll();dock.classList.toggle('packet-graph-collapsed',collapsed);
 if(dock._ntGraphMarkup!==markup){const focus=document.activeElement,zoomFocus=focus?.dataset?.packetZoom,collapseFocus=focus?.hasAttribute?.('data-packet-graph-collapse');hidePacketGraphPopup();const holder=dock.cloneNode?.(false)||document.createElement('div');holder.innerHTML=markup;if(!root.NetworkToolkitGraphNavigator?.preserve?.(dock,holder))dock.innerHTML=markup;dock._ntGraphMarkup=markup;if(node('packetGraphReader'))node('packetGraphReader').scrollTop=top;if(zoomFocus)dock.querySelector('[data-packet-zoom="'+zoomFocus+'"]')?.focus?.({preventScroll:true});else if(collapseFocus)dock.querySelector('[data-packet-graph-collapse]')?.focus?.({preventScroll:true});}
 layoutPacketGraph();restorePacketGraphScroll();updatePacketTimeline();attachPacketNavigator();
}
function refreshTimeDisplay(){
 if(!visible())return;hidePacketGraphPopup();
 if(model.view==='packets'){
  const body=node('packetRows');if(body){const html=packetWindowRows(model.loaded,model.windowStart,model.total);body.innerHTML=html;body._ntRows=html;}
  const heading=document.querySelector('[data-packet-sort="time"]');if(heading)heading.textContent='Recorded ('+packetZone()+')'+(model.sort==='time'?' '+(model.direction==='asc'?'▲':'▼'):'');
  const detailTime=node('packetInspector')?.querySelector('.packet-recorded-time time');if(detailTime)detailTime.textContent=packetLocalTime(model.detail?.packet?.time);refreshPacketGraph();renderDetail();
 }else if(['statistics','conversations','endpoints'].includes(model.view)&&model.statistics)renderStatistics(model.statistics,true);
}
function packetInspectionPaused(includeHold=true){if(includeHold&&model.graphHold)return true;const selected=root.getSelection?.(),keyboardFocus=model.keyboardInspection&&document.activeElement?.closest?.('#packetRows, #packetInspector, .packet-list-actions, .packet-detail-filters');return model.inspectPointer||model.inspectFocus||!!keyboardFocus||!!packetMenu||!!(selected&&!selected.isCollapsed&&String(selected).trim());}
function packetReaderViewport(reader=node('packetTableScroll')){
 if(!reader)return null;const bounds=reader.getBoundingClientRect?.(),height=Number(reader.clientHeight);if(!bounds||!Number.isFinite(bounds.top)||!Number.isFinite(bounds.height)||height<=0)return null;
 const innerTop=bounds.top+(Number(reader.clientTop)||0);let top=innerTop,bottom=innerTop+height;
 if(Number.isFinite(root.innerHeight)&&root.innerHeight>0){top=Math.max(0,top);bottom=Math.min(root.innerHeight,bottom);}
 for(let parent=reader.parentElement;parent&&parent.tagName!=='DOCUMENT';parent=parent.parentElement){const style=root.getComputedStyle?.(parent);if(!style||!/(auto|scroll|hidden|clip)/.test(style.overflowY||style.overflow||''))continue;const rect=parent.getBoundingClientRect?.(),client=Number(parent.clientHeight);if(!rect||!Number.isFinite(rect.top)||client<=0)continue;const edge=rect.top+(Number(parent.clientTop)||0);top=Math.max(top,edge);bottom=Math.min(bottom,edge+client);}
 for(const header of reader.querySelectorAll('thead th')){const rect=header.getBoundingClientRect?.();if(rect&&rect.bottom>top&&rect.top<bottom)top=Math.max(top,rect.bottom);}
 const dock=node('packetGraphDock'),graph=dock&&!dock.hidden?dock.getBoundingClientRect?.():null;if(graph&&graph.top<=top&&graph.bottom>top&&graph.bottom>graph.top)top=Math.max(top,graph.bottom);
 return {top,bottom,height:Math.max(0,bottom-top),innerTop,topOffset:top-innerTop,bottomOffset:bottom-innerTop};
}
function ensurePacketRowVisible(number){
 const reader=node('packetTableScroll'),row=reader?.querySelector('[data-packet-number="'+number+'"]');if(!reader||!row||!packetReaderViewport(reader))return;
 const correction=()=>{const view=packetReaderViewport(reader),rect=row.getBoundingClientRect?.();if(!view||view.height<=0||!rect||!Number.isFinite(rect.height)||rect.height<=0)return 0;return rect.top<view.top?rect.top-view.top:rect.bottom>view.bottom?rect.bottom-view.bottom:0;};
 let delta=correction();if(Math.abs(delta)<.5)return;reader.scrollTop+=delta;delta=correction();
 // A compact workspace can clip the actual table viewport even when the
 // table's own scrollbar is already at its edge. Reveal through the owning
 // scrollable ancestors without focusing the row or touching either sort.
 for(let parent=reader.parentElement;parent&&Math.abs(delta)>=.5;parent=parent.parentElement){const style=root.getComputedStyle?.(parent);if(!style||!/(auto|scroll)/.test(style.overflowY||style.overflow||'')||Number(parent.scrollHeight)<=Number(parent.clientHeight))continue;parent.scrollTop+=delta;delta=correction();}
 model.scrollTop=Number(reader.scrollTop)||0;
}
async function followPacketLatest(force=false){
 if(!visible()||model.view!=='packets'||model.paused||!model.total||(!force&&(!model.follow||packetInspectionPaused())))return;
 const index=clamp(Number(model.newestIndex)||0,0,model.total-1),scroll=node('packetTableScroll');if(!scroll)return;
 const {rowHeight,headerHeight}=packetMetrics(),view=packetReaderViewport(scroll),bottomOffset=view?.height>0?view.bottomOffset:scroll.clientHeight||400,centerOffset=view?.height>0?(view.topOffset+view.bottomOffset)/2:(scroll.clientHeight||400)/2,position=model.sort==='number'?(model.direction==='desc'?0:Math.max(0,model.total*rowHeight+headerHeight-bottomOffset)):Math.max(0,index*rowHeight+headerHeight-centerOffset);
 model.scrollTop=position;scroll.scrollTop=position;const offset=windowOffset(model.total,position);if(offset!==model.windowStart)await refreshPacketWindow(offset);if(!visible()||model.view!=='packets')return;
 const current=node('packetTableScroll');if(current){current.scrollTop=position;model.scrollTop=current.scrollTop;const newest=model.loaded[index-model.windowStart];if(newest){ensurePacketRowVisible(newest.number);if(model.liveDetails)await selectPacket(newest.number,{hold:false});}updateMapViewport();}
}
function flushPacketInspection(){if(packetInspectionPaused())return;if(model.pendingStatistics){const statistics=model.pendingStatistics;model.pendingStatistics=null;if(model.view==='statistics')renderStatistics(statistics);else{model.statistics=statistics;refreshPacketGraph();}}if(model.selected&&model.view==='packets'&&!model.windowSignature.startsWith(packetSourceSignature()+'|'))void refreshPacketWindow(windowOffset()).then(()=>followPacketLatest());else void followPacketLatest();}
function revealPacketInterval(index){if(model.graphHold)return;
 const bucket=model.statistics?.timeline?.[index],reader=node('packetTableScroll');if(!bucket||!reader)return;
 const match=model.loaded.findIndex(packet=>packetIntervalContains(bucket,packet.time,index===model.statistics.timeline.length-1));
 if(match>=0){const {rowHeight,headerHeight}=packetMetrics(),position=(model.windowStart+match)*rowHeight+headerHeight,top=Number(reader.scrollTop)||0,height=reader.clientHeight||400;if(position<top+headerHeight||position+rowHeight>top+height){model.scrollTop=Math.max(0,position-height/2);reader.scrollTop=model.scrollTop;updateMapViewport();}ensurePacketRowVisible(model.loaded[match].number);return;}
 if(Number(bucket.packets)>0)void refreshPacketWindow(model.windowStart,bucket);
}
function endPacketInspection(){model.inspectPointer=false;packetTimelineState().hover=null;hidePacketGraphPopup();updatePacketTimeline();flushPacketInspection();}
document.addEventListener('click',event=>{
 if(!visible())return;
 const graph=event.target.closest?.('[data-packet-timeline]'),point=event.target.closest?.('[data-packet-bucket]');
 if(!graph&&!point&&!event.target.closest?.('#packetGraphPopup')){if(!event.target.closest?.('#packetRows, #packetInspector')){model.inspectFocus=false;flushPacketInspection();}}
 const zoom=event.target.closest?.('[data-packet-zoom]'),collapse=event.target.closest?.('[data-packet-graph-collapse]'),stat=event.target.closest?.('[data-packet-stat]');
 const pan=event.target.closest?.('[data-packet-pan]');if(pan){event.preventDefault();panPacketGraph(Number(pan.dataset.packetPan));return;}
 if(zoom){event.preventDefault();event.stopImmediatePropagation();zoomPacketGraph(zoom.dataset.packetZoom);return;}
 if(collapse){event.preventDefault();event.stopImmediatePropagation();const wasCollapsed=state.packetGraphPreferences?.collapsed===true,area=node('packetCaptureAnalysis'),inspector=node('packetInspector'),height=inspector?.getBoundingClientRect?.().height,configurationHeight=document.querySelector('.packet-configuration-region')?.getBoundingClientRect?.().height;
 // In an overflowing short workspace, flex growth alone cannot redistribute
 // the removed plot. Transfer its actual reader/divider height to the list
 // floor, so collapsing enlarges the visible reader instead of only reducing
 // outer overflow. This display allocation never changes saved pane sizes.
 const listHeight=node('packetTableScroll')?.getBoundingClientRect?.().height,readerHeight=node('packetGraphReader')?.getBoundingClientRect?.().height,dividerHeight=node('packetGraphDivider')?.getBoundingClientRect?.().height;
 model.graphCollapseListHeight=!wasCollapsed&&Number.isFinite(listHeight)&&listHeight>0&&Number.isFinite(readerHeight)&&readerHeight>0?listHeight+readerHeight+(Number.isFinite(dividerHeight)?Math.max(0,dividerHeight):0):null;
 model.graphCollapseConfigurationHeight=!wasCollapsed&&Number.isFinite(configurationHeight)?Math.max(32,configurationHeight):null;model.graphCollapseInspectorHeight=!wasCollapsed&&area?.classList.contains('packet-details-bottom')&&!preferences().hidden&&Number.isFinite(height)&&height>0?height:null;state.packetGraphPreferences={...state.packetGraphPreferences,collapsed:!wasCollapsed};persistDetails();refreshPacketGraph();layoutDetails();return;}
 if(stat){event.preventDefault();event.stopImmediatePropagation();model.statFilter=model.statFilter===stat.dataset.packetStat?'':stat.dataset.packetStat;setView('packets');resetPacketFilterWindow();return;}
 if(event.target.closest?.('[data-packet-clear-stat]')){model.statFilter='';resetPacketFilterWindow();}
 if(event.target.closest?.('[data-packet-clear-detail-filters]')){model.severityFilter='any';model.typeFilter='any';resetPacketFilterWindow();}
},true);
document.addEventListener('change',event=>{if(event.target.id==='packetSeverityFilter'||event.target.id==='packetTypeFilter'){model[event.target.id==='packetSeverityFilter'?'severityFilter':'typeFilter']=event.target.value;resetPacketFilterWindow();}});
document.addEventListener('pointerdown',()=>{model.keyboardInspection=false;model.inspectFocus=false;},true);
document.addEventListener('keydown',event=>{model.keyboardInspection=true;if(event.target.closest?.('#packetRows, #packetInspector, [data-packet-bucket]'))model.inspectFocus=true;},true);
document.addEventListener('focusin',event=>{if(model.keyboardInspection&&event.target.closest?.('#packetRows, #packetInspector, [data-packet-bucket]'))model.inspectFocus=true;});
document.addEventListener('focusout',event=>{if(event.target.closest?.('#packetRows, #packetInspector, [data-packet-bucket]')&&!event.relatedTarget?.closest?.('#packetRows, #packetInspector, [data-packet-bucket]')){model.inspectFocus=false;flushPacketInspection();}});
document.addEventListener('selectionchange',()=>{if(!packetInspectionPaused())flushPacketInspection();});
document.addEventListener('pointerover',event=>{
 const point=event.target.closest?.('[data-packet-bucket]'),row=event.target.closest?.('#packetRows [data-packet-number]');
 if(!model.graphNavigating&&point&&point!==event.relatedTarget?.closest?.('[data-packet-bucket]')){model.inspectPointer=true;const index=Number(point.dataset.packetBucket);selectPacketTimeline(index);showPacketGraphPopup(index,event,point);revealPacketInterval(index);}
 else if(row&&row!==event.relatedTarget?.closest?.('#packetRows [data-packet-number]')){model.inspectPointer=true;const packet=model.loaded.find(p=>String(p.number)===row.dataset.packetNumber),index=packetBucketForPacket(packet);if(index>=0){selectPacketTimeline(index);showPacketGraphPopup(index,event,row);}else{packetTimelineState().hover=null;hidePacketGraphPopup();updatePacketTimeline();}}
});
document.addEventListener('pointermove',event=>{const point=event.target.closest?.('[data-packet-bucket]');if(point)showPacketGraphPopup(Number(point.dataset.packetBucket),event,point);});
document.addEventListener('pointerout',event=>{
 const point=event.target.closest?.('[data-packet-bucket]'),row=event.target.closest?.('#packetRows [data-packet-number]');
 if(!model.graphNavigating&&point&&point!==event.relatedTarget?.closest?.('[data-packet-bucket]')||row&&row!==event.relatedTarget?.closest?.('#packetRows [data-packet-number]')){if(event.relatedTarget?.closest?.('[data-packet-bucket], #packetRows [data-packet-number]'))return;endPacketInspection();}
});
document.addEventListener('focusin',event=>{const point=event.target.closest?.('[data-packet-bucket]');if(point){const index=Number(point.dataset.packetBucket);selectPacketTimeline(index);showPacketGraphPopup(index,event,point);}});
document.addEventListener('focusout',event=>{if(event.target.closest?.('[data-packet-bucket]')){packetTimelineState().hover=null;hidePacketGraphPopup();updatePacketTimeline();}});
document.addEventListener('click',event=>{
 if(model.graphDragged){model.graphDragged=false;return;}const point=event.target.closest?.('[data-packet-bucket]'),clear=event.target.closest?.('[data-packet-timeline-clear]'),follow=event.target.closest?.('[data-packet-graph-follow], [data-packet-selection-latest]');if(!point&&!clear&&!follow)return;event.preventDefault();event.stopImmediatePropagation();
 if(point)selectPacketTimeline(Number(point.dataset.packetBucket),true);else if(clear)clearPacketGraphSelection();else void resumePacketLatest();
},true);
document.addEventListener('click',event=>{const contributor=event.target.closest?.('[data-packet-contributor]'),page=event.target.closest?.('[data-packet-interval-offset]');if(contributor){event.preventDefault();const number=Number(contributor.dataset.packetContributor);void refreshPacketWindow(model.windowStart,{packetNumber:number}).then(()=>selectPacket(number)).then(()=>ensurePacketRowVisible(number));}else if(page){event.preventDefault();void loadPacketInterval(Number(page.dataset.packetIntervalOffset));}else if(event.target.matches?.('svg.packet-timeline'))clearPacketGraphSelection();});
document.addEventListener('networkToolkitGraphTimeChange',event=>{
 const wasFollowing=packetTimelineState().follow&&!model.graphHold;if(event.detail?.fit){state.packetGraphPreferences={...state.packetGraphPreferences,zoom:1};persistDetails();}model.graphViewEnd='';model.graphBaseScope=null;model.graphWindow=null;model.graphBaseSpan='';
 if(event.detail?.fitAll){model.graphHeldEnd='';packetTimelineState().follow=wasFollowing;if(!wasFollowing&&root.NetworkToolkitGraphHistory.instant(packetNavigatorScope().start)!==null)model.graphWindow=packetNavigatorScope();}
 if(visible()){if(!event.detail?.fitAll&&!model.graphHold&&root.NetworkToolkitGraphHistory?.preferences?.().anchor!=='latest'){clearPacketGraphSelection();packetTimelineState().follow=false;}model.statisticsSource='';void refresh(true);}
});
document.addEventListener('change',event=>{if(event.target.matches?.('[data-packet-graph-mode]'))setPacketGraphMode(event.target.value);});
document.addEventListener('scroll',event=>{if(event.target.matches?.('[data-packet-graph-scroll]'))rememberPacketGraphScroll();},true);
document.addEventListener('network-toolkit-graph-preferences',()=>{if(!visible())return;if(model.view==='packets')refreshPacketGraph();else if(model.view==='statistics'&&model.statistics)renderStatistics(model.statistics,true);});
document.addEventListener('keydown',event=>{
 const point=event.target.closest?.('[data-packet-bucket]');if(!point||!['Enter',' ','ArrowLeft','ArrowRight','ArrowUp','ArrowDown','Home','End','Escape'].includes(event.key))return;event.preventDefault();event.stopImmediatePropagation();
 if(event.key==='Escape'){clearPacketGraphSelection();return;}
 const index=Number(point.dataset.packetBucket),last=(model.statistics?.timeline?.length||1)-1,next=event.key==='Home'?0:event.key==='End'?last:['ArrowLeft','ArrowUp'].includes(event.key)?Math.max(0,index-1):['ArrowRight','ArrowDown'].includes(event.key)?Math.min(last,index+1):index;
 selectPacketTimeline(next,true);document.querySelector('[data-packet-bucket="'+next+'"]')?.focus?.({preventScroll:true});
},true);
let packetGraphDrag=null;
document.addEventListener('pointerdown',event=>{if(event.target.id!=='packetGraphDivider'||event.button!==undefined&&event.button!==0)return;event.preventDefault();event.stopPropagation();packetGraphDrag={y:event.clientY,height:node('packetGraphDock').getBoundingClientRect?.().height||packetGraphHeight(),pointer:event.pointerId};event.target.setPointerCapture?.(event.pointerId);});
root.addEventListener?.('pointermove',event=>{if(!packetGraphDrag||packetGraphDrag.pointer!==undefined&&event.pointerId!==undefined&&event.pointerId!==packetGraphDrag.pointer)return;state.packetGraphPreferences={...state.packetGraphPreferences,height:clamp(packetGraphDrag.height+event.clientY-packetGraphDrag.y,packetGraphMinimum(),600)};layoutPacketGraph();layoutDetails();});
function finishPacketGraphDrag(){if(!packetGraphDrag)return;packetGraphDrag=null;persistDetails();}
for(const name of ['pointerup','pointercancel','blur'])root.addEventListener?.(name,finishPacketGraphDrag);
document.addEventListener('keydown',event=>{if(event.target.id!=='packetGraphDivider'||!['ArrowUp','ArrowDown','Home'].includes(event.key))return;event.preventDefault();event.stopImmediatePropagation();state.packetGraphPreferences={...state.packetGraphPreferences,height:event.key==='Home'?230:clamp(packetGraphHeight()+(event.key==='ArrowDown'?1:-1)*(event.shiftKey?40:15),packetGraphMinimum(),600)};layoutPacketGraph();layoutDetails();persistDetails();});
function options(values,current){return values.map(([value,text])=>'<option value="'+esc(value)+'" '+(value===current?'selected':'')+'>'+esc(text)+'</option>').join('');}
function request(path,method='GET',body){return api('/packet-captures'+path,method,body);}
function error(message){if(node('packetCaptureError'))node('packetCaptureError').textContent=message||'';}
function render(){
 const target=node('pageContent');if(!target)return;packetColorDialog?.close();closePacketMenu();rememberDisclosures();const opening=!node('packetCaptureWorkspace');model.insightsOpen=state.packetInsightsOpen!==false;
 target.innerHTML='<div id="packetCaptureWorkspace" class="packet-workspace">'+
 '<div class="packet-commandbar" role="toolbar" aria-label="Packet capture commands"><strong>'+icon('activity')+'Packet capture</strong><label class="sr-only" for="packetCaptureSelect">Retained capture</label><select id="packetCaptureSelect"><option value="">New capture</option></select><button class="btn small primary" data-packet-action="start">'+icon('play')+'Start capture</button><button class="btn small danger" data-packet-action="stop" disabled>'+icon('stop')+'Stop capture</button><button class="btn small ghost" data-packet-action="import">'+icon('upload')+'Open capture file</button><button class="btn small ghost" data-packet-action="clear">'+icon('trash')+'Delete capture</button><button class="btn small ghost" data-packet-action="help">'+icon('info')+'How to use</button><input id="packetCaptureFile" type="file" accept=".pcap,.pcapng,.cap" class="hidden"></div>'+
 '<div class="packet-configuration-region" role="region" aria-label="Capture setup and filters"><details id="packetCaptureSetup" class="packet-guide-setup" data-capture="'+esc(model.selected)+'" '+(!model.selected?'open':'')+'><summary>Capture setup and step-by-step guide</summary><div class="packet-guided-setup"><div class="field packet-interface"><label for="packetInterface">Network adapter to record</label><div class="row gap6"><select id="packetInterface"><option value="nics">All network adapters</option></select><button class="btn small ghost" data-packet-action="interfaces">Refresh adapters</button></div><div id="packetAdapterStatus" class="hint" role="status" aria-live="polite"></div></div><div class="field packet-task"><label for="packetTask">What are you troubleshooting?</label><select id="packetTask">'+options(Object.entries(tasks).map(([id,t])=>[id,t.label]),model.goal)+'</select></div><div class="field packet-duration"><label for="packetDuration">Stop after (seconds)</label><input id="packetDuration" type="number" min="1" max="7200" value="60"></div><p class="packet-adapter-hint">Choose Wi-Fi for wireless or Ethernet for a wired connection. This normally records this computer\'s traffic, not every device on the network.</p><div class="packet-task-hint"><p id="packetTaskHint"></p><button class="btn small ghost" data-packet-action="goal-filter">View relevant packets</button></div></div>'+
 '<ol class="packet-workflow" aria-label="Capture troubleshooting steps"><li><strong>1. Start capture</strong><span>Begin before the problem happens.</span></li><li><strong>2. Reproduce the problem</strong><span>Try the website, app or device once.</span></li><li><strong>3. Stop and review</strong><span>Read Troubleshooting, then inspect examples.</span></li></ol>'+
 '<details id="packetAdvancedCapture" class="packet-advanced"><summary>Advanced capture settings and limits</summary><div class="packet-setup"><div class="field"><label for="packetEngine">Recording method</label><select id="packetEngine">'+options([['auto','Automatic (recommended)'],['live','Live packet stream'],['file','Compatibility file capture']],'auto')+'</select><span class="hint">Compatibility capture shows packets after you stop.</span></div><div class="field"><label for="packetKeepProtocol">Only record protocol</label><select id="packetKeepProtocol">'+options([['any','All protocols'],['ipv4','IPv4'],['ipv6','IPv6'],...protocols.slice(1).map(p=>[p.toLowerCase(),p])],'any')+'</select></div><div class="field"><label for="packetKeepHost">Only record host / network</label><input id="packetKeepHost" placeholder="Any IP, or 192.0.2.0/24" autocomplete="off"></div><div class="field packet-ports"><label for="packetKeepPorts">Only record ports</label><input id="packetKeepPorts" placeholder="Any, or 53,443" autocomplete="off"></div><div class="field"><label for="packetMaxMB">Packet data / file limit (MiB)</label><input id="packetMaxMB" type="number" min="1" max="128" value="32"></div><div class="field"><label for="packetMaxPackets">Packet limit</label><input id="packetMaxPackets" type="number" min="1" max="200000" value="50000"></div><div class="field"><label for="packetSnapLen">Bytes per packet</label><select id="packetSnapLen">'+options([['9000','Full live packet (up to 9000)'],['128','128 bytes'],['256','256 bytes'],['1518','1518 bytes'],['0','Full file packet / 9000 live']],'9000')+'</select></div><div class="field"><label for="packetRing">When decoded storage is full</label><select id="packetRing">'+options([['stop','Stop capture'],['ring','Overwrite oldest packets']],'stop')+'</select></div><div class="field"><label for="packetDirection">Only record direction</label><select id="packetDirection">'+options([['any','Inbound and outbound'],['in','Inbound (live only)'],['out','Outbound (live only)']],'any')+'</select></div><div class="field"><label for="packetKeepMAC">Only record MAC address</label><input id="packetKeepMAC" placeholder="Any MAC"></div><div class="field"><label for="packetKeepVLAN">Only record VLAN ID</label><input id="packetKeepVLAN" type="number" min="0" max="4094" placeholder="Any VLAN"></div></div><p class="hint">Recording filters discard nonmatching packets. Leave them open for general troubleshooting. Display filters below only hide packets and can be cleared later. Compatibility mode records a bounded circular file and applies recording filters after Stop.</p></details></details>'+
 '<div id="packetCaptureAvailability" class="hint"></div><div id="packetCaptureError" class="packet-error" role="alert"></div><div id="packetCaptureStatus" class="packet-status" role="status" aria-live="polite">Open a capture file or start recording.</div>'+
 '<details class="packet-display-options"><summary>Find or filter packets</summary><div class="packet-filterbar"><div class="field packet-search"><label for="packetSearch">Search packets</label><input id="packetSearch" type="search" value="'+esc(model.query)+'" placeholder="Address, port, protocol or description"></div><div class="field packet-expression"><label for="packetDisplayFilter">Display filter (does not discard packets)</label><input id="packetDisplayFilter" value="'+esc(model.expression)+'" placeholder="tcp && ip.addr == 192.0.2.10" spellcheck="false"></div><button class="btn small primary" data-packet-action="filter">Show matching packets</button><button class="btn small ghost" data-packet-action="reset-filter">Show all packets</button><button class="btn small ghost" data-packet-action="filter-help">Filter help</button><div class="packet-preset-filters" role="group" aria-label="Common display filters">'+quickFilters.map(([f,label,hint])=>'<button class="btn small ghost" data-packet-filter="'+esc(f)+'" title="'+esc(hint)+'">'+label+'</button>').join('')+'</div><p id="packetFilterHint" class="packet-filter-hint hint" aria-live="polite"></p></div></details></div>'+
 '<div class="packet-viewbar"><div class="tabs" role="tablist" aria-label="Capture analysis">'+[['troubleshooting','Troubleshooting'],['packets','Packets'],['conversations','Conversations'],['endpoints','Devices / endpoints'],['statistics','Statistics']].map(([v,label])=>'<button class="tab '+(model.view===v?'active':'')+'" role="tab" aria-selected="'+(model.view===v)+'" data-packet-view="'+v+'">'+label+'</button>').join('')+'</div><label class="packet-check" title="Freezes display updates. Recording continues until you stop it."><input id="packetPauseDisplay" type="checkbox" '+(model.paused?'checked':'')+'>Pause updates</label><label class="sr-only" for="packetExportFormat">Export format</label><select id="packetExportFormat">'+options([['pcapng','PCAPNG'],['pcap','PCAP'],['csv','CSV summary'],['json','JSON report'],['txt','Text summary']],'pcapng')+'</select><button class="btn small ghost" data-packet-action="export">'+icon('download')+'Export matching packets</button><button class="btn small ghost" data-packet-action="export-all">Export all packets</button></div><section id="packetGraphDock" class="packet-graph-dock" hidden aria-label="Packet traffic graph and interval details"></section><div id="packetCaptureAnalysis" class="packet-analysis"></div></div>';
 restoreDisclosures();applyPacketAppearance();node('packetTask').value=model.goal;renderTask();renderFilterHint();model.sequence++;void initialize(opening);
}
function renderTask(){const task=tasks[model.goal]||tasks.general;if(node('packetTaskHint'))node('packetTaskHint').textContent=task.hint;const button=document.querySelector('[data-packet-action="goal-filter"]');if(button){button.disabled=!model.selected;button.textContent=model.goal==='general'?'View all packets':'View relevant packets';}}
function renderFilterHint(){const area=node('packetFilterHint');if(!area)return;const shortcut=quickFilters.find(([expression])=>expression===model.expression);area.textContent=shortcut?shortcut[2]:'Display filters and Search work together. Named shortcuts replace the current display focus; Show all packets clears it. Recording filters are not changed.';}
function selectCapture(id){if(id!==model.selected){model.graphWindow=null;model.graphBaseSpan='';model.graphOverview=null;model.graphOverviewKey='';model.graphNavigatorSource=null;model.graphViewEnd="";model.graphBaseScope=null;closePacketMenu();rememberDisclosures();model.report=null;model.reportStamp='';model.statistics=null;model.pendingStatistics=null;model.graphCollapseInspectorHeight=null;model.graphCollapseConfigurationHeight=null;model.graphCollapseListHeight=null;model.graphHold=false;model.graphHeldEnd='';model.heldPacketNumber=null;model.intervalSelection=null;model.liveDetails=false;model.summaryStatistics=null;model.summaryStatisticsSource='';model.typeChoices=[];model.inspectPointer=false;model.inspectFocus=false;model.statisticsSource='';model.pendingPacket=0;model.detail=null;model.windowToken++;model.scrollTop=0;model.windowStart=0;model.overview=null;model.overviewSignature="";model.overviewLayoutSignature="";model.windowSignature="";model.layoutSignature="";model.selectedIndex=-1;model.pendingIndex=null;const area=node('packetCaptureAnalysis');if(area){area._ntReviewMarkup='';area.innerHTML='<div class="packet-empty">Loading the selected capture...</div>';}}model.selected=id;if(id)model.newCapture=false;}
async function initialize(opening){try{if(!model.capabilities)model.capabilities=await request('/capabilities');if(!visible())return;node('packetCaptureAvailability').textContent=model.capabilities.note;if(!model.interfacesInitialized){model.interfaces=model.capabilities.interfaces||[];model.interfacesInitialized=true;}renderInterfaces();const adapters=opening&&model.capabilities.available?loadInterfaces():Promise.resolve();await refresh(true);await adapters;}catch(e){error(e.message)}}
function renderInterfaces(){const picker=node('packetInterface');if(!picker)return;
 const old=picker.value;if(old&&old!=='nics')model.interfaceSelection=old;
 const selected=model.interfaceSelection||old||'nics',engine=node('packetEngine')?.value||'auto',interfaces=model.interfaces.filter(i=>i.id==='nics'||engine==='auto'||i.engine==='auto'||i.engine===engine);
 model.interfaceUnavailable=selected!=='nics'&&!interfaces.some(i=>i.id===selected);
 picker.innerHTML=interfaces.length?options(interfaces.map(i=>[i.id,i.name+(i.engine==='file'?' (file)':i.engine==='live'?' (live)':'')]),selected):'<option value="">No configurable capture adapters</option>';
 if(model.interfaceUnavailable){picker.innerHTML+='<option value="'+esc(selected)+'">Selected adapter unavailable — choose an adapter</option>';picker.value=selected;}else if(!interfaces.some(i=>i.id===picker.value))picker.value=interfaces[0]?.id||'';
 picker.disabled=model.interfacesLoading||!interfaces.length;
 if(model.interfaceUnavailable&&node('packetAdapterStatus'))node('packetAdapterStatus').textContent='The selected adapter is unavailable for this recording method. Refresh adapters or explicitly choose another adapter. Capture will not switch to all adapters.';
}
function loadInterfaces(){if(model.interfacesPromise)return model.interfacesPromise;model.interfacesLoading=true;const button=document.querySelector('[data-packet-action="interfaces"]');if(button){button.disabled=true;button.textContent='Loading adapters...';}if(node('packetAdapterStatus'))node('packetAdapterStatus').textContent='Loading configurable capture adapters...';renderInterfaces();updateStatus();model.interfacesPromise=(async()=>{try{const data=await request('/interfaces','POST',{});model.interfaces=data.interfaces||[];renderInterfaces();if(node('packetAdapterStatus'))node('packetAdapterStatus').textContent=model.interfaceUnavailable?'The selected adapter is unavailable. Refresh adapters or explicitly choose another adapter; capture will not switch to all adapters.':model.interfaces.some(i=>i.id!=='nics')?'Adapters loaded without elevation. Capture support for the selected adapter is verified only after Start capture and Windows approval.':'No configurable adapters are available for capture. Check that the adapter is enabled in Windows.';}catch(e){if(node('packetAdapterStatus'))node('packetAdapterStatus').textContent='Could not load adapters: '+e.message+' Use Refresh adapters to try again.';}finally{model.interfacesLoading=false;model.interfacesPromise=null;renderInterfaces();const current=document.querySelector('[data-packet-action="interfaces"]');if(current){current.disabled=false;current.textContent='Refresh adapters';}updateStatus();}})();return model.interfacesPromise;}
function packetStatisticExpression(){return {bytes:'frame.len > 0',duration:'frame.time_recorded',expert:'expert',traffic:'frame.time_recorded',dnsrequests:'dns.request',dnsreplies:'dns.response',tcpstarts:'tcp.flags.syn && !tcp.flags.ack && !tcp.flags.fin && !tcp.flags.rst',tcpresets:'tcp.flags.rst',icmperrors:'icmp.error'}[model.statFilter]||'';}
function packetFilterExpression(includeStatistic=true){return [model.expression,model.severityFilter!=='any'?'frame.severity == "'+model.severityFilter+'"':'',model.typeFilter!=='any'?'protocol == '+quotedFilter(model.typeFilter):'',includeStatistic?packetStatisticExpression():''].filter(Boolean).map(value=>'('+value+')').join(' && ');}
function packetHasFilters(){return !!(model.expression||model.query||model.protocol!=='any'||model.severityFilter!=='any'||model.typeFilter!=='any'||model.statFilter);}
function parameters(includeStatistic=true){const p=new URLSearchParams(),expression=packetFilterExpression(includeStatistic);if(expression)p.set('filter',model.severityFilter==='any'&&model.typeFilter==='any'&&!packetStatisticExpression()?model.expression:expression);if(model.query)p.set('query',model.query);if(model.protocol!=='any')p.set('protocol',model.protocol);return p;}
function packetDetailFiltersHTML(){const types=[...new Set([...(model.typeChoices||[]),...(model.typeFilter!=='any'?[model.typeFilter]:[]),...Object.keys(model.summaryStatistics?.protocols||{}),...model.loaded.map(packet=>packet.protocol)].filter(Boolean))].sort();return '<div class="packet-detail-filters" role="group" aria-label="Detailed packet filters"><label>Severity<select id="packetSeverityFilter">'+options([['any','All severities'],['error','Decoder error'],['warn','Warning'],['note','Informational note'],['normal','No recorded clue']],model.severityFilter)+'</select></label><label>Packet type<select id="packetTypeFilter">'+options([['any','All packet types'],...types.map(type=>[type,type])],model.typeFilter)+'</select></label><button type="button" class="btn small ghost" data-packet-clear-detail-filters>Reset detail filters</button>'+(model.statFilter?'<span class="packet-stat-filter-label">Statistic: '+esc(packetStatisticLabels[model.statFilter])+'. '+esc(packetStatisticExplanation(model.statFilter))+'</span><button type="button" class="btn small ghost" data-packet-clear-stat>Clear statistic</button>':'')+'</div>';}
function resetPacketFilterWindow(){model.scrollTop=0;model.windowStart=0;model.windowToken++;model.sequence++;model.summaryStatistics=null;model.summaryStatisticsSource='';model.pendingStatistics=null;model.follow=true;if(node('packetFollow'))node('packetFollow').checked=true;void refresh(true);}

async function refresh(force=false){
 clearTimeout(model.timer);if(!visible())return;
 if(model.busy){model.refreshQueued=true;model.refreshQueuedForce=model.refreshQueuedForce||force;return model.refreshPromise;}
 model.busy=true;
 const operation=(async()=>{
  let nextForce=force;
  try{do{
   const currentForce=nextForce,seq=model.sequence;model.refreshQueued=false;model.refreshQueuedForce=false;
   try{const data=await request('');if(!visible()||seq!==model.sequence){nextForce=model.refreshQueuedForce;continue;}model.captures=data.captures||[];if(!model.newCapture&&!model.captures.some(s=>s.id===model.selected))selectCapture(model.captures[0]?.id||'');const picker=node('packetCaptureSelect'),signature=model.captures.map(s=>s.id+':'+s.name+':'+s.state).join('|');if(picker.dataset.signature!==signature){picker.innerHTML='<option value="">New capture</option>'+options(model.captures.map(s=>[s.id,s.name+' ('+s.state+')']),model.selected);picker.dataset.signature=signature;}picker.value=model.selected;updateStatus();if(!model.paused||currentForce)await analysis(seq,currentForce);if(visible()&&seq===model.sequence)error('');}
   catch(e){if(visible()&&seq===model.sequence)error(e.message);}
   nextForce=model.refreshQueuedForce;
  }while(visible()&&model.refreshQueued);}
  finally{model.busy=false;model.refreshPromise=null;model.refreshQueued=false;model.refreshQueuedForce=false;if(visible())model.timer=setTimeout(()=>void refresh(),model.captures.some(active)?1000:3000);}
 })();
 model.refreshPromise=operation;return operation;
}
function updateStatus(){if(!visible())return;const selected=model.captures.find(s=>s.id===model.selected),running=model.captures.find(active);for(const button of document.querySelectorAll('[data-packet-action="start"]'))button.disabled=!online||!model.capabilities?.available||model.startPending||model.interfaceUnavailable||model.interfacesLoading||!node('packetInterface')?.value||!!running;renderTask();const setup=node('packetCaptureSetup');if(setup&&setup.dataset.capture!==model.selected){setup.dataset.capture=model.selected;if(!model.setupManual){model.disclosures.setup=!model.selected;if(model.disclosures.setup)setup.setAttribute('open','');else setup.removeAttribute('open');}}node('packetCaptureStatus').textContent=selected?[active(selected)?(selected.engine==='file'?'Recording in compatibility mode. Reproduce the problem once, then Stop capture to load the packets.':'Recording. Reproduce the problem once, then Stop capture and read Troubleshooting.'):selected.state==='failed'?'Capture stopped with an error. Review the message below.':'Capture ready to review.',selected.state.toUpperCase(),selected.engine,selected.count.toLocaleString()+' retained packets',bytes(selected.bytes),selected.lost?selected.lost.toLocaleString()+' capture events missed':'',selected.overwritten?selected.overwritten.toLocaleString()+' overwritten':'',selected.note,selected.error].filter(Boolean).join(' | '):'Open a PCAP / PCAPNG file or start a new capture. Defaults: 60 seconds, 32 MiB, 50000 packets.';for(const button of document.querySelectorAll('[data-packet-action="stop"]'))button.disabled=!running;for(const button of document.querySelectorAll('[data-packet-action="clear"]'))button.disabled=!selected||active(selected);for(const button of document.querySelectorAll('[data-packet-action="export"],[data-packet-action="export-all"]'))button.disabled=!selected?.count;}
function optionRequest(){const int=id=>{const input=node(id);if(!input.checkValidity())throw new Error('Check '+(input.labels?.[0]?.textContent||input.closest('.field')?.querySelector('label')?.textContent||'the capture setting')+'.');return Number(input.value)};let ports=[];const raw=node('packetKeepPorts').value.trim();if(raw){ports=raw.split(/[ ,;]+/).map(s=>{if(!/^\d+$/.test(s)||Number(s)<1||Number(s)>65535)throw new Error('Keep ports must be numbers from 1 to 65535.');return Number(s)});if(ports.length>32)throw new Error('Use no more than 32 keep ports.');}const vlan=node('packetKeepVLAN').value;return {engine:node('packetEngine').value,interface:node('packetInterface').value,snapLen:Number(node('packetSnapLen').value),duration:int('packetDuration'),maxMB:int('packetMaxMB'),maxPackets:int('packetMaxPackets'),ring:node('packetRing').value==='ring',filter:{protocol:node('packetKeepProtocol').value,host:node('packetKeepHost').value.trim(),ports,mac:node('packetKeepMAC').value.trim(),vlan:vlan===''?null:int('packetKeepVLAN'),direction:node('packetDirection').value}};}
async function start(){if(model.startPending||model.interfaceUnavailable||model.captures.some(active)||model.interfacesLoading)return;model.startPending=true;updateStatus();try{const info=await request('','POST',optionRequest());setView('troubleshooting');selectCapture(info.id);model.page=0;model.scrollTop=0;model.windowStart=0;model.windowToken++;model.detail=null;model.sequence++;await refresh(true);}catch(e){error(e.message)}finally{model.startPending=false;updateStatus();}}
async function stop(){const session=model.captures.find(active);if(!session)return;try{await request('/'+session.id+'/stop','POST',{});await refresh(true)}catch(e){error(e.message)}}
const packetRowHeight=21,packetWindowStep=180,packetHeaderHeight=26;
function packetMetrics(){return {rowHeight:model.measuredRowHeight||packetRowHeight,headerHeight:model.measuredHeaderHeight||packetHeaderHeight};}
function measurePacketTable(){
 const list=node('packetTableScroll'),row=list?.querySelector('tbody tr[data-packet-number]'),head=list?.querySelector('thead');
 const height=row?.getBoundingClientRect?.().height||packetRowHeight,header=head?.getBoundingClientRect?.().height||packetHeaderHeight;
 if(height>=17&&height<=40)model.measuredRowHeight=height;
 if(header>=15&&header<=60)model.measuredHeaderHeight=header;
}
function packetSourceSignature(){const capture=model.captures.find(item=>item.id===model.selected);return [model.selected,capture?.revision??0,capture?.count??0,capture?.state??'',model.expression,model.query,model.protocol,model.severityFilter,model.typeFilter,model.statFilter,model.sort,model.direction].join('|');}
function packetLayoutSignature(){return [model.selected,model.expression,model.query,model.protocol,model.severityFilter,model.typeFilter,model.statFilter,model.sort,model.direction].join('|');}
function windowOffset(total=model.total,scroll=model.scrollTop){const {rowHeight,headerHeight}=packetMetrics(),index=Math.max(0,Math.floor((scroll-headerHeight)/rowHeight)-60);return Math.min(Math.max(0,total-1),Math.floor(index/packetWindowStep)*packetWindowStep);}
function cachePacketWindow(key,data){packetWindowCache.delete(key);packetWindowCache.set(key,data);while(packetWindowCache.size>packetCacheLimit)packetWindowCache.delete(packetWindowCache.keys().next().value);}
function packetWindowRows(packets,offset,total){const pad=height=>height?'<tr class="packet-spacer" aria-hidden="true"><td colspan="8" style="height:'+height+'px"></td></tr>':'';
 return pad(offset*packetRowHeight)+packets.map((packet,i)=>packetRowHTML(packet,offset+i)).join('')+pad(Math.max(0,total-offset-packets.length)*packetRowHeight);
}
function packetLevel(packet){if(['error','warn','note'].includes(packet.highlight))return packet.highlight;const notes=(packet.expert||[]).join(' ').toLowerCase(),info=(packet.info||'').toLowerCase();if(/malformed|invalid|incomplete/.test(notes))return 'error';if(/drop reason/.test(notes)||packet.protocol==='DNS'&&/servfail|refused/.test(info)||packet.protocol==='DHCP'&&/nak|decline/.test(info)||['ICMP','ICMPv6'].includes(packet.protocol)&&/unreachable|administratively prohibited/.test(info))return 'warn';if(packet.transport==='TCP'&&packet.tcpFlags&4||notes||packet.protocol==='DNS'&&/nxdomain/.test(info)||packet.transport==='TCP'&&/retransmission|out-of-order/.test(info))return 'note';return '';}
function packetRowHTML(p,index){const level=packetLevel(p),full=''+(p.expert||[]).join('; '),proto=String(p.protocol||'other').toLowerCase().replace(/[^a-z0-9]/g,''),color=level||((Object.hasOwn(packetPaletteDefaults,proto)?proto:'other'));return '<tr style="--packet-row-color:var(--packet-color-'+color+');--packet-row-contrast:var(--packet-contrast-'+color+')" tabindex="0" data-packet-number="'+p.number+'" data-packet-index="'+index+'" data-packet-time="'+esc(p.time??'')+'" class="packet-proto-'+esc(p.protocol.toLowerCase().replace(/[^a-z0-9]/g,''))+(level?' packet-highlight-'+level:'')+(model.detail?.packet.number===p.number?' selected':'')+'" aria-selected="'+(model.detail?.packet.number===p.number)+'"><td>'+p.number+'</td>'+packetTimeCell(p)+'<td title="'+esc(p.srcMAC||'')+'">'+esc(p.source)+(p.sourcePort?':'+p.sourcePort:'')+'</td><td title="'+esc(p.dstMAC||'')+'">'+esc(p.destination)+(p.destinationPort?':'+p.destinationPort:'')+'</td><td>'+esc(p.protocol)+'</td><td>'+p.length+(p.captured<p.length?'*':'')+'</td><td class="packet-signal-cell" title="'+esc(full||level||'')+'">'+(level?'<span class="packet-signal-dot packet-map-'+level+'" aria-label="'+esc(level)+'">'+(level==='error'?'!':level==='warn'?'!':'·')+'</span>':'')+'</td><td title="'+esc(p.info||'')+'">'+esc(p.info)+'</td></tr>';}
// One line per packet, 1 CSS pixel by default; user-selectable up to 4 px.
// Virtualize these lines so large captures never allocate all stripes.
const mapProtocolNames=['other','tcp','udp','dns','dhcp','arp','icmp','icmpv6','tls','http','ssh','mdns','smb','ntp','wireguard','other'];
const mapAlphabet='ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_';
function packetMapLineClass(index){
 const code=(model.overview?.packetMarks||'')[index],value=mapAlphabet.indexOf(code||'');
 if(value<0)return 'packet-map-normal';
 const severity=['normal','note','warn','error'][Math.floor(value/16)]||'normal';
 return 'packet-map-'+severity+' packet-map-proto-'+mapProtocolNames[value%16];
}
function detailMapHTML(){return '<div class="packet-map-detail-scroll" id="packetDetailMap" role="region" tabindex="0" aria-label="Packet-level traffic map, one line per packet. Drag the viewport one-to-one to move through frames." title="Detailed map: one line per filtered packet, 1 pixel by default. Drag the outlined viewport one-to-one with the mouse; click a packet line to select."><div class="packet-map-detail-content" id="packetDetailMapContent" style="height:'+Math.max(1,model.total*mapLineHeight())+'px"><div id="packetDetailMapLines" class="packet-map-detail-lines"></div><span id="packetDetailMapViewport" class="packet-map-detail-viewport"></span><span id="packetDetailMapSelected" class="packet-map-detail-selected" hidden></span></div></div>';}
function renderPacketDetailMap(force=false){
 const lane=node('packetDetailMap'),content=node('packetDetailMapContent'),lines=node('packetDetailMapLines');if(!lane||!content||!lines)return;
 const height=Math.max(1,model.total*mapLineHeight());if(content.style.height!==height+'px')content.style.height=height+'px';
 const visible=Math.max(1,Math.ceil((lane.clientHeight||420)/mapLineHeight()));
 const first=Math.min(Math.max(0,model.total-1),Math.max(0,Math.floor((Number(lane.scrollTop)||0)/mapLineHeight())-48));
 const start=Math.floor(first/48)*48,end=Math.min(model.total,start+visible+144),key=packetSourceSignature()+'|'+start+'|'+end;
 if(!force&&lane._mapRange===key)return;
 lane._mapRange=key;const row=[];
 for(let i=start;i<end;i++)row.push('<span class="packet-detail-line packet-map-bin '+packetMapLineClass(i)+'" data-map-index="'+i+'" style="top:'+(i*mapLineHeight())+'px"></span>');
 lines.innerHTML=row.join('');
}
function fineMapPosition(scroll,height=mapLineHeight()){
 // The table's header does not represent a packet. Preserve sub-packet
 // precision here so every map scale tracks the pointer smoothly.
 const {rowHeight,headerHeight}=packetMetrics();
 const first=clamp((scroll.scrollTop-headerHeight)/rowHeight,0,Math.max(0,model.total-1));
 const count=Math.max(1,scroll.clientHeight/rowHeight);
 return {top:first*height,height:Math.max(height,Math.min(model.total-first,count)*height)};
}
function syncPacketDetailMap(scroll){
 const lane=node('packetDetailMap'),overlay=node('packetDetailMapViewport'),selected=node('packetDetailMapSelected');if(!lane||!overlay||!selected)return;
 const position=fineMapPosition(scroll),laneHeight=lane.clientHeight||420;
 // Keep the fine-map viewport wherever the operator dragged it. Pan the
 // packet-level strip only if the outlined range actually leaves its bounds.
 // Recentering after every scroll makes the thumb appear stuck in place.
 if(!model.granularDragging){
  const current=Number(lane.scrollTop)||0,maxTop=Math.max(0,model.total*mapLineHeight()-laneHeight);
  const destination=clamp(position.top<current?position.top:position.top+position.height>current+laneHeight?position.top+position.height-laneHeight:current,0,maxTop);
  if(Math.abs(current-destination)>0.5){model.mapExpectedTop=destination;lane.scrollTop=destination;}
 }
 overlay.style.transform='translate3d(0,'+position.top+'px,0)';
 overlay.style.height=position.height+'px';
 selected.hidden=!(model.selectedIndex>=0&&model.selectedIndex<model.total);
 if(!selected.hidden)selected.style.transform='translate3d(0,'+(model.selectedIndex*mapLineHeight())+'px,0)';
 renderPacketDetailMap();
}
function detailMapIndexFromPoint(lane,clientY){const bounds=lane.getBoundingClientRect();return clamp(Math.floor(((Number(lane.scrollTop)||0)+clientY-bounds.top)/mapLineHeight()),0,Math.max(0,model.total-1));}
function centerPacketAt(index){
 if(model.total===0)return;const scroll=node('packetTableScroll'),{rowHeight,headerHeight}=packetMetrics();
 if(!scroll)return;setPacketScroll((index+.5)*rowHeight+headerHeight-(scroll.clientHeight||400)/2);
}
function packetOverviewHTML(){const overview=model.overview||{},bins=Array.isArray(overview.bins)?overview.bins:[],signals=Array.isArray(overview.signals)?overview.signals:[];
 const scale=overviewLineHeight(),scaled=scale>0;
 const map='<div class="packet-intelligent-scroll'+(scaled?' packet-overview-scaled':'')+'" id="packetOverviewMap" tabindex="0" role="slider" aria-label="Full-capture traffic map. Drag the viewport to scroll; optionally display every packet at its selected pixel scale." aria-valuemin="0" aria-valuemax="'+Math.max(0,model.total-1)+'" aria-valuenow="'+Math.max(0,Math.min(model.total-1,Math.floor(model.scrollTop/packetRowHeight)))+'" title="Full capture map: drag the viewport to scroll; click a line to select."><div id="packetOverviewContent" class="packet-overview-content"'+(scaled?' style="height:'+Math.max(1,model.total*scale)+'px"':'')+'><div class="packet-map-bars">'+bins.map(bin=>'<span class="packet-map-bin packet-map-'+(bin.level||'normal')+' packet-map-proto-'+esc(Object.hasOwn(packetPaletteDefaults,String(bin.protocol||'other').toLowerCase().replace(/[^a-z0-9]/g,''))?String(bin.protocol).toLowerCase().replace(/[^a-z0-9]/g,''):'other')+'" title="'+esc((bin.interesting||0)+' highlighted events; '+(bin.protocol||'traffic'))+'"></span>').join('')+'</div><div id="packetOverviewLines" class="packet-map-overview-lines"></div><span id="packetMapViewport" class="packet-map-viewport"></span><span id="packetMapSelected" class="packet-map-selected" hidden></span></div></div>'+detailMapHTML();
 const signalsHTML=signals.length?signals.map(f=>'<button type="button" class="packet-insight-event packet-level-'+esc(f.severity)+'" data-packet-jump="'+Number(f.index)+'" data-packet-number="'+f.number+'" title="'+esc(f.explanation)+'"><span class="packet-insight-event-top"><span class="packet-signal-dot packet-map-'+esc(f.severity)+'">'+(f.severity==='error'?'!':f.severity==='warn'?'!':'·')+'</span><strong>'+esc(f.title)+'</strong><span class="packet-insight-count">'+f.count.toLocaleString()+'</span></span><span class="packet-insight-explanation">'+esc(f.explanation)+'</span><span class="packet-insight-location">Jump to frame '+f.number+'</span></button>').join(''):'<p class="packet-insight-empty">No packet-level events were highlighted by the supported decoder. This does not establish that the traffic is healthy.</p>';
 return map+'<aside class="packet-insights" id="packetInsights" '+(!model.insightsOpen?'hidden':'')+' aria-label="Interesting packet events"><div class="packet-insights-heading"><strong>Interesting traffic</strong><button type="button" class="iconbtn" data-packet-action="toggle-insights" aria-label="Hide packet insights" title="Hide insights">×</button></div><div class="packet-insight-totals"><strong>'+(overview.interesting||0).toLocaleString()+'</strong> highlighted packets <span class="packet-insight-levels">'+(overview.counts?.error||0)+' errors · '+(overview.counts?.warn||0)+' warnings · '+(overview.counts?.note||0)+' notes</span></div><div class="packet-insights-list">'+signalsHTML+'</div><p class="packet-insight-caution">Clues from decoded, displayed packets, not a diagnosis. See Troubleshooting for broader request/reply analysis.</p></aside>';
}
function emptyPacketsHTML(retained,filtered){
 if(!retained)return '<div class="packet-empty packet-no-results"><strong>No decoded packets are retained yet.</strong><p>For live recording, reproduce the issue on the selected adapter. Compatibility file recording loads packets after Stop capture. A completed empty recording may have missed the traffic or excluded it with recording filters.</p></div>';
 return '<div class="packet-empty packet-no-results"><strong>'+(filtered?'No packets match these display filters.':'No packets are available in the selected window.')+'</strong><p>'+(filtered?'Use Show all packets above to remove Search and display restrictions. All '+retained.toLocaleString()+' retained packets remain in the recording.':'Scroll to another part of the capture.')+'</p>'+((model.goal==='dhcp'||model.expression===dhcpFilter)?'<p>For DHCP, capture before an address request or renewal with full packet length. An existing lease, a static address or IPv6 SLAAC may produce no DHCP traffic. No matching packet does not prove address assignment failed.</p>':'')+'</div>';
}
function tableHTML(packets,offset=0){const filtered=packetHasFilters(),retained=model.captures.find(capture=>capture.id===model.selected)?.count||0,focus=[model.expression,model.query?'Search: '+model.query:'',model.protocol!=='any'?'Protocol: '+model.protocol:'',model.severityFilter!=='any'?'Severity: '+model.severityFilter:'',model.typeFilter!=='any'?'Type: '+model.typeFilter:'',model.statFilter?'Statistic: '+packetStatisticLabels[model.statFilter]:''].filter(Boolean).join(' | ');return packetDetailFiltersHTML()+'<div class="packet-list-scope" role="status"><span><strong>'+model.total.toLocaleString()+'</strong> displayed of <span class="packet-retained-count">'+retained.toLocaleString()+'</span> retained packets · <span class="packet-list-window">Continuous scroll</span>'+(filtered?'<span class="packet-active-filter">Filtered view: '+esc(focus)+'</span>':'')+'</span><div class="packet-list-actions">'+(filtered?'<button class="btn small ghost" data-packet-action="reset-filter">Show all</button>':'')+'<button class="btn small ghost" type="button" data-packet-action="toggle-insights" aria-expanded="'+model.insightsOpen+'" aria-controls="packetInsights">'+(model.insightsOpen?'Hide insights':'Show insights')+'</button><button class="btn small ghost" type="button" data-packet-action="toggle-guide" aria-expanded="'+model.guideOpen+'" aria-controls="packetQuickGuide">What to look for</button><button class="btn small ghost" type="button" data-packet-action="toggle-appearance" aria-expanded="'+model.appearanceOpen+'" aria-controls="packetAppearanceSettings">Colors & maps</button></div></div>'+packetGuideHTML()+appearancePanelHTML()+'<div class="packet-list-body"><div id="packetTableScroll" class="packet-table-scroll" tabindex="0" role="region" aria-label="Captured packets, continuous scroll"><table class="packet-table"><colgroup><col class="col-no"><col class="col-time"><col class="col-source"><col class="col-destination"><col class="col-protocol"><col class="col-length"><col class="col-severity"><col class="col-info"></colgroup><thead><tr>'+[['number','No.'],['time','Recorded ('+packetZone()+')'],['source','Source'],['destination','Destination'],['protocol','Protocol'],['length','Length'],['severity','!'],['info','Info']].map(([k,label])=>'<th><button '+(k==='severity'?'title="Highlight: decoder error, warning or note" disabled':'data-packet-sort="'+k+'"')+'>'+label+(model.sort===k?' '+(model.direction==='asc'?'▲':'▼'):'')+'</button></th>').join('')+'</tr></thead><tbody id="packetRows">'+packetWindowRows(packets,offset,model.total)+'</tbody></table>'+(packets.length?'':emptyPacketsHTML(retained,filtered))+'</div>'+packetOverviewHTML()+'</div><div class="packet-list-footer"><span>'+packets.length.toLocaleString()+' rendered of '+model.total.toLocaleString()+' matching · scroll to load more</span><button class="btn small ghost" data-packet-action="scroll-first">Top</button></div>';
}
function renderPacketOverviewLines(force=false){
 const lane=node('packetOverviewMap'),lines=node('packetOverviewLines'),height=overviewLineHeight();if(!lane||!lines||!height)return;
 const visible=Math.max(1,Math.ceil((lane.clientHeight||420)/height));
 const first=Math.min(Math.max(0,model.total-1),Math.max(0,Math.floor((Number(lane.scrollTop)||0)/height)-48));
 const start=Math.floor(first/48)*48,end=Math.min(model.total,start+visible+144),key=packetSourceSignature()+'|'+height+'|'+start+'|'+end;
 if(!force&&lane._mapRange===key)return;
 lane._mapRange=key;const rows=[];
 for(let i=start;i<end;i++)rows.push('<span class="packet-overview-line packet-map-bin '+packetMapLineClass(i)+'" data-overview-index="'+i+'" style="top:'+(i*height)+'px"></span>');
 lines.innerHTML=rows.join('');
}
function updateOverviewScale(force=false){
 const bar=node('packetOverviewMap'),content=node('packetOverviewContent');if(!bar||!content)return;
 const scale=overviewLineHeight(),scaled=scale>0,wasScaled=bar.classList.contains('packet-overview-scaled');
 bar.classList.toggle('packet-overview-scaled',scaled);
 content.style.height=scaled?Math.max(1,model.total*scale)+'px':'';
 if(force||wasScaled!==scaled){bar._mapRange='';bar._selectedIndex=-1;model.overviewMapExpectedTop=-1;}
 if(!scaled){if(bar.scrollTop)bar.scrollTop=0;const lines=node('packetOverviewLines');if(lines)lines.innerHTML='';return;}
 renderPacketOverviewLines(force||wasScaled!==scaled);
}
function syncOverviewMap(scroll){
 const bar=node('packetOverviewMap'),overlay=node('packetMapViewport'),scale=overviewLineHeight();if(!bar||!overlay||!scale)return;
 const position=fineMapPosition(scroll,scale),laneHeight=bar.clientHeight||420;
 // Preserve the operator's thumb position. Pan the strip only when its visible
 // packet range moves out of the map viewport.
 if(!model.overviewDragging){
  const current=Number(bar.scrollTop)||0,maxTop=Math.max(0,model.total*scale-laneHeight);
  const destination=clamp(position.top<current?position.top:position.top+position.height>current+laneHeight?position.top+position.height-laneHeight:current,0,maxTop);
  if(Math.abs(destination-current)>0.5){model.overviewMapExpectedTop=destination;bar.scrollTop=destination;}
 }
 overlay.style.transform='translate3d(0,'+position.top+'px,0)';overlay.style.height=position.height+'px';
 renderPacketOverviewLines();
}
function overviewMapIndexFromPoint(bar,clientY){const rect=bar.getBoundingClientRect();return clamp(Math.floor(((Number(bar.scrollTop)||0)+clientY-rect.top)/Math.max(1,overviewLineHeight())),0,Math.max(0,model.total-1));}
function updateMapSelection(){
 const marker=node('packetMapSelected'),bar=node('packetOverviewMap');if(!marker||!bar)return;
 if(model.selectedIndex<0||model.selectedIndex>=model.total){marker.hidden=true;const detail=node('packetDetailMapSelected');if(detail)detail.hidden=true;bar._selectedIndex=-1;return;}
 marker.hidden=false;const height=bar.clientHeight||bar.getBoundingClientRect().height||1;
 const scale=overviewLineHeight();if(bar._selectedIndex===model.selectedIndex&&bar._selectedHeight===height&&bar._selectedScale===scale)return;
 bar._selectedIndex=model.selectedIndex;bar._selectedHeight=height;bar._selectedScale=scale;
 marker.style.transform='translate3d(0,'+(scale?model.selectedIndex*scale:Math.round(clamp((model.selectedIndex+.5)/Math.max(1,model.total)*height-1,0,Math.max(0,height-2))))+'px,0)';
 marker.title='Selected packet position: '+(model.selectedIndex+1)+' of '+model.total;
 const detail=node('packetDetailMapSelected');if(detail){detail.hidden=false;detail.style.transform='translate3d(0,'+(model.selectedIndex*mapLineHeight())+'px,0)';}
}
function updateMapViewport(){
 const bar=node('packetOverviewMap'),overlay=node('packetMapViewport'),scroll=node('packetTableScroll');if(!bar||!overlay||!scroll)return;
 // The highlighted rectangle represents the entire visible packet-list range,
 // not a fixed 9 px stripe. Its travel range is H - its own height.
 const barHeight=bar.clientHeight||bar.getBoundingClientRect().height||1,content=Math.max(1,scroll.scrollHeight),visibleHeight=Math.max(0,scroll.clientHeight),maximum=Math.max(0,content-visibleHeight);
 if(overviewLineHeight()){syncOverviewMap(scroll);}
 else{const indicatorHeight=Math.min(barHeight,Math.max(10,barHeight*visibleHeight/content)),travel=Math.max(0,barHeight-indicatorHeight),ratio=maximum?clamp(scroll.scrollTop/maximum,0,1):0;
  if(overlay.style.height!==indicatorHeight+'px')overlay.style.height=indicatorHeight+'px';
  overlay.style.transform='translate3d(0,'+Math.round(travel*ratio)+'px,0)';}
 const {rowHeight,headerHeight}=packetMetrics(),center=clamp(Math.floor((scroll.scrollTop+visibleHeight/2-headerHeight)/rowHeight),0,Math.max(0,model.total-1));
 bar.setAttribute('aria-valuenow',String(model.selectedIndex>=0?model.selectedIndex:center));
 updateMapSelection();syncPacketDetailMap(scroll);
}
async function completePacketSelection(){
 const target=model.pendingPacket?model.loaded.find(p=>p.number===model.pendingPacket):(model.pendingIndex!==null?model.loaded[model.pendingIndex-model.windowStart]:null);
 if(!target)return;
 model.pendingPacket=0;model.pendingIndex=null;await selectPacket(target.number);
}
function setPacketScroll(position,selectNumber=0,selectIndex=null){
 const scroll=node('packetTableScroll');if(!scroll)return;
 model.follow=false;if(selectNumber){model.pendingPacket=selectNumber;model.pendingIndex=null;}else if(selectIndex!==null){model.pendingIndex=selectIndex;model.pendingPacket=0;model.selectedIndex=selectIndex;}
 scroll.scrollTop=clamp(position,0,scroll.scrollHeight?Math.max(0,scroll.scrollHeight-scroll.clientHeight):Math.max(0,model.total*packetRowHeight));model.scrollTop=scroll.scrollTop;
 updateMapViewport();const target=windowOffset();
 if(target!==model.windowStart)void refreshPacketWindow(target);else void completePacketSelection();
}
function packetJump(index,number){
 if(!Number.isFinite(index)||index<0||index>=model.total)return;
 const scroll=node('packetTableScroll'),{rowHeight,headerHeight}=packetMetrics(),visibleHeight=scroll?.clientHeight||400;
 // Center the frame represented by the clicked map coordinate. The frame's
 // map marker should align with the middle of the viewport rectangle.
 const desired=index*rowHeight+headerHeight+rowHeight/2-visibleHeight/2;
 setPacketScroll(desired,number||0,number?null:index);
}
function packetOverviewRefresh(){
 const map=node('packetOverviewMap'),insights=node('packetInsights');if(!map||!insights)return;
 // Only the overview elements change. The scroll element and its packet rows
 // must remain mounted across window advances and live capture polling.
 const temp=document.createElement('div');temp.innerHTML=packetOverviewHTML();
 const freshMap=temp.querySelector('#packetOverviewMap'),bars=map.querySelector('.packet-map-bars'),freshBars=freshMap?.querySelector('.packet-map-bars');
 if(bars&&freshBars)bars.innerHTML=freshBars.innerHTML;map._mapRange='';updateOverviewScale(true);const lane=node('packetDetailMap');if(lane)lane._mapRange='';
 map.setAttribute('aria-valuemax',String(Math.max(0,model.total-1)));
 const freshInsights=temp.querySelector('#packetInsights');if(freshInsights)insights.innerHTML=freshInsights.innerHTML;
 insights.hidden=!model.insightsOpen;
 updateMapViewport();renderPacketDetailMap(true);
}
// Overview building is deliberately independent of packet-window delivery.
// A large capture can take time to filter/analyze; it must not block scrolling
// or the live Follow new packets path.
function requestPacketOverview(){
 if(!visible()||!model.selected||model.view!=='packets')return;
 const settings=mapSettings();if(!settings.showOverview&&!settings.showDetail&&!model.insightsOpen)return;
 const signature=packetSourceSignature(),layout=packetLayoutSignature();
 if(model.overviewSignature===signature||overviewInFlight)return;
 const recording=model.captures.find(c=>c.id===model.selected),now=Date.now();
 if(recording&&active(recording)&&model.overview&&model.overviewLayoutSignature===layout&&now-lastOverviewFetch<2500)return;
 const id=model.selected,sequence=model.sequence,params=parameters();
 params.set('sort',model.sort);params.set('direction',model.direction);
 overviewInFlight=signature;lastOverviewFetch=now;
 void request('/'+id+'/overview?'+params).then(value=>{
  if(!visible()||id!==model.selected||sequence!==model.sequence||layout!==packetLayoutSignature())return;
  model.overview=value;model.overviewSignature=signature;model.overviewLayoutSignature=layout;
  packetOverviewRefresh();
 }).catch(e=>{if(visible()&&id===model.selected&&layout===packetLayoutSignature())error('Traffic map could not update: '+e.message);})
 .finally(()=>{if(overviewInFlight===signature)overviewInFlight=null;});
}
function updatePacketListCounts(){const panel=node('packetCaptureAnalysis')?.querySelector('.packet-list-panel');if(!panel)return;const count=panel.querySelector('.packet-list-scope strong'),retained=panel.querySelector('.packet-retained-count'),footer=panel.querySelector('.packet-list-footer span');if(count)count.textContent=model.total.toLocaleString();if(retained)retained.textContent=(model.captures.find(capture=>capture.id===model.selected)?.count||0).toLocaleString();if(footer)footer.textContent=model.loaded.length.toLocaleString()+' rendered of '+model.total.toLocaleString()+' matching · scroll to load more';}
async function refreshPacketWindow(offset,locate=null,metadataOnly=false){
 if(!visible()||!model.selected||model.view!=='packets')return;
 const seq=model.sequence,source=packetSourceSignature(),key=source+'|'+offset+(locate?'|'+packetBucketKey(locate):''),layout=packetLayoutSignature(),area=node('packetCaptureAnalysis'),panel=area?.querySelector('.packet-list-panel');
 if(key===model.windowSignature&&panel?.querySelector('#packetTableScroll')){void completePacketSelection();updateMapViewport();return;}
 const token=++model.windowToken,params=parameters();params.set('offset',String(offset));params.set('limit',String(model.limit));params.set('sort',model.sort);params.set('direction',model.direction);
 // Recomputing the full-capture map on each 200-packet window is expensive.
 // The map is requested when a filter, sort, or capture revision changes.
 params.set('overview','0');if(locate?.packetNumber)params.set('locate_number',String(locate.packetNumber));else if(locate){params.set('locate_start',locate.time);params.set('locate_end',locate.endTime);if(locate.endInclusive||locate._includesEnd||locate===model.statistics?.timeline?.at(-1))params.set('locate_end_inclusive','1');}
 try{
  let data=packetWindowCache.get(key);
  if(!data){let pending=packetWindowInFlight.get(key);
   if(!pending){pending=request('/'+model.selected+'/packets?'+params);packetWindowInFlight.set(key,pending);
    // If the user scrolls faster than the response, reuse the in-flight
    // request for that range rather than launching it repeatedly.
    void pending.finally(()=>{if(packetWindowInFlight.get(key)===pending)packetWindowInFlight.delete(key);}).catch(()=>{});
   }
   data=await pending;
  }
  if(!visible()||seq!==model.sequence||token!==model.windowToken||model.view!=='packets')return;if(locate?.selectionVersion!=null&&locate.selectionVersion!==model.graphSelectionVersion)return;if(locate&&!locate.packetNumber&&!locate.selectionLocate&&(!model.inspectPointer||packetBucketKey(model.statistics?.timeline?.[packetTimelineState().hover])!==packetBucketKey(locate)))return;
  cachePacketWindow(key,data);model.total=data.total;model.newestIndex=Number.isInteger(data.newestIndex)?data.newestIndex:model.sort==='number'?(model.direction==='desc'?0:model.total-1):model.newestIndex;
  if(metadataOnly){updatePacketListCounts();return;}
  if(data.total>0&&data.offset>=data.total){model.scrollTop=Math.max(0,(data.total-1)*packetRowHeight);return refreshPacketWindow(windowOffset(data.total,model.scrollTop));}
  const overviewChanged=!!data.overview&&(source!==model.overviewSignature);if(data.overview){model.overview=data.overview;model.overviewSignature=source;lastOverviewFetch=Date.now();}
  model.windowStart=data.offset;model.loaded=data.packets||[];model.windowSignature=key;if(locate&&Number.isInteger(data.locateIndex)&&data.locateIndex>=0){const reader=node('packetTableScroll'),{rowHeight,headerHeight}=packetMetrics();model.scrollTop=Math.max(0,data.locateIndex*rowHeight+headerHeight-(reader?.clientHeight||400)/2);}
  if(!node('packetInspector')||!area.querySelector('.packet-list-panel')){area.innerHTML='<div class="packet-list-panel"></div><div id="packetDetailsDivider" role="separator" tabindex="0" aria-controls="packetInspector"></div><aside id="packetInspector" class="packet-inspector" aria-label="Packet details"></aside><div id="packetDetailsRail" class="packet-details-rail" hidden><button class="iconbtn" type="button" data-packet-action="toggle-details" aria-controls="packetInspector"></button></div>';}
  const target=area.querySelector('.packet-list-panel'),rebuild=target.dataset.layout!==layout||!target.querySelector('#packetTableScroll');
  const filterFocus=rebuild&&target.contains(document.activeElement)&&['packetSeverityFilter','packetTypeFilter'].includes(document.activeElement?.id)?document.activeElement.id:'';
  if(rebuild){target.innerHTML=tableHTML(model.loaded,model.windowStart);target.dataset.layout=layout;model.layoutSignature=layout;const tbody=target.querySelector('#packetRows');if(tbody)tbody._ntRows=tbody.innerHTML;}
  else{
   // Updating just tbody keeps the scroll owner and intelligent bar alive.
   const tbody=target.querySelector('#packetRows'),newRows=packetWindowRows(model.loaded,model.windowStart,model.total);
   if(tbody&&tbody._ntRows!==newRows){tbody.innerHTML=newRows;tbody._ntRows=newRows;}
   const footer=target.querySelector('.packet-list-footer span');if(footer)footer.textContent=model.loaded.length.toLocaleString()+' rendered of '+model.total.toLocaleString()+' matching · scroll to load more';
   const scope=target.querySelector('.packet-list-scope strong');if(scope)scope.textContent=model.total.toLocaleString();
   const retained=target.querySelector('.packet-retained-count');if(retained)retained.textContent=(model.captures.find(capture=>capture.id===model.selected)?.count||0).toLocaleString();
   if(overviewChanged)packetOverviewRefresh();
   const reader=target.querySelector('#packetTableScroll'),empty=reader?.querySelector('.packet-no-results');
   if(model.loaded.length){empty?.remove();}
   else if(reader){
    const retained=model.captures.find(c=>c.id===model.selected)?.count||0,filtered=packetHasFilters(),message=emptyPacketsHTML(retained,filtered);
    if(empty){if(empty._ntMessage!==message){const container=document.createElement('div');container.innerHTML=message;empty.innerHTML=container.firstElementChild?.innerHTML||'';empty._ntMessage=message;}}
    else{const container=document.createElement('div');container.innerHTML=message;reader.append(container.firstElementChild);}
   }
  }
  applyPacketAppearance();updatePacketGraphRows();
  if(rebuild)measurePacketTable();
  const reader=node('packetTableScroll');if(reader){
   // A virtual-row swap must not reset scrollTop or replace this element.
   if(rebuild||locate)reader.scrollTop=model.scrollTop;
   if(locate){const linked=model.loaded.find(packet=>locate.packetNumber?packet.number===locate.packetNumber:packetIntervalContains(locate,packet.time,locate===model.statistics?.timeline?.at(-1)));if(linked)ensurePacketRowVisible(linked.number);}
   model.scrollTop=reader.scrollTop;
  }
  updateMapViewport();if(rebuild){renderDetail();layoutDetails();if(filterFocus)node(filterFocus)?.focus?.({preventScroll:true});}await completePacketSelection();
  requestPacketOverview();
 }catch(e){if(seq===model.sequence&&token===model.windowToken)error(e.message);}
}
async function analysis(seq=model.sequence,force=false){
 const area=node('packetCaptureAnalysis');if(!area)return;rememberDisclosures();area.classList.toggle('packet-analysis-summary',model.view!=='packets');layoutDetails();
 if(!model.selected){refreshPacketGraph();if(!area.querySelector('.packet-welcome')){area.innerHTML='<div class="packet-empty packet-welcome"><h2>Start with the problem you want to reproduce</h2><p>Choose the network adapter above, select a troubleshooting task, and start capture before trying the connection. Stop once the problem occurs.</p><p>Already have a recording? Use <strong>Open capture file</strong> to review a PCAP or PCAPNG file.</p><p>The Troubleshooting view explains observed patterns and suggests next checks. Packets and technical details remain available in the other tabs.</p>'+glossaryHTML()+'</div>';restoreDisclosures(area);}return;}
 if(model.view==='troubleshooting'){
  const capture=model.captures.find(s=>s.id===model.selected),stamp=model.selected+':'+(capture?.revision??capture?.count??0)+':'+capture?.state;
  if(!model.report||model.reportStamp!==stamp){const value=await request('/'+model.selected+'/troubleshoot');if(!visible()||seq!==model.sequence)return;model.report=value;model.reportStamp=stamp;}
  renderTroubleshooting();return;
 }
 await refreshPacketNavigator();if(!visible()||seq!==model.sequence)return;
 if(model.graphNavigating)return;
 const params=parameters();
  if(model.view==='packets'){
  const updateLayout=area.querySelector('.packet-list-panel')?.dataset.layout!==packetLayoutSignature();
  await refreshPacketWindow(windowOffset(),packetHeldLocate(),!updateLayout&&packetInspectionPaused(false)&&!!node('packetTableScroll'));if(!visible()||seq!==model.sequence||model.view!=='packets')return;
  const graphSource=packetSourceSignature()+'|graph:'+packetGraphParameters();if(force||!model.statistics||model.statisticsSource!==graphSource){const graphQuery=packetGraphParameters().toString(),statistics=await request('/'+model.selected+'/statistics?'+graphQuery);if(graphQuery!==packetGraphParameters().toString())return;model.graphZoomPending=false;if(!visible()||seq!==model.sequence||model.view!=='packets')return;if(!updateLayout&&packetInspectionPaused(false))model.pendingStatistics=statistics;else{model.statistics=statistics;model.pendingStatistics=null;}model.statisticsSource=graphSource;}
  const summarySource=graphSource.split('|').slice(0,4).join('|')+'|'+parameters(false);if(force||!model.summaryStatistics||model.summaryStatisticsSource!==summarySource){const summary=model.statFilter?await request('/'+model.selected+'/statistics?'+parameters(false)):model.statistics;if(!visible()||seq!==model.sequence||model.view!=='packets')return;model.summaryStatistics=summary;model.summaryStatisticsSource=summarySource;}
  model.typeChoices=[...new Set([...(model.typeChoices||[]),...(model.typeFilter!=='any'?[model.typeFilter]:[]),...Object.keys(model.summaryStatistics?.protocols||{}),...(model.typeFilter!=='any'?[model.typeFilter]:[])])];const typeControl=node('packetTypeFilter');if(typeControl){const html=options([['any','All packet types'],...model.typeChoices.sort().map(type=>[type,type])],model.typeFilter);if(typeControl._ntOptions!==html){typeControl.innerHTML=html;typeControl._ntOptions=html;}typeControl.value=model.typeFilter;}
  if(updateLayout||!packetInspectionPaused(false))refreshPacketGraph();await followPacketLatest(updateLayout);
 }else{
  const graphQuery=packetGraphParameters().toString(),data=await request('/'+model.selected+'/statistics?'+graphQuery);if(graphQuery!==packetGraphParameters().toString())return;model.graphZoomPending=false;if(!visible()||seq!==model.sequence)return;model.statisticsSource=packetSourceSignature()+'|graph:'+graphQuery;renderStatistics(data);
 }
}
function glossaryHTML(){return '<details class="packet-glossary"><summary>Quick guide to the terms</summary><dl><dt>Packet</dt><dd>A small piece of network traffic with sender, destination and protocol information.</dd><dt>Protocol</dt><dd>The kind of conversation, such as DNS name lookups or TCP connections.</dd><dt>Port</dt><dd>A numbered service on a device, such as HTTPS commonly using port 443.</dd><dt>Conversation</dt><dd>Packets exchanged between a pair of addresses and service ports.</dd><dt>Recording filter</dt><dd>Decides what is saved. Excluded traffic is not available to review.</dd><dt>Display filter</dt><dd>Only changes what is shown. Clear it to see the other retained packets.</dd></dl></details>';}
function renderTroubleshooting(){
 const area=node('packetCaptureAnalysis'),r=model.report;if(!area||!r)return;rememberDisclosures();
 const metrics=r.metrics||{},findings=Array.isArray(r.findings)?r.findings:[],limitations=Array.isArray(r.limitations)?r.limitations:[];
 const markup='<div id="packetTroubleshooting" class="packet-summary-scroll packet-troubleshooting" tabindex="0" role="region" aria-label="Packet capture troubleshooting review"><div class="packet-review-heading"><div><h2>What this capture shows</h2><p>'+esc(r.summary||'Review the observed traffic and the next checks below.')+'</p></div><div class="packet-review-actions"><button class="btn small ghost" data-packet-action="copy-review">'+icon('copy')+'Copy findings</button><button class="btn small ghost" data-packet-action="save-review">'+icon('download')+'Save troubleshooting report</button></div></div><p class="packet-review-scope">'+esc(r.scope||'Review of all retained packets.')+' Packet-list search and display filters do not change this review.</p><div class="packet-review-metrics">'+[['packets','Retained packets',r.packetCount],['dnsrequests','Name lookup requests',metrics.dnsQueries],['dnsreplies','Name lookup replies',metrics.dnsResponses],['tcpstarts','TCP connection attempts',metrics.tcpConnectionAttempts],['tcpresets','Connection resets',metrics.tcpResets],['icmperrors','Delivery / hop-limit messages',metrics.icmpErrors]].map(([key,label,value])=>'<button type="button" data-packet-stat="'+key+'" aria-label="'+esc(label)+': '+Number(value||0).toLocaleString()+'. Show related packets" title="'+esc(packetStatisticExplanation(key))+'"><strong>'+Number(value||0).toLocaleString()+'</strong><span>'+esc(label)+'</span></button>').join('')+'</div><div class="packet-findings">'+findings.map((f,index)=>'<article class="packet-finding '+(f.severity==='attention'?'packet-finding-attention':'packet-finding-info')+'"><div class="packet-finding-heading"><h3>'+esc(f.title)+'</h3><span>'+(f.severity==='attention'?'Worth checking':'Information')+'</span></div><p><strong>Observed:</strong> '+esc(f.observation||'')+'</p><p><strong>What it means:</strong> '+esc(f.meaning||'')+'</p>'+(f.nextSteps?.length?'<h4>What to check next</h4><ul>'+f.nextSteps.map(step=>'<li>'+esc(step)+'</li>').join('')+'</ul>':'')+'<div class="packet-finding-actions">'+(f.filter&&f.packetNumbers?.length?'<button class="btn small ghost" data-packet-finding="'+index+'" data-packet-finding-id="'+esc(f.id||String(index))+'">View example packets</button><span class="hint">'+f.packetNumbers.length+' example'+(f.packetNumbers.length===1?'':'s')+' from '+Number(f.count||f.packetNumbers.length).toLocaleString()+' observation'+(f.count===1?'':'s')+'</span>':'')+'</div></article>').join('')+'</div>'+(limitations.length?'<section class="packet-review-limitations"><h3>Keep these limits in mind</h3><ul>'+limitations.map(text=>'<li>'+esc(text)+'</li>').join('')+'</ul></section>':'')+glossaryHTML()+'</div>';
 if(area._ntReviewMarkup===markup&&node('packetTroubleshooting'))return;
 const previous=node('packetTroubleshooting'),scroll=previous?.scrollTop||0,glossaryOpen=previous?.querySelector('.packet-glossary')?.hasAttribute('open'),focused=document.activeElement,inside=previous?.contains(focused),action=inside?focused?.closest('[data-packet-action]')?.dataset.packetAction:null,finding=inside?focused?.closest('[data-packet-finding-id]')?.dataset.packetFindingId:null,glossaryFocused=inside&&focused?.closest('.packet-glossary')&&focused?.tagName==='SUMMARY';
 if(previous){const shell=document.createElement('div');shell.innerHTML=markup;previous.innerHTML=shell.firstElementChild.innerHTML;}else area.innerHTML=markup;
 area._ntReviewMarkup=markup;const reader=node('packetTroubleshooting');if(reader){reader.scrollTop=scroll;restoreDisclosures(reader);if(glossaryOpen)reader.querySelector('.packet-glossary')?.setAttribute('open','');const restore=action?reader.querySelector('[data-packet-action="'+action+'"]'):finding!==null&&finding!==undefined?[...reader.querySelectorAll('[data-packet-finding-id]')].find(button=>button.dataset.packetFindingId===finding):glossaryFocused?reader.querySelector('.packet-glossary summary'):inside?reader:null;restore?.focus?.({preventScroll:true});}
}
function reviewText(){const r=model.report;if(!r)return '';const capture=model.captures.find(s=>s.id===model.selected);return ['Network Toolkit - Packet capture troubleshooting','Capture: '+(capture?.name||model.selected),'State: '+(capture?.state||'Unknown'),'',r.summary||'',r.scope||'All retained packets','Packet-list search and display filters do not change this review.','Retained packets: '+Number(r.packetCount||0),'Capture span: '+Number(r.duration||0).toFixed(3)+' seconds','',...(r.findings||[]).flatMap((f,i)=>[(i+1)+'. '+f.title,'Observed: '+f.observation,'What it means: '+f.meaning,'What to check next:',...(f.nextSteps||[]).map(s=>'  - '+s),...(f.packetNumbers?.length?['Example packet numbers: '+f.packetNumbers.join(', ')]:[]),'']), 'Limits:',...(r.limitations||[]).map(s=>'  - '+s),'','Captured traffic may include addresses and unencrypted content. Review the recording before sharing it.'].join('\n');}
function setView(view){closePacketMenu();hidePacketGraphPopup();rememberPacketGraphScroll();rememberDisclosures();const changed=model.view!==view;model.view=view;if(changed){const area=node('packetCaptureAnalysis');if(area){area.innerHTML='<div class="packet-empty" role="status">Loading view...</div>';area._ntStatisticsMarkup='';area._ntReviewMarkup='';}}refreshPacketGraph();node('packetCaptureAnalysis')?.classList.toggle('packet-analysis-summary',view!=='packets');layoutDetails();for(const tab of document.querySelectorAll('[data-packet-view]')){const yes=tab.dataset.packetView===view;tab.classList.toggle('active',yes);tab.setAttribute('aria-selected',String(yes));}}
function showFinding(index){const finding=model.report?.findings?.[index];if(!finding?.filter)return;model.pendingPacket=finding.packetNumbers?.[0]||0;model.detail=null;model.sort='number';model.direction='asc';applyFocusFilter(finding.filter);}
function statisticsHTML(s){
 const observed=value=>'<td class="packet-observed-time">'+esc(packetLocalTime(value))+'</td>';
 if(model.view==='conversations')return '<div class="packet-summary-scroll"><table class="packet-summary-table"><thead><tr><th>Protocol</th><th>Endpoint A</th><th>Endpoint B</th><th>First seen ('+esc(packetZone())+')</th><th>Last seen ('+esc(packetZone())+')</th><th>Packets</th><th>A to B / B to A</th><th>Bytes</th><th>Duration</th><th>Resets</th><th>Repeated payloads</th><th>Actions</th></tr></thead><tbody>'+(s.conversations||[]).map(c=>'<tr><td>'+esc(c.protocol)+'</td><td>'+esc(c.a)+'</td><td>'+esc(c.b)+'</td>'+observed(c.start)+observed(c.end)+'<td>'+c.packets+'</td><td>'+c.aToB+' / '+c.bToA+'</td><td>'+bytes(c.bytes)+'</td><td>'+Number(c.duration||0).toFixed(3)+' s</td><td>'+c.resets+'</td><td>'+c.retransmissions+'</td><td><button class="btn small ghost" data-packet-stream="'+esc(c.id)+'">Follow</button><button class="btn small ghost" data-packet-filter="stream == &quot;'+esc(c.id)+'&quot;">Filter</button></td></tr>').join('')+'</tbody></table><p class="hint">Repeated identical TCP payloads may indicate retransmission or multiple observations within the network stack.</p></div>';
 if(model.view==='endpoints')return '<div class="packet-summary-scroll"><table class="packet-summary-table"><thead><tr><th>Endpoint</th><th>First seen ('+esc(packetZone())+')</th><th>Last seen ('+esc(packetZone())+')</th><th>Packets</th><th>Sent</th><th>Received</th><th>Bytes</th><th>Actions</th></tr></thead><tbody>'+(s.endpoints||[]).map(e=>'<tr><td>'+esc(e.address)+'</td>'+observed(e.start)+observed(e.end)+'<td>'+e.packets+'</td><td>'+e.sent+'</td><td>'+e.received+'</td><td>'+bytes(e.bytes)+'</td><td><button class="btn small ghost" data-packet-endpoint="'+esc(e.address)+'">Filter endpoint</button></td></tr>').join('')+'</tbody></table></div>';
 const protocols=Object.entries(s.protocols||{}).sort((a,b)=>b[1]-a[1]),total=protocols.reduce((n,[,count])=>n+Math.max(0,Number(count)||0),0),duration=Number(s.duration)||0,timedBytes=Number(s.timedBytes),measured=duration>0&&Number.isFinite(timedBytes)&&Number.isFinite(packetEpoch(s.started))&&Number.isFinite(packetEpoch(s.finished));
 return '<div class="packet-summary-scroll packet-statistics">'+packetStatisticSummary(s)+'<div class="packet-graph-heading"><strong>Traffic graph</strong>'+packetFollowHTML()+'</div>'+packetTrafficHTML(s)+'<section><h3>Protocol distribution</h3><table class="packet-summary-table packet-protocol-distribution"><thead><tr><th>Protocol</th><th>Packets</th><th>Share of displayed packets</th></tr></thead><tbody>'+protocols.map(([protocol,count])=>{const share=total?Math.max(0,Math.min(100,Number(count)/total*100)):0,color=String(protocol).toLowerCase().replace(/[^a-z0-9]/g,'');return '<tr><td>'+esc(protocol)+'</td><td>'+packetNumber(count,0)+'</td><td><span>'+packetNumber(share,1)+'%</span><span class="packet-protocol-share" role="img" aria-label="'+esc(protocol)+' '+packetNumber(share,1)+' percent of displayed packets"><span style="width:'+share.toFixed(4)+'%;background:var(--packet-color-'+(Object.hasOwn(packetPaletteDefaults,color)?color:'other')+',var(--accent))"></span></span></td></tr>';}).join('')+'</tbody></table></section></div>';
}
async function selectPacket(number,{hold=true}={}){
 if(!model.selected)return;
 if(hold){model.graphSelectionVersion=(model.graphSelectionVersion||0)+1;model.graphHold=true;model.liveDetails=false;model.heldPacketNumber=number;model.intervalSelection=null;model.graphHeldEnd=model.statistics?.graphFinished||model.statistics?.finished||'';packetTimelineState().follow=false;preferences().hidden=false;}
 const relative=model.loaded.findIndex(p=>p.number===number);
 if(relative>=0){model.selectedIndex=model.windowStart+relative;updateMapSelection();}
 const id=model.selected,requestID=++model.detailRequest;
 if(model.detail?.packet?.number===number){renderDetail();return;}
 try{const value=await request('/'+id+'/packet/'+number);if(!visible()||model.selected!==id||requestID!==model.detailRequest)return;model.detail=value;model.range=null;const bucketIndex=packetBucketForPacket(value.packet),selection=packetTimelineState();if(hold&&bucketIndex>=0){selection.pinned=true;selection.index=bucketIndex;selection.pin={...model.statistics.timeline[bucketIndex],_includesEnd:model.statistics.timeline[bucketIndex].endInclusive===true||bucketIndex===model.statistics.timeline.length-1};}updatePacketTimeline();
  for(const row of document.querySelectorAll('#packetTableScroll [data-packet-number]')){const selected=Number(row.dataset.packetNumber)===number;row.classList.toggle('selected',selected);row.setAttribute('aria-selected',String(selected));}
  renderDetail();
 }catch(e){if(requestID===model.detailRequest)error(e.message);}
}
function plainPacketHTML(p){
 const descriptions={DNS:'Traffic with a DNS server, such as looking up the address for a website or device name.',TCP:'Connection-based traffic. Open the conversation to see how the connection starts, exchanges data and ends.',UDP:'Messages sent without a TCP-style connection setup. Some applications reply; others do not.',ARP:'A local IPv4 device is looking up the hardware address of another device or its gateway.',DHCP:'Decoded DHCPv4 traffic for automatic IPv4 address settings. DHCPv6 port traffic is shown separately as UDP.',ICMP:'Network control traffic, including ping checks and messages about delivery problems.',ICMPv6:'IPv6 network control traffic, including ping checks and messages about delivery problems.',HTTP:'An unencrypted web request or response. The start line may show what the server returned.',TLS:'Encrypted application traffic. Some handshake information may be visible; encrypted contents are not readable here.',SSH:'Secure remote-terminal traffic. An identification banner may be visible; later commands and data are encrypted.',LLDP:'A nearby network device is advertising information about itself.',CDP:'A nearby network device is advertising information about itself.'};
 const flags=p.tcpFlags||0,flagHint=flags&4?'This side sent a reset to end or reject the connection. A reset alone does not identify the cause.':flags&2?(flags&16?'This side answered a connection request. The rest of the exchange is needed to confirm setup.':'This side is asking to start a TCP connection.'):flags&1?'This side is closing its sending direction normally.':flags&16?'This acknowledges TCP traffic; it does not prove the application processed it.':'';
 const addresses=p.source&&p.destination?'<p class="packet-explanation-path"><strong>From</strong> '+esc(p.source)+(p.sourcePort?' port '+p.sourcePort:'')+' <strong>to</strong> '+esc(p.destination)+(p.destinationPort?' port '+p.destinationPort:'')+'</p>':'';
 return '<section class="packet-plain-explanation"><h3>In plain language</h3><p>'+esc(descriptions[p.protocol]||'This packet is not fully decoded. Its captured bytes remain available for inspection or export.')+'</p>'+addresses+dhcpPacketHint(p)+(flagHint?'<p>'+esc(flagHint)+'</p>':'')+'</section>';
}
function dhcpPacketHint(packet){
 const port=number=>packet.sourcePort===number||packet.destinationPort===number;
 if(packet.protocol==='UDP'&&(port(546)||port(547)))return '<p class="packet-protocol-note">This packet uses a DHCPv6 port (546 or 547). DHCPv6 messages are not decoded here; inspect or export the captured bytes. A port match alone does not confirm DHCP.</p>';
 if(packet.protocol==='UDP'&&(port(67)||port(68)))return '<p class="packet-protocol-note">This packet uses a DHCPv4 port (67 or 68), but a DHCPv4 message was not decoded. Short or incomplete packets, BOOTP, or other traffic on these ports can appear as UDP. Use full packets for a new recording.</p>';
 if(packet.protocol!=='DHCP')return '';
 const type=/^DHCP\s+(Discover|Offer|Request|Decline|ACK|NAK|Release|Inform)\b/i.exec(packet.info||'')?.[1]?.toLowerCase(),meaning={discover:'Discover: a client is looking for a DHCPv4 server.',offer:'Offer: a server proposes an address. This does not mean the client has accepted or applied it.',request:'Request: a client is requesting or renewing address settings.',ack:'ACK: a server acknowledges a request or supplies requested settings. This does not prove the client applied them.',nak:'NAK: a server rejects the requested configuration. This alone does not show that the server is unavailable.',decline:'Decline: a client reports that the proposed address appears to be in use. Check the surrounding traffic before concluding there is an address conflict.',release:'Release: a client is giving up its lease.',inform:'Inform: a client that already has an address is asking for other configuration.'}[type];
 return '<div class="packet-dhcp-guidance">'+(meaning?'<p>'+esc(meaning)+'</p>':'')+'<p>A new IPv4 lease commonly uses Discover, Offer, Request and ACK. Renewals may use only Request and ACK. Missing messages may be outside this capture; they are not proof of failure. Compare transaction IDs in the DHCP summaries.</p></div>';
}
function packetBinary(value){try{return typeof value?.data==='string'?atob(value.data):'';}catch(_){return null;}}
function renderDetail(){
 const area=node('packetInspector');if(!area)return;if(renderPacketInterval(area))return;const value=model.detail,p=value?.packet,key=model.selected+':'+(p?.number||'')+':'+!!model.graphHold+':'+!!model.liveDetails;if(area.dataset.packetDetail===key)return;
 const hide='<button class="iconbtn" type="button" data-packet-action="hide-details" aria-controls="packetInspector"></button>';
 if(!p){area.innerHTML='<div class="packet-detail-heading">'+hide+'<strong>Packet details</strong>'+dockSelector()+'</div><p class="packet-empty">Select a packet to inspect its protocol fields, hexadecimal bytes and ASCII payload.</p>';area.dataset.packetDetail=key;layoutDetails();return;}
 const layers=Array.isArray(value.layers)?value.layers.filter(layer=>layer&&typeof layer==='object'):[],notes=Array.isArray(p.expert)?p.expert:[],binary=packetBinary(value),ascii=typeof value.ascii==='string'?value.ascii:(binary||'').replace(/[^\x20-\x7e\r\n\t]/g,'.');
 const markup='<div class="packet-detail-heading">'+hide+'<strong>Packet '+esc(p.number)+'</strong>'+dockSelector()+'<button class="btn small ghost" data-packet-action="copy-packet">'+icon('copy')+'Copy details</button><button class="btn small ghost" data-packet-action="save-bytes">Save bytes</button><button class="btn small ghost" data-packet-action="packet-menu" aria-haspopup="menu" aria-controls="packetActionsMenu" aria-expanded="false">Packet actions</button>'+(p.stream?'<button class="btn small ghost" data-packet-stream="'+esc(p.stream)+'">Follow conversation</button>' :'')+'</div><div id="packetDetailScroll" class="packet-detail-scroll" tabindex="0" role="region" aria-label="Packet fields and bytes">'+'<div class="packet-frame-metadata"><span>Frame '+p.number+'</span><span>'+p.length+' bytes on wire</span><span>'+p.captured+' captured</span><span>'+esc(p.protocol)+'</span><span>Response time: '+(Number.isFinite(p.responseTimeMs??p.rttMs)?esc(p.responseTimeMs??p.rttMs)+' ms':'Not measured')+'</span><span class="packet-recorded-time" title="'+esc(p.time||'Not recorded')+'">Recorded: <time'+(Number.isFinite(packetEpoch(p.time))?' datetime="'+esc(p.time)+'"':'')+'>'+esc(packetLocalTime(p.time))+'</time></span></div>'+packetSelectionControls()+'<details class="packet-plain-guide"><summary>Explain this packet</summary>'+plainPacketHTML(p)+'</details>'+(notes.length?'<p class="packet-expert-note">'+esc(notes.join('; '))+'</p>':'')+'<div class="packet-layer-tree">'+layers.map((layer,index)=>{const fields=Array.isArray(layer.fields)?layer.fields.filter(field=>field&&typeof field==='object'):[];return '<details '+(index<3?'open':'')+'><summary data-packet-range="'+Number(layer.offset||0)+','+Number(layer.length||0)+'">'+esc(layer.name||'Protocol layer')+' ('+Number(layer.length||0)+' bytes)</summary>'+fields.map(field=>'<button class="packet-field" title="Select matching bytes. Right-click or press Shift+F10 to copy this value." data-packet-range="'+Number(field.offset||0)+','+Number(field.length||0)+'"><span>'+esc(field.name||'Field')+'</span><span>'+esc(field.value??'')+'</span></button>').join('')+(fields.length?'':'<p class="hint">No additional fields were decoded for this layer. Its captured bytes are available below.</p>')+'</details>';}).join('')+'</div><details open class="packet-byte-section"><summary>Hexadecimal and ASCII</summary><div id="packetBytes" class="packet-hex"></div></details><details class="packet-ascii-section"><summary>ASCII payload</summary><pre>'+esc(ascii.slice(0,65536))+'</pre></details></div>';
 area.innerHTML=markup;renderBytes();area.dataset.packetDetail=key;updatePacketViewContext();if(packetMenu)area.querySelector('[data-packet-action="packet-menu"]')?.setAttribute('aria-expanded','true');layoutDetails();
}
function renderBytes(){const area=node('packetBytes');if(!area||!model.detail)return;const binary=packetBinary(model.detail);if(binary===null){area.innerHTML='<p class="hint">The captured bytes could not be read. Select the packet again or reopen the recording.</p>';return;}const length=Math.min(binary.length,16384),range=model.range;let html='';for(let offset=0;offset<length;offset+=8){let hex='',ascii='';for(let i=offset;i<Math.min(offset+8,length);i++){const byte=binary.charCodeAt(i),selected=range&&i>=range[0]&&i<range[0]+range[1],cls=selected?' class="selected"':'';hex+='<span'+cls+'>'+byte.toString(16).padStart(2,'0')+'</span> ';ascii+='<span'+cls+'>'+esc(byte>=32&&byte<127?String.fromCharCode(byte):'.')+'</span>';}html+='<div><span class="packet-byte-offset">'+offset.toString(16).padStart(6,'0')+'</span><span class="packet-byte-hex">'+hex+'</span><span class="packet-byte-ascii">'+ascii+'</span></div>';}if(binary.length>length)html+='<p class="hint">The first 16384 bytes are shown. Save bytes exports every captured byte.</p>';if(!binary.length)html='<p class="hint">This packet has no captured bytes.</p>';area.innerHTML=html;}
async function follow(id){try{const data=await request('/'+model.selected+'/stream?id='+encodeURIComponent(id));openModal('Follow conversation','<p class="hint">'+esc(data.note)+'</p>'+data.directions.map((d,i)=>'<section><h3>'+esc(d.from)+' to '+esc(d.to)+'</h3><p class="hint">'+d.bytes+' retained bytes, '+d.gaps+' gap markers'+(d.truncated?', output limit reached':'')+'</p><textarea id="packetStream'+i+'" class="mono packet-stream-text" readonly spellcheck="false">'+esc(d.text)+'</textarea></section>').join(''),'<button class="btn ghost" data-action="close-modal">Close</button><button class="btn primary" data-packet-action="copy-stream">Copy conversation</button>',true);}catch(e){error(e.message)}}
function saveBlob(blob,name){download(blob,name,blob.type||'application/octet-stream');}
async function exportCapture(all){if(!model.selected)return;try{const params=all?new URLSearchParams():parameters(),format=node('packetExportFormat').value;params.set('format',format);const response=await fetch('/api/packet-captures/'+model.selected+'/export?'+params,{headers:{'X-Session-Token':SESSION}});if(!response.ok){const data=await response.json();throw new Error(data.error||'Could not export capture.')}saveBlob(await response.blob(),'Network_Toolkit_Capture_'+model.selected+'.'+format);}catch(e){error(e.message)}}
function applyFilter(expression){setView('packets');if(expression!==undefined)node('packetDisplayFilter').value=expression;model.expression=node('packetDisplayFilter').value.trim();model.query=node('packetSearch').value.trim();model.page=0;model.scrollTop=0;model.windowStart=0;model.windowToken++;renderFilterHint();model.sequence++;void refresh(true);}
function applyFocusFilter(expression){model.query='';model.protocol='any';model.severityFilter='any';model.typeFilter='any';model.statFilter='';node('packetSearch').value='';applyFilter(expression);}
function closePacketMenu(focus=false){const context=packetMenuContext,anchor=context?.anchor;packetMenu?.remove();packetMenu=null;packetMenuContext=null;for(const button of document.querySelectorAll('[data-packet-action="packet-menu"]'))button.setAttribute('aria-expanded','false');if(focus)(anchor?.isConnected?anchor:context?.capture===model.selected?document.querySelector('[data-packet-number="'+Number(context.packet.number)+'"]'):null)?.focus?.({preventScroll:true});}
function quotedFilter(value){return '"'+String(value).replace(/\\/g,'\\\\').replace(/"/g,'\\"')+'"';}
function packetAddressFilter(packet,side){
 const value=packet?.[side];if(typeof value!=='string'||!value)return '';const suffix=side==='source'?'src':'dst';
 if(/^[\da-f]{2}(:[\da-f]{2}){5}$/i.test(value))return 'eth.'+suffix+' == '+quotedFilter(value);
 const ipv4=/^\d{1,3}(\.\d{1,3}){3}$/.test(value)&&value.split('.').every(part=>Number(part)<=255),ipv6=value.includes(':')&&/^[\da-f:.]+(?:%[\w.-]+)?$/i.test(value);
 return ipv4&&packet.ipVersion===4?'ip.'+suffix+' == '+quotedFilter(value):ipv6&&packet.ipVersion===6?'ipv6.'+suffix+' == '+quotedFilter(value):'';
}
function endpointText(packet,side){const value=packet?.[side];if(typeof value!=='string'||!value)return '';const port=packet[side==='source'?'sourcePort':'destinationPort'],address=value.includes(':')&&!/^[\da-f]{2}(:[\da-f]{2}){5}$/i.test(value)?'['+value+']':value;return port?address+':'+port:value;}
function openPacketMenu(packet,anchor,x,y,field=null,valueTarget=null){
 if(!packet||!model.selected)return;closePacketMenu();packetMenuContext={packet:{...packet},capture:model.selected,anchor,field};packetMenu=document.createElement('div');packetMenu.id='packetActionsMenu';packetMenu.className='packet-actions-menu';packetMenu.setAttribute('role','menu');packetMenu.setAttribute('aria-label',field?'Protocol field actions':'Packet actions');
 const items=field?[['copy-field','Copy value',true]]:[['filter-source','Show traffic from this source',!!packetAddressFilter(packet,'source')],['filter-destination','Show traffic to this destination',!!packetAddressFilter(packet,'destination')],['filter-conversation','Show this conversation',!!packet.stream],['filter-protocol','Show this protocol',!!packet.protocol],['exclude-source','Hide traffic from this source',!!packetAddressFilter(packet,'source')],['show-all','Show all packets',true],['copy-source','Copy source endpoint',!!endpointText(packet,'source')],['copy-destination','Copy destination endpoint',!!endpointText(packet,'destination')]];
 const valueAction=!field&&valueTarget?root.NetworkToolkitValueCopy?.contextActionHTML?.(valueTarget,{close:()=>closePacketMenu(true)})||'':'';packetMenu.innerHTML='<div class="packet-actions-label">'+esc(field?.name||'Packet '+packet.number)+'</div>'+valueAction+items.map(([action,label,enabled])=>'<button type="button" role="menuitem" data-packet-menu-action="'+action+'" '+(enabled?'':'disabled')+'>'+label+'</button>').join('');document.body.append(packetMenu);const rect=packetMenu.getBoundingClientRect(),origin=anchor?.getBoundingClientRect(),left=Number.isFinite(x)?x:origin?.left||6,top=Number.isFinite(y)?y:origin?.bottom||6;packetMenu.style.left=clamp(left,6,Math.max(6,(root.innerWidth||1200)-(rect.width||250)-6))+'px';packetMenu.style.top=clamp(top,6,Math.max(6,(root.innerHeight||800)-(rect.height||300)-6))+'px';for(const button of document.querySelectorAll('[data-packet-action="packet-menu"]'))button.setAttribute('aria-expanded','true');[...packetMenu.querySelectorAll('button')].find(button=>!button.disabled&&!button.hasAttribute('disabled'))?.focus?.({preventScroll:true});
}
function packetMenuAction(action){
 const context=packetMenuContext;if(!context||context.capture!==model.selected){closePacketMenu();return;}const packet=context.packet;let filter;
 if(action==='filter-source')filter=packetAddressFilter(packet,'source');else if(action==='filter-destination')filter=packetAddressFilter(packet,'destination');else if(action==='filter-conversation'&&packet.stream)filter='stream == '+quotedFilter(packet.stream);else if(action==='filter-protocol'&&packet.protocol)filter='protocol == '+quotedFilter(packet.protocol);else if(action==='exclude-source'){const source=packetAddressFilter(packet,'source');if(source)filter='!('+source+')';}else if(action==='show-all')filter='';
 if(filter!==undefined){closePacketMenu();model.query='';model.protocol='any';node('packetSearch').value='';applyFilter(filter);node('packetTableScroll')?.focus?.({preventScroll:true});return;}
 const value=action==='copy-source'?endpointText(packet,'source'):action==='copy-destination'?endpointText(packet,'destination'):action==='copy-field'?context.field?.value:null;closePacketMenu(true);if(value!==null&&value!==undefined)void NetworkToolkitTerminalInteraction.copyValue(value);
}
function packetContextMenu(event){
 if(!visible())return false;const field=event.target.closest('.packet-field'),row=event.target.closest('[data-packet-number]');if(!field&&!row)return false;const packet=field?model.detail?.packet:model.loaded.find(item=>item.number===Number(row.dataset.packetNumber));if(!packet)return false;
 event.preventDefault();event.stopImmediatePropagation();const spans=field?.querySelectorAll('span'),value=field?{name:spans[0]?.textContent||'Protocol field',value:spans[1]?.textContent||''}:null;openPacketMenu(packet,field||row,event.clientX,event.clientY,value,row?event.target:null);if(row)void selectPacket(packet.number);return true;
}
function help(filters){openModal(filters?'Packet display filters':'Packet capture','<div class="packet-help">'+(filters?'<p>Display filters support &&, ||, ! and parentheses. These are the supported fields; this is a focused filter language.</p><ul><li>Protocols: tcp, udp, ip, ipv6, arp, icmp, icmpv6, dns, dhcp, http, tls, ssh, lldp, cdp.</li><li>IP fields: ip.addr, ip.src, ip.dst, ipv6.addr, ipv6.src, ipv6.dst. Address comparisons accept CIDR networks.</li><li>Ports: tcp.port, udp.port, tcp.srcport, tcp.dstport, udp.srcport, udp.dstport.</li><li>TCP flags: tcp.flags.syn, tcp.flags.ack, tcp.flags.fin, tcp.flags.rst.</li><li>Other fields: eth.addr, eth.src, eth.dst, vlan.id, frame.number, frame.len, frame.cap_len, frame.time_relative, frame.time_recorded, frame.severity (error / warn / note / normal), dns.request, dns.response, icmp.error, protocol, info, interface, direction, stream and expert.</li></ul><p>Examples:</p><pre>tcp &amp;&amp; ip.addr == 192.0.2.0/24\ndns || arp\ntcp.port == 443 &amp;&amp; !tcp.flags.rst\ninfo contains "example.com"\nframe.len &gt; 1400</pre>':'<h3>Capture a problem in three steps</h3><ol><li>Choose the adapter used by the affected computer and start capture.</li><li>Try the failing website, application or device once while recording.</li><li>Stop capture and read Troubleshooting. Open example packets to inspect the evidence.</li></ol><p>Choose Wi-Fi for a wireless connection or Ethernet for a wired connection. Recording normally sees this computer\'s traffic, not every device on the network.</p><p>For name-lookup problems, cached answers, the Hosts file or encrypted DNS can explain why ordinary DNS packets are absent. A short recording with no findings does not prove the network is healthy.</p><p>TLS and SSH contents are encrypted and are not decrypted by this tool. Captures can include addresses and unencrypted content; review them before sharing.</p>'+glossaryHTML()+'<h3>Recording and file details</h3><p>Live capture uses the Windows Packet Monitor API when available. It creates its own monitoring session and does not install a capture driver. Administrator approval is needed for recording and adapter enumeration.</p><p>Compatibility file capture uses Windows Packet Monitor and shows decoded packets after Stop. Both directions are required in this mode. Its file is circular. Existing Packet Monitor filters or another active file capture can prevent a new recording.</p><p>Only traffic visible to this computer or its selected adapter can be recorded. This does not enable switch mirroring, wireless monitor mode or TLS decryption.</p><p>Capture duration, memory and packet limits are explicit. Pause display freezes the view while recording continues. Keep filters determine retained packets; display filters only change analysis and filtered export.</p><p>PCAP and PCAPNG imports support Ethernet, VLAN, raw IP, loopback and Linux cooked frames. Protocol decoding includes IPv4/IPv6, ARP, TCP/UDP, ICMP, DNS, DHCP, HTTP start lines, TLS record headers/SNI, SSH identification, LLDP and CDP. Other link types retain their bytes for export.</p><p>Captured payload remains in this application session until cleared or the application closes. Three captures are retained; starting or importing another removes the oldest completed capture.</p>')+'</div>','<button class="btn primary" data-action="close-modal">Close</button>',true);}
document.addEventListener('click',event=>{
 const choice=event.target.closest('[data-packet-menu-action]');if(choice){event.preventDefault();event.stopImmediatePropagation();if(!choice.disabled&&!choice.hasAttribute('disabled'))packetMenuAction(choice.dataset.packetMenuAction);return;}
 if(packetMenu&&!packetMenu.contains(event.target)&&!event.target.closest('[data-packet-action="packet-menu"]'))closePacketMenu();
 const summary=event.target.closest('summary'),details=summary?.parentElement,key=disclosureKey(details);if(key){event.preventDefault();model.disclosures[key]=!details.hasAttribute('open');if(key==='setup')model.setupManual=true;if(model.disclosures[key])details.setAttribute('open','');else details.removeAttribute('open');if(key==='setup'||key==='display')layoutDetails();return;}
 const trigger=event.target.closest('[data-packet-action]'),action=trigger?.dataset.packetAction;if(action==='packet-menu'){event.preventDefault();event.stopImmediatePropagation();if(packetMenu)closePacketMenu(true);else openPacketMenu(model.detail?.packet,trigger);return;}if(action==='hide-details'||action==='toggle-details'){event.preventDefault();event.stopImmediatePropagation();closePacketMenu();toggleDetails(action==='hide-details'?true:undefined);}
});
document.addEventListener('contextmenu',packetContextMenu);
document.addEventListener('input-clear',event=>{if(!visible())return;const id=event.target.id;if(id!=='packetSearch'&&id!=='packetDisplayFilter')return;if(id==='packetSearch')model.query='';else model.expression='';renderFilterHint();model.page=0;model.scrollTop=0;model.windowStart=0;model.windowToken++;model.sequence++;void refresh(true);});
document.addEventListener('pointerdown',event=>{if(packetMenu&&!packetMenu.contains(event.target)&&!event.target.closest('[data-packet-action="packet-menu"]'))closePacketMenu();},true);
document.addEventListener('keydown',event=>{
 if((event.key==='F10'&&event.shiftKey)||event.key==='ContextMenu'){if(packetContextMenu(event))return;}
 if(!packetMenu||!packetMenu.contains(event.target))return;if(event.key==='Escape'){event.preventDefault();event.stopImmediatePropagation();closePacketMenu(true);return;}if(event.key==='Tab'){closePacketMenu(true);return;}
 if(['ArrowUp','ArrowDown','Home','End'].includes(event.key)){event.preventDefault();event.stopImmediatePropagation();const buttons=[...packetMenu.querySelectorAll('button')].filter(button=>!button.disabled&&!button.hasAttribute('disabled')),index=buttons.indexOf(document.activeElement),next=event.key==='Home'?0:event.key==='End'?buttons.length-1:event.key==='ArrowDown'?(index+1)%buttons.length:(index-1+buttons.length)%buttons.length;buttons[next]?.focus?.({preventScroll:true});}
});
document.addEventListener('dblclick',event=>{if(event.target.id==='packetDetailsDivider'){event.preventDefault();event.stopPropagation();toggleDetails(undefined,false);}});
document.addEventListener('pointerdown',event=>{if(event.target.id==='packetDetailsDivider')resizeDetails(event,event.target);});
document.addEventListener('keydown',event=>{
 if(event.target.id!=='packetDetailsDivider'||!['Enter',' ','ArrowLeft','ArrowRight','ArrowUp','ArrowDown','Home'].includes(event.key))return;event.preventDefault();event.stopImmediatePropagation();if(event.key==='Enter'||event.key===' '){toggleDetails(undefined,false);return;}
 const bottom=node('packetCaptureAnalysis').classList.contains('packet-details-bottom');if(event.key!=='Home'&&!(bottom?['ArrowUp','ArrowDown']:['ArrowLeft','ArrowRight']).includes(event.key))return;
 const pref=preferences(),name=bottom?'height':'width',divider=event.target;if(pref.hidden){pref.hidden=false;layoutDetails();}const amount=event.key==='Home'?detailDefaults[name]:clamp(Number(divider.getAttribute('aria-valuenow'))+(['ArrowLeft','ArrowUp'].includes(event.key)?1:-1)*(event.shiftKey?30:10),Number(divider.getAttribute('aria-valuemin')),Number(divider.getAttribute('aria-valuemax')));pref[name]=Math.round(clamp(amount,name==='width'?260:80,name==='width'?700:650));if(bottom&&state.packetGraphPreferences?.collapsed)model.graphCollapseInspectorHeight=pref.height;layoutDetails();persistDetails();
});
let packetWheelFrame=null;
document.addEventListener('wheel',event=>{const svg=event.target.closest?.('svg.packet-timeline'),history=root.NetworkToolkitGraphHistory;if(!svg||!history||!visible())return;const next=history.wheelLevel(event,packetWheelFrame?.next??packetGraphZoom(),Math.min(.25,packetGraphZoom()),Math.max(16,packetGraphZoom()));if(next===(packetWheelFrame?.next??packetGraphZoom()))return;event.preventDefault();const bounds=svg.getBoundingClientRect(),share=Math.max(0,Math.min(1,(event.clientX-bounds.left)/bounds.width));if(packetWheelFrame){packetWheelFrame.next=next;return;}packetWheelFrame={next,share,scope:{...packetPlotScope()}};const apply=()=>{const frame=packetWheelFrame;packetWheelFrame=null;if(visible()&&document.querySelector('svg.packet-timeline'))zoomPacketGraph(frame.next,{anchorShare:frame.share,scope:frame.scope});};if(root.requestAnimationFrame)root.requestAnimationFrame(apply);else apply();},{passive:false});
let graphPan=null;
document.addEventListener('pointerdown',event=>{const svg=event.target.closest?.('svg.packet-timeline');if(svg&&event.button===0){const bounds=svg.getBoundingClientRect();graphPan={x:event.clientX,left:bounds.left,width:bounds.width,share:(event.clientX-bounds.left)/bounds.width,scope:{...packetPlotScope()},mode:packetGraphPreferences().mode,id:event.pointerId};}});
document.addEventListener('pointerup',event=>{if(!graphPan||graphPan.id!==event.pointerId)return;const p=graphPan;graphPan=null;if(Math.abs(event.clientX-p.x)>8&&p.width>0){model.graphDragged=true;endPacketInspection();navigatePacketWindow(root.NetworkToolkitGraphHistory.panScope(p.scope,p.share,(event.clientX-p.left)/p.width,p.mode));root.setTimeout?.(()=>{model.graphDragged=false;},0);}});
document.addEventListener('pointercancel',()=>{graphPan=null;});
root.addEventListener?.('resize',layoutDetails);root.addEventListener?.('resize',refreshPacketGraphAxis);
root.addEventListener?.('resize',()=>closePacketMenu());root.addEventListener?.('blur',()=>closePacketMenu());
let packetScrollFrame=0;
function packetScrollUpdate(){packetScrollFrame=0;if(!visible()||model.view!=='packets')return;updateMapViewport();const wanted=windowOffset();if(wanted!==model.windowStart)void refreshPacketWindow(wanted);}
document.addEventListener('scroll',event=>{if(event.target?.id!=='packetTableScroll'||!visible()||model.view!=='packets')return;model.scrollTop=event.target.scrollTop;
 if(typeof root.requestAnimationFrame==='function'){if(!packetScrollFrame)packetScrollFrame=root.requestAnimationFrame(packetScrollUpdate);}else packetScrollUpdate();
},{capture:true,passive:true});
// The main traffic bar is a draggable proportional scrollbar. Starting on
// the viewport preserves the click offset, so the thumb never snaps its top
// to the pointer. Starting outside the thumb jumps to that part of the trace.
let mapDrag=null;
function mapThumbMetrics(bar){
 const rect=bar.getBoundingClientRect(),overlay=node('packetMapViewport'),barHeight=rect.height||bar.clientHeight||1;
 const height=parseFloat(overlay?.style.height)||10,top=clamp(parseFloat(overlay?.style.transform.match(/,([-\d.]+)px/)?.[1])||0,0,Math.max(0,barHeight-height));
 return {rect,barHeight,height,top,travel:Math.max(0,barHeight-height)};
}
function scrubMainMap(drag,clientY){
 const {rect,travel}=mapThumbMetrics(drag.bar),scroll=node('packetTableScroll');if(!scroll)return;
 const thumbY=clamp(clientY-rect.top-drag.offset,0,travel);
 setPacketScroll(travel?thumbY/travel*Math.max(0,scroll.scrollHeight-scroll.clientHeight):0);
}
function beginMapDrag(event){
 if(event.button!==undefined&&event.button!==0||!model.selected||model.total<=0)return;
 const bar=event.target.closest('#packetOverviewMap'),lane=event.target.closest('#packetDetailMap');if(!bar&&!lane)return;
 event.preventDefault?.();model.suppressMapClick=false;
 if(bar&&overviewLineHeight()){
  const scroll=node('packetTableScroll');if(!scroll)return;
  const rect=bar.getBoundingClientRect(),position=fineMapPosition(scroll,overviewLineHeight()),clickY=event.clientY-rect.top+(Number(bar.scrollTop)||0);
  const inThumb=clickY>=position.top&&clickY<=position.top+position.height;
  mapDrag={kind:'overview-scaled',bar,lane:bar,pointerId:event.pointerId,startY:event.clientY,lastY:event.clientY,
   startListTop:scroll.scrollTop,pressedIndex:overviewMapIndexFromPoint(bar,event.clientY),inThumb,initialized:inThumb,moved:false};
  model.overviewDragging=true;
 }else if(bar){const m=mapThumbMetrics(bar),y=event.clientY-m.rect.top,inThumb=y>=m.top&&y<=m.top+m.height;
  mapDrag={kind:'overview',bar,pointerId:event.pointerId,startY:event.clientY,lastY:event.clientY,moved:false,inThumb,offset:inThumb?y-m.top:m.height/2};
  model.suppressMapClick=inThumb;
  if(!inThumb)scrubMainMap(mapDrag,event.clientY);
 }else{
  const scroll=node('packetTableScroll');if(!scroll)return;
  const rect=lane.getBoundingClientRect(),position=fineMapPosition(scroll),clickY=event.clientY-rect.top+(Number(lane.scrollTop)||0);
  const inThumb=clickY>=position.top&&clickY<=position.top+position.height;
  mapDrag={kind:'detail',lane,pointerId:event.pointerId,startY:event.clientY,lastY:event.clientY,
   startListTop:scroll.scrollTop,pressedIndex:detailMapIndexFromPoint(lane,event.clientY),inThumb,initialized:inThumb,moved:false};
  model.granularDragging=true;
 }
 const owner=bar||lane;owner.setPointerCapture?.(event.pointerId);
}
function moveMapDrag(event){
 const drag=mapDrag;if(!drag||(drag.pointerId!==undefined&&event.pointerId!==undefined&&drag.pointerId!==event.pointerId))return;
 const delta=event.clientY-drag.startY;if(Math.abs(delta)>2)drag.moved=true;
 if(!drag.moved)return;
 drag.lastY=event.clientY;event.preventDefault?.();
 if(drag.kind==='overview'){scrubMainMap(drag,event.clientY);model.suppressMapClick=true;}
 else{
  const scroll=node('packetTableScroll');if(!scroll)return;
  if(!drag.initialized){
   // A drag starting on a colored packet line first brings that frame into
   // view. A drag starting on the outlined viewport has NO initial jump.
   centerPacketAt(drag.pressedIndex);drag.startListTop=scroll.scrollTop;drag.initialized=true;
  }
  const {rowHeight}=packetMetrics(),scale=drag.kind==='overview-scaled'?overviewLineHeight():mapLineHeight();
  // One logical pixel is one packet at the default scale. With a custom
  // line height, N pixels travel N / scale packets. Do not move the map's
  // own scrollTop on every pointer move: it would cancel the thumb motion.
  setPacketScroll(drag.startListTop+delta*rowHeight/Math.max(1,scale));
  const position=fineMapPosition(scroll,scale),lane=drag.lane,laneHeight=lane.clientHeight||420;
  const current=Number(lane.scrollTop)||0,maxTop=Math.max(0,model.total*scale-laneHeight);
  // Auto-pan the strip only when the moving range crosses an edge. Dragging
  // within the strip keeps the viewport under the pointer, pixel for pixel.
  const destination=clamp(position.top<current?position.top:position.top+position.height>current+laneHeight?position.top+position.height-laneHeight:current,0,maxTop);
  if(Math.abs(destination-current)>0.5){if(drag.kind==='detail')model.mapExpectedTop=destination;else model.overviewMapExpectedTop=destination;lane.scrollTop=destination;
   if(drag.kind==='detail')renderPacketDetailMap();else renderPacketOverviewLines();}
  model.suppressMapClick=true;
 }
}
function endMapDrag(event){const drag=mapDrag;if(!drag||(drag.pointerId!==undefined&&event.pointerId!==undefined&&drag.pointerId!==event.pointerId))return;
 if(drag.moved)model.suppressMapClick=true;
 drag.bar?.releasePointerCapture?.(drag.pointerId);drag.lane?.releasePointerCapture?.(drag.pointerId);
 model.granularDragging=false;model.overviewDragging=false;mapDrag=null;
 if((drag.kind==='detail'||drag.kind==='overview-scaled')&&!drag.moved&&event.type!=='pointercancel'){
  // Use the original clicked line. Synchronized strip scrolling must not
  // change which packet a click selects between pointerdown and pointerup.
  packetJump(drag.pressedIndex);model.suppressMapClick=true;
 }
 updateMapViewport();
}
document.addEventListener('pointerdown',beginMapDrag);
root.addEventListener?.('pointermove',moveMapDrag);
root.addEventListener?.('pointerup',endMapDrag);
root.addEventListener?.('pointercancel',endMapDrag);
root.addEventListener?.('blur',()=>{mapDrag=null;model.granularDragging=false;model.overviewDragging=false;});
document.addEventListener('click',event=>{const bar=event.target.closest('#packetOverviewMap'),lane=event.target.closest('#packetDetailMap');if(!bar&&!lane)return;
 event.preventDefault();if(model.suppressMapClick){model.suppressMapClick=false;return;}
 if(lane){packetJump(detailMapIndexFromPoint(lane,event.clientY));return;}
 if(overviewLineHeight()){packetJump(overviewMapIndexFromPoint(bar,event.clientY));return;}
 const box=bar.getBoundingClientRect(),ratio=clamp((event.clientY-box.top)/Math.max(1,box.height),0,1),index=Math.min(model.total-1,Math.floor(ratio*model.total));packetJump(index);
});
document.addEventListener('scroll',event=>{if(event.target?.id!=='packetOverviewMap'||!visible()||model.view!=='packets'||!overviewLineHeight())return;
 const bar=event.target,top=Number(bar.scrollTop)||0;renderPacketOverviewLines();
 if(model.overviewDragging||Math.abs(top-model.overviewMapExpectedTop)<1)return;
 model.overviewMapExpectedTop=top;
 const center=clamp(Math.floor((top+(bar.clientHeight||420)/2)/overviewLineHeight()),0,Math.max(0,model.total-1));centerPacketAt(center);
},{capture:true,passive:true});
// Native wheel/trackpad scrolling on the detailed strip updates the list.
// Programmatic synchronization is detected by matching its expected position.
document.addEventListener('scroll',event=>{if(event.target?.id!=='packetDetailMap'||!visible()||model.view!=='packets')return;
 const lane=event.target,top=Number(lane.scrollTop)||0;
 renderPacketDetailMap();
 // Browser-delivered scroll events can arrive after a programmatic move.
 // In particular, a drag must never be converted back into a recenter.
 if(model.granularDragging||Math.abs(top-model.mapExpectedTop)<1)return;
 model.mapExpectedTop=top;const center=clamp(Math.floor((top+(lane.clientHeight||420)/2)/mapLineHeight()),0,Math.max(0,model.total-1));centerPacketAt(center);
},{capture:true,passive:true});
document.addEventListener('keydown',event=>{if(event.target?.id==='packetDetailMap'){let at=clamp(model.selectedIndex>=0?model.selectedIndex:Math.floor(((Number(event.target.scrollTop)||0)+(event.target.clientHeight||420)/2)/mapLineHeight()),0,Math.max(0,model.total-1));if(event.key==='ArrowDown')at++;else if(event.key==='ArrowUp')at--;else if(event.key==='PageDown')at+=30;else if(event.key==='PageUp')at-=30;else if(event.key==='Home')at=0;else if(event.key==='End')at=model.total-1;else return;event.preventDefault();packetJump(clamp(at,0,model.total-1));return;}if(event.target?.id!=='packetOverviewMap')return;const {rowHeight,headerHeight}=packetMetrics(),scroll=node('packetTableScroll'),index=clamp(Math.floor((model.scrollTop+(scroll?.clientHeight||0)/2-headerHeight)/rowHeight),0,Math.max(0,model.total-1));let next=index;if(event.key==='Home')next=0;else if(event.key==='End')next=model.total-1;else if(event.key==='ArrowDown')next=Math.min(model.total-1,index+30);else if(event.key==='ArrowUp')next=Math.max(0,index-30);else return;event.preventDefault();packetJump(next);});
document.addEventListener('click',event=>{if(model.appearanceOpen&&!event.target.closest('#packetAppearanceSettings')&&!event.target.closest('[data-packet-action="toggle-appearance"]'))toggleAppearance(false);if(model.guideOpen&&!event.target.closest('#packetQuickGuide')&&!event.target.closest('[data-packet-action="toggle-guide"]')){model.guideOpen=false;const guide=node('packetQuickGuide');if(guide)guide.hidden=true;}const jump=event.target.closest('[data-packet-jump]');if(jump){event.preventDefault();packetJump(Number(jump.dataset.packetJump),Number(jump.dataset.packetNumber));return;}const finding=event.target.closest('[data-packet-finding]');if(finding){event.preventDefault();showFinding(Number(finding.dataset.packetFinding));return;}const action=event.target.closest('[data-packet-action]'),filter=event.target.closest('[data-packet-filter]'),view=event.target.closest('[data-packet-view]'),row=event.target.closest('[data-packet-number]'),sort=event.target.closest('[data-packet-sort]'),stream=event.target.closest('[data-packet-stream]'),range=event.target.closest('[data-packet-range]'),endpoint=event.target.closest('[data-packet-endpoint]');if(filter){if(filter.closest('#packetQuickGuide')){model.guideOpen=false;const guide=node('packetQuickGuide');if(guide)guide.hidden=true;}applyFocusFilter(filter.dataset.packetFilter);return}if(view){setView(view.dataset.packetView);model.sequence++;void refresh(true);return}if(stream){void follow(stream.dataset.packetStream);return}if(endpoint){const value=endpoint.dataset.packetEndpoint,field=/^[\da-f]{2}(:[\da-f]{2}){5}$/i.test(value)?'eth.addr':value.includes(':')?'ipv6.addr':'ip.addr';applyFilter(field+' == "'+value+'"');return}if(sort){const field=sort.dataset.packetSort;if(field==='info')return;model.direction=model.sort===field&&model.direction==='asc'?'desc':'asc';model.sort=field;model.scrollTop=0;model.windowStart=0;model.selectedIndex=-1;model.pendingIndex=null;model.windowToken++;model.sequence++;void refresh(true);return}if(range){model.range=range.dataset.packetRange.split(',').map(Number);renderBytes();return}if(row){void selectPacket(Number(row.dataset.packetNumber));return}if(!action||action.disabled)return;event.preventDefault();switch(action.dataset.packetAction){case 'toggle-appearance':toggleAppearance();break;case 'close-appearance':toggleAppearance(false);break;case 'reset-appearance':state.packetMapSettings=defaultMapSettings();{const panel=node('packetAppearanceSettings');if(panel){for(const input of panel.querySelectorAll('[data-packet-color]')){input.value=mapSettings().colors[input.dataset.packetColor];if(input.tagName==='BUTTON')root.NetworkToolkitColorPicker?.syncButton(input,input.value);}for(const select of panel.querySelectorAll('[data-packet-appearance]'))select.value=String(mapSettings()[select.dataset.packetAppearance]);for(const input of panel.querySelectorAll('[data-packet-visibility]'))input.checked=mapSettings()[input.dataset.packetVisibility];}}applyPacketAppearance();updateOverviewScale(true);{const lane=node('packetDetailMap');if(lane){lane._mapRange='';const content=node('packetDetailMapContent');if(content)content.style.height=Math.max(1,model.total*mapLineHeight())+'px';updateMapViewport();renderPacketDetailMap(true);}}persistDetails();break;case 'toggle-guide':case 'close-guide':{model.guideOpen=action.dataset.packetAction==='toggle-guide'?!model.guideOpen:false;const guide=node('packetQuickGuide');if(guide)guide.hidden=!model.guideOpen;const button=document.querySelector('[data-packet-action="toggle-guide"]');if(button)button.setAttribute('aria-expanded',String(model.guideOpen));break;}case 'toggle-insights':model.insightsOpen=!model.insightsOpen;state.packetInsightsOpen=model.insightsOpen;persistDetails();{const panel=node('packetInsights');if(panel)panel.hidden=!model.insightsOpen;for(const button of document.querySelectorAll('[data-packet-action="toggle-insights"]'))if(button.closest('.packet-list-scope')){button.textContent=model.insightsOpen?'Hide insights':'Show insights';button.setAttribute('aria-expanded',String(model.insightsOpen));}}break;case 'scroll-first':setPacketScroll(0);break;case 'scroll-last':void resumePacketLatest();break;case 'goal-filter':applyFocusFilter((tasks[model.goal]||tasks.general).filter);break;case 'copy-review':if(model.report)void NetworkToolkitTerminalInteraction.copyValue(reviewText());break;case 'save-review':if(model.report)download(reviewText(),'Network_Toolkit_Troubleshooting_'+model.selected+'.txt','text/plain;charset=utf-8');break;case 'start':void start();break;case 'stop':void stop();break;case 'import':node('packetCaptureFile').click();break;case 'clear':void request('/'+model.selected,'DELETE').then(()=>{selectCapture('');model.detail=null;void refresh(true)}).catch(e=>error(e.message));break;case 'interfaces':void loadInterfaces();break;case 'filter':applyFilter();break;case 'reset-filter':model.protocol='any';model.severityFilter='any';model.typeFilter='any';model.statFilter='';node('packetSearch').value='';applyFilter('');break;case 'filter-help':help(true);break;case 'help':help(false);break;case 'export':void exportCapture(false);break;case 'export-all':void exportCapture(true);break;case 'save-bytes':if(model.detail){const raw=packetBinary(model.detail);if(raw===null){error('The captured bytes could not be read.');break;}saveBlob(new Blob([Uint8Array.from(raw,c=>c.charCodeAt(0))]),'Packet_'+model.detail.packet.number+'.bin')}break;case 'copy-packet':if(model.detail)void NetworkToolkitTerminalInteraction.copyValue(JSON.stringify({packet:model.detail.packet,layers:model.detail.layers,hex:model.detail.hex},null,2));break;case 'copy-stream':void NetworkToolkitTerminalInteraction.copyValue([...document.querySelectorAll('.packet-stream-text')].map(t=>t.value).join('\n\n'));break;}});
document.addEventListener('click',event=>{const button=event.target.closest?.('button[data-packet-color]');if(button){event.preventDefault();openPacketColor(button);}});
document.addEventListener('input',event=>{if(event.target?.dataset?.packetColor)updateAppearanceSetting(event.target);});
document.addEventListener('change',event=>{const el=event.target;if(el.dataset?.packetColor||el.dataset?.packetAppearance||el.dataset?.packetVisibility){updateAppearanceSetting(el);return;}if(el.id==='packetDetailsDock'){preferences().dock=el.value;model.graphCollapseInspectorHeight=null;layoutDetails();persistDetails();return;}if(el.id==='packetTask'){model.goal=Object.hasOwn(tasks,el.value)?el.value:'general';renderTask();}else if(el.id==='packetInterface'){model.interfaceSelection=el.value;model.interfaceUnavailable=false;renderInterfaces();if(!model.interfaceUnavailable&&node('packetAdapterStatus'))node('packetAdapterStatus').textContent='Adapter selected. Capture support is verified only after explicit Start capture and Windows approval.';updateStatus();}else if(el.id==='packetEngine'){renderInterfaces();updateStatus();}else if(el.id==='packetCaptureSelect'){model.newCapture=el.value==='';if(model.newCapture)setView('troubleshooting');selectCapture(el.value);model.detail=null;model.page=0;model.scrollTop=0;model.windowStart=0;model.windowToken++;model.sequence++;void refresh(true)}else if(el.id==='packetPauseDisplay'){model.paused=el.checked;if(!model.paused)void refresh(true)}else if(el.id==='packetFollow'){model.follow=el.checked;if(el.checked)void resumePacketLatest();else void refresh(true)}else if(el.id==='packetCaptureFile'&&el.files?.[0]){const file=el.files[0];if(file.size>128*1024*1024){error('Capture imports are limited to 128 MiB.');el.value='';return}void fetch('/api/packet-captures/import?name='+encodeURIComponent(file.name),{method:'POST',headers:{'X-Session-Token':SESSION,'Content-Type':'application/octet-stream'},body:file}).then(async r=>{const data=await r.json();if(!r.ok)throw new Error(data.error||'Could not import capture.');setView('troubleshooting');selectCapture(data.id);model.page=0;model.scrollTop=0;model.windowStart=0;model.windowToken++;model.detail=null;model.sequence++;await refresh(true)}).catch(e=>error(e.message)).finally(()=>el.value='')}});
document.addEventListener('keydown',event=>{if(!visible())return;if(event.key==='Escape'&&model.appearanceOpen){toggleAppearance(false);document.querySelector('[data-packet-action="toggle-appearance"]')?.focus?.({preventScroll:true});return;}if(event.key==='Escape'&&model.guideOpen){model.guideOpen=false;const guide=node('packetQuickGuide');if(guide)guide.hidden=true;document.querySelector('[data-packet-action="toggle-guide"]')?.focus?.({preventScroll:true});return;}if(['packetSearch','packetDisplayFilter'].includes(event.target.id)&&event.key==='Enter'){event.preventDefault();applyFilter();return}if(event.target.closest('#packetTableScroll')&&['ArrowUp','ArrowDown','Enter'].includes(event.key)){event.preventDefault();let index=model.loaded.findIndex(p=>p.number===model.detail?.packet.number);if(index<0)index=0;if(event.key==='ArrowUp')index--;if(event.key==='ArrowDown')index++;const p=model.loaded[index];if(p){void selectPacket(p.number);const row=document.querySelector('[data-packet-number="'+p.number+'"]');row?.scrollIntoView?.({block:'nearest'});}else if(event.key!=='Enter'){packetJump(clamp(model.windowStart+index,0,model.total-1));}}});
root.NetworkToolkitPacketCapture=Object.freeze({render,refresh,refreshTimeDisplay,optionRequest,mergePreferences,layoutDetails,model,parameters,clearSelection:clearPacketGraphSelection,resumeLatest:resumePacketLatest,packetLevel,followPacketLatest,packetReaderViewport,packetVisibleTimeRange,refreshPacketGraphAxis,zoom:zoomPacketGraph,navigate:navigatePacketWindow,plotScope:packetPlotScope});
})(window);
