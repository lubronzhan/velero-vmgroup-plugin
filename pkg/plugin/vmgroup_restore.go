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
// 2. Adds the VirtualMachineGroup as an additional item to restore first
func (p *VMRestoreItemAction) Execute(input *veleroplugin.RestoreItemActionExecuteInput) (*veleroplugin.RestoreItemActionExecuteOutput, error) {
	p.log.Infof("Executing VMRestoreItemAction for restore %s", input.Restore.Name)

	// Convert unstructured to VirtualMachine
	vm := &vmopv1.VirtualMachine{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(input.Item.UnstructuredContent(), vm); err != nil {
		return nil, errors.Wrap(err, "failed to convert item to VirtualMachine")
	}

	namespace := vm.Namespace
	vmName := vm.Name
	p.log.Infof("Processing VirtualMachine %s/%s", namespace, vmName)

	// Remove cluster-specific fields that shouldn't be restored
	modified := false

	// 1. Remove instanceUUID - this is cluster-specific and will be regenerated
	if vm.Spec.InstanceUUID != "" {
		p.log.Infof("Removing instanceUUID from VM %s/%s", namespace, vmName)
		vm.Spec.InstanceUUID = ""
		modified = true
	}

	// 2. Remove first-boot-done annotation - VM should go through first boot again
	if vm.Annotations != nil {
		if _, exists := vm.Annotations["virtualmachine.vmoperator.vmware.com/first-boot-done"]; exists {
			p.log.Infof("Removing first-boot-done annotation from VM %s/%s", namespace, vmName)
			delete(vm.Annotations, "virtualmachine.vmoperator.vmware.com/first-boot-done")
			modified = true
		}
	}

	// Convert back to unstructured if we made modifications
	var updatedItem runtime.Unstructured
	if modified {
		unstructuredVM, err := runtime.DefaultUnstructuredConverter.ToUnstructured(vm)
		if err != nil {
			return nil, errors.Wrap(err, "failed to convert VM to unstructured")
		}
		updatedItem = &unstructured.Unstructured{Object: unstructuredVM}
	} else {
		updatedItem = input.Item
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
