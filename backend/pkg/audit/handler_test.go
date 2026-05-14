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
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
)

type recordingEmitter struct {
	events []Event
}

func (e *recordingEmitter) Emit(event Event) error {
	e.events = append(e.events, event)
	return nil
}

func TestHandlerRejectsMissingCredential(t *testing.T) {
	t.Parallel()

	emitter := &recordingEmitter{}
	handler := NewHandler(Config{
		Enabled:          true,
		LogUIActions:     true,
		LogTerminalInput: true,
		AllowPartialUser: true,
	}, emitter, nil, nil)

	req := httptest.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"/clusters/test/audit/events",
		strings.NewReader(`{"event_type":"ui_action","action":"list_view"}`),
	)
	req = mux.SetURLVars(req, map[string]string{"clusterName": "test"})
	res := httptest.NewRecorder()

	handler.ServeHTTP(res, req)

	assert.Equal(t, http.StatusUnauthorized, res.Code)
	assert.Empty(t, emitter.events)
}

func TestHandlerRejectsUnknownEventType(t *testing.T) {
	t.Parallel()

	emitter := &recordingEmitter{}
	handler := NewHandler(Config{
		Enabled:          true,
		LogUIActions:     true,
		LogTerminalInput: true,
		AllowPartialUser: true,
	}, emitter, nil, nil)

	req := httptest.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"/clusters/test/audit/events",
		strings.NewReader(`{"event_type":"not_audit","action":"anything"}`),
	)
	req = mux.SetURLVars(req, map[string]string{"clusterName": "test"})
	res := httptest.NewRecorder()

	handler.ServeHTTP(res, req)

	assert.Equal(t, http.StatusBadRequest, res.Code)
	assert.Empty(t, emitter.events)
}

func TestHandlerSkipsDisabledKnownEventType(t *testing.T) {
	t.Parallel()

	emitter := &recordingEmitter{}
	handler := NewHandler(Config{
		Enabled:          true,
		LogUIActions:     true,
		LogTerminalInput: false,
		AllowPartialUser: true,
	}, emitter, nil, nil)

	req := httptest.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"/clusters/test/audit/events",
		strings.NewReader(`{"event_type":"terminal_input","command":"ls"}`),
	)
	req = mux.SetURLVars(req, map[string]string{"clusterName": "test"})
	res := httptest.NewRecorder()

	handler.ServeHTTP(res, req)

	assert.Equal(t, http.StatusNoContent, res.Code)
	assert.Empty(t, emitter.events)
}
