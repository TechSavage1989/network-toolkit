//go:build windows

package desktop

import (
	"context"
	"encoding/binary"
	"errors"
	"runtime"
	"sync"
	"syscall"
	"unicode/utf16"
	"unicode/utf8"
	"unsafe"
)

var (
	vaultDialogBoxIndirect = wndUser32.NewProc("DialogBoxIndirectParamW")
	vaultEndDialog         = wndUser32.NewProc("EndDialog")
	vaultGetDialogItem     = wndUser32.NewProc("GetDlgItem")
	vaultSetFocus          = wndUser32.NewProc("SetFocus")
	vaultIsWindow          = wndUser32.NewProc("IsWindow")
	// One process-lifetime callback, not one uncollectable callback per prompt.
	// Native memory contains scalar registry tokens, never a pointer into Go.
	vaultDialogProcedurePointer = syscall.NewCallback(vaultDialogProcedure)
	vaultDialogPending          sync.Map
	vaultDialogWindows          sync.Map
)

const (
	vaultWMInitDialog       = 0x0110
	vaultWMGetText          = 0x000d
	vaultWMGetTextLength    = 0x000e
	vaultWMSetText          = 0x000c
	vaultEMSetLimitText     = 0x00c5
	vaultEMEmptyUndoBuffer  = 0x00cd
	vaultBMGetCheck         = 0x00f0
	vaultBMSetCheck         = 0x00f1
	vaultDMSetDefaultID     = 0x0401
	vaultNativeTimer        = 17004
	vaultControlRemember    = 110
	vaultControlRecovery    = 111
	vaultControlAcknowledge = 112
	vaultControlError       = 113
	// Above every valid UTF-8 password's possible UTF-16 length. Otherwise a
	// long paste could truncate both password fields identically and silently
	// establish a different password. Values above the actual 1024-byte policy
	// are rejected, not truncated into acceptance.
	vaultNativeEditLimit = 8192
)

func PromptVaultSecret(ctx context.Context, request VaultSecretRequest) (VaultSecretResponse, error) {
	return promptVaultSecret(ctx, request, showNativeVaultDialog)
}

func ConfirmVaultAction(ctx context.Context, request VaultActionRequest) (bool, error) {
	return confirmVaultAction(ctx, request, showNativeVaultDialog)
}

func ConfirmVaultRecovery(ctx context.Context, key string) (bool, error) {
	return confirmVaultRecovery(ctx, key, showNativeVaultDialog)
}

// Standard DLGTEMPLATE/DLGITEMTEMPLATE data, aligned on DWORD boundaries.
// DialogBox owns keyboard navigation, modal owner disabling and its message loop.
// Reference: learn.microsoft.com/windows/win32/dlgbox/using-dialog-boxes.
type vaultTemplateBuilder struct{ data []byte }

func (b *vaultTemplateBuilder) word(value uint16) {
	b.data = binary.LittleEndian.AppendUint16(b.data, value)
}
func (b *vaultTemplateBuilder) dword(value uint32) {
	b.data = binary.LittleEndian.AppendUint32(b.data, value)
}
func (b *vaultTemplateBuilder) text(value string) {
	for _, ch := range value {
		if ch > 0xffff {
			first, second := utf16.EncodeRune(ch)
			b.word(uint16(first))
			b.word(uint16(second))
		} else {
			b.word(uint16(ch))
		}
	}
	b.word(0)
}
func (b *vaultTemplateBuilder) align() {
	for len(b.data)%4 != 0 {
		b.data = append(b.data, 0)
	}
}
func (b *vaultTemplateBuilder) control(class, id uint16, text string, style uint32, x, y, width, height int) {
	b.align()
	b.dword(0x50000000 | style) // WS_CHILD | WS_VISIBLE
	b.dword(0)
	for _, value := range []int{x, y, width, height} {
		b.word(uint16(value))
	}
	b.word(id)
	b.word(0xffff)
	b.word(class)
	b.text(text)
	b.word(0) // No creation data or native-held Go pointer.
}

