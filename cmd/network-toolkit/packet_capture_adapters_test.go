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

func TestPacketOrdinaryInventoryUsesStableGUIDsWithoutClaimingEngineSupport(t *testing.T) {
	one := packetSystemAdapter{ID: "{A0000000-0000-0000-0000-000000000001}", Name: "Ethernet", Description: "Synthetic adapter", MAC: "AA:BB:CC:DD:EE:01"}
	two := one
	two.ID = "{00000000-0000-0000-0000-000000000002}"
	two.Name = "Wi-Fi"
	views := packetOrdinaryInterfaces([]packetSystemAdapter{one, two})
	if len(views) != 3 || views[0].ID != "nics" || views[1].ID != "adapter:{a0000000-0000-0000-0000-000000000001}" || views[1].Engine != "auto" {
		t.Fatal(views)
	}
	duplicate := one
	duplicate.ID = "{a0000000-0000-0000-0000-000000000001}"
	if got := packetOrdinaryInterfaces([]packetSystemAdapter{one, duplicate}); len(got) != 0 {
		t.Fatal("duplicate identities must not be offered", got)
	}
	if got := packetOrdinaryInterfaces([]packetSystemAdapter{{ID: "driver.sys", Name: "Not an adapter"}}); len(got) != 0 {
		t.Fatal(got)
	}
}

func TestPacketCaptureSelectionNeverFallsBackToAnotherAdapter(t *testing.T) {
	id := "{00000000-0000-0000-0000-000000000001}"
	other := "{00000000-0000-0000-0000-000000000002}"
	for _, legacy := range []string{"live:0123456789abcdef01234567", "file:23"} {
		if !packetCaptureSourceSelected(packetAdapterSelectionID(id), legacy, id) || packetCaptureSourceSelected(packetAdapterSelectionID(id), legacy, other) {
			t.Fatal("explicit GUID mapping changed destination")
		}
		if !packetCaptureSourceSelected(legacy, legacy, id) || !packetCaptureSourceSelected("nics", legacy, id) {
			t.Fatal("existing explicit source/all-adapter choices were lost")
		}
	}
	for _, valid := range []string{"nics", packetAdapterSelectionID(id), "live:0123456789abcdef01234567", "file:23"} {
		if !packetValidInterfaceSelection(valid) {
			t.Fatal(valid)
		}
	}
	for _, invalid := range []string{"", "all", "adapter:driver.sys", "adapter:" + id + "extra", "live:x", "file:0", "file:01", "file:-1", "file:4294967296", "file:1 --capture", "nics\n"} {
		if packetValidInterfaceSelection(invalid) {
			t.Fatal("invalid source accepted", invalid)
		}
	}
}
