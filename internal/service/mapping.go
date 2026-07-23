package service

import (
	"strings"

	"google.golang.org/protobuf/types/known/timestamppb"

	netbootV1 "github.com/go-tangra/go-tangra-netboot/gen/go/netboot/service/v1"
	"github.com/go-tangra/go-tangra-netboot/internal/netbootd"
)

// This file is the single place where netbootd's string-typed wire values
// become the module's enums and back. Keeping the conversion tables here (as
// opposed to inline switches) means an upstream that grows a new state shows
// up as one UNSPECIFIED value rather than as a silently dropped machine.

var (
	firmwareToProto = map[string]netbootV1.Firmware{
		"bios":     netbootV1.Firmware_FIRMWARE_BIOS,
		"uefi_x64": netbootV1.Firmware_FIRMWARE_UEFI_X64,
		"unknown":  netbootV1.Firmware_FIRMWARE_UNKNOWN,
	}
	firmwareToUpstream = invert(firmwareToProto)

	provisionStateToProto = map[string]netbootV1.ProvisionState{
		"new":        netbootV1.ProvisionState_PROVISION_STATE_NEW,
		"ready":      netbootV1.ProvisionState_PROVISION_STATE_READY,
		"installing": netbootV1.ProvisionState_PROVISION_STATE_INSTALLING,
		"installed":  netbootV1.ProvisionState_PROVISION_STATE_INSTALLED,
		"failed":     netbootV1.ProvisionState_PROVISION_STATE_FAILED,
	}
	provisionStateToUpstream = invert(provisionStateToProto)

	sessionStateToProto = map[string]netbootV1.SessionState{
		"active":    netbootV1.SessionState_SESSION_STATE_ACTIVE,
		"completed": netbootV1.SessionState_SESSION_STATE_COMPLETED,
		"failed":    netbootV1.SessionState_SESSION_STATE_FAILED,
		"stale":     netbootV1.SessionState_SESSION_STATE_STALE,
	}
	sessionStateToUpstream = invert(sessionStateToProto)

	eventOutcomeToProto = map[string]netbootV1.EventOutcome{
		"ok":     netbootV1.EventOutcome_EVENT_OUTCOME_OK,
		"error":  netbootV1.EventOutcome_EVENT_OUTCOME_ERROR,
		"denied": netbootV1.EventOutcome_EVENT_OUTCOME_DENIED,
	}

	artifactKindToProto = map[string]netbootV1.ArtifactKind{
		"kernel":   netbootV1.ArtifactKind_ARTIFACT_KIND_KERNEL,
		"initrd":   netbootV1.ArtifactKind_ARTIFACT_KIND_INITRD,
		"ipxe_bin": netbootV1.ArtifactKind_ARTIFACT_KIND_IPXE_BIN,
		"other":    netbootV1.ArtifactKind_ARTIFACT_KIND_OTHER,
	}

	transferProtocolToProto = map[string]netbootV1.TransferProtocol{
		"tftp": netbootV1.TransferProtocol_TRANSFER_PROTOCOL_TFTP,
		"http": netbootV1.TransferProtocol_TRANSFER_PROTOCOL_HTTP,
	}
)

// invert builds the reverse lookup of an upstream-string to enum table.
func invert[E comparable](m map[string]E) map[E]string {
	out := make(map[E]string, len(m))
	for k, v := range m {
		out[v] = k
	}
	return out
}

// lookup normalises the upstream string before matching so that casing or
// padding differences do not produce spurious UNSPECIFIED values.
func lookup[E any](m map[string]E, raw string) E {
	var zero E
	if raw == "" {
		return zero
	}
	if v, ok := m[strings.ToLower(strings.TrimSpace(raw))]; ok {
		return v
	}
	return zero
}

func toTimestamp(t netbootd.Timestamp) *timestamppb.Timestamp {
	if t.IsZero() {
		return nil
	}
	return timestamppb.New(t.Time)
}

