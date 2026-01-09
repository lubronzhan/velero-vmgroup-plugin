# Velero VM Group Plugin

A Velero backup plugin for backing up VMware VM Operator VirtualMachineGroup custom resources along with their dependencies.

## Overview

This plugin extends Velero to properly backup VirtualMachineGroup CRs (`virtualmachinegroups.vmoperator.vmware.com`) by automatically including:

1. **VirtualMachine members** - All VMs referenced in `vmg.spec.bootOrder.members`
2. **Bootstrap secrets** - Secrets referenced by `vm.spec.bootstrap.cloudInit.rawCloudConfig.name`
3. **Persistent Volume Claims** - PVCs referenced by `vm.spec.volumes[x].persistentVolumeClaim.claimName`

## Features

- Automatically discovers and backs up VirtualMachine resources that are members of a VirtualMachineGroup
- Backs up bootstrap secrets used for cloud-init configuration
- Backs up all PVCs attached to the VirtualMachines
- **Ensures correct restore order**: VirtualMachineGroup is restored before VirtualMachines
- **Automatic resource cleanup**: Removes cluster-specific fields during restore
  - Removes `instanceUUID` from VirtualMachines
  - Removes `first-boot-done` annotation from VirtualMachines
  - Removes `volumehealth` annotation from PVCs
- Supports VM Operator API v1alpha5
- Works with standard Velero backup workflows

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
lubronzhan.io/vmgroup-backup           BackupItemAction
lubronzhan.io/vm-restore               RestoreItemAction
lubronzhan.io/pvc-restore              RestoreItemAction
```

## Usage

Once the plugin is installed, it will automatically be invoked when backing up VirtualMachineGroup resources.

### Create a backup including VirtualMachineGroups

```bash
# Backup all resources in a namespace
velero backup create my-vmgroup-backup --include-namespaces my-namespace

# Backup only VirtualMachineGroups and their dependencies
velero backup create my-vmgroup-backup \
  --include-resources virtualmachinegroups.vmoperator.vmware.com \
  --include-namespaces my-namespace
```

### Example VirtualMachineGroup

```yaml
apiVersion: vmoperator.vmware.com/v1alpha5
kind: VirtualMachineGroup
metadata:
  name: my-vm-group
  namespace: my-namespace
spec:
  bootOrder:
    members:
      - name: vm-1
      - name: vm-2
      - name: vm-3
```

When backing up this VirtualMachineGroup, the plugin will automatically include:
- The VirtualMachineGroup itself
- VirtualMachines: `vm-1`, `vm-2`, `vm-3`
- Any Secrets referenced by these VMs' cloud-init configuration
- Any PVCs attached to these VMs

### Restore

Restoring works with standard Velero restore commands:

```bash
velero restore create --from-backup my-vmgroup-backup
```

**Restore Order**: The plugin ensures resources are restored in the correct order:
1. Namespace
2. Secrets (cloud-init configs)
3. PVCs (persistent storage) - **cluster-specific annotations removed**
4. **VirtualMachineGroup** (restored first)
5. **VirtualMachines** (wait for VMGroup to be ready) - **cluster-specific fields removed**

This ordering is automatically enforced by the restore plugin - no manual intervention needed!

**Resource Cleanup**: The plugin automatically removes cluster-specific fields during restore:
- VirtualMachines: `instanceUUID`, `first-boot-done` annotation
- PVCs: `volumehealth` annotation

This is equivalent to using Velero's resource modifiers ConfigMap, but implemented in code for better type safety and logging. See [docs/RESOURCE_MODIFIERS.md](docs/RESOURCE_MODIFIERS.md) for details.

## Architecture

The plugin implements two Velero plugin interfaces:

### Backup Item Action (`vmgroup_backup.go`)

1. Watches for `virtualmachinegroups.vmoperator.vmware.com` resources during backup
2. Converts the unstructured item to typed `VirtualMachineGroup` using VM Operator API
3. Iterates through `spec.bootOrder.members` to get VirtualMachine names
4. Uses controller-runtime client to fetch each typed `VirtualMachine`
5. **Adds tracking label** `vmgroup.vmoperator.vmware.com/name` to each VM
6. Extracts Secret references directly from `vm.Spec.Bootstrap.CloudInit.RawCloudConfig.Name`
7. Extracts PVC references directly from `vm.Spec.Volumes[x].PersistentVolumeClaim.ClaimName`
8. Returns these resources as additional items to be backed up by Velero

### Restore Item Actions

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
- `vmopv1.VirtualMachineGroup` for VM groups
- `vmopv1.VirtualMachine` for VMs
- Direct field access with compile-time type checking
- No manual type assertions or nested map traversals

## Implementation

The plugin provides a complete, production-ready implementation (`pkg/plugin/vmgroup_backup.go`) that:

- ✅ Uses VM Operator API types directly for type safety
- ✅ Uses controller-runtime client to fetch VirtualMachine resources from the cluster
- ✅ Automatically extracts and backs up all dependencies:
  - Bootstrap secrets from `vm.spec.bootstrap.cloudInit.rawCloudConfig.name`
  - PVCs from `vm.spec.volumes[x].persistentVolumeClaim.claimName`
- ✅ Handles errors gracefully with detailed logging
- ✅ Works with VM Operator API v1alpha5

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
        ├── vmgroup_backup.go           # Basic plugin implementation
        └── vmgroup_backup_with_client.go  # Full plugin implementation with K8s client
```

### Testing

To test the plugin locally:

1. Build and push the plugin image
2. Install it in your Velero deployment
3. Create a test VirtualMachineGroup with VMs
4. Run a Velero backup
5. Check the backup contents to verify all dependencies are included

```bash
# Create a backup
velero backup create test-backup --include-namespaces test-ns

# Check backup contents
velero backup describe test-backup --details

# Verify logs
kubectl logs -n velero deployment/velero
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

### Resources not being backed up

Enable debug logging in Velero:

```bash
kubectl edit deployment velero -n velero
# Add --log-level=debug to the args
```

Check the backup details:

```bash
velero backup describe <backup-name> --details
```

### API version mismatch

If you're using a different version of VM Operator, update the API version in `pkg/plugin/vmgroup_backup_with_client.go`:

```go
vmGVR := schema.GroupVersionResource{
    Group:    "vmoperator.vmware.com",
    Version:  "v1alpha5", // Change this to match your VM Operator version
    Resource: "virtualmachines",
}
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
