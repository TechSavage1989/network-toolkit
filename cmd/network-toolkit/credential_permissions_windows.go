//go:build windows

package main

import (
 "errors"
 "os"
 "runtime"
 "syscall"
 "unsafe"
)

var credentialSecurityDLL = syscall.NewLazyDLL("advapi32.dll")
var credentialConvertDescriptor = credentialSecurityDLL.NewProc("ConvertStringSecurityDescriptorToSecurityDescriptorW")
var credentialGetDescriptorDACL = credentialSecurityDLL.NewProc("GetSecurityDescriptorDacl")
var credentialSetNamedSecurity = credentialSecurityDLL.NewProc("SetNamedSecurityInfoW")
var credentialGetNamedSecurity = credentialSecurityDLL.NewProc("GetNamedSecurityInfoW")
var credentialDescriptorToString = credentialSecurityDLL.NewProc("ConvertSecurityDescriptorToStringSecurityDescriptorW")

func credentialCurrentUserSID() (string,error) {
 token,err:=syscall.OpenCurrentProcessToken();if err!=nil{return "",err};defer token.Close()
 user,err:=token.GetTokenUser();if err!=nil{return "",err}
 return user.User.Sid.String()
}

// Restrict newly-created encrypted files before writing. This uses an actual
// protected Windows DACL; Go's Unix permission bits alone do not enforce it.
// Existing user files/directories are never rewritten merely by opening a vault.
func secureCredentialTempFile(file *os.File) error {
 before,err:=file.Stat();if err!=nil{return err}
 sid,err:=credentialCurrentUserSID();if err!=nil{return err}
 text,err:=syscall.UTF16PtrFromString("D:P(A;;FA;;;"+sid+")(A;;FA;;;SY)(A;;FA;;;BA)");if err!=nil{return err}
 var descriptor uintptr
 ok,_,callErr:=credentialConvertDescriptor.Call(uintptr(unsafe.Pointer(text)),1,uintptr(unsafe.Pointer(&descriptor)),0)
 runtime.KeepAlive(text)
 if ok==0{return callErr}
 defer syscall.LocalFree(syscall.Handle(descriptor))
 var present,defaulted int32
 var dacl uintptr
 ok,_,callErr=credentialGetDescriptorDACL.Call(descriptor,uintptr(unsafe.Pointer(&present)),uintptr(unsafe.Pointer(&dacl)),uintptr(unsafe.Pointer(&defaulted)))
 if ok==0{return callErr};if present==0||dacl==0{return errors.New("Private credential file permissions could not be prepared")}
 path,err:=syscall.UTF16PtrFromString(file.Name());if err!=nil{return err}
 // SE_FILE_OBJECT; DACL_SECURITY_INFORMATION | PROTECTED_DACL_SECURITY_INFORMATION.
 code,_,_:=credentialSetNamedSecurity.Call(uintptr(unsafe.Pointer(path)),1,0x80000004,0,0,dacl,0)
 runtime.KeepAlive(path)
 if code!=0{return syscall.Errno(code)}
 after,err:=os.Lstat(file.Name());if err!=nil{return err}
 if !after.Mode().IsRegular()||!os.SameFile(before,after){return errors.New("Credential temporary file changed while protecting permissions")}
 return nil
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
