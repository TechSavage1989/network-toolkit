'use strict';
// Owner report (v1.20.1): the install-and-restart progress bar stopped moving.
// Chromium stops animating an indeterminate <progress> when the element
// carries a border-radius while both pseudo-elements are styled. This loads
// the shipped stylesheets and checks that an indeterminate bar still changes
// between frames in both themes, and that a determinate bar keeps its fill.
const fs=require('node:fs'),path=require('node:path'),assert=require('node:assert/strict');
const {chromium}=require(process.env.PLAYWRIGHT_MODULE||'playwright');
const root=path.resolve(__dirname,'..'),out=process.env.TOOLKIT_TEST_OUTPUT||process.env.NTK_BROWSER_OUTPUT_DIR;
if(!out)throw Error('An artifact output directory is required');fs.mkdirSync(out,{recursive:true});
const report={passed:false,cases:[],errors:[]};
(async()=>{let browser;try{
 const css=['ux-foundations.css','discovery-workspace.css'].map(name=>fs.readFileSync(path.join(root,'web/assets',name),'utf8')).join('\n');
 browser=await chromium.launch({headless:true});const page=await browser.newPage({viewport:{width:600,height:200}});
 for(const theme of ['dark','light']){
  const tokens=theme==='dark'?':root{--accent:#6eabff;--input:#1b2230;--muted:#9aa4b2;--ux-radius-sm:6px}body{background:#0f1420}':':root{--accent:#1765c5;--input:#e3e8ef;--muted:#5b6672;--ux-radius-sm:6px}body{background:#fff}';
  await page.setContent('<html data-theme="'+theme+'"><style>'+tokens+'body{margin:20px}'+css+'</style><div class="app-update-progress"><div class="row between gap6"><span>Installing and restarting</span></div><progress aria-label="Installing and restarting" style="width:400px"></progress></div><progress value="3" max="10" style="display:block;width:400px;margin-top:16px"></progress>');
  await page.waitForTimeout(250);
  const first=await page.screenshot({clip:{x:0,y:0,width:600,height:140}});await page.waitForTimeout(450);const second=await page.screenshot({clip:{x:0,y:0,width:600,height:140}});
  fs.writeFileSync(path.join(out,'progress-animation-'+theme+'.png'),second);
  const animating=!first.equals(second);
  const styles=await page.evaluate(()=>{const bars=[...document.querySelectorAll('progress')];return bars.map(p=>({radius:getComputedStyle(p).borderRadius,height:getComputedStyle(p).height,valueRadius:getComputedStyle(p,'::-webkit-progress-value').borderRadius,barRadius:getComputedStyle(p,'::-webkit-progress-bar').borderRadius,value:p.value,max:p.max,indeterminate:p.matches(':indeterminate')}));});
  assert(animating,theme+': an indeterminate progress bar must change between frames under the app stylesheets');
  assert.equal(styles[0].indeterminate,true);assert.equal(styles[1].indeterminate,false);
  assert.equal(styles[0].radius,'0px',theme+': no border-radius on the progress element itself (it stops the animation)');
  // Headless Chromium does not expose computed styles for the -webkit-progress
  // pseudo-elements, so the rounded look is reviewed from the saved screenshots.
  report.cases.push({theme,animating,styles});
 }
 report.passed=true;console.log(JSON.stringify(report));
}catch(e){report.errors.push(e.stack||e.message);console.error(e.stack||e.message);process.exitCode=1;}
finally{fs.writeFileSync(path.join(out,'progress-animation-report.json'),JSON.stringify(report,null,2));await browser?.close();}})();
