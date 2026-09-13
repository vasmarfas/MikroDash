package collect

import (
	"testing"

	"mikrodash/internal/routeros"
)

// THE ACCESS POINT A NETWORK IS BROADCAST BY, on both CAPsMAN trees.
//
// ── WHY THIS TEST CARRIES A LEDGER ENTRY ────────────────────────────────────
//
// `addedSinceNode` in fixture_test.go names this test, because the golden corpus
// cannot reach the field: no capture in this repository has a CAP-provisioned
// `/interface/wifi` row, and on the AX3 that was captured the answer is
// correctly "" on every interface. So the golden proves the field exists and
// this proves it is filled. Deleting or renaming this fails that ledger.

func TestBuildWifiViewReadsTheAccessPointFromCap(t *testing.T) {
	// `cap` reads `identity@base-mac%id` and is present only on the MASTER
	// interface — which is the whole reason the chase exists.
	nets, radios := BuildWifiView(WifiViewInput{
		Ifaces: []routeros.Reply{
			{".id": "*1", "name": "cap_ax24-service", "radio-mac": "AA:BB:CC:DD:EE:01",
				"cap":                "cap_ax_voleyball@AA:BB:CC:DD:EE:00%1",
				"configuration.ssid": "service", "channel.band": "2ghz-ax", "running": "true"},
			{".id": "*2", "name": "cap_ax24-LAGUNA", "master-interface": "cap_ax24-service",
				"configuration.ssid": "LAGUNA", "running": "true"},
			{".id": "*3", "name": "cap_ax5-service", "radio-mac": "AA:BB:CC:DD:EE:02",
				"cap":                "cap_ax_voleyball@AA:BB:CC:DD:EE:00%2",
				"configuration.ssid": "service", "channel.band": "5ghz-ax", "running": "true"},
			// A LOCAL radio: no `cap` at all, which must read as "this router"
			// rather than as an access point called "".
			{".id": "*4", "name": "wifi1", "radio-mac": "AA:BB:CC:DD:EE:10",
				"configuration.ssid": "home", "channel.band": "5ghz-ax", "running": "true"},
		},
	})

	byName := map[string]WifiNetwork{}
	for _, n := range nets {
		byName[n.Name] = n
	}
	if got := byName["cap_ax24-service"].AP; got != "cap_ax_voleyball" {
		t.Errorf("master AP = %q, want cap_ax_voleyball", got)
	}
	if got := byName["cap_ax5-service"].AP; got != "cap_ax_voleyball" {
		t.Errorf("the other band's AP = %q, want cap_ax_voleyball — one box, one "+
			"pin on the map and one group on the page", got)
	}
	if got := byName["cap_ax24-LAGUNA"].AP; got != "cap_ax_voleyball" {
		t.Errorf("the virtual AP's AP = %q, want cap_ax_voleyball — only the master "+
			"carries `cap`, so the chase did not run", got)
	}
	if got := byName["wifi1"].AP; got != "" {
		t.Errorf("a local radio reported AP %q, want empty", got)
	}

	// The RADIO list carries it too, because the map pins radios and the page's
	// AP view groups by it.
	found := 0
	for _, r := range radios {
		if r.Name == "wifi1" && r.AP != "" {
			t.Errorf("local radio %q reported AP %q", r.Name, r.AP)
		}
		if r.AP == "cap_ax_voleyball" {
			found++
		}
	}
	if found != 2 {
		t.Errorf("%d radios attributed to the CAP, want 2", found)
	}
	// A virtual AP is not a radio.
	if len(radios) != 3 {
		t.Errorf("%d radios, want 3", len(radios))
	}
}

// The legacy tree answers the same question through `/caps-man/radio`, because a
// `/caps-man/interface` row carries no `cap` field at all.
func TestCapsLegacyAPsJoinsThroughTheRadio(t *testing.T) {
	// ── THE ROWS ARE SHAPED LIKE THE ROUTER'S, AND THAT IS THE POINT ────────
	//
	// A master answers `master-interface=none`, not an empty string, and a SLAVE
	// carries a `radio-mac` of its own — the master's with the
	// locally-administered bit set (`6E:` against `6C:`), which belongs to no
	// radio. Both were got wrong on the first pass, and between them they made
	// every legacy interface read as a virtual AP: no radios in the payload, an
	// empty access-point tray on the Wi-Fi map, and every slave grouped under
	// "This router". Written as the router writes them so that cannot recur.
	ifaces := []routeros.Reply{
		{"name": "RB951-Vagon-1", "master-interface": "none", "radio-mac": "6C:3B:6B:58:6F:6A"},
		{"name": "RB951-Vagon-1-1", "master-interface": "RB951-Vagon-1",
			"radio-mac": "6E:3B:6B:58:6F:6A"},
		{"name": "orphan-1", "master-interface": "none", "radio-mac": "00:00:00:00:00:99"},
	}
	radios := []routeros.Reply{
		{"radio-mac": "6C:3B:6B:58:6F:6A", "remote-cap-identity": "RB951-Vagon"},
	}
	got := CapsLegacyAPs(ifaces, radios)

	if got["RB951-Vagon-1"] != "RB951-Vagon" {
		t.Errorf("master = %q, want RB951-Vagon", got["RB951-Vagon-1"])
	}
	if got["RB951-Vagon-1-1"] != "RB951-Vagon" {
		t.Errorf("virtual AP = %q, want RB951-Vagon — the chase must follow "+
			"master-interface to the ROOT rather than stopping at the slave's own "+
			"radio-mac, which matches no radio", got["RB951-Vagon-1-1"])
	}

	// AND "none" IS NOT A MASTER. Reading it as an interface name would send the
	// chase looking for a device called "none" and find nothing.
	if capsV1Master(ifaces[0]) != "" {
		t.Error(`master-interface="none" read as a real master`)
	}
	if capsV1Master(ifaces[1]) != "RB951-Vagon-1" {
		t.Error("a real master-interface was dropped")
	}
	if _, ok := got["orphan-1"]; ok {
		t.Error("an interface whose radio no CAP reported was attributed anyway")
	}

	// AND THE OTHER SPELLING. MikroTik's own page prints the column as
	// REMOTE-AP-IDENT; the detail view says `remote-cap-identity`. Both are read,
	// because a wrong guess detaches every radio from its CAP rather than
	// costing a column.
	alt := CapsLegacyAPs(ifaces, []routeros.Reply{
		{"radio-mac": "6C:3B:6B:58:6F:6A", "remote-ap-ident": "RB951-Vagon"},
	})
	if alt["RB951-Vagon-1"] != "RB951-Vagon" {
		t.Errorf("the REMOTE-AP-IDENT spelling did not resolve: %q", alt["RB951-Vagon-1"])
	}
}
