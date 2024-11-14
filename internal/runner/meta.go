package runner

import (
	"context"
	"log/slog"
	"net/netip"
	"time"

	"github.com/leg100/otf/internal"
	"github.com/leg100/otf/internal/authz"
	"github.com/leg100/otf/internal/rbac"
	"github.com/leg100/otf/internal/resource"
)

// RunnerMeta is information about a runner.
type RunnerMeta struct {
	ID resource.ID `jsonapi:"primary,runners"`
	// Optional name
	Name string `jsonapi:"attribute" json:"name"`
	// Version of runner
	Version string `jsonapi:"attribute" json:"version"`
	// Current status of runner
	Status RunnerStatus `jsonapi:"attribute" json:"status"`
	// Max number of jobs runner can execute
	MaxJobs int `jsonapi:"attribute" json:"max_jobs"`
	// Current number of jobs allocated to runner.
	CurrentJobs int `jsonapi:"attribute" json:"current_jobs"`
	// Last time a ping was received from the runner.
	LastPingAt time.Time `jsonapi:"attribute" json:"last-ping-at"`
	// Last time the status was updated
	LastStatusAt time.Time `jsonapi:"attribute" json:"last-status-at"`
	// IP address of runner.
	IPAddress netip.Addr `jsonapi:"attribute" json:"ip-address"`
	// Info about the runner's agent pool. Non-nil if agent runner; nil if server
	// runner.
	AgentPool *RunnerMetaAgentPool `jsonapi:"attribute" json:"agent-pool"`
}

type RunnerMetaAgentPool struct {
	// ID of agent's pool.
	ID resource.ID `json:"id"`
	// Name of agent's pool
	Name string `json:"name"`
	// Agent pool's organization.
	OrganizationName string `json:"organization-name"`
	// ID of agent token that was used to authenticate runner.
	TokenID resource.ID `json:"token-id"`
}

type registerOptions struct {
	// Descriptive name. Optional.
	Name string `json:"name"`
	// Version of agent.
	Version string `json:"version"`
	// Number of jobs the agent can handle at any one time.
	Concurrency int `json:"concurrency"`
	// IPAddress of agent. Optional. Not sent over the wire; instead the server
	// handler is responsible for determining client's IP address.
	IPAddress *netip.Addr `json:"-"`
	// ID of agent's pool. Only set if runner is an agent.
	AgentPoolID *string `json:"-"`
	// CurrentJobs are those jobs the agent has discovered leftover from a
	// previous agent. Not currently used but may be made use of in later
	// versions.
	CurrentJobs []resource.ID `json:"current-jobs,omitempty"`
}

// register registers an unregistered runner, constructing a RunnerMeta which
// provides info about the newly registered runner.
func register(runner *unregistered, opts registerOptions) (*RunnerMeta, error) {
	meta := &RunnerMeta{
		ID:        resource.NewID(resource.RunnerKind),
		Name:      opts.Name,
		Version:   opts.Version,
		MaxJobs:   opts.Concurrency,
		AgentPool: runner.pool,
	}
	if err := meta.setStatus(RunnerIdle, true); err != nil {
		return nil, err
	}
	if opts.IPAddress != nil {
		meta.IPAddress = *opts.IPAddress
	} else {
		// IP address not provided: try to get local IP address used for
		// outbound comms, and if that fails, use localhost
		ip, err := internal.GetOutboundIP()
		if err != nil {
			ip = netip.IPv6Loopback()
		}
		meta.IPAddress = ip
	}

	return meta, nil
}

func (m *RunnerMeta) setStatus(status RunnerStatus, ping bool) error {
	// the agent fsm is as follows:
	//
	// idle -> any
	// busy -> any
	// unknown -> any
	// errored (final state)
	// exited (final state)
	switch m.Status {
	case RunnerErrored, RunnerExited:
		return internal.ErrConflict
	}
	m.Status = status
	now := internal.CurrentTimestamp(nil)
	m.LastStatusAt = now
	// also update ping time if requested
	if ping {
		m.LastPingAt = now
	}
	return nil
}

func (m *RunnerMeta) IsAgent() bool {
	return m.AgentPool != nil
}

func (m *RunnerMeta) LogValue() slog.Value {
	attrs := []slog.Attr{
		slog.String("id", m.ID.String()),
		slog.Bool("agent", m.IsAgent()),
		slog.String("status", string(m.Status)),
		slog.String("ip_address", m.IPAddress.String()),
	}
	if m.AgentPool != nil {
		attrs = append(attrs, slog.Any("pool", m.AgentPool))
	}
	if m.Name != "" {
		attrs = append(attrs, slog.String("name", m.Name))
	}
	return slog.GroupValue(attrs...)
}

func (m *RunnerMetaAgentPool) LogValue() slog.Value {
	return slog.GroupValue(
		slog.String("id", m.ID.String()),
		slog.String("name", m.Name),
		slog.String("organization", m.OrganizationName),
		slog.String("token-id", m.TokenID.String()),
	)
}

func (m *RunnerMeta) String() string { return m.ID.String() }

func (m *RunnerMeta) CanAccess(action rbac.Action, req *authz.AccessRequest) bool {
	if req == nil {
		// Don't permit runners to carry out site-level actions
		return false
	}
	// TODO: permit only those actions that an agent needs to carry out (get
	// agent jobs, etc).
	if m.IsAgent() {
		// Agents can only carry out actions on the organization their pool
		// belongs to.
		return m.AgentPool.OrganizationName == req.Organization
	} else {
		// Server runners can carry out actions on all organizations.
		return true
	}
}

func runnerFromContext(ctx context.Context) (*RunnerMeta, error) {
	subject, err := authz.SubjectFromContext(ctx)
	if err != nil {
		return nil, err
	}
	meta, ok := subject.(*RunnerMeta)
	if !ok {
		return nil, internal.ErrAccessNotPermitted
	}
	return meta, nil
}

func authorizeRunner(ctx context.Context, id resource.ID) error {
	runner, err := runnerFromContext(ctx)
	if err != nil {
		return err
	}
	if id != runner.ID {
		return internal.ErrAccessNotPermitted
	}
	return nil
}