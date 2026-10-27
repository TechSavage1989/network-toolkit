package main

import (
 "bytes"
 "context"
 "crypto/aes"
 "crypto/cipher"
 "crypto/rand"
 "encoding/json"
 "errors"
 "net/http"
 "os"
 "path/filepath"
 "runtime"
 "strings"
 "testing"

 "networktoolkit/internal/desktop"
 "networktoolkit/internal/terminal"
)

// Explicit encrypted test provider, scoped to one synthetic vault. Production
// constructors cannot select it, and no development flag enables it.
func syntheticCredentialVault(t *testing.T, root string) *credentialVault {
 t.Helper()
 key := make([]byte,32); if _,err := rand.Read(key); err != nil { t.Fatal(err) }
 block,err := aes.NewCipher(key); if err != nil { t.Fatal(err) }
 gcm,err := cipher.NewGCM(block); if err != nil { t.Fatal(err) }
 protect := func(plain []byte)([]byte,error) {
  nonce := make([]byte,gcm.NonceSize()); if _,err := rand.Read(nonce); err != nil { return nil,err }
  return gcm.Seal(nonce,nonce,plain,nil),nil
 }
 unprotect := func(raw []byte)([]byte,error) {
  if len(raw)<gcm.NonceSize() { return nil,errors.New("invalid synthetic ciphertext") }
  return gcm.Open(nil,raw[:gcm.NonceSize()],raw[gcm.NonceSize():],nil)
 }
 vault,err := newCredentialVaultWithProtection(root,protect,unprotect); if err != nil { t.Fatal(err) }
 return vault
}
func credentialTestApplication(t *testing.T) *application {
 a:=testApplication(t)
 a.credentials=syntheticCredentialVault(t,filepath.Dir(a.dataPath))
 a.inspectCredentialDestination=func(context.Context,credentialBinding)(string,error){return "SHA256:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",nil}
 a.confirmCredentialDestination=func(context.Context,desktop.CredentialDestinationConfirmation)(bool,error){return true,nil}
 return a
}
func fixtureCredential(t *testing.T,a *application,bindings ...credentialBinding) credentialMetadata {
 t.Helper()
 item,err:=a.credentials.put(credentialRecord{Name:"Synthetic SSH",Username:"operator",Kind:"password",Secret:"fixture-only-private-secret",Bindings:bindings})
 if err!=nil {t.Fatal(err)}
 return item
}
func fixtureBinding() credentialBinding { return credentialBinding{Protocol:"ssh",Host:"switch.example",Port:22,Username:"operator",Fingerprint:"SHA256:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"} }

