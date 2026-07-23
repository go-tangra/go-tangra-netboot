package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	grpcMD "google.golang.org/grpc/metadata"

	"github.com/go-tangra/go-tangra-common/grpcx"

	"github.com/go-tangra/go-tangra-netboot/internal/authz"
	"github.com/go-tangra/go-tangra-netboot/internal/netbootd"
)

// The service layer is exercised against a stub upstream rather than a mock
// client: the mapping between netbootd's JSON and this module's protobuf is
// exactly the part worth testing, and a hand-written mock would assert on the
// shape we already believe rather than on the wire format netbootd emits.

type stubUpstream struct {
	*httptest.Server

	// routes maps "METHOD /path" to a handler. Unmatched requests fail the
	// test loudly, which catches a service calling the wrong endpoint.
	routes map[string]http.HandlerFunc

	t *testing.T
}

func newStubUpstream(t *testing.T) *stubUpstream {
	t.Helper()

	s := &stubUpstream{routes: map[string]http.HandlerFunc{}, t: t}
	s.Server = httptest.NewServer(http.HandlerFunc(s.serve))
	t.Cleanup(s.Close)

	// Every stub accepts the module's login and hands out a session.
	s.on(http.MethodPost, "/api/v1/auth/login", func(w http.ResponseWriter, r *http.Request) {
		http.SetCookie(w, &http.Cookie{Name: "nb_session", Value: "test-session", Path: "/"})
		writeData(w, http.StatusOK, map[string]any{"id": "op-1", "username": "operator", "active": true})
	})
	return s
}

func (s *stubUpstream) on(method, path string, h http.HandlerFunc) {
	s.routes[method+" "+path] = h
}

// reply registers a handler that returns a fixed JSON document.
func (s *stubUpstream) reply(method, path string, status int, data any) {
	s.on(method, path, func(w http.ResponseWriter, r *http.Request) {
		writeData(w, status, data)
	})
}

// failWith registers a handler that returns a netbootd-shaped error.
func (s *stubUpstream) failWith(method, path string, status int, reason, message string) {
	s.on(method, path, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"success": false,
			"error":   map[string]any{"reason": reason, "message": message},
		})
	})
}

func (s *stubUpstream) serve(w http.ResponseWriter, r *http.Request) {
	if h, ok := s.routes[r.Method+" "+r.URL.Path]; ok {
		h(w, r)
		return
	}
	s.t.Errorf("stub upstream received an unexpected request: %s %s", r.Method, r.URL.Path)
	http.Error(w, "unexpected request", http.StatusNotImplemented)
}

func writeData(w http.ResponseWriter, status int, data any) {
	raw, _ := json.Marshal(data)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "data": json.RawMessage(raw)})
}

// newTestClient points a netbootd client at the stub.
func newTestClient(t *testing.T, s *stubUpstream) *netbootd.Client {
	t.Helper()

	cfg := &netbootd.Config{
		Endpoint:         s.URL,
		Username:         "operator",
		Password:         netbootd.Secret("pw"),
		AllowPlaintext:   true,
		Timeout:          2 * time.Second,
		MaxRetries:       0,
		MaxResponseBytes: netbootd.DefaultMaxResponseBytes,
	}
	client, err := netbootd.NewClient(cfg, testLogger())
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	t.Cleanup(client.Close)
	return client
}

// unconfiguredClient models a module deployed without NETBOOTD_ENDPOINT.
func unconfiguredClient(t *testing.T) *netbootd.Client {
	t.Helper()

	client, err := netbootd.NewClient(&netbootd.Config{
		Timeout:          time.Second,
		MaxResponseBytes: 1024,
	}, testLogger())
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	t.Cleanup(client.Close)
	return client
}

func testLogger() log.Logger { return log.NewStdLogger(discardWriter{}) }

type discardWriter struct{}

func (discardWriter) Write(p []byte) (int, error) { return len(p), nil }

func testHelper(name string) *log.Helper {
	return log.NewHelper(log.With(testLogger(), "module", name))
}

// adminCtx returns a context whose principal holds every netboot permission.
func adminCtx() context.Context { return ctxWithRoles(authz.RolePlatformAdmin) }

// viewerCtx returns a read-only principal.
func viewerCtx() context.Context { return ctxWithRoles(authz.RoleNetbootViewer) }

// anonCtx returns a principal with no roles at all.
func anonCtx() context.Context { return context.Background() }

func ctxWithRoles(roles ...string) context.Context {
	md := grpcMD.New(nil)
	if len(roles) > 0 {
		joined := roles[0]
		for _, r := range roles[1:] {
			joined += "," + r
		}
		md.Set(grpcx.MDRoles, joined)
	}
	md.Set(grpcx.MDUsername, "tester")
	md.Set(grpcx.MDTenantID, "7")
	return grpcMD.NewIncomingContext(context.Background(), md)
}

// Service constructors take a *bootstrap.Context purely to obtain a logger,
// which a unit test has no way to build without a config file; the structs
// are assembled directly instead. A nil collector is legal - every Collector
// method tolerates a nil receiver - which keeps tests free of a Prometheus
// registry and its global state.

func newMachineServiceForTest(client *netbootd.Client) *MachineService {
	return &MachineService{log: testHelper("machine"), client: client, checker: authz.NewChecker()}
}

func newProfileServiceForTest(client *netbootd.Client) *ProfileService {
	return &ProfileService{log: testHelper("profile"), client: client, checker: authz.NewChecker()}
}

func newDhcpServiceForTest(client *netbootd.Client) *DhcpService {
	return &DhcpService{log: testHelper("dhcp"), client: client, checker: authz.NewChecker()}
}

func newSessionServiceForTest(client *netbootd.Client) *SessionService {
	return &SessionService{log: testHelper("session"), client: client, checker: authz.NewChecker()}
}

func newArtifactServiceForTest(client *netbootd.Client) *ArtifactService {
	return &ArtifactService{log: testHelper("artifact"), client: client, checker: authz.NewChecker()}
}

func newSystemServiceForTest(client *netbootd.Client) *SystemService {
	return &SystemService{log: testHelper("system"), client: client, checker: authz.NewChecker()}
}
