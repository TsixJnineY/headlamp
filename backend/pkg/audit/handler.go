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
	"encoding/json"
	"errors"
	"net/http"

	"github.com/gorilla/mux"
)

type ValidateFunc func(http.ResponseWriter, *http.Request, string) error

type Handler struct {
	config        Config
	emitter       Emitter
	verify        ValidateFunc
	contextGetter ContextGetter
}

func NewHandler(config Config, emitter Emitter, verify ValidateFunc, contextGetter ContextGetter) *Handler {
	if emitter == nil {
		emitter = StdoutEmitter{}
	}

	return &Handler{
		config:        config,
		emitter:       emitter,
		verify:        verify,
		contextGetter: contextGetter,
	}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if !h.config.Enabled {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	clusterName := mux.Vars(r)["clusterName"]
	if clusterName == "" {
		http.Error(w, "clusterName is required", http.StatusBadRequest)
		return
	}

	if h.verify != nil {
		if err := h.verify(w, r, clusterName); err != nil {
			return
		}
	}

	var input FrontendEvent
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, "invalid audit payload", http.StatusBadRequest)
		return
	}

	if input.EventType == "" {
		http.Error(w, "event_type is required", http.StatusBadRequest)
		return
	}

	if !IsKnownEventType(input.EventType) {
		http.Error(w, "unsupported event_type", http.StatusBadRequest)
		return
	}

	if !h.config.ShouldEmit(input.EventType) {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	input.Cluster = clusterName
	event := NewEvent(input)
	if ok := EnrichFromRequest(r, &event, h.config, h.contextGetter); !ok {
		http.Error(w, "missing cluster session", http.StatusUnauthorized)
		return
	}

	if err := h.emitter.Emit(event); err != nil {
		http.Error(w, errors.New("failed to emit audit event").Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusAccepted)
}
