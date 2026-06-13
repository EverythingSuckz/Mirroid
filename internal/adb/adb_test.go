package adb

import "testing"

func TestParseDeviceStates(t *testing.T) {
	out := "* daemon not running; starting now at tcp:5037\r\n" +
		"* daemon started successfully\r\n" +
		"List of devices attached\r\n" +
		"192.168.2.48:39067\toffline\r\n" +
		"192.168.2.11:42405\tdevice\r\n" +
		"adb-RZ8M81AB-aBcDeF._adb-tls-connect._tcp\tdevice\r\n" +
		"ABCD1234\tunauthorized\r\n" +
		"EFGH5678\tno permissions (verify udev rules)\r\n" +
		"\r\n"

	states := parseDeviceStates(out)

	want := map[string]string{
		"192.168.2.48:39067": "offline",
		"192.168.2.11:42405": "device",
		"adb-RZ8M81AB-aBcDeF._adb-tls-connect._tcp": "device",
		"ABCD1234": "unauthorized",
		// multi word statuses truncate to the first word; callers only
		// ever match "device"/"offline" so this must just not be either
		"EFGH5678": "no",
	}
	if len(states) != len(want) {
		t.Fatalf("got %d entries, want %d: %v", len(states), len(want), states)
	}
	for serial, state := range want {
		if states[serial] != state {
			t.Errorf("state[%q] = %q, want %q", serial, states[serial], state)
		}
	}
}

func TestParseMdnsConnectAddr(t *testing.T) {
	out := "List of discovered mdns services\r\n" +
		"adb-e377a5d7-JClw3Z\t_adb-tls-pairing._tcp\t192.168.2.11:37113\r\n" +
		"adb-e377a5d7-JClw3Z\t_adb-tls-connect._tcp\t192.168.2.11:43439\r\n" +
		"adb-OTHER-zZ\t_adb-tls-connect._tcp\t192.168.2.50:40001\r\n"

	if got := parseMdnsConnectAddr(out, "adb-e377a5d7-JClw3Z"); got != "192.168.2.11:43439" {
		t.Errorf("matched guid: got %q, want 192.168.2.11:43439", got)
	}
	// must pick the connect service, not the pairing service of the same guid
	if got := parseMdnsConnectAddr(out, "adb-OTHER-zZ"); got != "192.168.2.50:40001" {
		t.Errorf("other guid: got %q, want 192.168.2.50:40001", got)
	}
	if got := parseMdnsConnectAddr(out, "adb-NOPE"); got != "" {
		t.Errorf("absent guid: got %q, want empty", got)
	}
	if got := parseMdnsConnectAddr(out, ""); got != "" {
		t.Errorf("empty guid should not match: got %q", got)
	}
}

func TestParsePairGuid(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"Successfully paired to 192.168.2.48:37997 [guid=adb-RZ8M81AB-aBcDeF]", "adb-RZ8M81AB-aBcDeF"},
		{"Successfully paired to 192.168.2.48:37997 [guid=adb-RZ8M81AB-aBcDeF]\n", "adb-RZ8M81AB-aBcDeF"},
		{"Successfully paired to 192.168.2.48:37997", ""},
		{"Successfully paired to 192.168.2.48:37997 [guid=adb-broken", ""},
		{"", ""},
	}
	for _, tt := range tests {
		if got := parsePairGuid(tt.input); got != tt.want {
			t.Errorf("parsePairGuid(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