func toPageMeta(m *netbootd.PageMeta) *netbootV1.PageMeta {
	if m == nil {
		return &netbootV1.PageMeta{}
	}
	return &netbootV1.PageMeta{
		Total:    m.Total.Int64(),
		Page:     m.Page,
		PageSize: m.PageSize,
	}
}

// toPage converts a request's pagination window into the client's.
func toPage(p *netbootV1.PageRequest) netbootd.Page {
	if p == nil {
		return netbootd.Page{}
	}
	return netbootd.Page{Page: p.GetPage(), PageSize: p.GetPageSize()}
}

func toInstallNetwork(n *netbootd.InstallNetwork) *netbootV1.InstallNetwork {
	if n == nil {
		return nil
	}
	return &netbootV1.InstallNetwork{
		Address: n.Address,
		Gateway: n.Gateway,
		Dns:     n.DNS,
	}
}

func fromInstallNetwork(n *netbootV1.InstallNetwork) *netbootd.InstallNetwork {
	if n == nil {
		return nil
	}
	return &netbootd.InstallNetwork{
		Address: n.GetAddress(),
		Gateway: n.GetGateway(),
		DNS:     n.GetDns(),
	}
}

func toMachine(m *netbootd.Machine) *netbootV1.Machine {
	if m == nil {
		return nil
	}
	return &netbootV1.Machine{
		Id:              m.ID,
		Mac:             m.MAC,
		Name:            m.Name,
		Firmware:        lookup(firmwareToProto, m.Firmware),
		ProfileId:       m.ProfileID,
		ReservationIp:   m.ReservationIP,
		ProvisionState:  lookup(provisionStateToProto, m.ProvisionState),
		Notes:           m.Notes,
		CreateTime:      toTimestamp(m.CreatedAt),
		UpdateTime:      toTimestamp(m.UpdatedAt),
		ActiveSessionId: m.ActiveSessionID,
		NetworkConfig:   m.NetworkConfig,
		InstallNetwork:  toInstallNetwork(m.InstallNetwork),
	}
}

func toMachines(in []*netbootd.Machine) []*netbootV1.Machine {
	out := make([]*netbootV1.Machine, 0, len(in))
	for _, m := range in {
		if converted := toMachine(m); converted != nil {
			out = append(out, converted)
		}
	}
	return out
}

func toUnknownBoots(in []*netbootd.UnknownBoot) []*netbootV1.UnknownBoot {
	out := make([]*netbootV1.UnknownBoot, 0, len(in))
	for _, b := range in {
		if b == nil {
			continue
		}
		out = append(out, &netbootV1.UnknownBoot{
			Mac:      b.MAC,
			LastSeen: toTimestamp(b.LastSeen),
			Attempts: b.Attempts.Int64(),
		})
	}
	return out
}

func toProfile(p *netbootd.Profile) *netbootV1.Profile {
	if p == nil {
		return nil
	}
	return &netbootV1.Profile{
		Id:                 p.ID,
		Name:               p.Name,
		Version:            p.Version,
		UbuntuRelease:      p.UbuntuRelease,
		StorageLayout:      p.StorageLayout,
		NetworkConfig:      p.NetworkConfig,
		Packages:           p.Packages,
		SshAuthorizedKeys:  p.SSHAuthorizedKeys,
		UserDataTemplate:   p.UserDataTemplate,
		LateCommands:       p.LateCommands,
		KernelCmdlineExtra: p.KernelCmdlineArgs,
		CreateTime:         toTimestamp(p.CreatedAt),
		UpdateTime:         toTimestamp(p.UpdatedAt),
		AssignedMachines:   p.AssignedMachines.Int64(),
		KeyboardLayout:     p.KeyboardLayout,
		KeyboardVariant:    p.KeyboardVariant,
		Locale:             p.Locale,
		Timezone:           p.Timezone,
		InstallUsername:    p.InstallUsername,
		HasPassword:        p.HasPassword,
		DefaultDns:         p.DefaultDNS,
	}
}