func nativeVaultTemplate(spec vaultDialogSpec) []uint32 {
	const width = 390
	b := vaultTemplateBuilder{}
	b.dword(0x80c80000 | 0x0080 | 0x0040 | 0x0800) // POPUP | CAPTION | SYSMENU | DS_MODALFRAME | DS_SETFONT | DS_CENTER
	b.dword(0x00000001)                            // WS_EX_DLGMODALFRAME
	b.word(0)                                      // Control count filled below.
	b.word(0)
	b.word(0)
	b.word(width)
	b.word(0) // Height filled below.
	b.word(0) // No menu.
	b.word(0) // System dialog class.
	b.text(spec.title)
	b.word(9)
	b.text("Segoe UI")
	count := 0
	control := func(class, id uint16, text string, style uint32, x, y, w, h int) {
		b.control(class, id, text, style, x, y, w, h)
		count++
	}
	bodyHeight := 62
	if len(spec.fields) == 0 {
		bodyHeight = 150 // Includes worst-case ASCII-quoted metadata expansion.
	}
	control(0x82, 200, spec.body, 0x0080, 10, 10, width-20, bodyHeight) // SS_NOPREFIX
	y := 10 + bodyHeight + 8
	for _, field := range spec.fields {
		control(0x82, 201+field.id, field.label, 0x0080, 10, y, width-20, 10)
		style := uint32(0x00800000 | 0x00010000 | 0x0080) // BORDER | TABSTOP | ES_AUTOHSCROLL
		if field.masked {
			style |= 0x0020 // ES_PASSWORD; not an HTML field.
		}
		if field.readonly {
			style |= 0x0800 // ES_READONLY; manual copy is possible, never automatic.
		}
		control(0x81, field.id, field.value, style, 10, y+11, width-20, 14)
		y += 31
	}
	if spec.recovery {
		control(0x80, vaultControlRecovery, "Create a new optional offline recovery key", 0x00010003, 10, y, width-20, 14)
		y += 18
	}
	if spec.remember {
		control(0x80, vaultControlRemember, "Remember unlock across restarts and reboots on this Windows account", 0x00010003, 10, y, width-20, 14)
		y += 18
		control(0x82, 202, vaultRememberDisclosure, 0x0080, 10, y, width-20, 34)
		y += 38
	}
	if spec.acknowledge {
		control(0x80, vaultControlAcknowledge, "I retained this key offline. I understand losing both password and key is unrecoverable.", 0x00012003, 10, y, width-20, 24)
		y += 28
		control(0x82, 203, "Re-entry acknowledges retention; Network Toolkit cannot verify that your backup is safe or recoverable.", 0x0080, 10, y, width-20, 24)
		y += 28
	}
	control(0x82, vaultControlError, "", 0x0080, 10, y, width-20, 26)
	y += 30
	acceptStyle, cancelStyle := uint32(0x00010001), uint32(0x00010000)
	cancelLabel := "Cancel"
	if spec.defaultNo {
		acceptStyle, cancelStyle = cancelStyle, acceptStyle
		if len(spec.fields) == 0 {
			cancelLabel = "No"
		}
	}
	control(0x80, 1, spec.acceptLabel, acceptStyle, width-158, y, 70, 16)
	control(0x80, 2, cancelLabel, cancelStyle, width-80, y, 70, 16)
	binary.LittleEndian.PutUint16(b.data[8:10], uint16(count))
	binary.LittleEndian.PutUint16(b.data[16:18], uint16(y+26))
	b.align()
	words := make([]uint32, len(b.data)/4)
	for i := range words {
		words[i] = binary.LittleEndian.Uint32(b.data[i*4:])
	}
	clear(b.data) // May contain a recovery key; best effort only.
	return words
}

type nativeVaultDialog struct {
	ctx    context.Context
	spec   vaultDialogSpec
	hwnd   uintptr
	done   bool
	result vaultDialogResult
	err    error
	// Test-local observer, nil in production. No runtime bypass or env hook.
	ready func(*nativeVaultDialog)
}

func showNativeVaultDialog(ctx context.Context, spec vaultDialogSpec) (vaultDialogResult, error) {
	return showNativeVaultDialogObserved(ctx, spec, nil)
}

