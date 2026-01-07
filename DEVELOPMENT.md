# Development Guide

This guide provides instructions for developing and testing the Velero VMGroup Plugin.

## Prerequisites

- Go 1.22 or later
- Docker
- Access to a Kubernetes cluster with:
  - Velero installed
  - VM Operator installed
  - kubectl configured

## Setting Up Development Environment

### 1. Clone the repository

```bash
git clone https://github.com/lubronzhan/velero-vmgroup-plugin.git
cd velero-vmgroup-plugin
```

### 2. Download dependencies

```bash
go mod download
go mod tidy
```

Note: You may need to adjust the VM Operator API version in `go.mod` based on your installation.

### 3. Build the plugin

```bash
make build
```

## Project Structure

```
velero-vmgroup-plugin/
├── main.go                              # Plugin entry point
├── pkg/
│   └── plugin/
│       ├── vmgroup_restore.go           # VM restore plugin
│       └── pvc_restore.go               # PVC restore plugin
├── examples/                            # Example manifests
│   ├── vmgroup-example.yaml
│   ├── backup-example.yaml
│   └── README.md
├── Dockerfile                           # Container image
├── Makefile                            # Build automation
├── go.mod                              # Go module
└── README.md                           # Main documentation
```

## Plugin Implementation

The plugin provides restore functionality:
- **VM Restore Plugin** (`pkg/plugin/vmgroup_restore.go`): Ensures VirtualMachineGroup is restored before VMs, removes cluster-specific fields
- **PVC Restore Plugin** (`pkg/plugin/pvc_restore.go`): Removes cluster-specific annotations from PVCs
- Uses VM Operator API types for type safety
- Handles errors gracefully with detailed logging

## Building and Testing

### Local Build

```bash
# Build binary
make build

# Run formatting
make fmt

# Run vet
make vet

# Run all checks
make check
```

### Build Container Image

```bash
# Build with default settings
make container

# Build with custom image name and version
IMAGE=myregistry/velero-vmgroup-plugin VERSION=v0.1.0 make container
```

### Push to Registry

```bash
# Push with default settings
make push

# Push with custom settings
IMAGE=myregistry/velero-vmgroup-plugin VERSION=v0.1.0 make push
```

## Testing the Plugin

### 1. Deploy to Velero

```bash
# Install the plugin
velero plugin add <your-registry>/velero-vmgroup-plugin:latest

# Verify installation
velero plugin get
```

### 2. Deploy Test Resources

```bash
# Deploy example VirtualMachineGroup with dependencies
kubectl apply -f examples/vmgroup-example.yaml

# Verify resources
kubectl get virtualmachinegroup,virtualmachine,secret,pvc -n vm-demo
```

### 3. Create a Backup

```bash
# Create backup (using standard Velero or another backup plugin)
velero backup create test-backup --include-namespaces vm-demo

# Check backup status
velero backup describe test-backup
```

### 4. Test Restore

```bash
# Delete test resources
kubectl delete namespace vm-demo

# Create restore
velero restore create test-restore --from-backup test-backup

# Check restore status
velero restore describe test-restore

# View restore logs
velero restore logs test-restore

# Verify plugin execution
velero restore logs test-restore | grep -E "(Removing|Processing|VirtualMachineGroup)"
```

### 5. Verify Plugin Execution

Check that resources were restored correctly:

```bash
# Check resources exist
kubectl get vmgroup,vm,pvc -n vm-demo

# Verify cluster-specific fields were removed
kubectl get vm -n vm-demo -o yaml | grep -E "(instanceUUID|first-boot-done)"
# (should return nothing)

kubectl get pvc -n vm-demo -o yaml | grep volumehealth
# (should return nothing)
```

Expected restore logs should include:
- "Processing VirtualMachine vm-demo/vm-1"
- "Removing instanceUUID from VM vm-demo/vm-1"
- "Removing first-boot-done annotation from VM vm-demo/vm-1"
- "Processing PVC vm-demo/vm-1-data"
- "Removing volumehealth annotation from PVC vm-demo/vm-1-data"

## Debugging

### Enable Debug Logging

Edit the Velero deployment to enable debug logging:

```bash
kubectl edit deployment velero -n velero
```

Add `--log-level=debug` to the container args.

### View Plugin Logs

```bash
# Velero server logs
kubectl logs -n velero deployment/velero -f

# Plugin logs (part of Velero logs)
kubectl logs -n velero deployment/velero | grep vmgroup
```

### Common Issues

#### Plugin Not Found

```bash
# Check plugin is registered
velero plugin get

# Check plugin pod
kubectl get pods -n velero -l component=velero
```

#### API Version Mismatch

If you see errors about API versions, update the version in `vmgroup_restore.go`:

```go
import vmopv1 "github.com/vmware-tanzu/vm-operator/api/v1alpha5"
// Change v1alpha5 to match your VM Operator version
```

#### Resources Not Being Modified

If cluster-specific fields aren't being removed during restore:

1. Check that the plugin is being invoked:
   ```bash
   velero restore logs <restore-name> | grep "Removing"
   ```

2. Verify the plugin is registered:
   ```bash
   velero plugin get
   ```

3. Check for errors in restore logs:
   ```bash
   velero restore logs <restore-name>
   ```

## Code Style

Follow standard Go conventions:

```bash
# Format code
go fmt ./...

# Run linter
go vet ./...

# Run golangci-lint if available
golangci-lint run
```

## API Version Compatibility

The plugin is designed for VM Operator API v1alpha5. If you're using a different version:

1. Update `go.mod` with the correct VM Operator version
2. Update API version in `vmgroup_backup_with_client.go`
3. Verify field paths match your API version

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Test thoroughly
5. Submit a pull request

## Resources

- [Velero Plugin Documentation](https://velero.io/docs/main/custom-plugins/)
- [VM Operator API Reference](https://github.com/vmware-tanzu/vm-operator/tree/main/api)
- [VM Operator Documentation](https://vm-operator.readthedocs.io/)
- [Velero Plugin Example](https://github.com/vmware-tanzu/velero-plugin-example)

## License

Apache License 2.0 - See LICENSE file for details.
