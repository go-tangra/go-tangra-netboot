package netbootd

import (
	"encoding/json"
	"testing"
	"time"
)

// protojson renders 64-bit integers as JSON strings and 32-bit ones as
// numbers; the decoder must accept both without the caller knowing which.
func TestFlexInt64UnmarshalsBothForms(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		want    int64
		wantErr bool
	}{
		{"json number", `123`, 123, false},
		{"json string", `"123"`, 123, false},
		{"negative string", `"-9"`, -9, false},
		{"null", `null`, 0, false},
		{"empty string", `""`, 0, false},
		{"large value", `"9007199254740993"`, 9007199254740993, false},
		{"non-numeric string", `"twelve"`, 0, true},
		{"wrong type", `{}`, 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got flexInt64
			err := json.Unmarshal([]byte(tt.raw), &got)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("Unmarshal(%s) = nil error, want a failure", tt.raw)
				}
				return
			}
			if err != nil {
				t.Fatalf("Unmarshal(%s) error = %v", tt.raw, err)
			}
			if got.Int64() != tt.want {
				t.Errorf("Int64() = %d, want %d", got.Int64(), tt.want)
			}
		})
	}
}

func TestFlexInt64MarshalsAsANumber(t *testing.T) {
	encoded, err := json.Marshal(flexInt64(42))
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	if string(encoded) != "42" {
		t.Errorf("Marshal() = %s, want 42", encoded)
	}
}

func TestTimestampUnmarshal(t *testing.T) {
	tests := []struct {
		name     string
		raw      string
		wantZero bool
		wantErr  bool
	}{
		{"rfc3339", `"2026-01-02T03:04:05Z"`, false, false},
		{"rfc3339 with nanos", `"2026-01-02T03:04:05.123456789Z"`, false, false},
		{"offset", `"2026-01-02T03:04:05+02:00"`, false, false},
		// EmitUnpopulated renders an unset timestamp as null.
		{"null", `null`, true, false},
		{"empty string", `""`, true, false},
		{"garbage", `"yesterday"`, false, true},
		{"wrong type", `5`, false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got Timestamp
			err := json.Unmarshal([]byte(tt.raw), &got)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("Unmarshal(%s) = nil error, want a failure", tt.raw)
				}
				return
			}
			if err != nil {
				t.Fatalf("Unmarshal(%s) error = %v", tt.raw, err)
			}
			if got.IsZero() != tt.wantZero {
				t.Errorf("IsZero() = %v, want %v", got.IsZero(), tt.wantZero)
			}
		})
	}
}

func TestTimestampMarshal(t *testing.T) {
	encoded, err := json.Marshal(Timestamp{})
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	if string(encoded) != "null" {
		t.Errorf("Marshal(zero) = %s, want null", encoded)
	}

	moment := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	encoded, err = json.Marshal(Timestamp{Time: moment})
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	if string(encoded) != `"2026-01-02T03:04:05Z"` {
		t.Errorf("Marshal() = %s, want the RFC 3339 form", encoded)
	}
}

func TestTimestampRoundTrip(t *testing.T) {
	original := Timestamp{Time: time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)}
	encoded, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	var decoded Timestamp
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if !decoded.Time.Equal(original.Time) {
		t.Errorf("round trip = %v, want %v", decoded.Time, original.Time)
	}
}

// The whole point of ProfileBody.MarshalJSON is that the password reaches the
// wire while never existing as a plain field on the struct.
func TestProfileBodyMarshalsThePassword(t *testing.T) {
	body := ProfileBody{Name: "p", UbuntuRelease: "noble", Password: Secret("plaintext")}

	encoded, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if got := decoded["password"]; got != "plaintext" {
		t.Errorf("password = %v, want the plaintext on the wire", got)
	}
	if got := decoded["name"]; got != "p" {
		t.Errorf("name = %v, want p", got)
	}
}

func TestProfileBodyOmitsAnEmptyPassword(t *testing.T) {
	encoded, err := json.Marshal(ProfileBody{Name: "p", UbuntuRelease: "noble"})
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if _, present := decoded["password"]; present {
		t.Error("an unset password was serialised, which is ambiguous upstream")
	}
}

// Decoding a realistic upstream machine document exercises every custom
// decoder at once.
func TestMachineDecodesFromUpstreamJSON(t *testing.T) {
	const raw = `{
		"id": "m-1",
		"mac": "aa:bb:cc:dd:ee:ff",
		"name": "worker-01",
		"firmware": "uefi_x64",
		"profile_id": "p-1",
		"reservation_ip": "10.0.0.10",
		"provision_state": "installing",
		"notes": "",
		"created_at": "2026-01-02T03:04:05Z",
		"updated_at": null,
		"active_session_id": "s-1",
		"network_config": "",
		"install_network": {"address": "10.1.0.10/24", "gateway": "10.1.0.1", "dns": ["10.1.0.53"]}
	}`

	var m Machine
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if m.ID != "m-1" || m.Name != "worker-01" {
		t.Errorf("decoded machine = %+v, want the fixture values", m)
	}
	if m.CreatedAt.IsZero() {
		t.Error("createdAt is zero, want the parsed timestamp")
	}
	if !m.UpdatedAt.IsZero() {
		t.Error("updatedAt is set, want the null to decode as zero")
	}
	if m.InstallNetwork == nil || m.InstallNetwork.Gateway != "10.1.0.1" {
		t.Errorf("installNetwork = %+v, want the nested object", m.InstallNetwork)
	}
}

func TestPageMetaDecodesStringTotals(t *testing.T) {
	var meta PageMeta
	if err := json.Unmarshal([]byte(`{"total":"1234","page":2,"page_size":50}`), &meta); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if meta.Total.Int64() != 1234 {
		t.Errorf("total = %d, want 1234", meta.Total.Int64())
	}
	if meta.Page != 2 || meta.PageSize != 50 {
		t.Errorf("page/pageSize = %d/%d, want 2/50", meta.Page, meta.PageSize)
	}
}
