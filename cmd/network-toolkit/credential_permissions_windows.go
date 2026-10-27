//go:build windows

package main

import (
 "crypto/rand"
 "encoding/hex"
 "errors"
 "os"
 "path/filepath"
 "runtime"
 "syscall"
 "unsafe"
)

var credentialSecurityDLL = syscall.NewLazyDLL("advapi32.dll")
var credentialConvertDescriptor = credentialSecurityDLL.NewProc("ConvertStringSecurityDescriptorToSecurityDescriptorW")
var credentialGetDescriptorDACL = credentialSecurityDLL.NewProc("GetSecurityDescriptorDacl")
var credentialGetNamedSecurity = credentialSecurityDLL.NewProc("GetNamedSecurityInfoW")
var credentialDescriptorToString = credentialSecurityDLL.NewProc("ConvertSecurityDescriptorToStringSecurityDescriptorW")

func credentialCurrentUserSID() (string,error) {
 token,err:=syscall.OpenCurrentProcessToken();if err!=nil{return "",err};defer token.Close()
 user,err:=token.GetTokenUser();if err!=nil{return "",err}
 return user.User.Sid.String()
}

// Create the encrypted temporary file with its protected DACL already in place.
// CREATE_NEW never opens an existing file, and no pathname-based ACL mutation is
// performed. A non-inheritable, exclusive handle prevents another process from
// reading, writing, renaming or deleting this file while it is being written.
// Existing user files/directories are never rewritten merely by opening a vault.
func createSecureCredentialTempFile(dir string) (*os.File, error) {
 for attempt := 0; attempt < 100; attempt++ {
  var random [24]byte
  if _,err:=rand.Read(random[:]);err!=nil{return nil,err}
  path:=filepath.Join(dir,".network-toolkit-credential-"+hex.EncodeToString(random[:]))
  file,err:=createSecureCredentialFileAt(path)
  if errors.Is(err,syscall.ERROR_FILE_EXISTS)||errors.Is(err,syscall.ERROR_ALREADY_EXISTS){continue}
  return file,err
 }
 return nil,errors.New("Could not allocate a unique credential temporary file")
}

// Kept separate so native tests can prove that an existing-name collision has
// no side effects. Callers must provide a freshly generated temporary pathname.
func createSecureCredentialFileAt(name string) (*os.File, error) {
 sid,err:=credentialCurrentUserSID();if err!=nil{return nil,err}
 text,err:=syscall.UTF16PtrFromString("D:P(A;;FA;;;"+sid+")(A;;FA;;;SY)(A;;FA;;;BA)");if err!=nil{return nil,err}
 var descriptor uintptr
 ok,_,callErr:=credentialConvertDescriptor.Call(uintptr(unsafe.Pointer(text)),1,uintptr(unsafe.Pointer(&descriptor)),0)
 runtime.KeepAlive(text)
 if ok==0{return nil,callErr}
 defer syscall.LocalFree(syscall.Handle(descriptor))
 var present,defaulted int32
 var dacl uintptr
 ok,_,callErr=credentialGetDescriptorDACL.Call(descriptor,uintptr(unsafe.Pointer(&present)),uintptr(unsafe.Pointer(&dacl)),uintptr(unsafe.Pointer(&defaulted)))
 if ok==0{return nil,callErr};if present==0||dacl==0{return nil,errors.New("Private credential file permissions could not be prepared")}
 path,err:=syscall.UTF16PtrFromString(name);if err!=nil{return nil,err}
 attributes:=syscall.SecurityAttributes{SecurityDescriptor:descriptor}
 attributes.Length=uint32(unsafe.Sizeof(attributes))
 handle,err:=syscall.CreateFile(path,syscall.GENERIC_READ|syscall.GENERIC_WRITE,0,&attributes,syscall.CREATE_NEW,syscall.FILE_ATTRIBUTE_NORMAL,0)
 runtime.KeepAlive(path)
 runtime.KeepAlive(attributes)
 if err!=nil{return nil,&os.PathError{Op:"create",Path:name,Err:err}}
 file:=os.NewFile(uintptr(handle),name)
 before,err:=file.Stat()
 if err!=nil{file.Close();return nil,err}
 after,err:=os.Lstat(name)
 if err!=nil{file.Close();return nil,err}
 if !before.Mode().IsRegular()||!after.Mode().IsRegular()||!os.SameFile(before,after){
  file.Close()
  return nil,errors.New("Credential temporary file changed while creating private storage")
 }
 return file,nil
}

// Used by native regression checks to inspect the DACL actually written by the
// operating system, not the requested Unix mode or an assumed inherited ACL.
func credentialFileDACL(path string) (string,error) {
 name,err:=syscall.UTF16PtrFromString(path);if err!=nil{return "",err}
 var descriptor uintptr
 code,_,_:=credentialGetNamedSecurity.Call(uintptr(unsafe.Pointer(name)),1,4,0,0,0,0,uintptr(unsafe.Pointer(&descriptor)))
 runtime.KeepAlive(name)
 if code!=0{return "",syscall.Errno(code)}
 defer syscall.LocalFree(syscall.Handle(descriptor))
 var result uintptr
 var size uint32
 ok,_,callErr:=credentialDescriptorToString.Call(descriptor,1,4,uintptr(unsafe.Pointer(&result)),uintptr(unsafe.Pointer(&size)))
 if ok==0{return "",callErr}
 defer syscall.LocalFree(syscall.Handle(result))
 if result==0||size==0||size>16384{return "",errors.New("Invalid credential DACL response")}
 return syscall.UTF16ToString(unsafe.Slice((*uint16)(unsafe.Pointer(result)),int(size))),nil
}