func toProfiles(in []*netbootd.Profile) []*netbootV1.Profile {
	out := make([]*netbootV1.Profile, 0, len(in))
	for _, p := range in {
		if converted := toProfile(p); converted != nil {
			out = append(out, converted)
		}
	}
	return out
}

// fromProfileInput converts a request body into the upstream shape. The
// plaintext password is moved into a netbootd.Secret here and never exists
// as a bare string in this module again.
func fromProfileInput(in *netbootV1.ProfileInput) *netbootd.ProfileBody {
	if in == nil {
		return nil
	}
	return &netbootd.ProfileBody{
		Name:              in.GetName(),
		UbuntuRelease:     in.GetUbuntuRelease(),
		StorageLayout:     in.GetStorageLayout(),
		NetworkConfig:     in.GetNetworkConfig(),
		Packages:          in.GetPackages(),
		SSHAuthorizedKeys: in.GetSshAuthorizedKeys(),
		UserDataTemplate:  in.GetUserDataTemplate(),
		LateCommands:      in.GetLateCommands(),
		KernelCmdlineArgs: in.GetKernelCmdlineExtra(),
		KeyboardLayout:    in.GetKeyboardLayout(),
		KeyboardVariant:   in.GetKeyboardVariant(),
		Locale:            in.GetLocale(),
		Timezone:          in.GetTimezone(),
		InstallUsername:   in.GetInstallUsername(),
		Password:          netbootd.Secret(in.GetPassword()),
		ClearPassword:     in.GetClearPassword(),
		DefaultDNS:        in.GetDefaultDns(),
	}
}

func toDhcpSubnets(in []*netbootd.DhcpSubnet) []*netbootV1.DhcpSubnet {
	out := make([]*netbootV1.DhcpSubnet, 0, len(in))
	for _, s := range in {
		if s == nil {
			continue
		}
		out = append(out, &netbootV1.DhcpSubnet{
			Id:         s.ID,
			Network:    s.Network,
			RangeStart: s.RangeStart,
			RangeEnd:   s.RangeEnd,
			Gateway:    s.Gateway,
			Dns:        s.DNS,
		})
	}
	return out
}

func fromDhcpSubnets(in []*netbootV1.DhcpSubnet) []*netbootd.DhcpSubnet {
	out := make([]*netbootd.DhcpSubnet, 0, len(in))
	for _, s := range in {
		if s == nil {
			continue
		}
		out = append(out, &netbootd.DhcpSubnet{
			ID:         s.GetId(),
			Network:    s.GetNetwork(),
			RangeStart: s.GetRangeStart(),
			RangeEnd:   s.GetRangeEnd(),
			Gateway:    s.GetGateway(),
			DNS:        s.GetDns(),
		})
	}
	return out
}

func toDhcpConfig(c *netbootd.DhcpConfig) *netbootV1.DhcpConfig {
	if c == nil {
		return nil
	}
	return &netbootV1.DhcpConfig{
		Enabled:         c.Enabled,
		Version:         c.Version,
		LeaseTtlSeconds: c.LeaseTTLSeconds,
		Subnets:         toDhcpSubnets(c.Subnets),
		UpdateTime:      toTimestamp(c.UpdatedAt),
	}
}

func toLeases(in []*netbootd.Lease) []*netbootV1.Lease {
	out := make([]*netbootV1.Lease, 0, len(in))
	for _, l := range in {
		if l == nil {
			continue
		}
		out = append(out, &netbootV1.Lease{
			Ip:          l.IP,
			Mac:         l.MAC,
			MachineId:   l.MachineID,
			MachineName: l.MachineName,
			ExpiresAt:   toTimestamp(l.ExpiresAt),
		})
	}
	return out
}

