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

import { getHeadlampAPIHeaders } from '../../helpers/getHeadlampAPIHeaders';
import { getCluster } from '../../lib/cluster';
import { backendFetch } from '../../lib/k8s/api/v2/fetch';
import type { AuditEventPayload } from './types';

export async function emitAuditEvent(event: AuditEventPayload): Promise<void> {
  const cluster = event.cluster || event.resource?.cluster || getCluster();
  if (!cluster) {
    console.debug('audit emit skipped: cluster is required', event);
    return;
  }

  try {
    await backendFetch(`/clusters/${encodeURIComponent(cluster)}/audit/events`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        ...getHeadlampAPIHeaders(),
      },
      body: JSON.stringify({
        ...event,
        cluster,
      }),
    });
  } catch (error) {
    console.debug('audit emit failed', error);
  }
}
