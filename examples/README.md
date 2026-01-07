# Examples

This directory contains example manifests for testing the Velero VM restore plugin.

## Files

- `vmgroup-example.yaml` - Complete example with VirtualMachineGroup, VirtualMachines, Secrets, and PVCs
- `backup-example.yaml` - Example Velero Backup resource
- `resource-modifiers-configmap.yaml` - Reference ConfigMap (not needed with this plugin)

## Usage

### 1. Deploy the example resources

```bash
kubectl apply -f vmgroup-example.yaml
```

This will create:
- A namespace `vm-demo`
- Two Secrets for cloud-init configuration
- Two PVCs for VM data
- Two VirtualMachines
- One VirtualMachineGroup containing both VMs

### 2. Verify the resources

```bash
# Check VirtualMachineGroup
kubectl get virtualmachinegroup -n vm-demo

# Check VirtualMachines
kubectl get virtualmachine -n vm-demo

# Check Secrets
kubectl get secret -n vm-demo

# Check PVCs
kubectl get pvc -n vm-demo
```

### 3. Create a backup

Using the Velero CLI:

```bash
velero backup create vmgroup-backup --include-namespaces vm-demo
```

Or using the example manifest:

```bash
kubectl apply -f backup-example.yaml
```

### 4. Test restore

```bash
# Delete the namespace
kubectl delete namespace vm-demo

# Restore from backup
velero restore create vmgroup-restore --from-backup vmgroup-backup

# Check restore status
velero restore describe vmgroup-restore

# Check restore logs to see plugin in action
velero restore logs vmgroup-restore | grep -E "(Removing|Processing)"

# Verify resources are restored
kubectl get vmgroup,vm,secret,pvc -n vm-demo

# Verify cluster-specific fields were removed
kubectl get vm -n vm-demo -o yaml | grep -E "(instanceUUID|first-boot-done)"
# (should return nothing)

kubectl get pvc -n vm-demo -o yaml | grep volumehealth
# (should return nothing)
```

Expected restore logs:
- "Processing VirtualMachine vm-demo/vm-1"
- "Removing instanceUUID from VM vm-demo/vm-1"
- "Removing first-boot-done annotation from VM vm-demo/vm-1"
- "Processing PVC vm-demo/vm-1-data"
- "Removing volumehealth annotation from PVC vm-demo/vm-1-data"

## Cleanup

```bash
# Delete the backup
velero backup delete vmgroup-backup

# Delete the example resources
kubectl delete -f vmgroup-example.yaml
```
