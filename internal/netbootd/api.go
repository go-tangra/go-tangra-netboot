package netbootd

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
)

// Page is the caller-supplied pagination window. Both fields are optional;
// zero means "let the upstream decide".
type Page struct {
	Page     uint32
	PageSize uint32
}

// apply writes the pagination parameters into q.
func (p Page) apply(q url.Values) {
	if p.Page > 0 {
		q.Set("page", strconv.FormatUint(uint64(p.Page), 10))
	}
	if p.PageSize > 0 {
		q.Set("page_size", strconv.FormatUint(uint64(p.PageSize), 10))
	}
}

// MachineFilter narrows a machine listing.
type MachineFilter struct {
	Page      Page
	State     string
	ProfileID string
	Query     string
}

// ListMachines returns a page of registered machines.
func (c *Client) ListMachines(ctx context.Context, f MachineFilter) (*ListMachinesReply, error) {
	q := url.Values{}
	f.Page.apply(q)
	setIfNotEmpty(q, "state", f.State)
	setIfNotEmpty(q, "profile_id", f.ProfileID)
	setIfNotEmpty(q, "q", f.Query)

	var reply ListMachinesReply
	if err := c.do(ctx, &request{method: http.MethodGet, path: pathMachines, query: q}, &reply); err != nil {
		return nil, err
	}
	return &reply, nil
}

// GetMachine returns a single machine by id.
func (c *Client) GetMachine(ctx context.Context, id string) (*Machine, error) {
	var m Machine
	err := c.do(ctx, &request{
		method: http.MethodGet,
		path:   pathMachines + "/" + escapeSegment(id),
	}, &m)
	if err != nil {
		return nil, err
	}
	return &m, nil
}

// CreateMachine registers a new machine.
func (c *Client) CreateMachine(ctx context.Context, body *CreateMachineBody) (*Machine, error) {
	var m Machine
	err := c.do(ctx, &request{method: http.MethodPost, path: pathMachines, body: body}, &m)
	if err != nil {
		return nil, err
	}
	return &m, nil
}

// UpdateMachine patches the mutable fields of a machine.
func (c *Client) UpdateMachine(ctx context.Context, id string, body *UpdateMachineBody) (*Machine, error) {
	var m Machine
	err := c.do(ctx, &request{
		method: http.MethodPatch,
		path:   pathMachines + "/" + escapeSegment(id),
		body:   body,
	}, &m)
	if err != nil {
		return nil, err
	}
	return &m, nil
}

// DeleteMachine removes a machine.
func (c *Client) DeleteMachine(ctx context.Context, id string) error {
	return c.do(ctx, &request{
		method: http.MethodDelete,
		path:   pathMachines + "/" + escapeSegment(id),
	}, nil)
}

// ProvisionMachine arms a machine for its next boot.
func (c *Client) ProvisionMachine(ctx context.Context, id string) (*Machine, error) {
	var m Machine
	err := c.do(ctx, &request{
		method: http.MethodPost,
		path:   pathMachines + "/" + escapeSegment(id) + "/provision",
		body:   struct{}{},
	}, &m)
	if err != nil {
		return nil, err
	}
	return &m, nil
}

// CancelProvision cancels an armed or running provisioning session.
func (c *Client) CancelProvision(ctx context.Context, id string) (*Machine, error) {
	var m Machine
	err := c.do(ctx, &request{
		method: http.MethodPost,
		path:   pathMachines + "/" + escapeSegment(id) + "/cancel",
		body:   struct{}{},
	}, &m)
	if err != nil {
		return nil, err
	}
	return &m, nil
}

// ListUnknownBoots returns MACs that attempted to boot without registration.
func (c *Client) ListUnknownBoots(ctx context.Context, p Page) (*ListUnknownBootsReply, error) {
	q := url.Values{}
	p.apply(q)

	var reply ListUnknownBootsReply
	if err := c.do(ctx, &request{method: http.MethodGet, path: pathUnknownBoots, query: q}, &reply); err != nil {
		return nil, err
	}
	return &reply, nil
}

