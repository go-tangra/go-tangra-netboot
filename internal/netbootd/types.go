package netbootd

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"
)

// The wire types below mirror netbootd's protojson output, which is emitted
// with UseProtoNames (snake_case field names) and EmitUnpopulated. Note that
// protojson renders 64-bit integers as JSON *strings*, hence flexInt64.

// flexInt64 decodes a 64-bit integer that may arrive as a JSON number or as
// a JSON string.
type flexInt64 int64

func (f *flexInt64) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		*f = 0
		return nil
	}
	if len(data) > 1 && data[0] == '"' {
		var s string
		if err := json.Unmarshal(data, &s); err != nil {
			return err
		}
		if s == "" {
			*f = 0
			return nil
		}
		n, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return fmt.Errorf("parse int64 %q: %w", s, err)
		}
		*f = flexInt64(n)
		return nil
	}
	var n int64
	if err := json.Unmarshal(data, &n); err != nil {
		return err
	}
	*f = flexInt64(n)
	return nil
}

func (f flexInt64) Int64() int64 { return int64(f) }

// MarshalJSON keeps round-tripping symmetric for tests and fixtures.
func (f flexInt64) MarshalJSON() ([]byte, error) { return json.Marshal(int64(f)) }

// Timestamp decodes an RFC 3339 timestamp, tolerating the empty/null value
// that EmitUnpopulated produces for an unset google.protobuf.Timestamp.
type Timestamp struct {
	time.Time
}

func (t *Timestamp) UnmarshalJSON(data []byte) error {
	s := string(data)
	if s == "null" || s == `""` {
		t.Time = time.Time{}
		return nil
	}
	var raw string
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	if raw == "" {
		t.Time = time.Time{}
		return nil
	}
	parsed, err := time.Parse(time.RFC3339Nano, raw)
	if err != nil {
		return fmt.Errorf("parse timestamp %q: %w", raw, err)
	}
	t.Time = parsed
	return nil
}

func (t Timestamp) MarshalJSON() ([]byte, error) {
	if t.Time.IsZero() {
		return []byte("null"), nil
	}
	return json.Marshal(t.Time.UTC().Format(time.RFC3339Nano))
}

// PageMeta is the pagination envelope returned by every upstream list call.
type PageMeta struct {
	Total    flexInt64 `json:"total"`
	Page     int32     `json:"page"`
	PageSize int32     `json:"page_size"`
}

// Operator is the authenticated netbootd operator.
type Operator struct {
	ID          string `json:"id"`
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
	Active      bool   `json:"active"`
}

// InstallNetwork is the machine's production network configuration.
type InstallNetwork struct {
	Address string   `json:"address"`
	Gateway string   `json:"gateway"`
	DNS     []string `json:"dns"`
}

// Machine is a netboot target.
type Machine struct {
	ID              string          `json:"id"`
	MAC             string          `json:"mac"`
	Name            string          `json:"name"`
	Firmware        string          `json:"firmware"`
	ProfileID       string          `json:"profile_id"`
	ReservationIP   string          `json:"reservation_ip"`
	ProvisionState  string          `json:"provision_state"`
	Notes           string          `json:"notes"`
	CreatedAt       Timestamp       `json:"created_at"`
	UpdatedAt       Timestamp       `json:"updated_at"`
	ActiveSessionID string          `json:"active_session_id"`
	NetworkConfig   string          `json:"network_config"`
	InstallNetwork  *InstallNetwork `json:"install_network"`
}

type ListMachinesReply struct {
	Machines []*Machine `json:"machines"`
	Meta     *PageMeta  `json:"meta"`
}

// CreateMachineBody is the upstream CreateMachine request body.
type CreateMachineBody struct {
	MAC            string          `json:"mac"`
	Name           string          `json:"name"`
	Firmware       string          `json:"firmware,omitempty"`
	ProfileID      string          `json:"profile_id,omitempty"`
	ReservationIP  string          `json:"reservation_ip,omitempty"`
	Notes          string          `json:"notes,omitempty"`
	NetworkConfig  string          `json:"network_config,omitempty"`
	InstallNetwork *InstallNetwork `json:"install_network,omitempty"`
}