func toForeignServers(in []*netbootd.ForeignServer) []*netbootV1.ForeignServer {
	out := make([]*netbootV1.ForeignServer, 0, len(in))
	for _, s := range in {
		if s == nil {
			continue
		}
		out = append(out, &netbootV1.ForeignServer{
			ServerId:   s.ServerID,
			LastSeen:   toTimestamp(s.LastSeen),
			OffersSeen: s.OffersSeen.Int64(),
		})
	}
	return out
}

func toSession(s *netbootd.ProvisioningSession) *netbootV1.ProvisioningSession {
	if s == nil {
		return nil
	}
	return &netbootV1.ProvisioningSession{
		Id:             s.ID,
		MachineId:      s.MachineID,
		MachineName:    s.MachineName,
		MachineMac:     s.MachineMAC,
		ProfileId:      s.ProfileID,
		ProfileVersion: s.ProfileVersion,
		State:          lookup(sessionStateToProto, s.State),
		StartedAt:      toTimestamp(s.StartedAt),
		EndedAt:        toTimestamp(s.EndedAt),
		FailurePhase:   s.FailurePhase,
	}
}

func toSessions(in []*netbootd.ProvisioningSession) []*netbootV1.ProvisioningSession {
	out := make([]*netbootV1.ProvisioningSession, 0, len(in))
	for _, s := range in {
		if converted := toSession(s); converted != nil {
			out = append(out, converted)
		}
	}
	return out
}

func toEvents(in []*netbootd.ProvisioningEvent) []*netbootV1.ProvisioningEvent {
	out := make([]*netbootV1.ProvisioningEvent, 0, len(in))
	for _, e := range in {
		if e == nil {
			continue
		}
		out = append(out, &netbootV1.ProvisioningEvent{
			Time:       toTimestamp(e.Time),
			SessionId:  e.SessionID,
			MachineMac: e.MachineMAC,
			Phase:      e.Phase,
			Outcome:    lookup(eventOutcomeToProto, e.Outcome),
			Detail:     rawJSONString(e.Detail),
		})
	}
	return out
}

// rawJSONString renders an opaque upstream JSON blob as a string, collapsing
// the JSON null literal to the empty string so clients do not have to
// special-case it.
func rawJSONString(raw []byte) string {
	s := string(raw)
	if s == "null" {
		return ""
	}
	return s
}

func toArtifact(a *netbootd.BootArtifact) *netbootV1.BootArtifact {
	if a == nil {
		return nil
	}
	return &netbootV1.BootArtifact{
		Id:            a.ID,
		Kind:          lookup(artifactKindToProto, a.Kind),
		UbuntuRelease: a.UbuntuRelease,
		Filename:      a.Filename,
		SizeBytes:     a.SizeBytes.Int64(),
		Sha256:        a.SHA256,
		UploadedBy:    a.UploadedBy,
		CreateTime:    toTimestamp(a.CreatedAt),
		UpdateTime:    toTimestamp(a.UpdatedAt),
	}
}

func toArtifacts(in []*netbootd.BootArtifact) []*netbootV1.BootArtifact {
	out := make([]*netbootV1.BootArtifact, 0, len(in))
	for _, a := range in {
		if converted := toArtifact(a); converted != nil {
			out = append(out, converted)
		}
	}
	return out
}

func toTransfers(in []*netbootd.Transfer) []*netbootV1.Transfer {
	out := make([]*netbootV1.Transfer, 0, len(in))
	for _, t := range in {
		if t == nil {
			continue
		}
		out = append(out, &netbootV1.Transfer{
			Time:      toTimestamp(t.Time),
			ClientIp:  t.ClientIP,
			Filename:  t.Filename,
			BytesSent: t.BytesSent.Int64(),
			Success:   t.Success,
			Error:     t.Error,
			Protocol:  lookup(transferProtocolToProto, t.Protocol),
		})
	}
	return out
}
