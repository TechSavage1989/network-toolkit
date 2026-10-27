//go:build windows

package main

import (
 "bytes"
 "errors"
 "os"
 "path/filepath"
 "runtime"
 "syscall"
 "testing"
 "unsafe"
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
 assertPrivateCredentialDACL(t,vault.path)
 raw[len(raw)-1]^=1
 if _,err:=unprotectCredentialBlob(raw);err==nil{t.Fatal("tampered DPAPI blob decrypted")}
}

func TestWindowsCredentialTempFileProtectedAtCreation(t *testing.T) {
	dir := t.TempDir()
	file, err := createSecureCredentialTempFile(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	if filepath.Dir(file.Name()) != dir {
		t.Fatal("temporary credential file escaped its directory")
	}
	info, err := file.Stat()
	if err != nil {
		t.Fatal(err)
	}
	if !info.Mode().IsRegular() || info.Size() != 0 {
		t.Fatal("protected creation did not return an empty regular file")
	}
	var handleFlags uint32
	ok, _, callErr := syscall.NewLazyDLL("kernel32.dll").NewProc("GetHandleInformation").Call(file.Fd(), uintptr(unsafe.Pointer(&handleFlags)))
	if ok == 0 { t.Fatal(callErr) }
	if handleFlags & 1 != 0 { t.Fatal("credential temporary handle can be inherited by child processes") }
	// Inspect the actual native DACL before any credential bytes are written.
	assertPrivateCredentialDACL(t,file.Name())
	if _, err := file.Write([]byte("synthetic encrypted bytes")); err != nil {
		t.Fatal(err)
	}
	// The writer's handle does not share data access or delete/rename access.
	if reader, err := os.Open(file.Name()); err == nil {
		reader.Close()
		t.Fatal("second process-style reader opened an active credential temp file")
	}
	if writer, err := os.OpenFile(file.Name(), os.O_WRONLY, 0); err == nil {
		writer.Close()
		t.Fatal("second process-style writer opened an active credential temp file")
	}
	if err := os.Rename(file.Name(), filepath.Join(dir, "unexpected-move")); err == nil {
		t.Fatal("credential temporary file could be renamed while open")
	}
	if err := os.Remove(file.Name()); err == nil {
		t.Fatal("credential temporary file could be removed while open")
	}
}

// Inspect native ACE trustees and masks rather than SDDL display aliases. For
// example, Windows can render the actual current-user SID as LA on a hosted
// Administrator account. This still requires that exact SID, not any LA alias.
func assertPrivateCredentialDACL(t *testing.T, path string) {
	t.Helper()
	name, err := syscall.UTF16PtrFromString(path)
	if err != nil { t.Fatal(err) }
	var descriptor uintptr
	code, _, _ := credentialGetNamedSecurity.Call(uintptr(unsafe.Pointer(name)), 1, 4, 0, 0, 0, 0, uintptr(unsafe.Pointer(&descriptor)))
	runtime.KeepAlive(name)
	if code != 0 { t.Fatal(syscall.Errno(code)) }
	defer syscall.LocalFree(syscall.Handle(descriptor))
	var control uint16
	var revision uint32
	ok, _, callErr := credentialSecurityDLL.NewProc("GetSecurityDescriptorControl").Call(descriptor, uintptr(unsafe.Pointer(&control)), uintptr(unsafe.Pointer(&revision)))
	if ok == 0 { t.Fatal(callErr) }
	// SE_DACL_PRESENT and SE_DACL_PROTECTED must both be set. AI is an OS
	// bookkeeping bit; inheritance is forbidden on each individual ACE below.
	if control & (0x0004|0x1000) != (0x0004|0x1000) { t.Fatalf("credential DACL is absent or unprotected: %#x", control) }
	var present, defaulted int32
	var dacl uintptr
	ok, _, callErr = credentialGetDescriptorDACL.Call(descriptor, uintptr(unsafe.Pointer(&present)), uintptr(unsafe.Pointer(&dacl)), uintptr(unsafe.Pointer(&defaulted)))
	if ok == 0 { t.Fatal(callErr) }
	if present == 0 || dacl == 0 || defaulted != 0 { t.Fatal("credential DACL is missing, null or defaulted") }
	var info struct { AceCount, AclBytesInUse, AclBytesFree uint32 }
	ok, _, callErr = credentialSecurityDLL.NewProc("GetAclInformation").Call(dacl, uintptr(unsafe.Pointer(&info)), unsafe.Sizeof(info), 2)
	if ok == 0 { t.Fatal(callErr) }
	if info.AceCount != 3 { t.Fatalf("credential DACL has %d ACEs, expected exactly 3", info.AceCount) }
	userSID, err := credentialCurrentUserSID()
	if err != nil { t.Fatal(err) }
	want := map[string]int{}
	for _, sid := range []string{userSID, "S-1-5-18", "S-1-5-32-544"} { want[sid]++ }
	for i := uint32(0); i < info.AceCount; i++ {
		var ace uintptr
		ok, _, callErr = credentialSecurityDLL.NewProc("GetAce").Call(dacl, uintptr(i), uintptr(unsafe.Pointer(&ace)))
		if ok == 0 { t.Fatal(callErr) }
		if ace == 0 { t.Fatal("native ACE is null") }
		header := (*struct { Kind, Flags uint8; Size uint16; Mask uint32 })(unsafe.Pointer(ace))
		// ACCESS_ALLOWED_ACE_TYPE, no inheritance/propagation flags, FILE_ALL_ACCESS.
		if header.Kind != 0 || header.Flags != 0 || header.Size < 16 || header.Mask != 0x001f01ff { t.Fatalf("unexpected credential ACE: %#v", *header) }
		sid := (*syscall.SID)(unsafe.Pointer(ace + 8))
		valid, _, _ := credentialSecurityDLL.NewProc("IsValidSid").Call(uintptr(unsafe.Pointer(sid)))
		if valid == 0 { t.Fatal("invalid native ACE SID") }
		length, _, _ := credentialSecurityDLL.NewProc("GetLengthSid").Call(uintptr(unsafe.Pointer(sid)))
		if length < 8 || length > uintptr(header.Size)-8 { t.Fatal("ACE SID extends beyond its native entry") }
		value, err := sid.String()
		if err != nil { t.Fatal(err) }
		if want[value] == 0 { t.Fatalf("unexpected or duplicate credential trustee: %s", value) }
		want[value]--
	}
	for sid, count := range want { if count != 0 { t.Fatalf("missing credential trustee: %s", sid) } }
}

func TestWindowsCredentialTempCollisionDoesNotTouchExistingFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "existing-synthetic-file")
	want := []byte("existing synthetic file must not change")
	if err := os.WriteFile(path, want, 0600); err != nil {
		t.Fatal(err)
	}
	before, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	beforeDACL, err := credentialFileDACL(path)
	if err != nil {
		t.Fatal(err)
	}
	file, err := createSecureCredentialFileAt(path)
	if file != nil {
		file.Close()
		t.Fatal("CREATE_NEW returned an existing file")
	}
	if !errors.Is(err, os.ErrExist) {
		t.Fatalf("existing path did not fail with file-exists: %v", err)
	}
	after, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	afterDACL, err := credentialFileDACL(path)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !os.SameFile(before, after) || !bytes.Equal(raw, want) || beforeDACL != afterDACL {
		t.Fatal("existing file identity, contents or permissions changed on collision")
	}
}
