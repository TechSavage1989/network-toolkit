package main

import "testing"

func TestPacketAdapterMatchingRequiresRealUniqueIdentity(t *testing.T) {
	adapters := []packetSystemAdapter{
		{ID: "{00000000-0000-0000-0000-000000000001}", Name: "Wi-Fi", Description: "MediaTek Wi-Fi 6E", MAC: "AA:BB:CC:DD:EE:01", Driver: "rt26cx21x64.sys"},
		{ID: "{00000000-0000-0000-0000-000000000002}", Name: "Ethernet", Description: "USB Ethernet", MAC: "AA:BB:CC:DD:EE:02", Driver: "usbnic.sys"},
		{ID: "{00000000-0000-0000-0000-000000000003}", Name: "VPN", Description: "Configurable VPN", MAC: "AA:BB:CC:DD:EE:03", Driver: "vpn.sys"},
	}
	for _, name := range []string{"Wi-Fi", "MEDIATEK WI-FI 6E", "rt26cx21x64.sys", "AA-BB-CC-DD-EE-01 MediaTek Wi-Fi 6E", "{00000000-0000-0000-0000-000000000001}"} {
		got, ok := packetMatchAdapter(name, adapters)
		if !ok || got.Name != "Wi-Fi" {
			t.Fatalf("%q matched %#v, %v", name, got, ok)
		}
	}
	if got, ok := packetMatchAdapter("AA-BB-CC-DD-EE-03 Configurable VPN", adapters); !ok || got.Name != "VPN" {
		t.Fatal(got, ok)
	}
	for _, name := range []string{"vmswitch.sys", "bthpan.sys", "unknown.sys", "AA-BB-CC-DD-EE-99 Unknown adapter", "Wi-Fi extra", ""} {
		if got, ok := packetMatchAdapter(name, adapters); ok {
			t.Fatalf("Non-interface %q matched %#v", name, got)
		}
	}
	adapters = append(adapters, packetSystemAdapter{Name: "Other Wi-Fi", Description: "Other Wi-Fi card", Driver: "rt26cx21x64.sys", MAC: "AA:BB:CC:DD:EE:01"})
	if _, ok := packetMatchAdapter("rt26cx21x64.sys", adapters); ok {
		t.Fatal("Shared driver selected an arbitrary interface")
	}
	if _, ok := packetMatchAdapter("AA-BB-CC-DD-EE-01", adapters); ok {
		t.Fatal("Shared MAC selected an arbitrary interface")
	}
	if got, ok := packetMatchAdapter("AA-BB-CC-DD-EE-01 Other Wi-Fi card", adapters); !ok || got.Name != "Other Wi-Fi" {
		t.Fatal(got, ok)
	}
}

func TestPacketAdapterInventoryIncludesDisconnectedAndVirtualConfigurableNICs(t *testing.T) {
	base := localIPAdapter{ID: "{00000000-0000-0000-0000-000000000001}", Index: 1, Name: "Ethernet", Editable: true, Type: "Ethernet", Status: "Down"}
	rows := []localIPAdapter{base}
	virtual := base
	virtual.ID = "{00000000-0000-0000-0000-000000000002}"
	virtual.Name = "vEthernet (Lab)"
	virtual.Index = 2
	rows = append(rows, virtual)
	for _, invalid := range []string{"uneditable", "missing", "loopback", "no-id", "no-index", "no-name"} {
		row := base
		switch invalid {
		case "uneditable":
			row.Editable = false
		case "missing":
			row.Status = "Not present"
		case "loopback":
			row.Type = "Loopback"
		case "no-id":
			row.ID = "driver.sys"
		case "no-index":
			row.Index = 0
		case "no-name":
			row.Name = ""
		}
		rows = append(rows, row)
	}
	got := packetConfigurableAdapters(localIPSnapshot{Adapters: rows})
	if len(got) != 2 || got[0].Name != "Ethernet" || got[1].Name != "vEthernet (Lab)" {
		t.Fatal(got)
	}
}
