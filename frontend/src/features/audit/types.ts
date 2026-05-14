/*
 * Copyright 2025 The Kubernetes Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

export interface AuditResourceRef {
  kind?: string;
  name?: string;
  cluster?: string;
  namespace?: string;
  container?: string;
}

export interface AuditUserRef {
  sub?: string;
  email?: string;
  groups?: string[];
}

export interface AuditEventPayload {
  source?: 'headlamp';
  event_type: 'ui_action' | 'terminal_input' | 'portforward' | string;
  action?: string;
  result?: string;
  cluster?: string;
  namespace?: string;
  session_id?: string;
  pod?: string;
  container?: string;
  command?: string;
  res_kind?: string;
  res_name?: string;
  res_namespace?: string;
  res_container?: string;
  resource?: AuditResourceRef;
  user?: AuditUserRef;
  extra?: Record<string, unknown>;
  details?: Record<string, unknown>;
}
