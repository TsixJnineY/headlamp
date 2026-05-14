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

import { emitAuditEvent } from './emitter';
import type { AuditEventPayload } from './types';

export interface TerminalAuditInput {
  cluster?: string;
  namespace?: string;
  pod?: string;
  container?: string;
  sessionId: string;
  command: string;
  action?: 'exec' | 'attach' | 'pod-debug' | 'node-shell' | string;
  resKind?: string;
  resName?: string;
  resNamespace?: string;
  resContainer?: string;
  details?: Record<string, unknown>;
}

export interface TerminalInputCollector {
  record: (data: string) => void;
  flush: () => void;
}

export function emitTerminalInputAudit(event: TerminalAuditInput): Promise<void> {
  const payload: AuditEventPayload = {
    source: 'headlamp',
    event_type: 'terminal_input',
    action: event.action,
    cluster: event.cluster,
    res_kind: event.resKind || (event.pod ? 'Pod' : undefined),
    res_name: event.resName || event.pod,
    res_namespace: event.resNamespace ?? event.namespace,
    res_container: event.resContainer ?? event.container,
    session_id: event.sessionId,
    command: event.command,
    details: event.details || {},
  };

  return emitAuditEvent(payload);
}

export function createTerminalInputCollector(
  getEvent: () => Omit<TerminalAuditInput, 'command'>
): TerminalInputCollector {
  let buffer = '';
  let escapeState: 'none' | 'esc' | 'csi' = 'none';

  function emitBufferedInput() {
    const command = buffer.trim();
    buffer = '';

    if (!command) {
      return;
    }

    void emitTerminalInputAudit({
      ...getEvent(),
      command,
    });
  }

  return {
    record(data: string) {
      for (const char of data) {
        if (char === '\u001b') {
          escapeState = 'esc';
          continue;
        }

        if (escapeState === 'esc') {
          if (char === '[') {
            escapeState = 'csi';
            continue;
          }

          escapeState = 'none';
          continue;
        }

        if (escapeState === 'csi') {
          const code = char.charCodeAt(0);
          if (code >= 0x40 && code <= 0x7e) {
            escapeState = 'none';
          }
          continue;
        }

        if (char === '\r' || char === '\n') {
          emitBufferedInput();
          continue;
        }

        if (char === '\u007f' || char === '\b') {
          buffer = buffer.slice(0, -1);
          continue;
        }

        if (char === '\u0015') {
          buffer = '';
          continue;
        }

        const code = char.charCodeAt(0);
        if (char === '\t' || code >= 0x20) {
          buffer += char;
        }
      }
    },
    flush() {
      emitBufferedInput();
    },
  };
}
