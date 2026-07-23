package service

import (
	"net"
	"strings"
)

// normaliseMAC renders a validated MAC address in the lower-case
// colon-separated form netbootd stores.
//
// The proto pattern already accepts only 48-bit colon- or hyphen-separated
// input, so parsing cannot realistically fail here; when it does the value is
// passed through untouched and the upstream renders the verdict, rather than
// this module silently substituting something the operator did not type.
func normaliseMAC(raw string) string {
	trimmed := strings.TrimSpace(raw)
	hw, err := net.ParseMAC(trimmed)
	if err != nil {
		return trimmed
	}
	return hw.String()
}
