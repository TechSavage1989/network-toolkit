//go:build windows

package desktop

import (
	"bytes"
	"context"
	"errors"
	"os"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"
	"unsafe"
)

func requireVaultDialogRuntime(t *testing.T) {
	t.Helper()
	if os.Getenv("NETWORK_TOOLKIT_NATIVE_SECURITY_TEST") != "1" {
		t.Skip("set NETWORK_TOOLKIT_NATIVE_SECURITY_TEST=1 for isolated synthetic Win32 vault-dialog acceptance")
	}
}

func assertVaultDialogRegistryEmpty(t *testing.T) {
	t.Helper()
	count := 0
	vaultDialogPending.Range(func(_, _ any) bool { count++; return true })
	vaultDialogWindows.Range(func(_, _ any) bool { count++; return true })
	if count != 0 {
		t.Fatal("native dialog roots outlived their native windows")
	}
}

func TestVaultDialogRuntimeSecretEntryModes(t *testing.T) {
	requireVaultDialogRuntime(t)
	for _, mode := range []VaultSecretMode{VaultSecretSetup, VaultSecretUnlock, VaultSecretRotate, VaultSecretRecover} {
		t.Run(string(mode), func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			const password = "  synthetic🔐master  "
			var issues []string
			var owner uintptr
			response, err := promptVaultSecret(ctx, VaultSecretRequest{Mode: mode}, func(ctx context.Context, spec vaultDialogSpec) (vaultDialogResult, error) {
				return showNativeVaultDialogObserved(ctx, spec, func(d *nativeVaultDialog) {
					owner, _, _ = wndUser32.NewProc("GetWindow").Call(d.hwnd, 4) // GW_OWNER
					if owner == 0 || owner == d.hwnd {
						issues = append(issues, "modal dialog had no distinct native owner")
					}
					for _, field := range spec.fields {
						style, _, _ := wndUser32.NewProc("GetWindowLongW").Call(d.item(field.id), ^uintptr(15)) // GWL_STYLE -16
						mask, _, _ := wndSendMessage.Call(d.item(field.id), 0x00d2, 0, 0)                       // EM_GETPASSWORDCHAR
						if ((style&0x20) != 0) != field.masked || (mask != 0) != field.masked {
							issues = append(issues, "actual native EDIT masking did not match the field")
						}
						limit, _, _ := wndSendMessage.Call(d.item(field.id), 0x00d5, 0, 0) // EM_GETLIMITTEXT
						if limit != vaultNativeEditLimit {
							issues = append(issues, "native edit input was not bounded")
						}
						value := password
						if field.id == vaultFieldRecovery {
							value = "SYNTHETIC-OFFLINE-KEY"
						}
						nativeVaultSetText(d.item(field.id), value)
					}
					if !d.checked(vaultControlRemember) || d.checked(vaultControlRecovery) {
						issues = append(issues, "remember/recovery defaults were not checked/unchecked respectively")
					}
					if mode == VaultSecretUnlock {
						wndSendMessage.Call(d.item(vaultControlRemember), vaultBMSetCheck, 0, 0)
					} else {
						wndSendMessage.Call(d.item(vaultControlRecovery), vaultBMSetCheck, 1, 0)
					}
					wndPostMessage.Call(d.hwnd, wmCommand, 1, 0)
				})
			})
			if len(issues) != 0 {
				t.Fatal(strings.Join(issues, "; "))
			}
			if err != nil || !bytes.Equal(response.Password, []byte(password)) || response.Remember != (mode != VaultSecretUnlock) || response.GenerateRecovery != (mode != VaultSecretUnlock) {
				t.Fatalf("real native secret entry failed: %v", err)
			}
			clearVaultSecretResponse(&response)
			if exists, _, _ := vaultIsWindow.Call(owner); exists != 0 {
				t.Fatal("hidden native owner survived dialog return")
			}
			assertVaultDialogRegistryEmpty(t)
		})
	}
}

func TestVaultDialogRuntimeOverlimitPasteCannotCreateTruncatedPassword(t *testing.T) {
	requireVaultDialogRuntime(t)
	for _, value := range []string{strings.Repeat("x", 1025), strings.Repeat("x", vaultNativeEditLimit+1), strings.Repeat("🔐", 257)} {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		var issue bool
		response, err := promptVaultSecret(ctx, VaultSecretRequest{Mode: VaultSecretSetup}, func(ctx context.Context, spec vaultDialogSpec) (vaultDialogResult, error) {
			return showNativeVaultDialogObserved(ctx, spec, func(d *nativeVaultDialog) {
				text, _ := syscall.UTF16FromString(value)
				for _, id := range []uint16{vaultFieldPassword, vaultFieldConfirm} {
					// EM_REPLACESEL follows the native user-entry text limit, unlike
					// WM_SETTEXT. No real or test clipboard contents are accessed.
					wndSendMessage.Call(d.item(id), 0x00c2, 1, uintptr(unsafe.Pointer(&text[0])))
				}
				clear(text)
				runtime.KeepAlive(text)
				d.accept()
				issue = d.done || d.result.accepted
				if !d.done {
					wndPostMessage.Call(d.hwnd, wmClose, 0, 0)
				}
			})
		})
		cancel()
		if issue || !errors.Is(err, ErrVaultDialogCancelled) || len(response.Password) != 0 {
			t.Fatalf("native overlimit paste was truncated into an accepted password: %v", err)
		}
		assertVaultDialogRegistryEmpty(t)
	}
}

