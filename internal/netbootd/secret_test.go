package netbootd

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

// A Secret must not leak through any of the rendering paths a credential
// realistically reaches: fmt verbs, struct printing, and JSON encoding.
func TestSecretNeverRendersPlaintext(t *testing.T) {
	const plaintext = "sup3r-s3cret-pa55w0rd"
	s := Secret(plaintext)

	renderings := map[string]string{
		"%v":     fmt.Sprintf("%v", s),
		"%s":     fmt.Sprintf("%s", s),
		"%q":     fmt.Sprintf("%q", s),
		"%#v":    fmt.Sprintf("%#v", s),
		"%+v":    fmt.Sprintf("%+v", s),
		"nested": fmt.Sprintf("%+v", struct{ Password Secret }{s}),
	}

	for verb, got := range renderings {
		if strings.Contains(got, plaintext) {
			t.Errorf("rendering with %s leaked the secret: %s", verb, got)
		}
		if !strings.Contains(got, redactedPlaceholder) {
			t.Errorf("rendering with %s = %q, want it to contain %q", verb, got, redactedPlaceholder)
		}
	}
}

func TestSecretMarshalJSONRedacts(t *testing.T) {
	payload := struct {
		Password Secret `json:"password"`
	}{Secret("hunter2")}

	encoded, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	if strings.Contains(string(encoded), "hunter2") {
		t.Fatalf("Marshal() leaked the secret: %s", encoded)
	}
	if want := `{"password":"[REDACTED]"}`; string(encoded) != want {
		t.Errorf("Marshal() = %s, want %s", encoded, want)
	}
}

func TestSecretReveal(t *testing.T) {
	if got := Secret("abc").Reveal(); got != "abc" {
		t.Errorf("Reveal() = %q, want %q", got, "abc")
	}
	if got := Secret("").Reveal(); got != "" {
		t.Errorf("Reveal() on empty = %q, want empty", got)
	}
}

func TestSecretIsZero(t *testing.T) {
	tests := []struct {
		name string
		s    Secret
		want bool
	}{
		{"empty", Secret(""), true},
		{"set", Secret("x"), false},
		{"whitespace is a real value", Secret(" "), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.s.IsZero(); got != tt.want {
				t.Errorf("IsZero() = %v, want %v", got, tt.want)
			}
		})
	}
}
