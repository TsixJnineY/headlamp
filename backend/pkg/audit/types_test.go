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

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewEventNormalizesLegacyResourceAndExtra(t *testing.T) {
	t.Parallel()

	event := NewEvent(FrontendEvent{
		Source:    "headlamp",
		EventType: "ui_action",
		Action:    "scale_resource",
		Result:    "confirmed",
		Cluster:   "test-eks",
		Resource: &ResourceRef{
			Kind:      "Deployment",
			Name:      "api",
			Namespace: "ops",
		},
		Extra: map[string]interface{}{
			"previous_replicas": 1,
			"desired_replicas":  2,
		},
	})

	assert.Equal(t, "info", event.Level)
	assert.Equal(t, "headlamp", event.Source)
	assert.Equal(t, "test-eks", event.Cluster)
	assert.Equal(t, "ui_action", event.EventType)
	assert.Equal(t, "scale_resource", event.Action)
	assert.Equal(t, "confirmed", event.Result)
	assert.Equal(t, "Deployment", event.ResKind)
	assert.Equal(t, "api", event.ResName)
	assert.Equal(t, "ops", event.ResNamespace)
	assert.Equal(t, map[string]interface{}{
		"replicas_before": 1,
		"replicas_after":  2,
	}, event.Details)
}

func TestNewEventNormalizesTerminalInput(t *testing.T) {
	t.Parallel()

	event := NewEvent(FrontendEvent{
		Source:    "headlamp",
		EventType: "terminal_input",
		Cluster:   "test-eks",
		Namespace: "ops",
		Pod:       "nginx-7f9d8c",
		Container: "app",
		SessionID: "exec-abc123",
		Command:   "ls -al /tmp",
		Extra: map[string]interface{}{
			"mode":              "exec",
			"input_interpreter": "best_effort_line_reconstruction",
		},
	})

	assert.Equal(t, "terminal_input", event.EventType)
	assert.Equal(t, "exec", event.Action)
	assert.Equal(t, "Pod", event.ResKind)
	assert.Equal(t, "nginx-7f9d8c", event.ResName)
	assert.Equal(t, "ops", event.ResNamespace)
	assert.Equal(t, "app", event.ResContainer)
	assert.Equal(t, "exec-abc123", event.SessionID)
	assert.Equal(t, "ls -al /tmp", event.Command)
	assert.NotContains(t, event.Details, "mode")
	assert.NotContains(t, event.Details, "input_interpreter")
}
