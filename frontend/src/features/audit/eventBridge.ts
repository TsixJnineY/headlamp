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
    case HeadlampEventType.LOGIN:
      await emitAuditEvent({
        source: 'headlamp',
        event_type: 'ui_action',
        action: data?.details?.method === 'token' ? 'login_token' : 'login_oidc',
        cluster,
        namespace,
        result: data?.result,
        details: data?.details,
      });
      break;
    case HeadlampEventType.LOGOUT:
      await emitAuditEvent({
        source: 'headlamp',
        event_type: 'ui_action',
        action: 'logout',
        cluster,
        namespace,
        result: data?.result,
        details: data?.details,
      });
      break;
    case HeadlampEventType.SWITCH_CLUSTER:
      await emitAuditEvent({
        source: 'headlamp',
        event_type: 'ui_action',
        action: 'switch_cluster',
        cluster,
        namespace,
        details: data?.details,
      });
      break;
    case HeadlampEventType.SWITCH_NAMESPACE:
      await emitAuditEvent({
        source: 'headlamp',
        event_type: 'ui_action',
        action: 'switch_namespace',
        cluster,
        namespace: Array.isArray(data?.details?.namespaces)
          ? data.details.namespaces.join(',')
          : namespace,
        details: data?.details,
      });
      break;
    case HeadlampEventType.PORT_FORWARD:
      await emitAuditEvent({
        source: 'headlamp',
        event_type: 'ui_action',
        action: data?.status === EventStatus.CLOSED ? 'stop_port_forward' : 'start_port_forward',
        cluster,
        namespace,
        resource: toAuditResource(resource),
        result: data?.result,
        details: data?.details,
      });
      break;
    case HeadlampEventType.NAVIGATE_TO_RESOURCE:
    case HeadlampEventType.OPEN_RESOURCE_DRAWER:
      await emitAuditEvent({
        source: 'headlamp',
        event_type: 'ui_action',
        action:
          event.type === HeadlampEventType.OPEN_RESOURCE_DRAWER
            ? 'open_resource_drawer'
            : 'navigate_to_resource',
        cluster,
        namespace,
        resource: toAuditResource(resource),
        details: data?.details,
      });
      break;
    case HeadlampEventType.CORDON_NODE:
    case HeadlampEventType.UNCORDON_NODE:
    case HeadlampEventType.DRAIN_NODE:
      await emitAuditEvent({
        source: 'headlamp',
        event_type: 'ui_action',
        action:
          event.type === HeadlampEventType.DRAIN_NODE
            ? 'drain_node'
            : event.type === HeadlampEventType.UNCORDON_NODE
            ? 'uncordon_node'
            : 'cordon_node',
        cluster,
        namespace,
        resource: toAuditResource(resource),
        result: data?.result,
        details: data?.details,
      });
      break;
    case HeadlampEventType.TRIGGER_CRONJOB:
      await emitAuditEvent({
        source: 'headlamp',
        event_type: 'ui_action',
        action:
          data?.details?.desired_suspend === true
            ? 'suspend_cronjob'
            : data?.details?.desired_suspend === false
            ? 'resume_cronjob'
            : 'spawn_cronjob_job',
        cluster,
        namespace,
        resource: toAuditResource(resource),
        result: data?.result,
        details: data?.details,
      });
      break;
    case HeadlampEventType.CREATE_PROJECT:
    case HeadlampEventType.DELETE_PROJECT:
      await emitAuditEvent({
        source: 'headlamp',
        event_type: 'ui_action',
        action:
          event.type === HeadlampEventType.DELETE_PROJECT
            ? 'delete_project'
            : data?.details?.from_yaml
            ? 'create_project_from_yaml'
            : 'create_project',
        cluster,
        namespace,
        resource: toAuditResource(resource),
        result: data?.result,
        extra:
          auditResources.length > 1
            ? {
                resources: auditResources,
              }
            : undefined,
        details: data?.details,
      });
      break;
    case HeadlampEventType.POD_DEBUG_TERMINAL:
    case HeadlampEventType.NODE_SHELL_TERMINAL:
      await emitAuditEvent({
        source: 'headlamp',
        event_type: 'ui_action',
        action:
          event.type === HeadlampEventType.NODE_SHELL_TERMINAL
            ? data?.status === EventStatus.CLOSED
              ? 'close_node_shell_terminal'
              : 'open_node_shell_terminal'
            : data?.status === EventStatus.CLOSED
            ? 'close_pod_debug_terminal'
            : 'open_pod_debug_terminal',
        cluster,
        namespace,
        session_id: data?.session_id,
        resource: toAuditResource(resource),
        details: data?.details,
      });
      break;
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
    case HeadlampEventType.EDIT_RESOURCE: {
      const editAction =
        data?.details?.action ||
        (data?.status === EventStatus.CONFIRMED
          ? 'save_edit_resource'
          : data?.status === EventStatus.CLOSED
          ? 'close_edit_resource'
          : 'open_edit_resource');
      await emitAuditEvent({
        source: 'headlamp',
        event_type: 'ui_action',
        action: editAction,
        cluster,
        namespace,
        resource: toAuditResource(resource),
        result: data?.status,
        details: data?.details,
      });
      break;
    }
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
          replicas_before: data?.replicasBefore,
          replicas_after: data?.replicasAfter,
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
