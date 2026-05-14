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
	"os"
	"sync"
)

type Emitter interface {
	Emit(Event) error
}

type StdoutEmitter struct{}

var stdoutEmitterMu sync.Mutex

func (StdoutEmitter) Emit(event Event) error {
	if event.Level == "" {
		event.Level = "info"
	}
	if event.Groups == nil {
		event.Groups = []string{}
	}
	if event.Details == nil {
		event.Details = map[string]interface{}{}
	}

	payload, err := json.Marshal(event)
	if err != nil {
		return err
	}

	stdoutEmitterMu.Lock()
	defer stdoutEmitterMu.Unlock()

	_, err = os.Stdout.Write(append(payload, '\n'))
	return err
}
