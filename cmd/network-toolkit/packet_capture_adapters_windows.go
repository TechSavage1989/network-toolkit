//go:build windows

package main

import (
	"context"
	"strings"
	"syscall"
)

func packetRegistryString(path, name string) string {
	p, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return ""
	}
	var key syscall.Handle
	if syscall.RegOpenKeyEx(syscall.HKEY_LOCAL_MACHINE, p, 0, syscall.KEY_READ, &key) != nil {
		return ""
	}
	defer syscall.RegCloseKey(key)
	values, err := localRegistryStrings(key, name)
	if err != nil || len(values) != 1 {
		return ""
	}
	return strings.TrimSpace(values[0])
}

// Resolve the installed interface's own PnP service. Driver names alone are
// never proof of a network adapter; they are used only as an exact, unique
// alias after Windows has supplied the interface GUID and editable settings.
func packetAdapterDriver(id string) string {
	if !validAdapterID(id) {
		return ""
	}
	pnp := packetRegistryString(`SYSTEM\CurrentControlSet\Control\Network\{4D36E972-E325-11CE-BFC1-08002BE10318}\`+id+`\Connection`, "PnpInstanceID")
	if pnp == "" {
		return ""
	}
	service := packetRegistryString(`SYSTEM\CurrentControlSet\Enum\`+pnp, "Service")
	if service == "" || strings.ContainsAny(service, `\/`+"\x00") {
		return ""
	}
	image := strings.Trim(packetRegistryString(`SYSTEM\CurrentControlSet\Services\`+service, "ImagePath"), `"`)
	image = strings.ReplaceAll(image, `\`, "/")
	if last := strings.LastIndexByte(image, '/'); last >= 0 {
		image = image[last+1:]
	}
	if !strings.HasSuffix(strings.ToLower(image), ".sys") || strings.ContainsAny(image, "\x00\r\n\" ") {
		return ""
	}
	return image
}

func packetReadSystemAdapters(ctx context.Context) ([]packetSystemAdapter, error) {
	snapshot, err := readLocalIP(ctx)
	if err != nil {
		return nil, err
	}
	adapters := packetConfigurableAdapters(snapshot)
	for i := range adapters {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		adapters[i].Driver = packetAdapterDriver(adapters[i].ID)
	}
	return adapters, nil
}
