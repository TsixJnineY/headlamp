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

import { getCluster } from '../../lib/cluster';
import {
  addEventCallback,
  EventStatus,
  type HeadlampEvent,
  HeadlampEventType,
} from '../../redux/headlampEventSlice';
import type { AppStore } from '../../redux/stores/store';
import { emitAuditEvent } from './emitter';
import { toAuditResource } from './resourceAudit';

let isRegistered = false;

export function registerAuditEventBridge(store: AppStore) {
  if (isRegistered) {
    return;
  }

  isRegistered = true;

  store.dispatch(
    addEventCallback((event: HeadlampEvent) => {
      void handleHeadlampEvent(store, event);
    })
  );
}

async function handleHeadlampEvent(store: AppStore, event: HeadlampEvent) {
  const data = (event?.data ?? {}) as any;
  const resource = data?.resource;
  const resources = Array.isArray(data?.resources) ? data.resources : [];
  const auditResources = resources.map((item: any) => toAuditResource(item)).filter(Boolean);
  const hasConcreteResource = Boolean(resource) || resources.length > 0;
  const cluster =
    data?.cluster || resource?.cluster || resources[0]?.cluster || getCluster() || undefined;
  const namespace =
    resource?.metadata?.namespace ||
    resources[0]?.metadata?.namespace ||
    (!hasConcreteResource ? getCurrentNamespace(store) : undefined);

  switch (event.type) {
    case HeadlampEventType.LOGS:
      await emitAuditEvent({
        source: 'headlamp',
        event_type: 'ui_action',
        action: data?.status === EventStatus.CLOSED ? 'close_logs' : 'open_logs',
        cluster,
        namespace,
        resource: toAuditResource(resource),
      });
      break;
    case HeadlampEventType.TERMINAL:
      await emitAuditEvent({
        source: 'headlamp',
        event_type: 'ui_action',
        action: data?.status === EventStatus.CLOSED ? 'close_exec_terminal' : 'open_exec_terminal',
        cluster,
        namespace,
        session_id: data?.session_id,
        resource: toAuditResource(resource),
      });
      break;
    case HeadlampEventType.POD_ATTACH:
      await emitAuditEvent({
        source: 'headlamp',
        event_type: 'ui_action',
        action:
          data?.status === EventStatus.CLOSED ? 'close_attach_terminal' : 'open_attach_terminal',
        cluster,
        namespace,
        session_id: data?.session_id,
        resource: toAuditResource(resource),
      });
      break;
    case HeadlampEventType.CREATE_RESOURCE:
      await emitAuditEvent({
        source: 'headlamp',
        event_type: 'ui_action',
        action: 'create_resource',
        cluster,
        namespace,
        resource: resource ? toAuditResource(resource) : auditResources[0],
        result: data?.status,
        extra:
          auditResources.length > 1
            ? {
                resources: auditResources,
              }
            : undefined,
      });
      break;
    case HeadlampEventType.DELETE_RESOURCE:
      await emitAuditEvent({
        source: 'headlamp',
        event_type: 'ui_action',
        action: 'delete_resource',
        cluster,
        namespace,
        resource: toAuditResource(resource),
        result: data?.status,
      });
      break;
    case HeadlampEventType.DELETE_RESOURCES:
      await emitAuditEvent({
        source: 'headlamp',
        event_type: 'ui_action',
        action: 'delete_resources',
        cluster,
        namespace,
        result: data?.status,
        extra: {
          resources: auditResources,
        },
      });
      break;
    case HeadlampEventType.EDIT_RESOURCE:
      await emitAuditEvent({
        source: 'headlamp',
        event_type: 'ui_action',
        action:
          data?.status === EventStatus.CONFIRMED
            ? 'save_edit_resource'
            : data?.status === EventStatus.CLOSED
            ? 'close_edit_resource'
            : 'open_edit_resource',
        cluster,
        namespace,
        resource: toAuditResource(resource),
        result: data?.status,
        details: data?.details,
      });
      break;
    case HeadlampEventType.SCALE_RESOURCE:
      await emitAuditEvent({
        source: 'headlamp',
        event_type: 'ui_action',
        action: 'scale_resource',
        cluster,
        namespace,
        resource: toAuditResource(resource),
        result: data?.status,
        extra: {
          previous_replicas: data?.previousReplicas,
          desired_replicas: data?.desiredReplicas,
        },
      });
      break;
    case HeadlampEventType.RESTART_RESOURCE:
      await emitAuditEvent({
        source: 'headlamp',
        event_type: 'ui_action',
        action: 'restart_resource',
        cluster,
        namespace,
        resource: toAuditResource(resource),
        result: data?.status,
      });
      break;
    case HeadlampEventType.RESTART_RESOURCES:
      await emitAuditEvent({
        source: 'headlamp',
        event_type: 'ui_action',
        action: 'restart_resources',
        cluster,
        namespace,
        result: data?.status,
        extra: {
          resources: auditResources,
        },
      });
      break;
    case HeadlampEventType.ROLLBACK_RESOURCE:
      await emitAuditEvent({
        source: 'headlamp',
        event_type: 'ui_action',
        action: 'rollback_resource',
        cluster,
        namespace,
        resource: toAuditResource(resource),
        result: data?.status,
      });
      break;
    case HeadlampEventType.DETAILS_VIEW:
      await emitAuditEvent({
        source: 'headlamp',
        event_type: 'ui_action',
        action: 'details_view',
        cluster,
        namespace,
        resource: toAuditResource(resource),
      });
      break;
    case HeadlampEventType.LIST_VIEW:
      await emitAuditEvent({
        source: 'headlamp',
        event_type: 'ui_action',
        action: 'list_view',
        cluster,
        namespace,
        extra: {
          resourceKind: data?.resourceKind,
          resourceCount: Array.isArray(data?.resources) ? data.resources.length : undefined,
        },
      });
      break;
    default:
      break;
  }
}

function getCurrentNamespace(store: AppStore): string | undefined {
  const namespaces = store.getState().filter.namespaces;
  if (namespaces.size === 1) {
    return [...namespaces][0];
  }

  return undefined;
}