func TestCredentialProtectionUnavailableFailsClosed(t *testing.T) {
 if runtime.GOOS=="windows" {t.Skip("Windows has the native provider")}
 dir:=t.TempDir()
 if _,err:=newCredentialVault(dir);err==nil {t.Fatal("unsupported build opened a production vault")}
 for _,operation:=range []func([]byte)([]byte,error){protectCredentialBlob,unprotectCredentialBlob} {
  if output,err:=operation([]byte("secret"));err==nil||len(output)!=0 {t.Fatal("unsupported build returned plaintext")}
 }
 entries,err:=os.ReadDir(dir);if err!=nil||len(entries)!=0 {t.Fatal("unsupported provider changed disk")}
}
func TestCredentialStorageContainsNoPlaintextAndDetectsTamper(t *testing.T) {
 a:=credentialTestApplication(t);item:=fixtureCredential(t,a,fixtureBinding())
 raw,err:=os.ReadFile(a.credentials.path);if err!=nil {t.Fatal(err)}
 if bytes.Contains(raw,[]byte("fixture-only-private-secret"))||bytes.Contains(raw,[]byte("Synthetic SSH")) {t.Fatal("plaintext credential data persisted")}
 loaded,err:=newCredentialVaultWithProtection(filepath.Dir(a.dataPath),a.credentials.protect,a.credentials.unprotect);if err!=nil {t.Fatal(err)}
 if record,ok:=loaded.get(item.ID);!ok||record.Secret!="fixture-only-private-secret" {t.Fatal("synthetic encrypted round trip failed")}
 raw[len(raw)-1]^=1;if err:=os.WriteFile(a.credentials.path,raw,0600);err!=nil {t.Fatal(err)}
 if _,err:=newCredentialVaultWithProtection(filepath.Dir(a.dataPath),a.credentials.protect,a.credentials.unprotect);err==nil {t.Fatal("tampered ciphertext accepted")}
}
func TestCredentialMetadataNeverReturnsSavedSecret(t *testing.T) {
 a:=credentialTestApplication(t);item:=fixtureCredential(t,a,fixtureBinding())
 for _,path:=range []string{"/api/credentials","/api/credentials/"+item.ID,"/api/state"} {
  result:=requestApp(a,http.MethodGet,path,"",true)
  if result.Code!=200||strings.Contains(result.Body.String(),"fixture-only-private-secret")||strings.Contains(result.Body.String(),`"secret"`) {t.Fatalf("secret API response %s: %d %s",path,result.Code,result.Body.String())}
 }
}
func TestCredentialDestinationApprovalCannotBeForgedInJSON(t *testing.T) {
 a:=credentialTestApplication(t);item:=fixtureCredential(t,a)
 calls:=0
 a.confirmCredentialDestination=func(ctx context.Context,p desktop.CredentialDestinationConfirmation)(bool,error){calls++;if p.Host!="switch.example"||p.Username!="operator" {t.Fatal("wrong native prompt target")};return false,nil}
 body:=`{"binding":{"protocol":"ssh","host":"switch.example","port":22,"username":"operator"},"approved":true}`
 result:=requestApp(a,http.MethodPost,"/api/credentials/"+item.ID+"/bindings",body,true)
 if result.Code==200||calls!=1 {t.Fatalf("forged approval accepted: %d / %d",result.Code,calls)}
 record,_:=a.credentials.get(item.ID);if len(record.Bindings)!=0 {t.Fatal("denied binding persisted")}
 a.confirmCredentialDestination=func(context.Context,desktop.CredentialDestinationConfirmation)(bool,error){return true,nil}
 result=requestApp(a,http.MethodPost,"/api/credentials/"+item.ID+"/bindings",body,true)
 if result.Code!=200||strings.Contains(result.Body.String(),"fixture-only-private-secret") {t.Fatalf("approved binding failed: %d %s",result.Code,result.Body.String())}
 record,_=a.credentials.get(item.ID);if len(record.Bindings)!=1 {t.Fatal("approved binding not saved")}
}
func TestCredentialResolutionRejectsUnboundAndRedirectedUse(t *testing.T) {
 a:=credentialTestApplication(t);item:=fixtureCredential(t,a,fixtureBinding())
 good:=terminal.ConnectRequest{Type:"ssh",Host:"switch.example",Port:22,Username:"operator",CredentialID:item.ID}
 resolved,err:=a.resolveTerminalCredential(context.Background(),good);if err!=nil||resolved.Password!="fixture-only-private-secret" {t.Fatalf("approved backend resolve: %v",err)}
 for _,mutate:=range []func(*terminal.ConnectRequest){
  func(r *terminal.ConnectRequest){r.Host="evil.example"},func(r *terminal.ConnectRequest){r.Port=2222},func(r *terminal.ConnectRequest){r.Username="root"},func(r *terminal.ConnectRequest){r.Type="telnet"},func(r *terminal.ConnectRequest){r.Password="mixed"},func(r *terminal.ConnectRequest){r.PrivateKey="mixed"},func(r *terminal.ConnectRequest){r.Passphrase="mixed"},func(r *terminal.ConnectRequest){r.CredentialID="missing"},
 } {r:=good;mutate(&r);if _,err:=a.resolveTerminalCredential(context.Background(),r);err==nil {t.Fatalf("unauthorized credential use accepted: %#v",r)}}
 legacy:=fixtureCredential(t,a);good.CredentialID=legacy.ID
 if _,err:=a.resolveTerminalCredential(context.Background(),good);err==nil {t.Fatal("legacy unbound credential was silently authorized")}
}
func TestCredentialPlainFTPAndMixedTransportRejected(t *testing.T) {
 a:=credentialTestApplication(t);item:=fixtureCredential(t,a,credentialBinding{Protocol:"ftp-explicit",Host:"files.example",Port:21,Username:"operator"})
 good:=remoteBrowserRequest{Protocol:"ftp",Security:"explicit",Host:"files.example",Port:21,Username:"operator",CredentialID:item.ID,Action:"list"}
 resolved,err:=a.resolveRemoteCredential(context.Background(),good);if err!=nil||resolved.Password=="" {t.Fatal(err)}
 for _,mutate:=range []func(*remoteBrowserRequest){func(r *remoteBrowserRequest){r.Security="plain"},func(r *remoteBrowserRequest){r.ConnectionID="existing"},func(r *remoteBrowserRequest){r.SessionID="existing"},func(r *remoteBrowserRequest){r.Password="mixed"},func(r *remoteBrowserRequest){r.KeyPath="key.pem"},func(r *remoteBrowserRequest){r.Action="fingerprint"}} {r:=good;mutate(&r);if _,err:=a.resolveRemoteCredential(context.Background(),r);err==nil {t.Fatal("mixed or unencrypted authentication accepted")}}
}
func TestCredentialMetadataUpdatePreservesSecretAndBinding(t *testing.T) {
 a:=credentialTestApplication(t);item:=fixtureCredential(t,a,fixtureBinding())
 body,_:=json.Marshal(map[string]any{"id":item.ID,"name":"Renamed synthetic","username":"operator","kind":"password"})
 result:=requestApp(a,http.MethodPost,"/api/credentials",string(body),true)
 if result.Code!=200||strings.Contains(result.Body.String(),"fixture-only-private-secret") {t.Fatalf("metadata save: %d %s",result.Code,result.Body.String())}
 record,_:=a.credentials.get(item.ID);if record.Secret!="fixture-only-private-secret"||len(record.Bindings)!=1 {t.Fatal("metadata edit changed secret or destination")}
}
func TestCredentialBindingNormalization(t *testing.T) {
 good,err:=normalizeCredentialBinding(credentialBinding{Protocol:"ssh",Host:"SWITCH.Example.",Username:"Operator"});if err!=nil||good.Host!="switch.example"||good.Port!=22||good.Username!="Operator" {t.Fatal(good,err)}
 for _,host:=range []string{"https://switch.example","operator@switch.example","switch.example:22","switch.example\nother","switch example"} {if _,err:=normalizeCredentialBinding(credentialBinding{Protocol:"ssh",Host:host,Username:"operator"});err==nil {t.Fatalf("invalid binding host accepted %q",host)}}
}