// UpdateMachineBody carries only the fields the caller asked to change;
// pointer fields distinguish "leave alone" from "clear".
type UpdateMachineBody struct {
	Name           *string         `json:"name,omitempty"`
	ProfileID      *string         `json:"profile_id,omitempty"`
	ReservationIP  *string         `json:"reservation_ip,omitempty"`
	Notes          *string         `json:"notes,omitempty"`
	NetworkConfig  *string         `json:"network_config,omitempty"`
	InstallNetwork *InstallNetwork `json:"install_network,omitempty"`
}

type UnknownBoot struct {
	MAC      string    `json:"mac"`
	LastSeen Timestamp `json:"last_seen"`
	Attempts flexInt64 `json:"attempts"`
}

type ListUnknownBootsReply struct {
	Boots []*UnknownBoot `json:"boots"`
	Meta  *PageMeta      `json:"meta"`
}

type RegisterFromUnknownBody struct {
	MAC       string `json:"mac"`
	Name      string `json:"name"`
	ProfileID string `json:"profile_id,omitempty"`
}

// Profile is an installation profile. The upstream never returns the login
// password hash, only has_password.
type Profile struct {
	ID                string    `json:"id"`
	Name              string    `json:"name"`
	Version           int32     `json:"version"`
	UbuntuRelease     string    `json:"ubuntu_release"`
	StorageLayout     string    `json:"storage_layout"`
	NetworkConfig     string    `json:"network_config"`
	Packages          []string  `json:"packages"`
	SSHAuthorizedKeys []string  `json:"ssh_authorized_keys"`
	UserDataTemplate  string    `json:"user_data_template"`
	LateCommands      []string  `json:"late_commands"`
	KernelCmdlineArgs string    `json:"kernel_cmdline_extra"`
	CreatedAt         Timestamp `json:"created_at"`
	UpdatedAt         Timestamp `json:"updated_at"`
	AssignedMachines  flexInt64 `json:"assigned_machines"`
	KeyboardLayout    string    `json:"keyboard_layout"`
	KeyboardVariant   string    `json:"keyboard_variant"`
	Locale            string    `json:"locale"`
	Timezone          string    `json:"timezone"`
	InstallUsername   string    `json:"install_username"`
	HasPassword       bool      `json:"has_password"`
	DefaultDNS        []string  `json:"default_dns"`
}

// ProfileBody is the upstream ProfileInput. Password is a Secret so an
// accidental log of an outbound request body cannot leak the plaintext; it
// is unwrapped only by MarshalJSON's explicit alias below.
type ProfileBody struct {
	Name              string   `json:"name"`
	UbuntuRelease     string   `json:"ubuntu_release"`
	StorageLayout     string   `json:"storage_layout,omitempty"`
	NetworkConfig     string   `json:"network_config,omitempty"`
	Packages          []string `json:"packages,omitempty"`
	SSHAuthorizedKeys []string `json:"ssh_authorized_keys,omitempty"`
	UserDataTemplate  string   `json:"user_data_template,omitempty"`
	LateCommands      []string `json:"late_commands,omitempty"`
	KernelCmdlineArgs string   `json:"kernel_cmdline_extra,omitempty"`
	KeyboardLayout    string   `json:"keyboard_layout,omitempty"`
	KeyboardVariant   string   `json:"keyboard_variant,omitempty"`
	Locale            string   `json:"locale,omitempty"`
	Timezone          string   `json:"timezone,omitempty"`
	InstallUsername   string   `json:"install_username,omitempty"`
	Password          Secret   `json:"-"`
	ClearPassword     bool     `json:"clear_password,omitempty"`
	DefaultDNS        []string `json:"default_dns,omitempty"`
}

// MarshalJSON serialises the body for the wire, injecting the plaintext
// password only here. Every other rendering of a ProfileBody - logs, errors,
// %+v in a test failure - omits the field entirely.
func (b ProfileBody) MarshalJSON() ([]byte, error) {
	type alias ProfileBody
	return json.Marshal(struct {
		alias
		Password string `json:"password,omitempty"`
	}{alias: alias(b), Password: b.Password.Reveal()})
}

type ListProfilesReply struct {
	Profiles []*Profile `json:"profiles"`
	Meta     *PageMeta  `json:"meta"`
}

type UpdateProfileBody struct {
	Profile *ProfileBody `json:"profile"`
}

