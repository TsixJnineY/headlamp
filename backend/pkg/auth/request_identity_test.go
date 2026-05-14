package auth_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gorilla/mux"
	"github.com/kubernetes-sigs/headlamp/backend/pkg/auth"
	"github.com/kubernetes-sigs/headlamp/backend/pkg/kubeconfig"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/client-go/tools/clientcmd/api"
)

func TestResolveRequestIdentityFromCookie(t *testing.T) {
	t.Parallel()

	token := makeTestToken(t, map[string]interface{}{
		"sub":                "user-123",
		"preferred_username": "alice",
		"email":              "alice@example.com",
		"groups":             []string{"dev", "ops"},
		"exp":                float64(time.Now().Add(time.Hour).Unix()),
	})

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/clusters/test/audit/events", nil)
	req = mux.SetURLVars(req, map[string]string{"clusterName": "test"})
	req.AddCookie(&http.Cookie{
		Name:  fmt.Sprintf("headlamp-auth-%s.0", auth.SanitizeClusterName("test")),
		Value: token,
	})

	identity, err := auth.ResolveRequestIdentity(req, "test", auth.ResolveOptions{
		UsernamePaths: "preferred_username",
		EmailPaths:    "email",
		GroupsPaths:   "groups",
		AllowPartial:  true,
		ContextGetter: func(string) (*kubeconfig.Context, error) {
			return &kubeconfig.Context{
				OidcConf: &kubeconfig.OidcConfig{},
			}, nil
		},
	})
	require.NoError(t, err)

	assert.Equal(t, "test", identity.Cluster)
	assert.Equal(t, "alice", identity.Username)
	assert.Equal(t, "alice@example.com", identity.Email)
	assert.Equal(t, "user-123", identity.Sub)
	assert.Equal(t, []string{"dev", "ops"}, identity.Groups)
	assert.Equal(t, "alice@example.com", identity.Principal)
	assert.Equal(t, "cluster_cookie", identity.AuthSource)
	assert.Equal(t, "resolved", identity.IdentityStatus)
}

func TestResolveRequestIdentityAllowsPartial(t *testing.T) {
	t.Parallel()

	token := makeTestToken(t, map[string]interface{}{
		"exp": float64(time.Now().Add(time.Hour).Unix()),
	})

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/clusters/test/audit/events", nil)
	req = mux.SetURLVars(req, map[string]string{"clusterName": "test"})
	req.AddCookie(&http.Cookie{
		Name:  fmt.Sprintf("headlamp-auth-%s.0", auth.SanitizeClusterName("test")),
		Value: token,
	})

	identity, err := auth.ResolveRequestIdentity(req, "test", auth.ResolveOptions{
		UsernamePaths: "preferred_username",
		EmailPaths:    "email",
		GroupsPaths:   "groups",
		AllowPartial:  true,
	})
	require.NoError(t, err)

	assert.Equal(t, "partial", identity.IdentityStatus)
	assert.Equal(t, "cluster_cookie", identity.AuthSource)
	assert.Empty(t, identity.Principal)
}

func TestResolveRequestIdentityAllowsOpaqueBearerAsPartial(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/clusters/test/audit/events", nil)
	req = mux.SetURLVars(req, map[string]string{"clusterName": "test"})
	req.Header.Set("Authorization", "Bearer opaque-token-value")

	identity, err := auth.ResolveRequestIdentity(req, "test", auth.ResolveOptions{
		AllowPartial: true,
		ContextGetter: func(string) (*kubeconfig.Context, error) {
			return &kubeconfig.Context{}, nil
		},
	})
	require.NoError(t, err)

	assert.Equal(t, "authorization_header", identity.AuthSource)
	assert.Equal(t, "partial", identity.IdentityStatus)
	assert.Equal(t, "opaque-token-value", identity.Token)
}

func TestResolveRequestIdentityFallsBackToContextMetadata(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/clusters/test/audit/events", nil)
	req = mux.SetURLVars(req, map[string]string{"clusterName": "test"})

	identity, err := auth.ResolveRequestIdentity(req, "test", auth.ResolveOptions{
		AllowPartial: true,
		ContextGetter: func(string) (*kubeconfig.Context, error) {
			return &kubeconfig.Context{
				AuthInfo: &api.AuthInfo{
					ClientCertificate: "/tmp/client.crt",
					Username:          "cert-user",
				},
			}, nil
		},
	})
	require.NoError(t, err)

	assert.Equal(t, "client_certificate", identity.AuthSource)
	assert.Equal(t, "resolved", identity.IdentityStatus)
	assert.Equal(t, "cert-user", identity.Principal)
}

func TestResolveRequestIdentityServiceAccountMetadataAsPartial(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/clusters/test/audit/events", nil)
	req = mux.SetURLVars(req, map[string]string{"clusterName": "test"})
	req.Header.Set("Authorization", "Bearer opaque-service-account-token")

	identity, err := auth.ResolveRequestIdentity(req, "test", auth.ResolveOptions{
		AllowPartial: true,
		ContextGetter: func(string) (*kubeconfig.Context, error) {
			return &kubeconfig.Context{
				AuthInfo: &api.AuthInfo{
					TokenFile: "/var/run/secrets/kubernetes.io/serviceaccount/token",
				},
			}, nil
		},
	})
	require.NoError(t, err)

	assert.Equal(t, "serviceaccount_bearer", identity.AuthSource)
	assert.Equal(t, "partial", identity.IdentityStatus)
	assert.Equal(t, "opaque-service-account-token", identity.Token)
	assert.Empty(t, identity.Principal)
}

func TestResolveRequestIdentityAllowsUnresolvedWithoutToken(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/clusters/test/audit/events", nil)
	req = mux.SetURLVars(req, map[string]string{"clusterName": "test"})

	identity, err := auth.ResolveRequestIdentity(req, "test", auth.ResolveOptions{
		AllowPartial: true,
		ContextGetter: func(string) (*kubeconfig.Context, error) {
			return &kubeconfig.Context{
				AuthInfo: &api.AuthInfo{},
			}, nil
		},
	})
	require.NoError(t, err)

	assert.Equal(t, "none", identity.AuthSource)
	assert.Equal(t, "unresolved", identity.IdentityStatus)
	assert.Empty(t, identity.Principal)
}

func TestResolveRequestIdentityRequiresCredentialWhenConfigured(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/clusters/test/audit/events", nil)
	req = mux.SetURLVars(req, map[string]string{"clusterName": "test"})

	identity, err := auth.ResolveRequestIdentity(req, "test", auth.ResolveOptions{
		AllowPartial:      true,
		RequireCredential: true,
		ContextGetter: func(string) (*kubeconfig.Context, error) {
			return &kubeconfig.Context{
				AuthInfo: &api.AuthInfo{
					Username: "kubeconfig-user",
				},
			}, nil
		},
	})

	require.Error(t, err)
	assert.Nil(t, identity)
	assert.Equal(t, "unauthorized", err.Error())
}

func TestResolveRequestIdentityRejectsJWTWithoutExpiry(t *testing.T) {
	t.Parallel()

	token := makeTestToken(t, map[string]interface{}{
		"sub": "user-123",
	})

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/clusters/test/audit/events", nil)
	req = mux.SetURLVars(req, map[string]string{"clusterName": "test"})
	req.Header.Set("Authorization", "Bearer "+token)

	identity, err := auth.ResolveRequestIdentity(req, "test", auth.ResolveOptions{
		AllowPartial: true,
	})

	require.Error(t, err)
	assert.Nil(t, identity)
	assert.Equal(t, "token expiry missing or invalid", err.Error())
}
