(function(root){
'use strict';
// Phase 5 guidance is presentation only. Never copy connection secrets,
// unsaved editor text, terminal output, or device data into welcome content.
const guides=Object.freeze({
 quick:['Check a device','Enter a hostname or IP address, such as example.com, then select Run device check.'],
 ping:['Measure response time','Enter a hostname or IP address, such as 192.0.2.10, then select Start ping.'],
 trace:['See the network path','Enter a hostname or IP address, such as example.com, then select Trace route.'],
 dns:['Look up DNS records','Enter a domain such as example.com, choose a record type, then select Look up DNS.'],
 tcp:['Test a TCP port','Enter a host such as example.com and ports such as 22,80,443, then start the check.'],
 smtp:['Test a mail server','Enter a mail server hostname and review the connection and encryption options before testing.'],
 http:['Check a website','Enter an address such as https://example.com, then start the HTTP check.'],
 tls:['Check a website certificate','Enter a server name such as example.com, then check its certificate.'],
 identify:['Identify a device','Enter a hostname or IP address and start the device details check.'],
 discover:['Find devices on a network','Enter a permitted subnet such as 192.0.2.0/28, review the scope, then start discovery.'],
 monitor:['Watch a connection','Enter a hostname or IP address, choose a test interval, then start monitoring.'],
 capture:['Capture network traffic','Choose an interface and capture settings, then start recording packets. Only capture traffic you are authorized to inspect.'],
 wol:['Wake a device','Enter the device MAC address and select the network adapter before sending Wake on LAN.'],
 sftp:['Browse or transfer files','Choose a server and SFTP login method, then connect. Verify the server identity when prompted.'],
 ftp:['Transfer files','Choose FTP or FTPS, enter the server and credentials, then connect. Prefer an encrypted option when supported.'],
 tftp:['Transfer files using TFTP','Choose a server and local file before starting a transfer. TFTP has no encryption.'],
 routes:['Inspect routes','Refresh the view to see the routing table on this computer.'],
 arp:['Inspect nearby devices','Refresh the neighbor cache to see addresses this computer has recently contacted.'],
 connections:['Inspect active connections','Refresh the list to see current local network connections.'],
 wifi:['Inspect wireless networks','Choose a wireless adapter, then refresh the available network information.'],
 ipconfig:['Review network adapters','Refresh to see this computer’s adapter addresses, gateways and DNS servers.'],
 local:['Review local network settings','Choose an adapter or local network tool to review this computer’s configuration.'],
 hosts:['Review hostname mappings','Open the Hosts file to view or edit local hostname entries. Changes require review and Windows permission.'],
 subnet:['Calculate network ranges','Enter an IPv4 address and prefix, such as 192.0.2.10/24, then select Calculate.'],
 editor:['Edit a text file','Create a new document or open a local or remote text file. Save your changes before closing.'],
 terminal:['Connect to a device','Start a new SSH, Telnet or serial connection. Enter the address and connection settings before connecting.'],
 profiles:['Save a target','Add a commonly used device, host or network to return to it quickly.'],
 reports:['Build a report','Run a diagnostic or import a previously exported report, then choose the results to export.'],
 overview:['Start diagnosing','Use Device check for a hostname or IP address, or open a tool from the navigation.']
});
function guidance(tool,available=true){
 const entry=guides[tool]||guides.quick;
 return {title:entry[0],description:available?entry[1]:'Open Network_Toolkit.exe to use live network tools. Saved targets, imported reports and calculators are still available offline.'};
}
function recovery(tool){
 if(tool==='dns')return 'Check the name, DNS server and network connection, then try the lookup again.';
 if(['sftp','ftp','tftp'].includes(tool))return 'Check the server address, connection and access permissions, then retry. Do not bypass a certificate or host-key warning.';
 if(tool==='capture')return 'Check the selected interface and capture permissions, then try again.';
 if(['hosts','local','ipconfig'].includes(tool))return 'Review the selected adapter or file and Windows permissions before trying again.';
 if(tool==='smtp'||tool==='tls'||tool==='http')return 'Check the server address and connection settings, then try again. Keep certificate verification enabled.';
 return 'Check the target, your network connection and the selected options, then try again.';
}
let owner=null,element=null;
function dismiss(){
 if(element){element.remove();element=null;}
 if(!owner||!owner.ready())return;
 const saved=owner.getState();if(saved.firstLaunchWelcomeSeen===true)return;
 saved.firstLaunchWelcomeSeen=true;owner.save();
}
function open(manual=false){
 if(!owner||owner.detached()||!owner.ready())return false;
 if(!manual&&owner.getState().firstLaunchWelcomeSeen===true)return false;
 if(element){element.scrollIntoView({block:'nearest'});return true;}
 const anchor=root.document.getElementById('offlineBanner');if(!anchor)return false;
 element=root.document.createElement('section');
 element.id='uxFirstLaunchWelcome';element.className='ux-welcome';element.setAttribute('aria-label','Welcome to Network Toolkit');
 element.innerHTML='<div class="ux-welcome-heading"><div><h2>Welcome to Network Toolkit</h2><p>Choose a starting point. No connection or network change begins until you select an action.</p></div><button type="button" class="btn small ghost" data-ux-welcome-close aria-label="Dismiss welcome">Dismiss</button></div><div class="ux-welcome-choices"><button type="button" class="btn primary" data-ux-welcome-page="quick">Check a device</button><button type="button" class="btn" data-ux-welcome-page="terminal">Open a terminal</button><button type="button" class="btn" data-ux-welcome-page="sftp">Transfer files</button></div><p class="ux-welcome-foot">Open this guide again from F1 Help or the command search.</p>';
 anchor.after(element);
 element.addEventListener('click',event=>{
  if(event.target.closest('[data-ux-welcome-close]')){dismiss();return;}
  const button=event.target.closest('[data-ux-welcome-page]');if(!button)return;
  const page=button.dataset.uxWelcomePage;if(!['quick','terminal','sftp'].includes(page))return;
  dismiss();owner.navigate(page);
 });
 return true;
}
function initialize(config){owner=config;if(!owner.detached()&&owner.ready())open(false);}
if(typeof module==='object'&&module.exports)module.exports={guidance,recovery,guides};
root.NetworkToolkitPhase5=Object.freeze({guidance,recovery,initialize,open,dismiss});
})(typeof window==='undefined'?globalThis:window);