func showNativeVaultDialogObserved(ctx context.Context, spec vaultDialogSpec, ready func(*nativeVaultDialog)) (vaultDialogResult, error) {
	if err := validateCredentialConfirmationContext(ctx); err != nil {
		return vaultDialogResult{}, err
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	instance, _, callErr := wndGetModuleHandle.Call(0)
	if instance == 0 {
		return vaultDialogResult{}, nativeError("get the native vault dialog module", callErr)
	}
	owner := spec.owner
	if owner != 0 {
		if exists, _, _ := vaultIsWindow.Call(owner); exists == 0 {
			return vaultDialogResult{}, errors.New("The native vault dialog owner is no longer available")
		}
	} else {
		// The HTTP backend has no HWND. An invisible native window on this UI
		// thread owns the modal dialog and is destroyed after the dialog exits.
		class, title := utf16String("STATIC"), utf16String("Network Toolkit vault owner")
		owner, _, callErr = wndCreateWindow.Call(0x00000080, uintptr(unsafe.Pointer(class)), uintptr(unsafe.Pointer(title)), 0x80000000,
			0, 0, 0, 0, 0, 0, instance, 0)
		runtime.KeepAlive(class)
		runtime.KeepAlive(title)
		if owner == 0 {
			return vaultDialogResult{}, nativeError("create the native vault dialog owner", callErr)
		}
		defer wndDestroyWindow.Call(owner)
	}
	dialog := &nativeVaultDialog{ctx: ctx, spec: spec, ready: ready}
	token := uintptr(wndNextToken.Add(1))
	vaultDialogPending.Store(token, dialog)
	defer vaultDialogPending.Delete(token)
	template := nativeVaultTemplate(spec)
	defer clear(template)
	result, _, callErr := vaultDialogBoxIndirect.Call(instance, uintptr(unsafe.Pointer(&template[0])), owner, vaultDialogProcedurePointer, token)
	runtime.KeepAlive(template)
	runtime.KeepAlive(dialog)
	// Normally WM_NCDESTROY removes this root. Also clean up if creation failed.
	if dialog.hwnd != 0 {
		vaultDialogWindows.Delete(dialog.hwnd)
	}
	if cancelled := ctx.Err(); cancelled != nil {
		dialog.result.clear()
		return vaultDialogResult{}, cancelled
	}
	if result == ^uintptr(0) || result == 0 || !dialog.done {
		dialog.result.clear()
		return vaultDialogResult{}, nativeError("display the native vault dialog", callErr)
	}
	return dialog.result, dialog.err
}

func (d *nativeVaultDialog) item(id uint16) uintptr {
	hwnd, _, _ := vaultGetDialogItem.Call(d.hwnd, uintptr(id))
	return hwnd
}

func (d *nativeVaultDialog) checked(id uint16) bool {
	checked, _, _ := wndSendMessage.Call(d.item(id), vaultBMGetCheck, 0, 0)
	return checked == 1
}

func nativeVaultSetText(hwnd uintptr, value string) {
	text, err := syscall.UTF16FromString(value)
	if err != nil {
		return
	}
	wndSendMessage.Call(hwnd, vaultWMSetText, 0, uintptr(unsafe.Pointer(&text[0])))
	clear(text)
	runtime.KeepAlive(text)
}

func nativeVaultReadText(hwnd uintptr) ([]byte, error) {
	length, _, _ := wndSendMessage.Call(hwnd, vaultWMGetTextLength, 0, 0)
	if length > 1024 {
		return nil, errors.New("The entered secret exceeds the native input limit")
	}
	buffer := make([]uint16, int(length)+1)
	defer clear(buffer)
	copied, _, _ := wndSendMessage.Call(hwnd, vaultWMGetText, uintptr(len(buffer)), uintptr(unsafe.Pointer(&buffer[0])))
	runtime.KeepAlive(buffer)
	if copied != length {
		return nil, errors.New("The native secret entry could not be read completely")
	}
	value := make([]byte, 0, int(length))
	for i := 0; i < int(copied); i++ {
		ch := rune(buffer[i])
		if utf16.IsSurrogate(ch) {
			if ch < 0xd800 || ch > 0xdbff || i+1 >= int(copied) || buffer[i+1] < 0xdc00 || buffer[i+1] > 0xdfff {
				clear(value)
				return nil, errors.New("The entered secret contains invalid Unicode")
			}
			ch = utf16.DecodeRune(ch, rune(buffer[i+1]))
			i++
		}
		value = utf8.AppendRune(value, ch)
	}
	return value, nil
}

func (d *nativeVaultDialog) clearControls() {
	for _, field := range d.spec.fields {
		hwnd := d.item(field.id)
		if hwnd == 0 {
			continue
		}
		length, _, _ := wndSendMessage.Call(hwnd, vaultWMGetTextLength, 0, 0)
		if length > vaultNativeEditLimit {
			length = vaultNativeEditLimit
		}
		// Overwrite then empty the live edit/undo buffer before destruction.
		// Windows can retain other copies; this is not a secure-erasure claim.
		overwrite := make([]uint16, int(length)+1)
		for i := 0; i < int(length); i++ {
			overwrite[i] = '*'
		}
		wndSendMessage.Call(hwnd, vaultWMSetText, 0, uintptr(unsafe.Pointer(&overwrite[0])))
		clear(overwrite)
		wndSendMessage.Call(hwnd, vaultWMSetText, 0, uintptr(unsafe.Pointer(&overwrite[0])))
		wndSendMessage.Call(hwnd, vaultEMEmptyUndoBuffer, 0, 0)
		runtime.KeepAlive(overwrite)
	}
}

func (d *nativeVaultDialog) finish(accepted bool, err error) {
	if d.done {
		return
	}
	d.done = true
	d.err = err
	d.result.accepted = accepted && err == nil && d.ctx.Err() == nil
	if !d.result.accepted {
		d.result.clear()
	}
	wndKillTimer.Call(d.hwnd, vaultNativeTimer)
	d.clearControls()
	result := uintptr(2)
	if d.result.accepted {
		result = 1
	}
	vaultEndDialog.Call(d.hwnd, result)
}

func (d *nativeVaultDialog) accept() {
	if d.done {
		return
	}
	if err := d.ctx.Err(); err != nil {
		d.finish(false, err)
		return
	}
	result := vaultDialogResult{accepted: true, values: make(map[uint16][]byte), remember: d.spec.remember && d.checked(vaultControlRemember), generateRecovery: d.spec.recovery && d.checked(vaultControlRecovery), acknowledged: d.spec.acknowledge && d.checked(vaultControlAcknowledge)}
	var err error
	for _, field := range d.spec.fields {
		if field.readonly {
			continue // Never return the displayed key; only the re-entry response.
		}
		result.values[field.id], err = nativeVaultReadText(d.item(field.id))
		if err != nil {
			break
		}
	}
	if err == nil && d.spec.validate != nil {
		err = d.spec.validate(result)
	}
	if err != nil {
		result.clear()
		nativeVaultSetText(d.item(vaultControlError), err.Error()) // Static validation errors only, never an input value.
		return
	}
	d.result = result
	d.finish(true, nil)
}

func vaultDialogProcedure(hwnd uintptr, message uint32, wparam, lparam uintptr) uintptr {
	if message == vaultWMInitDialog {
		value, ok := vaultDialogPending.Load(lparam)
		if !ok {
			vaultEndDialog.Call(hwnd, 2)
			return 1
		}
		d := value.(*nativeVaultDialog)
		d.hwnd = hwnd
		vaultDialogWindows.Store(hwnd, d)
		for _, field := range d.spec.fields {
			control := d.item(field.id)
			if control == 0 {
				d.finish(false, errors.New("A native vault input control could not be created"))
				return 1
			}
			wndSendMessage.Call(control, vaultEMSetLimitText, vaultNativeEditLimit, 0)
		}
		if d.spec.remember {
			wndSendMessage.Call(d.item(vaultControlRemember), vaultBMSetCheck, 1, 0)
		}
		if timer, _, _ := wndSetTimer.Call(hwnd, vaultNativeTimer, 50, 0); timer == 0 {
			d.finish(false, errors.New("Native vault cancellation monitoring could not start"))
			return 1
		}
		if err := d.ctx.Err(); err != nil {
			d.finish(false, err)
			return 1
		}
		focus := d.item(2)
		defaultID := uintptr(2)
		if !d.spec.defaultNo {
			defaultID = 1
			for _, field := range d.spec.fields {
				if !field.readonly {
					focus = d.item(field.id)
					break
				}
			}
		}
		wndSendMessage.Call(hwnd, vaultDMSetDefaultID, defaultID, 0)
		vaultSetFocus.Call(focus)
		wndSetForeground.Call(hwnd)
		if d.ready != nil {
			d.ready(d)
		}
		return 0 // Focus was explicitly assigned.
	}
	value, ok := vaultDialogWindows.Load(hwnd)
	if !ok {
		return 0
	}
	d := value.(*nativeVaultDialog)
	switch message {
	case wmCommand:
		if uint16(wparam>>16) != 0 { // Only BN_CLICKED / keyboard dialog commands.
			return 0
		}
		switch uint16(wparam) {
		case 1:
			d.accept()
			return 1
		case 2:
			d.finish(false, ErrVaultDialogCancelled)
			return 1
		}
	case wmClose:
		d.finish(false, ErrVaultDialogCancelled)
		return 1
	case wmTimer:
		if wparam == vaultNativeTimer {
			if err := d.ctx.Err(); err != nil {
				d.finish(false, err)
			}
			return 1
		}
	case wmDestroy:
		if !d.done {
			d.clearControls()
			d.result.clear()
			d.err = ErrVaultDialogCancelled
		}
	case wmNCDestroy:
		vaultDialogWindows.Delete(hwnd)
	}
	return 0 // Let the system dialog manager handle all other messages.
}
