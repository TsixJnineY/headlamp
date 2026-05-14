package auth

import (
	"errors"
	"net/http"
	"time"

	"github.com/kubernetes-sigs/headlamp/backend/pkg/kubeconfig"
)

type RequestIdentity struct {
	Cluster        string
	Token          string
	Principal      string
	Username       string
	Sub            string
	Email          string
	Groups         []string
	AuthSource     string
	IdentityStatus string
	Claims         map[string]interface{}
}

type ResolveOptions struct {
	UsernamePaths     string
	EmailPaths        string
	GroupsPaths       string
	AllowPartial      bool
	RequireCredential bool
	ContextGetter     func(string) (*kubeconfig.Context, error)
}

func ResolveRequestIdentity(r *http.Request, clusterName string, opts ResolveOptions) (*RequestIdentity, error) {
	if clusterName == "" {
		return nil, errors.New("cluster not specified")
	}

	var ctx *kubeconfig.Context
	if opts.ContextGetter != nil {
		ctx, _ = opts.ContextGetter(clusterName)
	}

	parsedAuth := ParseRequestAuth(r)
	requestCluster := parsedAuth.Cluster
	token := parsedAuth.Token
	if requestCluster == "" {
		requestCluster = clusterName
	}

	if requestCluster != clusterName {
		return nil, errors.New("cluster mismatch")
	}

	if opts.RequireCredential && token == "" {
		return nil, errors.New("unauthorized")
	}

	usernamePaths, emailPaths, groupsPaths, _ := applyMeDefaults(
		opts.UsernamePaths,
		opts.EmailPaths,
		opts.GroupsPaths,
		"",
	)

	compiledUsernamePaths := compileJMESPaths(usernamePaths)
	compiledEmailPaths := compileJMESPaths(emailPaths)
	compiledGroupsPaths := compileJMESPaths(groupsPaths)

	identity := &RequestIdentity{
		Cluster:        clusterName,
		Token:          token,
		AuthSource:     resolveAuthSource(parsedAuth.TokenSource, ctx),
		IdentityStatus: "unresolved",
	}

	if token != "" {
		if claims, status, errMsg := parseClaimsFromToken(token); status == 0 {
			expiry, err := GetExpiryUnixTimeUTC(claims)
			if err != nil {
				return nil, errors.New("token expiry missing or invalid")
			}
			if time.Now().After(expiry) {
				return nil, errors.New("token expired")
			}

			identity.Claims = claims
			identity.Username = stringValueFromJMESPaths(claims, compiledUsernamePaths)
			identity.Email = stringValueFromJMESPaths(claims, compiledEmailPaths)
			identity.Groups = stringSliceFromJMESPaths(claims, compiledGroupsPaths)
			identity.Sub = firstStringClaim(claims, "sub")
			identity.Principal = firstNonEmpty(identity.Email, identity.Username, identity.Sub)
		} else if !opts.AllowPartial {
			return nil, errors.New(errMsg)
		}
	}

	if identity.Principal == "" {
		identity.Principal = principalFromContext(ctx)
	}

	if identity.Principal != "" || len(identity.Groups) > 0 {
		identity.IdentityStatus = "resolved"
		return identity, nil
	}

	if opts.AllowPartial {
		if token != "" || hasContextIdentityMetadata(ctx) {
			identity.IdentityStatus = "partial"
		}
		return identity, nil
	}

	if token == "" {
		return nil, errors.New("unauthorized")
	}

	return nil, errors.New("identity unresolved")
}

func resolveAuthSource(tokenSource string, ctx *kubeconfig.Context) string {
	if tokenSource == "authorization_header" {
		if hasServiceAccountToken(ctx) {
			return "serviceaccount_bearer"
		}
		if ctx != nil && ctx.AuthType() == "oidc" {
			return "oidc_bearer"
		}
		return "authorization_header"
	}

	if tokenSource == "cluster_cookie" {
		return "cluster_cookie"
	}

	switch {
	case hasClientCertificate(ctx):
		return "client_certificate"
	case hasServiceAccountToken(ctx):
		return "serviceaccount"
	case ctx != nil && ctx.AuthType() == "oidc":
		return "oidc_kubeconfig"
	case hasKubeconfigToken(ctx):
		return "kubeconfig_token"
	default:
		return "none"
	}
}

func principalFromContext(ctx *kubeconfig.Context) string {
	if ctx == nil || ctx.AuthInfo == nil {
		return ""
	}

	return firstNonEmpty(
		ctx.AuthInfo.Username,
		ctx.AuthInfo.Impersonate,
	)
}

func hasContextIdentityMetadata(ctx *kubeconfig.Context) bool {
	return hasClientCertificate(ctx) || hasServiceAccountToken(ctx) || hasKubeconfigToken(ctx) || principalFromContext(ctx) != "" || (ctx != nil && ctx.AuthType() == "oidc")
}

func hasServiceAccountToken(ctx *kubeconfig.Context) bool {
	if ctx == nil || ctx.AuthInfo == nil {
		return false
	}

	return ctx.AuthInfo.TokenFile != ""
}

func hasKubeconfigToken(ctx *kubeconfig.Context) bool {
	if ctx == nil || ctx.AuthInfo == nil {
		return false
	}

	return ctx.AuthInfo.Token != "" || ctx.AuthInfo.TokenFile != ""
}

func hasClientCertificate(ctx *kubeconfig.Context) bool {
	if ctx == nil || ctx.AuthInfo == nil {
		return false
	}

	return ctx.AuthInfo.ClientCertificate != "" || len(ctx.AuthInfo.ClientCertificateData) > 0
}

func firstStringClaim(payload map[string]interface{}, keys ...string) string {
	for _, key := range keys {
		value, ok := payload[key]
		if !ok {
			continue
		}

		if str, ok := value.(string); ok && str != "" {
			return str
		}
	}

	return ""
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}

	return ""
}
