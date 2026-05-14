/*
Copyright 2025 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package audit

import "time"

type ResourceRef struct {
	Kind      string `json:"kind,omitempty"`
	Name      string `json:"name,omitempty"`
	Cluster   string `json:"cluster,omitempty"`
	Namespace string `json:"namespace,omitempty"`
	Container string `json:"container,omitempty"`
}

type Event struct {
	Timestamp    string                 `json:"ts"`
	Level        string                 `json:"level"`
	Source       string                 `json:"source"`
	Cluster      string                 `json:"cluster"`
	EventType    string                 `json:"event_type"`
	Action       string                 `json:"action"`
	User         string                 `json:"user"`
	Groups       []string               `json:"groups"`
	ResKind      string                 `json:"res_kind"`
	ResName      string                 `json:"res_name"`
	ResNamespace string                 `json:"res_namespace"`
	ResContainer string                 `json:"res_container"`
	SessionID    string                 `json:"session_id"`
	Command      string                 `json:"command"`
	Result       string                 `json:"result"`
	AuthSource   string                 `json:"auth_source"`
	Details      map[string]interface{} `json:"details"`
}

type FrontendEvent struct {
	Source       string                 `json:"source,omitempty"`
	EventType    string                 `json:"event_type"`
	Action       string                 `json:"action,omitempty"`
	Result       string                 `json:"result,omitempty"`
	Cluster      string                 `json:"cluster,omitempty"`
	Namespace    string                 `json:"namespace,omitempty"`
	SessionID    string                 `json:"session_id,omitempty"`
	Pod          string                 `json:"pod,omitempty"`
	Container    string                 `json:"container,omitempty"`
	Command      string                 `json:"command,omitempty"`
	Input        string                 `json:"input,omitempty"`
	ResKind      string                 `json:"res_kind,omitempty"`
	ResName      string                 `json:"res_name,omitempty"`
	ResNamespace string                 `json:"res_namespace,omitempty"`
	ResContainer string                 `json:"res_container,omitempty"`
	Resource     *ResourceRef           `json:"resource,omitempty"`
	Extra        map[string]interface{} `json:"extra,omitempty"`
	Details      map[string]interface{} `json:"details,omitempty"`
}

func NewEvent(base FrontendEvent) Event {
	details := normalizeDetails(coalesceMap(base.Details, base.Extra))
	resource := base.Resource
	action := coalesce(base.Action, stringFromMap(details, "mode"))
	if base.Action == "" && action != "" {
		delete(details, "mode")
	}
	delete(details, "input_interpreter")
	command := coalesce(base.Command, base.Input)
	resKind := base.ResKind
	resName := base.ResName
	resNamespace := coalesce(base.ResNamespace, base.Namespace)
	resContainer := coalesce(base.ResContainer, base.Container)

	if resource != nil {
		resKind = coalesce(resKind, resource.Kind)
		resName = coalesce(resName, resource.Name)
		resNamespace = coalesce(resNamespace, resource.Namespace)
		resContainer = coalesce(resContainer, resource.Container)
	}

	if resName == "" && base.Pod != "" {
		resName = base.Pod
		resKind = coalesce(resKind, "Pod")
	}

	return Event{
		Timestamp:    time.Now().UTC().Format(time.RFC3339),
		Level:        "info",
		Source:       coalesce(base.Source, "headlamp"),
		Cluster:      base.Cluster,
		EventType:    base.EventType,
		Action:       action,
		Groups:       []string{},
		ResKind:      resKind,
		ResName:      resName,
		ResNamespace: resNamespace,
		ResContainer: resContainer,
		SessionID:    base.SessionID,
		Command:      command,
		Result:       base.Result,
		Details:      details,
	}
}

func coalesce(value string, fallback string) string {
	if value == "" {
		return fallback
	}

	return value
}

func coalesceMap(value, fallback map[string]interface{}) map[string]interface{} {
	if len(value) == 0 {
		return fallback
	}

	return value
}

func stringFromMap(values map[string]interface{}, key string) string {
	value, ok := values[key]
	if !ok {
		return ""
	}

	if str, ok := value.(string); ok {
		return str
	}

	return ""
}

func normalizeDetails(details map[string]interface{}) map[string]interface{} {
	if len(details) == 0 {
		return map[string]interface{}{}
	}

	normalized := make(map[string]interface{}, len(details))
	for key, value := range details {
		switch key {
		case "previous_replicas":
			normalized["replicas_before"] = value
		case "desired_replicas":
			normalized["replicas_after"] = value
		case "resources":
			normalized[key] = normalizeDetailResources(value)
		default:
			normalized[key] = normalizeDetailResourceValue(value)
		}
	}

	return normalized
}

func normalizeDetailResources(value interface{}) interface{} {
	items, ok := value.([]ResourceRef)
	if ok {
		resources := make([]map[string]interface{}, 0, len(items))
		for _, item := range items {
			resources = append(resources, flatResourceDetails(item))
		}
		return resources
	}

	values, ok := value.([]interface{})
	if !ok {
		return normalizeDetailResourceValue(value)
	}

	resources := make([]interface{}, 0, len(values))
	for _, item := range values {
		resources = append(resources, normalizeDetailResourceValue(item))
	}
	return resources
}

func normalizeDetailResourceValue(value interface{}) interface{} {
	switch typed := value.(type) {
	case ResourceRef:
		return flatResourceDetails(typed)
	case *ResourceRef:
		if typed == nil {
			return value
		}
		return flatResourceDetails(*typed)
	case map[string]interface{}:
		if looksLikeResourceMap(typed) {
			return flatResourceMapDetails(typed)
		}
		return typed
	default:
		return value
	}
}

func looksLikeResourceMap(value map[string]interface{}) bool {
	_, hasKind := value["kind"]
	_, hasName := value["name"]
	_, hasNamespace := value["namespace"]
	_, hasContainer := value["container"]
	return hasKind || hasName || hasNamespace || hasContainer
}

func flatResourceDetails(resource ResourceRef) map[string]interface{} {
	return map[string]interface{}{
		"res_kind":      resource.Kind,
		"res_name":      resource.Name,
		"res_namespace": resource.Namespace,
		"res_container": resource.Container,
	}
}

func flatResourceMapDetails(resource map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{
		"res_kind":      resource["kind"],
		"res_name":      resource["name"],
		"res_namespace": resource["namespace"],
		"res_container": resource["container"],
	}
}