func TestVaultDialogRuntimeDefaultNoAndExplicitApproval(t *testing.T) {
	requireVaultDialogRuntime(t)
	for _, action := range []VaultAction{VaultActionForget, VaultActionLock, VaultActionDelete, VaultActionLegacyCleanup} {
		for _, accept := range []bool{false, true} {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			var issue bool
			allowed, err := confirmVaultAction(ctx, VaultActionRequest{Action: action}, func(ctx context.Context, spec vaultDialogSpec) (vaultDialogResult, error) {
				return showNativeVaultDialogObserved(ctx, spec, func(d *nativeVaultDialog) {
					defaultID, _, _ := wndSendMessage.Call(d.hwnd, 0x0400, 0, 0) // DM_GETDEFID
					focus, _, _ := wndUser32.NewProc("GetFocus").Call()
					issue = uint16(defaultID) != 2 || uint16(defaultID>>16) != 0x534b || focus != d.item(2)
					command := uintptr(2)
					if accept {
						command = 1 // Explicit synthetic Yes, never the default.
					}
					wndPostMessage.Call(d.hwnd, wmCommand, command, 0)
				})
			})
			cancel()
			if err != nil || issue || allowed != accept {
				t.Fatalf("real native default-No/approval contract failed for %s: %v", action, err)
			}
			assertVaultDialogRegistryEmpty(t)
		}
	}
}

func TestVaultDialogRuntimeMismatchCanBeCorrectedWithoutDisclosure(t *testing.T) {
	requireVaultDialogRuntime(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var issue bool
	response, err := promptVaultSecret(ctx, VaultSecretRequest{Mode: VaultSecretSetup}, func(ctx context.Context, spec vaultDialogSpec) (vaultDialogResult, error) {
		return showNativeVaultDialogObserved(ctx, spec, func(d *nativeVaultDialog) {
			nativeVaultSetText(d.item(vaultFieldPassword), "synthetic master secret")
			nativeVaultSetText(d.item(vaultFieldConfirm), "different synthetic secret")
			d.accept()
			message, readErr := nativeVaultReadText(d.item(vaultControlError))
			issue = d.done || readErr != nil || len(message) == 0 || bytes.Contains(message, []byte("synthetic"))
			clear(message)
			nativeVaultSetText(d.item(vaultFieldConfirm), "synthetic master secret")
			wndPostMessage.Call(d.hwnd, wmCommand, 1, 0)
		})
	})
	defer clearVaultSecretResponse(&response)
	if err != nil || issue || len(response.Password) == 0 {
		t.Fatalf("native validation/correction flow failed: %v", err)
	}
	assertVaultDialogRegistryEmpty(t)
}

func TestVaultDialogRuntimeCloseAndContextCancellation(t *testing.T) {
	requireVaultDialogRuntime(t)
	for _, cancelContext := range []bool{false, true} {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		start := time.Now()
		response, err := promptVaultSecret(ctx, VaultSecretRequest{Mode: VaultSecretUnlock}, func(ctx context.Context, spec vaultDialogSpec) (vaultDialogResult, error) {
			return showNativeVaultDialogObserved(ctx, spec, func(d *nativeVaultDialog) {
				nativeVaultSetText(d.item(vaultFieldPassword), "synthetic cancelled secret")
				if cancelContext {
					cancel() // The real UI-thread timer must close the live dialog.
				} else {
					wndPostMessage.Call(d.hwnd, wmClose, 0, 0)
				}
			})
		})
		cancel()
		if err == nil || len(response.Password) != 0 || time.Since(start) > 2*time.Second || (cancelContext && !errors.Is(err, context.Canceled)) {
			t.Fatalf("native close/cancellation did not fail closed promptly: %v", err)
		}
		assertVaultDialogRegistryEmpty(t)
	}
}

func TestVaultDialogRuntimeOfflineRecoveryReentry(t *testing.T) {
	requireVaultDialogRuntime(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	const key = "SYNTHETIC-OFFLINE-RECOVERY-KEY"
	var issues []string
	sequence, _, _ := wndUser32.NewProc("GetClipboardSequenceNumber").Call() // No clipboard contents are read.
	allowed, err := confirmVaultRecovery(ctx, key, func(ctx context.Context, spec vaultDialogSpec) (vaultDialogResult, error) {
		return showNativeVaultDialogObserved(ctx, spec, func(d *nativeVaultDialog) {
			style, _, _ := wndUser32.NewProc("GetWindowLongW").Call(d.item(vaultFieldKeyView), ^uintptr(15))
			if style&0x0800 == 0 || style&0x0020 != 0 || d.checked(vaultControlAcknowledge) {
				issues = append(issues, "recovery display was not readonly/visible or acknowledgement was preselected")
			}
			nativeVaultSetText(d.item(vaultFieldRecovery), key)
			d.accept()
			if d.done {
				issues = append(issues, "key re-entry without acknowledgement was accepted")
				return
			}
			wndSendMessage.Call(d.item(vaultControlAcknowledge), vaultBMSetCheck, 1, 0)
			nativeVaultSetText(d.item(vaultFieldRecovery), key+" ")
			d.accept()
			if d.done {
				issues = append(issues, "inexact key re-entry was accepted")
				return
			}
			nativeVaultSetText(d.item(vaultFieldRecovery), key)
			wndPostMessage.Call(d.hwnd, wmCommand, 1, 0)
		})
	})
	after, _, _ := wndUser32.NewProc("GetClipboardSequenceNumber").Call()
	if !allowed || err != nil || len(issues) != 0 || sequence != after {
		t.Fatalf("native offline-key retention contract failed: %v; %s; clipboard unchanged=%v", err, strings.Join(issues, "; "), sequence == after)
	}
	assertVaultDialogRegistryEmpty(t)
}
