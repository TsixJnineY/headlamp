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

import { describe, expect, it } from 'vitest';
import { buildEditAuditDetails } from './editDiff';

describe('buildEditAuditDetails', () => {
  it('records whitelisted before and after changes', () => {
    const before = {
      metadata: {
        labels: { app: 'api' },
        annotations: {
          owner: 'platform',
          'kubectl.kubernetes.io/last-applied-configuration': '{"large":"ignored"}',
        },
      },
      spec: {
        replicas: 1,
        template: {
          metadata: {
            labels: { tier: 'backend' },
          },
          spec: {
            containers: [
              {
                name: 'api',
                image: 'nginx:1.25',
                env: [{ name: 'TOKEN', value: 'old-secret' }],
                resources: {
                  requests: { cpu: '100m' },
                },
              },
            ],
          },
        },
      },
    };
    const after = {
      metadata: {
        labels: { app: 'api', team: 'sre' },
        annotations: {
          owner: 'app',
          'kubectl.kubernetes.io/last-applied-configuration': '{"large":"ignored-new"}',
        },
      },
      spec: {
        replicas: 2,
        template: {
          metadata: {
            labels: { tier: 'api' },
          },
          spec: {
            containers: [
              {
                name: 'api',
                image: 'nginx:1.26',
                env: [{ name: 'TOKEN', value: 'new-secret' }],
                resources: {
                  requests: { cpu: '200m' },
                },
              },
            ],
          },
        },
      },
    };

    expect(buildEditAuditDetails(before, after)).toEqual({
      changes: [
        { path_values: 'spec.replicas', before_values: 1, after_values: 2 },
        { path_values: 'metadata.labels.team', before_values: undefined, after_values: 'sre' },
        {
          path_values: 'metadata.annotations.owner',
          before_values: 'platform',
          after_values: 'app',
        },
        {
          path_values: 'spec.template.metadata.labels.tier',
          before_values: 'backend',
          after_values: 'api',
        },
        {
          path_values: 'spec.template.spec.containers[name=api].image',
          before_values: 'nginx:1.25',
          after_values: 'nginx:1.26',
        },
        {
          path_values: 'spec.template.spec.containers[name=api].resources.requests.cpu',
          before_values: '100m',
          after_values: '200m',
        },
        {
          path_values: 'spec.template.spec.containers[name=api].env[name=TOKEN]',
          before_values: '<redacted>',
          after_values: '<redacted>',
        },
      ],
    });
  });

  it('redacts env values while still showing changed keys', () => {
    const before = {
      spec: {
        template: {
          spec: {
            containers: [{ name: 'api', env: [{ name: 'PASSWORD', value: 'old' }] }],
          },
        },
      },
    };
    const after = {
      spec: {
        template: {
          spec: {
            containers: [
              { name: 'api', env: [{ name: 'PASSWORD', valueFrom: { secretKeyRef: {} } }] },
            ],
          },
        },
      },
    };

    expect(buildEditAuditDetails(before, after)).toEqual({
      changes: [
        {
          path_values: 'spec.template.spec.containers[name=api].env[name=PASSWORD]',
          before_values: '<redacted>',
          after_values: '<valueFrom>',
        },
      ],
    });
  });
});
