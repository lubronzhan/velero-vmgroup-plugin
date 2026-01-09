/*
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
*/

// Package plugin implements Velero restore item action for VirtualMachine resources.
// It ensures VirtualMachines are restored after their VirtualMachineGroup.
package plugin

import (
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	vmopv1 "github.com/vmware-tanzu/vm-operator/api/v1alpha5"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	veleroplugin "github.com/vmware-tanzu/velero/pkg/plugin/velero"
)

// VMRestoreItemAction is a restore item action plugin for VirtualMachine
type VMRestoreItemAction struct {
	log logrus.FieldLogger
}

// NewVMRestoreItemAction creates a new VMRestoreItemAction
func NewVMRestoreItemAction(log logrus.FieldLogger) *VMRestoreItemAction {
	return &VMRestoreItemAction{
		log: log,
	}
}

// AppliesTo returns the resources this plugin applies to
func (p *VMRestoreItemAction) AppliesTo() (veleroplugin.ResourceSelector, error) {
	return veleroplugin.ResourceSelector{
		IncludedResources: []string{"virtualmachines.vmoperator.vmware.com"},
	}, nil
}

// Execute performs the restore action
// This plugin:
// 1. Removes cluster-specific fields that shouldn't be restored
// 2. Injects network configuration from status to spec to preserve IP addresses
// 3. Adds the VirtualMachineGroup as an additional item to restore first
func (p *VMRestoreItemAction) Execute(input *veleroplugin.RestoreItemActionExecuteInput) (*veleroplugin.RestoreItemActionExecuteOutput, error) {
	p.log.Infof("Executing VMRestoreItemAction for restore %s", input.Restore.Name)

	// Work with unstructured data directly for more flexibility
	obj := input.Item.UnstructuredContent()

	// Get metadata
	namespace, _, _ := unstructured.NestedString(obj, "metadata", "namespace")
	vmName, _, _ := unstructured.NestedString(obj, "metadata", "name")

	p.log.Infof("Processing VirtualMachine %s/%s", namespace, vmName)

	modified := false

	// 1. Remove instanceUUID - this is cluster-specific and will be regenerated
	if instanceUUID, found, _ := unstructured.NestedString(obj, "spec", "instanceUUID"); found && instanceUUID != "" {
		p.log.Infof("Removing instanceUUID from VM %s/%s", namespace, vmName)
		unstructured.SetNestedField(obj, "", "spec", "instanceUUID")
		modified = true
	}

	// 2. Remove first-boot-done annotation - VM should go through first boot again
	if annotations, found, _ := unstructured.NestedStringMap(obj, "metadata", "annotations"); found {
		if _, exists := annotations["virtualmachine.vmoperator.vmware.com/first-boot-done"]; exists {
			p.log.Infof("Removing first-boot-done annotation from VM %s/%s", namespace, vmName)
			delete(annotations, "virtualmachine.vmoperator.vmware.com/first-boot-done")
			unstructured.SetNestedStringMap(obj, annotations, "metadata", "annotations")
			modified = true
		}
	}

	// 3. Inject network configuration from status.network.config to spec.network
	if p.injectNetworkConfigFromStatus(obj, namespace, vmName) {
		modified = true
	}

	// Use the modified object
	var updatedItem runtime.Unstructured
	if modified {
		updatedItem = &unstructured.Unstructured{Object: obj}
	} else {
		updatedItem = input.Item
	}

	// Convert to typed object to get groupName
	vm := &vmopv1.VirtualMachine{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj, vm); err != nil {
		return nil, errors.Wrap(err, "failed to convert item to VirtualMachine")
	}

	// Check if this VM belongs to a VirtualMachineGroup
	vmGroupName := vm.Spec.GroupName

	output := veleroplugin.NewRestoreItemActionExecuteOutput(updatedItem)

	if vmGroupName != "" {
		p.log.Infof("VirtualMachine %s/%s belongs to VirtualMachineGroup %s", namespace, vmName, vmGroupName)

		// Add the VirtualMachineGroup as an additional item to restore
		// Velero will restore it before this VM
		output.AdditionalItems = []veleroplugin.ResourceIdentifier{
			{
				GroupResource: schema.GroupResource{
					Group:    "vmoperator.vmware.com",
					Resource: "virtualmachinegroups",
				},
				Namespace: namespace,
				Name:      vmGroupName,
			},
		}

		// Tell Velero to wait for the additional items to be ready
		output.WaitForAdditionalItems = true
		p.log.Infof("Will wait for VirtualMachineGroup %s/%s before restoring VM", namespace, vmGroupName)
	}

	return output, nil
}

// injectNetworkConfigFromStatus copies network configuration from status.network.config to spec.network
// This preserves the original IP address during restore
func (p *VMRestoreItemAction) injectNetworkConfigFromStatus(obj map[string]interface{}, namespace, vmName string) bool {
	// Check if spec.network already exists
	if specNetwork, found, _ := unstructured.NestedMap(obj, "spec", "network"); found && specNetwork != nil {
		p.log.Infof("VM %s/%s already has spec.network configuration - preserving as-is", namespace, vmName)
		return false
	}

	// Get status.network.config
	statusNetworkConfig, found, err := unstructured.NestedMap(obj, "status", "network", "config")
	if !found || err != nil {
		p.log.Warnf("VM %s/%s has no status.network.config - cannot inject network config", namespace, vmName)
		return false
	}

	// Get primary IP for logging
	primaryIP, _, _ := unstructured.NestedString(obj, "status", "network", "primaryIP4")

	p.log.Infof("Injecting network configuration for VM %s/%s with IP %s", namespace, vmName, primaryIP)

	// Copy status.network.config to spec.network
	// This preserves the exact network configuration including:
	// - interfaces with IP addresses
	// - DNS settings
	// - gateway configuration
	if err := unstructured.SetNestedMap(obj, statusNetworkConfig, "spec", "network"); err != nil {
		p.log.Errorf("Failed to inject network config for VM %s/%s: %v", namespace, vmName, err)
		return false
	}

	p.log.Infof("VM %s/%s network config injected successfully - IP %s will be preserved", namespace, vmName, primaryIP)

	return true
}
