# rancher-fip-manager-webhook

The rancher-fip-manager-webhook is a webhook service for the rancher-fip-manager which validates FloatingIP custom resources (CRs) against project quotas and IP availability in FloatingIPPools.

## Prerequisites

The following components need to be installed/configured to use the rancher-fip-manager-webhook:

* rancher-fip-manager
* Kubernetes cluster with:
  - FloatingIPPool CRD (cluster-scoped)
  - FloatingIPProjectQuota CRD (cluster-scoped)
  - FloatingIP CRD (namespace-scoped)
  - Validating webhook configuration permissions

## Validation Rules

The webhook validates FloatingIP CRs against:
1. **Pool existence**: Checks if requested FloatingIPPool exists
2. **IP availability**: Verifies requested IP is not already allocated
3. **Quota enforcement**: Ensures project quota isn't exceeded

## Building the container

There is a Dockerfile in the current directory which can be used to build the container, for example:

```SH
[docker|podman] build -t <DOCKER_REGISTRY_URI>/rancher-fip-manager-webhook:latest .
```

Then push it to the remote container registry target, for example:

```SH
[docker|podman] push <DOCKER_REGISTRY_URI>/rancher-fip-manager-webhook:latest
```

## Deploying the container

Use the deployment.yaml manifest which is located in the deployments directory, for example:

```SH
kubectl create -f deployments/deployment.yaml
```

### Configuration

**Environment Variables:**
- `CERTRENEWALPERIOD`: Certificate renewal period in minutes (default: 43200/30 days)
- `LOGLEVEL`: Logging level (INFO, DEBUG, TRACE)
- `KUBECONFIG`: Kubeconfig file path (optional, defaults to in-cluster config)
- `KUBECONTEXT`: Kubeconfig context (optional)

### Logging

By default only the startup, error and warning logs are enabled. More logging can be enabled by changing the LOGLEVEL environment setting in the rancher-fip-manager-webhook deployment. The supported loglevels are INFO, DEBUG and TRACE.

# License

Copyright (c) 2026 Joey Loman <joey@binbash.org>

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

[http://www.apache.org/licenses/LICENSE-2.0](http://www.apache.org/licenses/LICENSE-2.0)

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