type CloneProfileBody struct {
	NewName string `json:"new_name"`
}

type PreviewProfileBody struct {
	MachineID string `json:"machine_id,omitempty"`
}

type PreviewProfileReply struct {
	UserData string `json:"user_data"`
	Cmdline  string `json:"cmdline"`
}

type DhcpSubnet struct {
	ID         string   `json:"id,omitempty"`
	Network    string   `json:"network"`
	RangeStart string   `json:"range_start"`
	RangeEnd   string   `json:"range_end"`
	Gateway    string   `json:"gateway,omitempty"`
	DNS        []string `json:"dns,omitempty"`
}

type DhcpConfig struct {
	Enabled         bool          `json:"enabled"`
	Version         int32         `json:"version"`
	LeaseTTLSeconds int32         `json:"lease_ttl_seconds"`
	Subnets         []*DhcpSubnet `json:"subnets"`
	UpdatedAt       Timestamp     `json:"updated_at"`
}

type UpdateDhcpConfigBody struct {
	LeaseTTLSeconds int32         `json:"lease_ttl_seconds"`
	Subnets         []*DhcpSubnet `json:"subnets"`
}

type Lease struct {
	IP          string    `json:"ip"`
	MAC         string    `json:"mac"`
	MachineID   string    `json:"machine_id"`
	MachineName string    `json:"machine_name"`
	ExpiresAt   Timestamp `json:"expires_at"`
}

type ListLeasesReply struct {
	Leases []*Lease  `json:"leases"`
	Meta   *PageMeta `json:"meta"`
}

type ForeignServer struct {
	ServerID   string    `json:"server_id"`
	LastSeen   Timestamp `json:"last_seen"`
	OffersSeen flexInt64 `json:"offers_seen"`
}

type ListForeignServersReply struct {
	Servers []*ForeignServer `json:"servers"`
	Meta    *PageMeta        `json:"meta"`
}

type ProvisioningSession struct {
	ID             string    `json:"id"`
	MachineID      string    `json:"machine_id"`
	MachineName    string    `json:"machine_name"`
	MachineMAC     string    `json:"machine_mac"`
	ProfileID      string    `json:"profile_id"`
	ProfileVersion int32     `json:"profile_version"`
	State          string    `json:"state"`
	StartedAt      Timestamp `json:"started_at"`
	EndedAt        Timestamp `json:"ended_at"`
	FailurePhase   string    `json:"failure_phase"`
}

type ProvisioningEvent struct {
	Time       Timestamp       `json:"time"`
	SessionID  string          `json:"session_id"`
	MachineMAC string          `json:"machine_mac"`
	Phase      string          `json:"phase"`
	Outcome    string          `json:"outcome"`
	Detail     json.RawMessage `json:"detail"`
}

type ListSessionsReply struct {
	Sessions []*ProvisioningSession `json:"sessions"`
	Meta     *PageMeta              `json:"meta"`
}

type SessionDetail struct {
	Session  *ProvisioningSession `json:"session"`
	Timeline []*ProvisioningEvent `json:"timeline"`
	Evidence json.RawMessage      `json:"evidence"`
}

type BootArtifact struct {
	ID            string    `json:"id"`
	Kind          string    `json:"kind"`
	UbuntuRelease string    `json:"ubuntu_release"`
	Filename      string    `json:"filename"`
	SizeBytes     flexInt64 `json:"size_bytes"`
	SHA256        string    `json:"sha256"`
	UploadedBy    string    `json:"uploaded_by"`
	CreatedAt     Timestamp `json:"created_at"`
	UpdatedAt     Timestamp `json:"updated_at"`
}

type ListArtifactsReply struct {
	Artifacts []*BootArtifact `json:"artifacts"`
	Meta      *PageMeta       `json:"meta"`
}

type Transfer struct {
	Time      Timestamp `json:"time"`
	ClientIP  string    `json:"client_ip"`
	Filename  string    `json:"filename"`
	BytesSent flexInt64 `json:"bytes_sent"`
	Success   bool      `json:"success"`
	Error     string    `json:"error"`
	Protocol  string    `json:"protocol"`
}

type ListTransfersReply struct {
	Transfers []*Transfer `json:"transfers"`
	Meta      *PageMeta   `json:"meta"`
}
