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
│       ├── vmgroup_backup.go            # Basic implementation
│       └── vmgroup_backup_with_client.go # Full implementation with K8s client
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

The plugin (`pkg/plugin/vmgroup_backup.go`) provides complete functionality:
- Uses controller-runtime client to fetch VirtualMachine resources
- Extracts bootstrap secrets from VM specs
- Extracts PVCs from VM volumes
- Handles errors gracefully with detailed logging

The plugin automatically gets the Kubernetes configuration from the in-cluster config when running inside the Velero pod.

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
# Create backup
velero backup create test-backup --include-namespaces vm-demo

# Check backup status
velero backup describe test-backup

# View detailed backup contents
velero backup describe test-backup --details

# Check logs
velero backup logs test-backup
```

### 4. Verify Plugin Execution

Check that all dependencies were backed up:

```bash
velero backup describe test-backup --details | grep -E "(VirtualMachine|Secret|PersistentVolumeClaim)"
```

Expected output should include:
- VirtualMachineGroup: `my-vm-group`
- VirtualMachines: `vm-1`, `vm-2`
- Secrets: `vm-1-cloud-init`, `vm-2-cloud-init`
- PVCs: `vm-1-data`, `vm-2-data`

### 5. Test Restore

```bash
# Delete the namespace
kubectl delete namespace vm-demo

# Restore from backup
velero restore create --from-backup test-backup

# Verify restoration
kubectl get all,secret,pvc,virtualmachine,virtualmachinegroup -n vm-demo
```

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

If you see errors about API versions, update the version in `vmgroup_backup_with_client.go`:

```go
vmGVR := schema.GroupVersionResource{
    Group:    "vmoperator.vmware.com",
    Version:  "v1alpha5", // Update this
    Resource: "virtualmachines",
}
```

#### Missing Dependencies

If VMs or secrets aren't being backed up:

1. Check that the plugin is being invoked:
   ```bash
   velero backup logs <backup-name> | grep "Executing VMGroup"
   ```

2. Verify the VirtualMachineGroup structure matches expected format

3. Check for errors in Velero logs

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