// RegisterFromUnknown promotes an unknown boot into a registered machine.
func (c *Client) RegisterFromUnknown(ctx context.Context, body *RegisterFromUnknownBody) (*Machine, error) {
	var m Machine
	err := c.do(ctx, &request{method: http.MethodPost, path: pathRegisterFrom, body: body}, &m)
	if err != nil {
		return nil, err
	}
	return &m, nil
}

// ListProfiles returns a page of installation profiles.
func (c *Client) ListProfiles(ctx context.Context, p Page) (*ListProfilesReply, error) {
	q := url.Values{}
	p.apply(q)

	var reply ListProfilesReply
	if err := c.do(ctx, &request{method: http.MethodGet, path: pathProfiles, query: q}, &reply); err != nil {
		return nil, err
	}
	return &reply, nil
}

// GetProfile returns a single profile by id.
func (c *Client) GetProfile(ctx context.Context, id string) (*Profile, error) {
	var p Profile
	err := c.do(ctx, &request{
		method: http.MethodGet,
		path:   pathProfiles + "/" + escapeSegment(id),
	}, &p)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// CreateProfile creates a profile.
func (c *Client) CreateProfile(ctx context.Context, body *ProfileBody) (*Profile, error) {
	var p Profile
	err := c.do(ctx, &request{method: http.MethodPost, path: pathProfiles, body: body}, &p)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// UpdateProfile replaces a profile's mutable body.
func (c *Client) UpdateProfile(ctx context.Context, id string, body *ProfileBody) (*Profile, error) {
	var p Profile
	err := c.do(ctx, &request{
		method: http.MethodPut,
		path:   pathProfiles + "/" + escapeSegment(id),
		body:   &UpdateProfileBody{Profile: body},
	}, &p)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// CloneProfile copies a profile under a new name.
func (c *Client) CloneProfile(ctx context.Context, id, newName string) (*Profile, error) {
	var p Profile
	err := c.do(ctx, &request{
		method: http.MethodPost,
		path:   pathProfiles + "/" + escapeSegment(id) + "/clone",
		body:   &CloneProfileBody{NewName: newName},
	}, &p)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// DeleteProfile removes a profile.
func (c *Client) DeleteProfile(ctx context.Context, id string) error {
	return c.do(ctx, &request{
		method: http.MethodDelete,
		path:   pathProfiles + "/" + escapeSegment(id),
	}, nil)
}

// PreviewProfile renders a profile's autoinstall seed. The upstream redacts
// credentials before returning it.
func (c *Client) PreviewProfile(ctx context.Context, id, machineID string) (*PreviewProfileReply, error) {
	var reply PreviewProfileReply
	err := c.do(ctx, &request{
		method: http.MethodPost,
		path:   pathProfiles + "/" + escapeSegment(id) + "/preview",
		body:   &PreviewProfileBody{MachineID: machineID},
	}, &reply)
	if err != nil {
		return nil, err
	}
	return &reply, nil
}

// GetDhcpConfig returns the authoritative DHCP configuration.
func (c *Client) GetDhcpConfig(ctx context.Context) (*DhcpConfig, error) {
	var cfg DhcpConfig
	if err := c.do(ctx, &request{method: http.MethodGet, path: pathDhcpConfig}, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// UpdateDhcpConfig replaces the DHCP configuration.
func (c *Client) UpdateDhcpConfig(ctx context.Context, body *UpdateDhcpConfigBody) (*DhcpConfig, error) {
	var cfg DhcpConfig
	err := c.do(ctx, &request{method: http.MethodPut, path: pathDhcpConfig, body: body}, &cfg)
	if err != nil {
		return nil, err
	}
	return &cfg, nil
}

// EnableDhcp starts the authoritative DHCP server.
func (c *Client) EnableDhcp(ctx context.Context) (*DhcpConfig, error) {
	return c.dhcpToggle(ctx, pathDhcpEnable)
}

// DisableDhcp stops the authoritative DHCP server.
func (c *Client) DisableDhcp(ctx context.Context) (*DhcpConfig, error) {
	return c.dhcpToggle(ctx, pathDhcpDisable)
}

func (c *Client) dhcpToggle(ctx context.Context, path string) (*DhcpConfig, error) {
	var cfg DhcpConfig
	err := c.do(ctx, &request{method: http.MethodPost, path: path, body: struct{}{}}, &cfg)
	if err != nil {
		return nil, err
	}
	return &cfg, nil
}

// ListLeases returns a page of active DHCP leases.
func (c *Client) ListLeases(ctx context.Context, p Page) (*ListLeasesReply, error) {
	q := url.Values{}
	p.apply(q)

	var reply ListLeasesReply
	if err := c.do(ctx, &request{method: http.MethodGet, path: pathDhcpLeases, query: q}, &reply); err != nil {
		return nil, err
	}
	return &reply, nil
}

// ListForeignServers returns rogue DHCP servers observed on the segment.
func (c *Client) ListForeignServers(ctx context.Context, p Page) (*ListForeignServersReply, error) {
	q := url.Values{}
	p.apply(q)

	var reply ListForeignServersReply
	if err := c.do(ctx, &request{method: http.MethodGet, path: pathDhcpConflict, query: q}, &reply); err != nil {
		return nil, err
	}
	return &reply, nil
}

// SessionFilter narrows a provisioning-session listing.
type SessionFilter struct {
	Page      Page
	MachineID string
	State     string
}

// ListSessions returns a page of provisioning sessions.
func (c *Client) ListSessions(ctx context.Context, f SessionFilter) (*ListSessionsReply, error) {
	q := url.Values{}
	f.Page.apply(q)
	setIfNotEmpty(q, "machine_id", f.MachineID)
	setIfNotEmpty(q, "state", f.State)

	var reply ListSessionsReply
	if err := c.do(ctx, &request{method: http.MethodGet, path: pathSessions, query: q}, &reply); err != nil {
		return nil, err
	}
	return &reply, nil
}

// GetSession returns one session with its event timeline.
func (c *Client) GetSession(ctx context.Context, id string) (*SessionDetail, error) {
	var detail SessionDetail
	err := c.do(ctx, &request{
		method: http.MethodGet,
		path:   pathSessions + "/" + escapeSegment(id),
	}, &detail)
	if err != nil {
		return nil, err
	}
	return &detail, nil
}

// ListArtifacts returns a page of boot artifacts.
func (c *Client) ListArtifacts(ctx context.Context, p Page) (*ListArtifactsReply, error) {
	q := url.Values{}
	p.apply(q)

	var reply ListArtifactsReply
	if err := c.do(ctx, &request{method: http.MethodGet, path: pathArtifacts, query: q}, &reply); err != nil {
		return nil, err
	}
	return &reply, nil
}

// GetArtifact returns one boot artifact's metadata.
func (c *Client) GetArtifact(ctx context.Context, id string) (*BootArtifact, error) {
	var a BootArtifact
	err := c.do(ctx, &request{
		method: http.MethodGet,
		path:   pathArtifacts + "/" + escapeSegment(id),
	}, &a)
	if err != nil {
		return nil, err
	}
	return &a, nil
}

// DeleteArtifact removes a boot artifact.
func (c *Client) DeleteArtifact(ctx context.Context, id string) error {
	return c.do(ctx, &request{
		method: http.MethodDelete,
		path:   pathArtifacts + "/" + escapeSegment(id),
	}, nil)
}

// TransferFilter narrows a transfer-log listing.
type TransferFilter struct {
	Page     Page
	Filename string
}

// ListTransfers returns a page of boot-path transfer records.
func (c *Client) ListTransfers(ctx context.Context, f TransferFilter) (*ListTransfersReply, error) {
	q := url.Values{}
	f.Page.apply(q)
	setIfNotEmpty(q, "filename", f.Filename)

	var reply ListTransfersReply
	if err := c.do(ctx, &request{method: http.MethodGet, path: pathTransfers, query: q}, &reply); err != nil {
		return nil, err
	}
	return &reply, nil
}

func setIfNotEmpty(q url.Values, key, value string) {
	if value != "" {
		q.Set(key, value)
	}
}