func TestCredentialQuotaIncludesPreassignedHTTPIDs(t *testing.T) {
 a:=credentialTestApplication(t)
 for i:=0;i<maxCredentialEntries;i++ { id:=strings.Repeat("x",i+1);a.credentials.entries[id]=credentialRecord{ID:id,Name:"fixture",Kind:"password",Secret:"fixture"} }
 response:=requestApp(a,http.MethodPost,"/api/credentials",`{"name":"Over quota","kind":"password","secret":"fixture"}`,true)
 if response.Code==200||len(a.credentials.entries)!=maxCredentialEntries {t.Fatal("HTTP-created ID bypassed credential quota")}
}
func TestCredentialCancellationCannotCommitBinding(t *testing.T) {
 a:=credentialTestApplication(t);item:=fixtureCredential(t,a)
 ctx,cancel:=context.WithCancel(context.Background())
 a.confirmCredentialDestination=func(context.Context,desktop.CredentialDestinationConfirmation)(bool,error){cancel();return true,nil}
 request,_:=http.NewRequestWithContext(ctx,http.MethodPost,"http://"+a.host+"/api/credentials",nil)
 record,_:=a.credentials.get(item.ID);record.Bindings=[]credentialBinding{fixtureBinding()}
 if _,err:=a.saveCredentialFromRequest(request,record);err==nil {t.Fatal("late confirmation committed after cancellation")}
 record,_=a.credentials.get(item.ID);if len(record.Bindings)!=0 {t.Fatal("canceled approval changed vault")}
}
func TestCredentialUseCannotReplaceApprovedServerIdentity(t *testing.T) {
 a:=credentialTestApplication(t);item:=fixtureCredential(t,a,fixtureBinding())
 request:=remoteBrowserRequest{Protocol:"sftp",Host:"switch.example",Port:22,Username:"operator",CredentialID:item.ID,Action:"connect",Fingerprint:"SHA256:BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB"}
 if _,err:=a.resolveRemoteCredential(context.Background(),request);err==nil {t.Fatal("renderer replaced approved SSH identity")}
 request.Fingerprint=""
 resolved,err:=a.resolveRemoteCredential(context.Background(),request);if err!=nil||resolved.Fingerprint!=fixtureBinding().Fingerprint {t.Fatalf("approved identity not supplied by backend: %v",err)}
}
func TestCredentialFTPSWithoutApprovedPinUsesSystemTrust(t *testing.T) {
 a:=credentialTestApplication(t);item:=fixtureCredential(t,a,credentialBinding{Protocol:"ftp-explicit",Host:"files.example",Port:21,Username:"operator"})
 request:=remoteBrowserRequest{Protocol:"ftp",Security:"explicit",Host:"files.example",Port:21,Username:"operator",CredentialID:item.ID,Action:"list",Fingerprint:"SHA256:"+strings.Repeat("B",64)}
 resolved,err:=a.resolveRemoteCredential(context.Background(),request);if err!=nil {t.Fatal(err)}
 if resolved.Fingerprint!="" {t.Fatal("renderer supplied an unapproved TLS bypass")}
}

func TestCredentialOversizeSaveRollsBackBeforeDiskWrite(t *testing.T) {
 a:=credentialTestApplication(t);item:=fixtureCredential(t,a)
 before,err:=os.ReadFile(a.credentials.path);if err!=nil{t.Fatal(err)}
 original:=a.credentials.protect
 a.credentials.protect=func([]byte)([]byte,error){return make([]byte,(2<<20)+1),nil}
 _,err=a.credentials.put(credentialRecord{ID:item.ID,Name:item.Name,Kind:"password",Secret:"replacement"})
 a.credentials.protect=original
 if err==nil{t.Fatal("oversized encrypted vault was accepted")}
 after,err:=os.ReadFile(a.credentials.path);if err!=nil||!bytes.Equal(before,after){t.Fatal("oversized save changed disk")}
 record,_:=a.credentials.get(item.ID);if record.Secret!="fixture-only-private-secret"{t.Fatal("oversized save changed memory")}
}
