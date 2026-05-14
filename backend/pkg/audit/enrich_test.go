package audit

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gorilla/mux"
	"github.com/kubernetes-sigs/headlamp/backend/pkg/auth"
	"github.com/kubernetes-sigs/headlamp/backend/pkg/kubeconfig"
	"github.com/stretchr/testify/assert"
)

func makeAuditToken(t *testing.T, claims map[string]interface{}) string {
	t.Helper()
	return auth_test_makeToken(t, claims)
}

// keep token helper local to avoid exporting test helpers from auth_test package
func auth_test_makeToken(t *testing.T, claims map[string]interface{}) string {
	t.Helper()
	return fmt.Sprintf("%s.%s.signature",
		base64RawEncode(t, `{"alg":"none","typ":"JWT"}`),
		base64RawEncodeJSON(t, claims),
	)
}

func base64RawEncode(t *testing.T, value string) string {
	t.Helper()
	return base64.RawURLEncoding.EncodeToString([]byte(value))
}

func base64RawEncodeJSON(t *testing.T, value interface{}) string {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal test json: %v", err)
	}
	return base64.RawURLEncoding.EncodeToString(data)
}

func TestEnrichFromRequestResolvedIdentity(t *testing.T) {
	t.Parallel()

	token := makeAuditToken(t, map[string]interface{}{
		"sub":                "user-123",
		"preferred_username": "alice",
		"email":              "alice@example.com",
		"groups":             []string{"dev", "ops"},
		"exp":                float64(time.Now().Add(time.Hour).Unix()),
	})

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/clusters/test/audit/events", nil)
	req = mux.SetURLVars(req, map[string]string{"clusterName": "test"})
	req.AddCookie(&http.Cookie{
		Name:  fmt.Sprintf("headlamp-auth-%s.0", auth.SanitizeClusterName("test")),
		Value: token,
	})

	event := &Event{Cluster: "test"}
	ok := EnrichFromRequest(req, event, Config{
		Enabled:          true,
		LogUIActions:     true,
		LogTerminalInput: true,
		UsernamePaths:    "preferred_username",
		EmailPaths:       "email",
		GroupsPaths:      "groups",
		AllowPartialUser: true,
	}, func(string) (*kubeconfig.Context, error) {
		return &kubeconfig.Context{OidcConf: &kubeconfig.OidcConfig{}}, nil
	})

	assert.True(t, ok)
	assert.Equal(t, "alice@example.com", event.User)
	assert.Equal(t, []string{"dev", "ops"}, event.Groups)
	assert.Equal(t, "cluster_cookie", event.AuthSource)
}

func TestEnrichFromRequestPartialIdentity(t *testing.T) {
	t.Parallel()

	token := makeAuditToken(t, map[string]interface{}{
		"exp": float64(time.Now().Add(time.Hour).Unix()),
	})

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/clusters/test/audit/events", nil)
	req = mux.SetURLVars(req, map[string]string{"clusterName": "test"})
	req.AddCookie(&http.Cookie{
		Name:  fmt.Sprintf("headlamp-auth-%s.0", auth.SanitizeClusterName("test")),
		Value: token,
	})

	event := &Event{Cluster: "test"}
	ok := EnrichFromRequest(req, event, Config{
		Enabled:          true,
		LogUIActions:     true,
		LogTerminalInput: true,
		UsernamePaths:    "preferred_username",
		EmailPaths:       "email",
		GroupsPaths:      "groups",
		AllowPartialUser: true,
	}, nil)

	assert.True(t, ok)
	assert.Empty(t, event.User)
	assert.Empty(t, event.Groups)
}

func TestEnrichFromRequestRejectsMissingCredential(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/clusters/test/audit/events", nil)
	req = mux.SetURLVars(req, map[string]string{"clusterName": "test"})

	event := &Event{Cluster: "test"}
	ok := EnrichFromRequest(req, event, Config{
		Enabled:          true,
		LogUIActions:     true,
		LogTerminalInput: true,
		AllowPartialUser: true,
	}, func(string) (*kubeconfig.Context, error) {
		return &kubeconfig.Context{}, nil
	})

	assert.False(t, ok)
	assert.Empty(t, event.User)
}
