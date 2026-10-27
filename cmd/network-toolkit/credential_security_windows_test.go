//go:build windows

package main

import (
 "bytes"
 "os"
 "strings"
 "testing"
)

func TestWindowsCredentialDPAPIAndFilePermissions(t *testing.T) {
 root:=t.TempDir()
 vault,err:=newCredentialVault(root);if err!=nil{t.Fatal(err)}
 secret:="windows-native-synthetic-credential-canary"
 saved,err:=vault.put(credentialRecord{Name:"Native synthetic",Username:"operator",Kind:"password",Secret:secret});if err!=nil{t.Fatal(err)}
 raw,err:=os.ReadFile(vault.path);if err!=nil{t.Fatal(err)}
 if bytes.Contains(raw,[]byte(secret))||bytes.Contains(raw,[]byte("Native synthetic")){t.Fatal("DPAPI file exposed plaintext")}
 loaded,err:=newCredentialVault(root);if err!=nil{t.Fatal(err)}
 item,found:=loaded.get(saved.ID);if !found||item.Secret!=secret{t.Fatal("native DPAPI same-user reload failed")}
 sid,err:=credentialCurrentUserSID();if err!=nil{t.Fatal(err)}
 dacl,err:=credentialFileDACL(vault.path);if err!=nil{t.Fatal(err)}
 if !strings.HasPrefix(dacl,"D:P")||!strings.Contains(dacl,sid)||!strings.Contains(dacl,";;;SY)")||!strings.Contains(dacl,";;;BA)"){t.Fatalf("unexpected protected DACL: %s",dacl)}
 if strings.Count(dacl,"(A;")!=3||strings.Contains(dacl,";;;WD)")||strings.Contains(dacl,";;;BU)")||strings.Contains(dacl,";;;AU)"){t.Fatalf("broad credential DACL: %s",dacl)}
 raw[len(raw)-1]^=1
 if _,err:=unprotectCredentialBlob(raw);err==nil{t.Fatal("tampered DPAPI blob decrypted")}
}
