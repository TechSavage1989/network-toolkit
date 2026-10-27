package main

import (
 "context"
 "encoding/json"
 "encoding/base64"
 "encoding/hex"
 "strconv"
 "time"
 "errors"
 "fmt"
 "net"
 "strings"
 "unicode"

 "networktoolkit/internal/desktop"
 "networktoolkit/internal/diag"
 "networktoolkit/internal/model"
 "networktoolkit/internal/terminal"
)

const maxCredentialBindings = 32

type credentialBinding struct {
 Protocol string `json:"protocol"`
 Host string `json:"host"`
 Port int `json:"port"`
 Username string `json:"username"`
 Fingerprint string `json:"fingerprint,omitempty"`
}

func normalizeCredentialBinding(b credentialBinding) (credentialBinding, error) {
 b.Protocol = strings.ToLower(strings.TrimSpace(b.Protocol))
 switch b.Protocol {
 case "ssh", "ftp-explicit", "ftp-implicit":
 default: return b, errors.New("Saved credentials require SSH or encrypted FTPS.")
 }
 b.Host = strings.ToLower(strings.TrimSuffix(strings.TrimSpace(b.Host), "."))
 if strings.HasPrefix(b.Host, "[") && strings.HasSuffix(b.Host, "]") { b.Host = b.Host[1:len(b.Host)-1] }
 if ip := net.ParseIP(b.Host); ip != nil { b.Host = ip.String() } else {
  if b.Host == "" || len(b.Host) > 253 || strings.ContainsAny(b.Host, ":/\\@?#%") || strings.IndexFunc(b.Host, func(r rune) bool { return unicode.IsSpace(r) || unicode.IsControl(r) }) >= 0 { return b, errors.New("Enter a destination hostname or IP address without a URL, port, or control characters.") }
  for _,label := range strings.Split(b.Host,".") {
   if label == "" || len(label)>63 || strings.HasPrefix(label,"-") || strings.HasSuffix(label,"-") { return b, errors.New("Enter a literal destination hostname without wildcards or empty labels.") }
   for _,r := range label { if !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-' || r == '_') { return b,errors.New("Enter an ASCII destination hostname or IP address without wildcards.") } }
  }
 }
 if b.Port == 0 { if b.Protocol == "ssh" { b.Port = 22 } else if b.Protocol == "ftp-implicit" { b.Port = 990 } else { b.Port = 21 } }
 if b.Port < 1 || b.Port > 65535 { return b, errors.New("Credential destination port is invalid.") }
 // Usernames are case-sensitive on SSH servers. Do not lowercase or substitute
 // a different identity during lookup.
 if b.Username == "" || len(b.Username) > 256 || strings.IndexFunc(b.Username, unicode.IsControl) >= 0 { return b, errors.New("Enter the destination username without control characters.") }
 b.Fingerprint = strings.TrimSpace(b.Fingerprint)
 if b.Fingerprint != "" {
  if b.Protocol == "ssh" {
   raw,err:=base64.RawStdEncoding.DecodeString(strings.TrimPrefix(b.Fingerprint,"SHA256:"))
   if !strings.HasPrefix(b.Fingerprint,"SHA256:")||err!=nil||len(raw)!=32{return b,errors.New("SSH destination requires a valid SHA256 host-key fingerprint.")}
  } else {
   raw,err:=hex.DecodeString(strings.TrimPrefix(strings.ToUpper(b.Fingerprint),"SHA256:"))
   if err!=nil||len(raw)!=32{return b,errors.New("FTPS destination requires a valid SHA256 certificate fingerprint.")}
   b.Fingerprint="SHA256:"+strings.ToUpper(hex.EncodeToString(raw))
  }
 }
 return b, nil
}

func normalizeCredentialBindings(input []credentialBinding) ([]credentialBinding, error) {
 if len(input) > maxCredentialBindings { return nil, errors.New("A credential is limited to 32 approved destinations.") }
 out := make([]credentialBinding, 0, len(input))
 for _, item := range input {
  normalized, err := normalizeCredentialBinding(item); if err != nil { return nil, err }
  for _, prior := range out { if sameCredentialTarget(prior,normalized) && prior.Fingerprint!=normalized.Fingerprint { return nil,errors.New("A credential destination cannot have conflicting server identities.") } }
  if !hasCredentialBinding(out, normalized) { out = append(out, normalized) }
 }
 return out, nil
}

func sameCredentialTarget(a,b credentialBinding) bool { return a.Protocol==b.Protocol && a.Host==b.Host && a.Port==b.Port && a.Username==b.Username }

// Inspect SSH identity without sending usernames, passwords or keys. The native
// approval then requires the user to verify this public fingerprint independently.
func inspectCredentialDestination(ctx context.Context,b credentialBinding)(string,error) {
 if b.Protocol!="ssh" {return "",nil}
 probe,cancel:=context.WithTimeout(ctx,12*time.Second);defer cancel()
 result,err:=diag.Run(probe,model.Request{Tool:"sftp",Target:b.Host,Options:map[string]any{"operation":"fingerprint","port":b.Port,"timeoutMs":10000}},nil)
 if err!=nil{return "",errors.New("Could not inspect the SSH server identity. Verify the target and try approving it again.")}
 data,ok:=result.Data.(map[string]any);if !ok{return "",errors.New("SSH server identity was unavailable.")}
 fingerprint,_:=data["hostKeyFingerprint"].(string)
 if fingerprint==""{return "",errors.New("SSH server did not supply a host-key fingerprint.")}
 return fingerprint,nil
}

func (a *application) hydrateCredentialBindings(ctx context.Context,before,after []credentialBinding)([]credentialBinding,error) {
 out:=append([]credentialBinding{},after...)
 for i,b:=range out {
  if b.Fingerprint=="" { for _,old:=range before {if sameCredentialTarget(old,b){b.Fingerprint=old.Fingerprint;break}} }
  if b.Protocol=="ssh" && b.Fingerprint=="" && a.terminals!=nil {
   hosts,err:=a.terminals.KnownHosts();if err!=nil{return nil,err}
   endpoint:=net.JoinHostPort(b.Host,strconv.Itoa(b.Port))
   for _,host:=range hosts {if host.Host==endpoint{b.Fingerprint=host.Fingerprint;break}}
  }
  if b.Protocol=="ssh" && b.Fingerprint=="" {
   if a.inspectCredentialDestination==nil{return nil,errors.New("SSH identity inspection is unavailable.")}
   fingerprint,err:=a.inspectCredentialDestination(ctx,b);if err!=nil{return nil,err};b.Fingerprint=fingerprint
  }
  var err error;out[i],err=normalizeCredentialBinding(b);if err!=nil{return nil,err}
 }
 return normalizeCredentialBindings(out)
}

func hasCredentialBinding(bindings []credentialBinding, wanted credentialBinding) bool {
 for _, binding := range bindings { if binding == wanted { return true } }
 return false
}

func credentialMetadataFor(item credentialRecord) credentialMetadata {
 return credentialMetadata{ID:item.ID, Name:item.Name, Username:item.Username, Kind:item.Kind, UpdatedAt:item.UpdatedAt, Bindings:append([]credentialBinding{}, item.Bindings...)}
}

func (a *application) approveCredentialBindings(ctx context.Context, name string, before, after []credentialBinding) error {
 for _, candidate := range after {
  if hasCredentialBinding(before, candidate) { continue }
  if a.confirmCredentialDestination == nil { return errors.New("Native destination approval is unavailable. No credential destination was changed.") }
  allowed, err := a.confirmCredentialDestination(ctx, desktop.CredentialDestinationConfirmation{Action:"allow", CredentialName:name, Protocol:candidate.Protocol, Host:candidate.Host, Port:candidate.Port, Username:candidate.Username, Fingerprint:candidate.Fingerprint})
  if err != nil { return err }
  if err := ctx.Err(); err != nil { return err }
  if !allowed { return errors.New("Credential destination approval was canceled.") }
 }
 return nil
}

func (a *application) resolveCredential(ctx context.Context, id string, binding credentialBinding) (credentialRecord, error) {
 if err := ctx.Err(); err != nil { return credentialRecord{}, err }
 if a.credentials == nil { return credentialRecord{}, errors.New("The protected credential vault is unavailable.") }
 if id == "" || strings.TrimSpace(id) != id || len(id) > 120 || strings.ContainsAny(id, "/\\\x00") { return credentialRecord{}, errors.New("Saved credential ID is invalid.") }
 wanted, err := normalizeCredentialBinding(binding); if err != nil { return credentialRecord{}, err }
 item, found := a.credentials.get(id)
 if !found { return credentialRecord{}, errors.New("Saved credential was removed or is unavailable.") }
 for _,approved:=range item.Bindings {
  if sameCredentialTarget(approved,wanted) {
   if approved.Protocol=="ssh"&&approved.Fingerprint=="" {return credentialRecord{},errors.New("Approve the SSH server identity before using this credential.")}
   item.Bindings=[]credentialBinding{approved};return item,nil
  }
 }
 return credentialRecord{}, errors.New("This credential is not approved for the requested destination and username. Approve it in the password manager before connecting.")
}

func (a *application) resolveTerminalCredential(ctx context.Context, request terminal.ConnectRequest) (terminal.ConnectRequest, error) {
 if request.CredentialID == "" { return request, nil }
 if request.Type != "" && request.Type != "ssh" { return request, errors.New("Saved credentials may only authenticate SSH terminals.") }
 if request.Password != "" || request.PrivateKey != "" || request.Passphrase != "" { return request, errors.New("Choose a saved credential or enter authentication details, not both.") }
 item, err := a.resolveCredential(ctx, request.CredentialID, credentialBinding{Protocol:"ssh", Host:request.Host, Port:request.Port, Username:request.Username})
 if err != nil { return request, err }
 request.ExpectedHostFingerprint=item.Bindings[0].Fingerprint
 switch item.Kind {
 case "password": request.AuthMethod, request.Password = "password", item.Secret
 case "ssh-key":
  var key struct { PrivateKey string `json:"privateKey"`; Passphrase string `json:"passphrase"` }
  if err := json.Unmarshal([]byte(item.Secret), &key); err != nil || strings.TrimSpace(key.PrivateKey) == "" { return request, errors.New("Saved SSH key is invalid. Replace it in the password manager.") }
  request.AuthMethod, request.PrivateKey, request.Passphrase = "key", key.PrivateKey, key.Passphrase
 default: return request, errors.New("This saved credential type cannot authenticate an SSH connection.")
 }
 return request, nil
}

func (a *application) resolveRemoteCredential(ctx context.Context, r remoteBrowserRequest) (remoteBrowserRequest, error) {
 if r.CredentialID == "" { return r, nil }
 if r.SessionID != "" || r.ConnectionID != "" || r.Password != "" || r.PrivateKey != "" || r.KeyPath != "" || r.Passphrase != "" || r.Action == "fingerprint" { return r, errors.New("Saved credentials cannot be combined with another connection or authentication method.") }
 if r.Protocol == "ssh" || r.Protocol == "sftp" {
  request, err := a.resolveTerminalCredential(ctx, terminal.ConnectRequest{Type:"ssh", Host:r.Host, Port:r.Port, Username:r.Username, CredentialID:r.CredentialID})
  if err != nil { return r, err }
  if r.Fingerprint!="" && r.Fingerprint!=request.ExpectedHostFingerprint {return r,errors.New("The requested SSH identity differs from the approved credential destination.")}
  r.Fingerprint=request.ExpectedHostFingerprint
  r.AuthMode, r.Password, r.PrivateKey, r.Passphrase = request.AuthMethod, request.Password, request.PrivateKey, request.Passphrase
  return r, nil
 }
 if r.Protocol != "ftp" { return r, errors.New("Unsupported credential destination protocol.") }
 security := r.Security; if security == "" { security = "explicit" }
 if security != "explicit" && security != "implicit" { return r, errors.New("Saved passwords cannot be sent over unencrypted FTP. Select FTPS.") }
 item, err := a.resolveCredential(ctx, r.CredentialID, credentialBinding{Protocol:"ftp-"+security, Host:r.Host, Port:r.Port, Username:r.Username})
 if err != nil { return r, err }
 if item.Kind != "password" { return r, fmt.Errorf("FTPS requires a saved password, not %s.", item.Kind) }
 if r.Fingerprint!="" && item.Bindings[0].Fingerprint!="" && !strings.EqualFold(r.Fingerprint,item.Bindings[0].Fingerprint) {return r,errors.New("The requested FTPS identity differs from the approved credential destination.")}
 r.Fingerprint=item.Bindings[0].Fingerprint // Empty means normal CA and hostname verification, never caller-supplied trust.
 r.Password = item.Secret
 return r, nil
}
