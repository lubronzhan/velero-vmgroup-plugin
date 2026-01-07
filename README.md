# Velero VM Restore Plugin

A Velero restore plugin for VMware VM Operator VirtualMachine and VirtualMachineGroup custom resources.

## Overview

This plugin extends Velero to properly restore VM Operator resources by:

1. **Ensuring correct restore order**: VirtualMachineGroup is restored before VirtualMachines
2. **Automatic resource cleanup**: Removes cluster-specific fields during restore

## Features

- **Ensures correct restore order**: VirtualMachineGroup is restored before VirtualMachines
- **Automatic resource cleanup**: Removes cluster-specific fields during restore
  - Removes `instanceUUID` from VirtualMachines
  - Removes `first-boot-done` annotation from VirtualMachines
  - Removes `volumehealth` annotation from PVCs
- Supports VM Operator API v1alpha5
- Works with standard Velero restore workflows
- Type-safe implementation using VM Operator API types

## Prerequisites

- Velero 1.15+ installed in your cluster
- VM Operator installed (vSphere with Tanzu or standalone)
- VirtualMachineGroup CRD (`virtualmachinegroups.vmoperator.vmware.com`) available
- Docker or compatible container runtime for building the plugin image

## Building the Plugin

### Build the binary

```bash
make build
```

### Build the container image

```bash
make container
```

Or with custom image name and version:

```bash
REGISTRY=your-registry
IMAGE=${REGISTRY}/velero-vmgroup-plugin VERSION=v1.0.0 make container
```

### Push to registry

```bash
make push
```

Or with custom settings:

```bash
IMAGE=${REGISTRY}/velero-vmgroup-plugin VERSION=v1.0.0 make push
```

## Installation

### 1. Deploy the plugin to Velero

```bash
velero plugin add ${REGISTRY}/velero-vmgroup-plugin:latest
```


### 2. Verify the plugin is installed

```bash
velero plugin get
```

You should see output similar to:

```
NAME                                    KIND
lubronzhan.io/vm-restore               RestoreItemAction
lubronzhan.io/pvc-restore              RestoreItemAction
```

## Usage

Once the plugin is installed, it will automatically be invoked when restoring VirtualMachine and PVC resources.

### Restore

Restoring works with standard Velero restore commands:

```bash
velero restore create --from-backup my-backup
```

**Restore Order**: The plugin ensures resources are restored in the correct order:
1. Namespace
2. Secrets
3. PVCs - **cluster-specific annotations removed**
4. **VirtualMachineGroup** (restored first)
5. **VirtualMachines** (wait for VMGroup to be ready) - **cluster-specific fields removed**

This ordering is automatically enforced by the restore plugin - no manual intervention needed!

**Resource Cleanup**: The plugin automatically removes cluster-specific fields during restore:
- VirtualMachines: `instanceUUID`, `first-boot-done` annotation
- PVCs: `volumehealth` annotation

This is equivalent to using Velero's resource modifiers ConfigMap, but implemented in code for better type safety and logging.

## Architecture

The plugin implements Velero Restore Item Action interfaces:

#### VM Restore Plugin (`vmgroup_restore.go`)

1. Watches for `virtualmachines.vmoperator.vmware.com` resources during restore
2. **Removes cluster-specific fields**:
   - `spec.instanceUUID` (will be regenerated)
   - `metadata.annotations["virtualmachine.vmoperator.vmware.com/first-boot-done"]` (VM should go through first boot again)
3. Checks if VM belongs to a VirtualMachineGroup (via `spec.groupName`)
4. If yes, adds the VirtualMachineGroup as an additional item to restore first
5. Sets `WaitForAdditionalItems = true` to ensure Velero waits for VMGroup
6. This ensures VirtualMachineGroup is always created before VirtualMachines

#### PVC Restore Plugin (`pvc_restore.go`)

1. Watches for `persistentvolumeclaims` resources during restore
2. **Removes cluster-specific annotations**:
   - `metadata.annotations["volumehealth.storage.kubernetes.io/health"]` (will be regenerated)

### Type Safety

The plugin uses VM Operator API types directly instead of unstructured objects:
- `vmopv1.VirtualMachine` for VMs
- `corev1.PersistentVolumeClaim` for PVCs
- Direct field access with compile-time type checking
- No manual type assertions or nested map traversals

## Development

### Project Structure

```
.
├── Dockerfile                          # Container image definition
├── Makefile                            # Build automation
├── README.md                           # This file
├── go.mod                              # Go module definition
├── go.sum                              # Go module checksums
├── main.go                             # Plugin entry point
└── pkg/
    └── plugin/
        ├── vmgroup_restore.go          # VM restore plugin
        └── pvc_restore.go              # PVC restore plugin
```

### Testing

To test the plugin locally:

1. Build and push the plugin image
2. Install it in your Velero deployment
3. Create a backup with VirtualMachines and PVCs
4. Run a Velero restore
5. Verify resources are restored in correct order and cluster-specific fields are removed

```bash
# Create a restore
velero restore create test-restore --from-backup test-backup

# Check restore status
velero restore describe test-restore --details

# Verify logs
velero restore logs test-restore | grep -E "(Removing|Processing)"

# Check resources
kubectl get vmgroup,vm,pvc -n test-ns
```

## API References

- [VM Operator API](https://github.com/vmware-tanzu/vm-operator/tree/main/api)
- [Velero Plugin Documentation](https://velero.io/docs/main/custom-plugins/)
- [VM Operator Documentation](https://vm-operator.readthedocs.io/)

## Troubleshooting

### Plugin not being invoked

Check that the plugin is properly registered:

```bash
velero plugin get
```

Check Velero logs for errors:

```bash
kubectl logs -n velero deployment/velero
```

### Resources not being restored correctly

Enable debug logging in Velero:

```bash
kubectl edit deployment velero -n velero
# Add --log-level=debug to the args
```

Check the restore details:

```bash
velero restore describe <restore-name> --details
velero restore logs <restore-name>
```

### API version mismatch

If you're using a different version of VM Operator, update the API version in `pkg/plugin/vmgroup_restore.go`:

```go
import vmopv1 "github.com/vmware-tanzu/vm-operator/api/v1alpha5"
// Change v1alpha5 to match your VM Operator version
```

## Contributing

Contributions are welcome! Please feel free to submit issues or pull requests.

## License

Copyright 2026 the Velero contributors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.

## References

- [Velero Documentation](https://velero.io/docs/)
- [VM Operator GitHub Repository](https://github.com/vmware-tanzu/vm-operator)
- [Velero Plugin Example](https://github.com/vmware-tanzu/velero-plugin-example)
