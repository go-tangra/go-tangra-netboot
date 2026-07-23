package netbootd

import "encoding/json"

// Secret wraps a credential so that it cannot be leaked by accident.
//
// Go's fmt package consults Stringer/GoStringer for %v/%s/%#v, and
// encoding/json consults Marshaler, so a Secret embedded anywhere in a
// struct that reaches a log line, an error message or an audit record is
// rendered as a fixed placeholder. Call Reveal explicitly - and only at the
// point of use - to obtain the real value.
type Secret string

// redactedPlaceholder is what a Secret renders as everywhere but Reveal.
const redactedPlaceholder = "[REDACTED]"

// String implements fmt.Stringer.
func (s Secret) String() string { return redactedPlaceholder }

// GoString implements fmt.GoStringer so %#v is redacted too.
func (s Secret) GoString() string { return redactedPlaceholder }

// MarshalJSON implements json.Marshaler.
func (s Secret) MarshalJSON() ([]byte, error) { return json.Marshal(redactedPlaceholder) }

// Reveal returns the underlying credential. Every call site is a deliberate
// decision to handle plaintext.
func (s Secret) Reveal() string { return string(s) }

// IsZero reports whether the secret is unset.
func (s Secret) IsZero() bool { return len(s) == 0 }
