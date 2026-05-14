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

type Container = {
  name?: string;
  image?: string;
  env?: EnvVar[];
  resources?: {
    requests?: Record<string, unknown>;
    limits?: Record<string, unknown>;
  };
};

type EnvVar = { name?: string; value?: string; valueFrom?: unknown };

export interface AuditChange {
  path_values: string;
  before_values?: unknown;
  after_values?: unknown;
}

const REDACTED = '<redacted>';
const IGNORED_ANNOTATIONS = new Set([
  'kubectl.kubernetes.io/last-applied-configuration',
  'deployment.kubernetes.io/revision',
]);

export function buildEditAuditDetails(
  previousResource: unknown,
  desiredResource: unknown
): { changes: AuditChange[] } {
  const before = unwrapKubeObject(previousResource);
  const after = unwrapKubeObject(desiredResource);
  const changes: AuditChange[] = [];

  addScalarChange(
    changes,
    'spec.replicas',
    getPath(before, ['spec', 'replicas']),
    getPath(after, ['spec', 'replicas'])
  );

  addMapChanges(
    changes,
    'metadata.labels',
    getPath(before, ['metadata', 'labels']),
    getPath(after, ['metadata', 'labels'])
  );
  addMapChanges(
    changes,
    'metadata.annotations',
    filterAnnotations(getPath(before, ['metadata', 'annotations'])),
    filterAnnotations(getPath(after, ['metadata', 'annotations']))
  );
  addMapChanges(
    changes,
    'spec.template.metadata.labels',
    getPath(before, ['spec', 'template', 'metadata', 'labels']),
    getPath(after, ['spec', 'template', 'metadata', 'labels'])
  );
  addMapChanges(
    changes,
    'spec.template.metadata.annotations',
    filterAnnotations(getPath(before, ['spec', 'template', 'metadata', 'annotations'])),
    filterAnnotations(getPath(after, ['spec', 'template', 'metadata', 'annotations']))
  );

  addContainerChanges(
    changes,
    'spec.template.spec.containers',
    getPath(before, ['spec', 'template', 'spec', 'containers']),
    getPath(after, ['spec', 'template', 'spec', 'containers'])
  );
  addContainerChanges(
    changes,
    'spec.template.spec.initContainers',
    getPath(before, ['spec', 'template', 'spec', 'initContainers']),
    getPath(after, ['spec', 'template', 'spec', 'initContainers'])
  );

  return { changes };
}

function unwrapKubeObject(resource: unknown): Record<string, any> {
  if (!resource || typeof resource !== 'object') {
    return {};
  }

  return ((resource as any).jsonData || resource) as Record<string, any>;
}

function getPath(resource: Record<string, any>, path: string[]): unknown {
  return path.reduce<unknown>((value, key) => {
    if (!value || typeof value !== 'object') {
      return undefined;
    }

    return (value as Record<string, unknown>)[key];
  }, resource);
}

function addScalarChange(changes: AuditChange[], path: string, before: unknown, after: unknown) {
  if (isEqual(before, after)) {
    return;
  }

  changes.push({ path_values: path, before_values: before, after_values: after });
}

function addMapChanges(
  changes: AuditChange[],
  basePath: string,
  beforeValue: unknown,
  afterValue: unknown
) {
  const before = isRecord(beforeValue) ? beforeValue : {};
  const after = isRecord(afterValue) ? afterValue : {};
  const keys = new Set([...Object.keys(before), ...Object.keys(after)]);

  for (const key of Array.from(keys).sort()) {
    addScalarChange(changes, `${basePath}.${key}`, before[key], after[key]);
  }
}

function addContainerChanges(
  changes: AuditChange[],
  basePath: string,
  beforeValue: unknown,
  afterValue: unknown
) {
  const before = containersByName(beforeValue);
  const after = containersByName(afterValue);
  const names = new Set([...Object.keys(before), ...Object.keys(after)]);

  for (const name of Array.from(names).sort()) {
    const containerPath = `${basePath}[name=${name}]`;
    const beforeContainer = before[name] || {};
    const afterContainer = after[name] || {};

    addScalarChange(changes, `${containerPath}.image`, beforeContainer.image, afterContainer.image);
    addMapChanges(
      changes,
      `${containerPath}.resources.requests`,
      beforeContainer.resources?.requests,
      afterContainer.resources?.requests
    );
    addMapChanges(
      changes,
      `${containerPath}.resources.limits`,
      beforeContainer.resources?.limits,
      afterContainer.resources?.limits
    );
    addEnvChanges(changes, `${containerPath}.env`, beforeContainer.env, afterContainer.env);
  }
}

function addEnvChanges(
  changes: AuditChange[],
  basePath: string,
  beforeValue?: Container['env'],
  afterValue?: Container['env']
) {
  const before = envByName(beforeValue);
  const after = envByName(afterValue);
  const names = new Set([...Object.keys(before), ...Object.keys(after)]);

  for (const name of Array.from(names).sort()) {
    if (isEqual(before[name], after[name])) {
      continue;
    }

    changes.push({
      path_values: `${basePath}[name=${name}]`,
      before_values: sanitizeEnvValue(before[name]),
      after_values: sanitizeEnvValue(after[name]),
    });
  }
}

function containersByName(value: unknown): Record<string, Container> {
  if (!Array.isArray(value)) {
    return {};
  }

  return value.reduce<Record<string, Container>>((result, container, index) => {
    const name = container?.name || `index-${index}`;
    result[name] = container;
    return result;
  }, {});
}

function envByName(value: Container['env']): Record<string, EnvVar> {
  if (!Array.isArray(value)) {
    return {};
  }

  return value.reduce<Record<string, EnvVar>>((result, env, index) => {
    const name = env?.name || `index-${index}`;
    result[name] = env;
    return result;
  }, {});
}

function sanitizeEnvValue(value?: EnvVar): string | undefined {
  if (!value) {
    return undefined;
  }

  if ('valueFrom' in value && value.valueFrom) {
    return '<valueFrom>';
  }

  if ('value' in value) {
    return REDACTED;
  }

  return '<set>';
}

function filterAnnotations(value: unknown): Record<string, unknown> {
  if (!isRecord(value)) {
    return {};
  }

  return Object.fromEntries(Object.entries(value).filter(([key]) => !IGNORED_ANNOTATIONS.has(key)));
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return !!value && typeof value === 'object' && !Array.isArray(value);
}

function isEqual(before: unknown, after: unknown): boolean {
  return JSON.stringify(before) === JSON.stringify(after);
}
