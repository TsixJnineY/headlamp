package audit

import (
	"net/http"

	auth "github.com/kubernetes-sigs/headlamp/backend/pkg/auth"
	"github.com/kubernetes-sigs/headlamp/backend/pkg/kubeconfig"
)

type ContextGetter func(string) (*kubeconfig.Context, error)

func EnrichFromRequest(r *http.Request, event *Event, config Config, contextGetter ContextGetter) bool {
	if event == nil || event.Cluster == "" {
		return false
	}

	identity, err := auth.ResolveRequestIdentity(r, event.Cluster, auth.ResolveOptions{
		UsernamePaths:     config.UsernamePaths,
		EmailPaths:        config.EmailPaths,
		GroupsPaths:       config.GroupsPaths,
		AllowPartial:      config.AllowPartialUser,
		RequireCredential: true,
		ContextGetter:     contextGetter,
	})
	if err != nil {
		return false
	}

	event.User = firstNonEmpty(identity.Email, identity.Principal, identity.Username, identity.Sub)
	event.Groups = identity.Groups
	if event.Groups == nil {
		event.Groups = []string{}
	}
	event.AuthSource = identity.AuthSource

	return true
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}

	return ""
}
