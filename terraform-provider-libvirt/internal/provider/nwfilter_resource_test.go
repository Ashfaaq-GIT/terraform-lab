package provider

import (
	"testing"

	"libvirt.org/go/libvirtxml"
)

func TestBuildICMPNWFilterEntry(t *testing.T) {
	priority := int64(100)

	entry, err := buildICMPNWFilterEntry(
		"accept",
		"in",
		&priority,
		8,
		0,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if entry.Rule == nil {
		t.Fatal("expected rule to be created")
	}

	if entry.Rule.Action != "accept" {
		t.Fatalf("expected action accept, got %q", entry.Rule.Action)
	}

	if entry.Rule.Direction != "in" {
		t.Fatalf("expected direction in, got %q", entry.Rule.Direction)
	}

	if entry.Rule.Priority != 100 {
		t.Fatalf("expected priority 100, got %d", entry.Rule.Priority)
	}

	if entry.Rule.ICMP == nil {
		t.Fatal("expected ICMP rule")
	}

	if entry.Rule.ICMP.Type.Uint == nil || *entry.Rule.ICMP.Type.Uint != 8 {
		t.Fatal("expected ICMP type 8")
	}

	if entry.Rule.ICMP.Code.Uint == nil || *entry.Rule.ICMP.Code.Uint != 0 {
		t.Fatal("expected ICMP code 0")
	}

	filter := libvirtxml.NWFilter{
		Name:    "test-icmp-filter",
		Entries: []libvirtxml.NWFilterEntry{entry},
	}

	xmlDoc, err := filter.Marshal()
	if err != nil {
		t.Fatalf("failed to marshal filter XML: %v", err)
	}

	var parsed libvirtxml.NWFilter

	if err := parsed.Unmarshal(xmlDoc); err != nil {
		t.Fatalf("failed to parse generated XML: %v", err)
	}

	if len(parsed.Entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(parsed.Entries))
	}

	if parsed.Entries[0].Rule == nil || parsed.Entries[0].Rule.ICMP == nil {
		t.Fatal("expected ICMP rule after XML round trip")
	}

	if parsed.Entries[0].Rule.ICMP.Type.Uint == nil ||
		*parsed.Entries[0].Rule.ICMP.Type.Uint != 8 {
		t.Fatal("expected round-trip ICMP type 8")
	}

	if parsed.Entries[0].Rule.ICMP.Code.Uint == nil ||
		*parsed.Entries[0].Rule.ICMP.Code.Uint != 0 {
		t.Fatal("expected round-trip ICMP code 0")
	}
}

func TestBuildICMPNWFilterEntryRejectsInvalidValues(t *testing.T) {
	tests := []struct {
		name     string
		icmpType int64
		icmpCode int64
	}{
		{
			name:     "negative type",
			icmpType: -1,
			icmpCode: 0,
		},
		{
			name:     "type above 255",
			icmpType: 256,
			icmpCode: 0,
		},
		{
			name:     "negative code",
			icmpType: 8,
			icmpCode: -1,
		},
		{
			name:     "code above 255",
			icmpType: 8,
			icmpCode: 256,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := buildICMPNWFilterEntry(
				"accept",
				"in",
				nil,
				tt.icmpType,
				tt.icmpCode,
			)

			if err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}
