# Examples

This directory contains example manifests for testing the Velero VMGroup plugin.

## Files

- `vmgroup-example.yaml` - Complete example with VirtualMachineGroup, VirtualMachines, Secrets, and PVCs
- `backup-example.yaml` - Example Velero Backup resource

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

### 4. Check the backup

```bash
# Check backup status
velero backup describe vmgroup-backup

# Check backup details (shows all backed up resources)
velero backup describe vmgroup-backup --details

# Check logs
velero backup logs vmgroup-backup
```

### 5. Verify plugin execution

Check that the plugin backed up all dependencies:

```bash
velero backup describe vmgroup-backup --details | grep -E "(VirtualMachine|Secret|PersistentVolumeClaim)"
```

You should see:
- The VirtualMachineGroup
- Both VirtualMachines (vm-1, vm-2)
- Both Secrets (vm-1-cloud-init, vm-2-cloud-init)
- Both PVCs (vm-1-data, vm-2-data)

### 6. Test restore

```bash
# Delete the namespace
kubectl delete namespace vm-demo

# Restore from backup
velero restore create --from-backup vmgroup-backup

# Verify resources are restored
kubectl get all,secret,pvc,virtualmachine,virtualmachinegroup -n vm-demo
```

## Cleanup

```bash
# Delete the backup
velero backup delete vmgroup-backup

# Delete the example resources
kubectl delete -f vmgroup-example.yaml
```
