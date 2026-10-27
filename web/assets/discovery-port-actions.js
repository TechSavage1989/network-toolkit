/* Discovery TCP actions only prepare existing clients or invoke the constrained
   native browser/RDP launcher. A port number is a service hint, not detection. */
(function(root){'use strict';
 const names={ssh:'SSH terminal',telnet:'Telnet terminal',http:'HTTP browser',https:'HTTPS browser',rdp:'Remote Desktop',ftp:'FTP / FTPS files',ftps:'Implicit FTPS files',sftp:'SFTP files'};
 const conventional={20:'FTP data',21:'FTP',22:'SSH',23:'Telnet',25:'SMTP',53:'DNS',80:'HTTP',110:'POP3',135:'RPC',139:'NetBIOS',143:'IMAP',389:'LDAP',443:'HTTPS',445:'SMB',465:'SMTPS',587:'SMTP',636:'LDAPS',990:'FTPS',993:'IMAPS',995:'POP3S',1433:'SQL Server',1521:'Oracle',2049:'NFS',2222:'SSH',2323:'Telnet',3306:'MySQL',3389:'RDP',5432:'PostgreSQL',5900:'VNC',5985:'WinRM',5986:'WinRM TLS',8000:'HTTP',8008:'HTTP',8080:'HTTP',8443:'HTTPS',8888:'HTTP',9443:'HTTPS'};
 const aliases={ssh:'ssh',telnet:'telnet',http:'http','http-alt':'http','http alternate':'http',https:'https','https-alt':'https','https alternate':'https',rdp:'rdp','ms-wbt-server':'rdp',ftp:'ftp',ftps:'ftps','ftp-ssl':'ftps',sftp:'sftp'};
 function portNumber(value){const n=Number(value);return Number.isInteger(n)&&n>0&&n<=65535?n:0;}
 function ports(row){return [...new Set((Array.isArray(row?.openPorts)?row.openPorts:[]).map(portNumber).filter(Boolean))].sort((a,b)=>a-b);}
 function service(row,port){
  const recorded=(Array.isArray(row?.ports)?row.ports:[]).filter(p=>Number(p.port)===port&&p.state==='open');
  // Current scans populate `service` from the port map. Only an explicitly
  // verified detection may override that map; conflicting evidence needs choice.
  const detected=[...new Set(recorded.filter(p=>p.serviceVerified===true).map(p=>String(p.detectedService||p.service||'').trim().toLowerCase()).filter(Boolean))];
  if(detected.length){const protocol=detected.length===1?aliases[detected[0]]||'':'';return {protocol,label:detected.join(' / '),basis:'Recorded service detection',verified:true};}
  const label=conventional[port]||'',protocol=aliases[label.toLowerCase()]||'';
  return {protocol,label,basis:label?'Conventional '+label+' port; service not verified':'Service not identified',verified:false};
 }
 function button(row,port,key){
  const hint=service(row,port),action=hint.protocol?'Open '+names[hint.protocol]:'Choose a connection tool',host=deviceTarget(row),available=!!host&&!!key;
  return '<button type="button" class="network-port-badge network-port-connect" data-network-port="'+port+'" data-network-port-key="'+esc(key||'')+'"'+(!available?' disabled':'')+' title="'+esc('TCP '+port+' open · '+hint.basis+'. '+action+' for '+(host||'this device')+' on port '+port+'.')+'" aria-label="'+esc(action+' for '+(host||'device')+' TCP port '+port)+'"><span class="network-port-number">'+port+'</span>'+(hint.label?'<span class="network-port-service">'+esc(hint.label)+'</span>':'')+'</button>';
 }
 function render(row,key){const list=ports(row);if(!list.length)return '<span class="network-ports-empty">None reported</span>';return '<div class="network-open-ports" data-copy-value="'+esc(list.join(', '))+'" data-copy-label="Open TCP ports"><span class="network-port-list">'+list.map(port=>button(row,port,key)).join('')+'</span></div>';}
 function selected(key,port){const row=deviceRecords.get(key),n=portNumber(port);if(!row||!n||!ports(row).includes(n)||!deviceTarget(row))return null;return {...row,host:deviceTarget(row),address:deviceTarget(row),endpointPort:n,endpointProtocol:'tcp',endpointSelected:true,endpointRole:'Selected discovery TCP port'};}
 function choose(key,port,row){const hint=service(row,port);openModal('Connect to '+deviceTarget(row)+' · TCP '+port,'<p>'+esc(hint.basis)+'. Choose the actual service; Toolkit does not identify an application from an open port alone.</p><div class="device-open-actions">'+Object.entries(names).map(([protocol,name])=>'<button type="button" class="btn small" data-network-port-choice="'+protocol+'" data-network-port-key="'+esc(key)+'" data-network-port="'+port+'">'+esc(name)+'</button>').join('')+'</div><p class="help-line">Terminal and file clients open connection settings for review. Telnet and plain FTP are unencrypted; FTP starts with explicit TLS selected. Browser and Remote Desktop open separate applications.</p>','<button type="button" class="btn ghost" data-action="close-modal">Cancel</button>',true);}
 async function launch(key,port,protocol){
  const row=selected(key,port);if(!row){toast('This recorded device or open TCP port is no longer available.','error');return;}
  if(!online){toast('Open the executable to use device connections.','error');return;}
  if(!Object.hasOwn(names,protocol))return;
  if(['ssh','telnet'].includes(protocol)){
   if(!deviceApplicability(row,protocol).ok)return;
   closeModal();navigate('terminal');
   // Do not inherit another destination's saved identity or authentication.
   openTerminalConnect(safeTerminalConfig({type:protocol,host:row.host,port:row.endpointPort,credentialId:'',username:'',authMethod:'password',sshCompatibility:'modern'}));return;
  }
  if(['ftp','ftps','sftp'].includes(protocol)){
   const browser=root.NetworkToolkitRemoteBrowser;if(!browser?.prepareDeviceConnection){toast('The file client is unavailable.','error');return;}
   try{browser.prepareDeviceConnection(protocol,row.host,row.endpointPort);closeModal();}catch(error){toast(error.message,'error');}return;
  }
  const actionKey=key+'|connect-tcp|'+row.endpointPort;
  deviceRecords.set(actionKey,row);await openDeviceConnection(actionKey,protocol);
 }
 async function activate(key,port,choice){const row=selected(key,port);if(!row){toast('This recorded device or open TCP port is no longer available.','error');return;}const protocol=choice||service(row,Number(port)).protocol;if(!protocol){choose(key,Number(port),row);return;}await launch(key,Number(port),protocol);}
 document.addEventListener('click',event=>{const button=event.target.closest?.('[data-network-port]');if(!button)return;event.preventDefault();event.stopImmediatePropagation();if(!button.disabled)void activate(button.dataset.networkPortKey,button.dataset.networkPort,button.dataset.networkPortChoice);},true);
 root.NetworkToolkitDiscoveryPorts={ports,service,render,button,selected,activate,launch};
})(globalThis);
