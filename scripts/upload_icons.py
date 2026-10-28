"""One authorized additive icon-asset upload; no build, tag, body or old-asset writes."""
from pathlib import Path
import copy, hashlib, io, json, os, subprocess, sys, urllib.request, urllib.parse, zipfile
REPO='TechSavage1989/network-toolkit'
TAG='v1.17.109'
RELEASE_ID=426910194
COMMIT='d3494a879604fb2523e0867ad1fb175a2f9abbf5'
BRANCH='icon-download-2026-10-28'
NAME='Toolkit-Icons.zip'
SIZE=303227
SHA='fddf951082148f75f0034ddfe3557beeb0d2a302343a5afeaa55976d1068f284'
BASELINE_SHA='2a136ab141304ca2be28b50274f6e4dd52944148b2fb0de2156e13ca94ce3748'
BODY_SHA='fa6274bff2f0ad26a6efaf6c3ee4146d15ca4138eefcf18b9a1c4b1f06853cc7'
ASSET_FIELDS=('id','name','size','digest','state','browser_download_url','created_at','updated_at')
def require(ok,message):
    if not ok: raise RuntimeError(message)
def digest(b): return hashlib.sha256(b).hexdigest()
def api(endpoint):
    require(endpoint.startswith('repos/'+REPO+'/'),'Unexpected repository read')
    return json.loads(subprocess.check_output(['gh','api','--method','GET',endpoint],text=True))
def existing_asset(a):
    require(a['name']==NAME and a['size']==SIZE and a.get('digest')=='sha256:'+SHA and a['state']=='uploaded','Destination asset exists but differs; refusing any overwrite or deletion')
    require(a['browser_download_url']=='https://github.com/'+REPO+'/releases/download/'+TAG+'/'+NAME,'Unexpected new-asset download URL')
    require(a.get('content_type')=='application/zip','Unexpected new-asset MIME type')
def validate_release(r,baseline):
    require(r['id']==RELEASE_ID and r['tag_name']==TAG,'Release identity changed')
    require(not r['draft'] and not r['prerelease'] and not r.get('immutable',False),'Expected mutable stable published release')
    require(r['target_commitish']==COMMIT and r['name']==baseline['name'],'Release target/title changed')
    require(digest(r['body'].encode())==BODY_SHA and r['body']==baseline['body'],'Release body changed')
    originals={a['name']:a for a in baseline['assets']}
    assets={a['name']:a for a in r['assets']}
    require(len(assets)==len(r['assets']),'Duplicate release asset names')
    require(set(assets)==set(originals) or set(assets)==set(originals)|{NAME},'Unexpected asset inventory; refusing upload')
    for name,old in originals.items():
        require(all(assets[name].get(k)==old.get(k) for k in ASSET_FIELDS),'Original release asset changed: '+name)
    if NAME in assets: existing_asset(assets[NAME])
    return assets.get(NAME)
def validate_zip(data):
    require(len(data)==SIZE and digest(data)==SHA,'ZIP bytes/hash do not match authorized artifact')
    with zipfile.ZipFile(io.BytesIO(data)) as z:
        require(z.testzip() is None and len(z.namelist())==105,'Invalid ZIP or unexpected inventory')
        require(sum(n.endswith('.svg') for n in z.namelist())==92,'Unexpected SVG count')
        require(all(n.startswith('Network-Toolkit-Editable-SVG-Icons/') and '..' not in Path(n).parts for n in z.namelist()),'Unexpected archive paths')
class PublicRedirects(urllib.request.HTTPRedirectHandler):
    def redirect_request(self,req,fp,code,msg,headers,newurl):
        u=urllib.parse.urlsplit(newurl)
        require(u.scheme=='https' and u.hostname in {'github.com','release-assets.githubusercontent.com','objects.githubusercontent.com'},'Untrusted public download redirect')
        return super().redirect_request(req,fp,code,msg,headers,newurl)
def verify_public(a):
    existing_asset(a)
    # Intentionally no Authorization header/cookies: prove public bytes, not authenticated access.
    request=urllib.request.Request(a['browser_download_url'],headers={'User-Agent':'Network-Toolkit-Icon-Asset-Verification'})
    with urllib.request.build_opener(PublicRedirects()).open(request,timeout=90) as response:
        require(response.status==200,'Public download did not return200')
        data=response.read(SIZE+1)
    validate_zip(data)
def main():
    require(os.environ.get('GITHUB_REPOSITORY')==REPO,'Wrong Actions repository')
    require(os.environ.get('GITHUB_REF')=='refs/heads/'+BRANCH,'Wrong branch')
    require(os.environ.get('GITHUB_EVENT_NAME')=='push','Only the fixed branch push is authorized')
    raw=Path('release-baseline.json').read_bytes();require(digest(raw)==BASELINE_SHA,'Baseline changed')
    baseline=json.loads(raw);validate_release(baseline,baseline)
    validate_zip(Path(NAME).read_bytes())
    require(api('repos/'+REPO+'/releases/latest')['id']==RELEASE_ID,'Latest release changed; stop before upload')
    ref=api('repos/'+REPO+'/git/ref/tags/'+TAG)
    require(ref['object']=={'sha':COMMIT,'type':'commit','url':'https://api.github.com/repos/'+REPO+'/git/commits/'+COMMIT},'Published tag changed')
    main_before=api('repos/'+REPO+'/git/ref/heads/main')['object']['sha']
    before=api('repos/'+REPO+'/releases/'+str(RELEASE_ID));asset=validate_release(before,baseline)
    mode='already-present-identical'
    if asset is None:
        endpoint='https://uploads.github.com/repos/'+REPO+'/releases/'+str(RELEASE_ID)+'/assets?name='+NAME
        require(before['upload_url'].split('{',1)[0]+'?name='+NAME==endpoint,'Unexpected upload endpoint')
        # Exactly one additive POST, pinned release ID; no clobber, PATCH, DELETE or release creation.
        uploaded=subprocess.run(['gh','api','--method','POST','-H','Content-Type: application/zip','--input',NAME,endpoint],capture_output=True,text=True)
        if uploaded.returncode:
            # Read the uncertain result once; never automatically repeat a write.
            outcome=api('repos/'+REPO+'/releases/'+str(RELEASE_ID));asset=validate_release(outcome,baseline)
            require(asset is not None,'Upload failed with no verified matching destination; no retry performed: '+uploaded.stderr[-1000:])
            mode='verified-after-uncertain-response'
        else:
            asset=json.loads(uploaded.stdout);existing_asset(asset);mode='uploaded-once'
    after=api('repos/'+REPO+'/releases/'+str(RELEASE_ID));asset=validate_release(after,baseline)
    require(asset is not None,'New asset missing after upload')
    require(api('repos/'+REPO+'/git/ref/tags/'+TAG)==ref,'Published tag changed during operation')
    require(api('repos/'+REPO+'/git/ref/heads/main')['object']['sha']==main_before,'Main changed concurrently; verify separately')
    verify_public(asset)
    result={'result':mode,'asset':{k:asset[k] for k in ('id','name','size','digest','browser_download_url','content_type','state')},'public_download_hash_verified':True,'svg_count':92,'original_four_assets_unchanged':True,'release_body_unchanged':True,'tag_unchanged':True,'main_unchanged':main_before}
    print(json.dumps(result,indent=2))
    with open(os.environ['GITHUB_STEP_SUMMARY'],'a') as f:f.write('## Authorized additive icon download\n\n```json\n'+json.dumps(result,indent=2)+'\n```\n')
if __name__=='__main__': main()
