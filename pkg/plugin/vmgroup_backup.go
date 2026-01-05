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

// Package plugin implements a Velero backup item action plugin for VirtualMachineGroup resources.
// It automatically backs up VirtualMachineGroup CRs along with their dependencies:
// - VirtualMachine members
// - Bootstrap secrets (from vm.spec.bootstrap.cloudInit.rawCloudConfig.name)
// - PVCs (from vm.spec.volumes[x].persistentVolumeClaim.claimName)
package plugin

import (
	"context"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"

	vmopv1 "github.com/vmware-tanzu/vm-operator/api/v1alpha5"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"

	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	veleroplugin "github.com/vmware-tanzu/velero/pkg/plugin/velero"
)

// VMGroupBackupItemAction uses a Kubernetes client to fetch VirtualMachine
// details and their dependencies (secrets and PVCs)
type VMGroupBackupItemAction struct {
	log    logrus.FieldLogger
	client client.Client
}

// NewVMGroupBackupItemAction creates a new VMGroupBackupItemAction
func NewVMGroupBackupItemAction(log logrus.FieldLogger, config *rest.Config) (*VMGroupBackupItemAction, error) {
	// Register VM Operator types with the scheme
	if err := vmopv1.AddToScheme(scheme.Scheme); err != nil {
		return nil, errors.Wrap(err, "failed to add VM Operator types to scheme")
	}

	// Create controller-runtime client
	k8sClient, err := client.New(config, client.Options{
		Scheme: scheme.Scheme,
	})
	if err != nil {
		return nil, errors.Wrap(err, "failed to create Kubernetes client")
	}

	return &VMGroupBackupItemAction{
		log:    log,
		client: k8sClient,
	}, nil
}

// AppliesTo returns the resources this plugin applies to
func (p *VMGroupBackupItemAction) AppliesTo() (veleroplugin.ResourceSelector, error) {
	return veleroplugin.ResourceSelector{
		IncludedResources: []string{"virtualmachinegroups.vmoperator.vmware.com"},
	}, nil
}

// Execute performs the backup action
func (p *VMGroupBackupItemAction) Execute(item runtime.Unstructured, backup *velerov1api.Backup) (runtime.Unstructured, []veleroplugin.ResourceIdentifier, error) {
	p.log.Infof("Executing plugin for backup %s", backup.Name)

	// Convert unstructured to VirtualMachineGroup
	vmGroup := &vmopv1.VirtualMachineGroup{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(item.UnstructuredContent(), vmGroup); err != nil {
		return nil, nil, errors.Wrap(err, "failed to convert item to VirtualMachineGroup")
	}

	namespace := vmGroup.Namespace
	p.log.Infof("Processing VirtualMachineGroup %s/%s", namespace, vmGroup.Name)

	// List to hold additional items to backup
	var additionalItems []veleroplugin.ResourceIdentifier

	// 1. Get VirtualMachine members from bootOrder.members
	if vmGroup.Spec.BootOrder == nil {
		p.log.Warn("VirtualMachineGroup has no boot orders")
		return item, additionalItems, nil
	}

	// 2. For each VirtualMachine, fetch it and extract dependencies
	for _, bootOrder := range vmGroup.Spec.BootOrder {
		for _, member := range bootOrder.Members {
			vmName := member.Name
			p.log.Infof("Processing VirtualMachine %s/%s", namespace, vmName)

			// Add the VirtualMachine itself
			additionalItems = append(additionalItems, veleroplugin.ResourceIdentifier{
				GroupResource: schema.GroupResource{
					Group:    "vmoperator.vmware.com",
					Resource: "virtualmachines",
				},
				Namespace: namespace,
				Name:      vmName,
			})

			// Fetch the VirtualMachine to get its dependencies
			vm, err := p.getVirtualMachine(namespace, vmName)
			if err != nil {
				p.log.Errorf("Failed to get VirtualMachine %s/%s: %v", namespace, vmName, err)
				continue
			}

			// Extract secrets from bootstrap configuration
			secrets := p.extractSecretsFromVM(vm, namespace)
			additionalItems = append(additionalItems, secrets...)

			// Extract PVCs from volumes
			pvcs := p.extractPVCsFromVM(vm, namespace)
			additionalItems = append(additionalItems, pvcs...)
		}
	}

	p.log.Infof("Found %d additional items to backup for VirtualMachineGroup", len(additionalItems))

	return item, additionalItems, nil
}

// getVirtualMachine fetches a VirtualMachine from the API server
func (p *VMGroupBackupItemAction) getVirtualMachine(namespace, name string) (*vmopv1.VirtualMachine, error) {
	vm := &vmopv1.VirtualMachine{}
	key := client.ObjectKey{
		Namespace: namespace,
		Name:      name,
	}

	ctx := context.TODO()
	if err := p.client.Get(ctx, key, vm); err != nil {
		return nil, errors.Wrapf(err, "failed to get VirtualMachine %s/%s", namespace, name)
	}

	return vm, nil
}

// extractSecretsFromVM extracts Secret references from a VirtualMachine
func (p *VMGroupBackupItemAction) extractSecretsFromVM(vm *vmopv1.VirtualMachine, namespace string) []veleroplugin.ResourceIdentifier {
	var secrets []veleroplugin.ResourceIdentifier

	// Extract bootstrap secret from spec.bootstrap.cloudInit.rawCloudConfig.name
	if vm.Spec.Bootstrap == nil {
		return secrets
	}

	if vm.Spec.Bootstrap.CloudInit == nil {
		return secrets
	}

	if vm.Spec.Bootstrap.CloudInit.RawCloudConfig == nil {
		return secrets
	}

	secretName := vm.Spec.Bootstrap.CloudInit.RawCloudConfig.Name
	if secretName == "" {
		return secrets
	}

	p.log.Infof("Adding Secret %s/%s to backup", namespace, secretName)
	secrets = append(secrets, veleroplugin.ResourceIdentifier{
		GroupResource: schema.GroupResource{
			Group:    "",
			Resource: "secrets",
		},
		Namespace: namespace,
		Name:      secretName,
	})

	return secrets
}

// extractPVCsFromVM extracts PVC references from a VirtualMachine
func (p *VMGroupBackupItemAction) extractPVCsFromVM(vm *vmopv1.VirtualMachine, namespace string) []veleroplugin.ResourceIdentifier {
	var pvcs []veleroplugin.ResourceIdentifier

	// Extract PVCs from spec.volumes[x].persistentVolumeClaim.claimName
	for i, volume := range vm.Spec.Volumes {
		if volume.PersistentVolumeClaim == nil {
			continue
		}

		claimName := volume.PersistentVolumeClaim.ClaimName
		if claimName == "" {
			continue
		}

		p.log.Infof("Adding PVC %s/%s to backup (from volume %d: %s)", namespace, claimName, i, volume.Name)
		pvcs = append(pvcs, veleroplugin.ResourceIdentifier{
			GroupResource: schema.GroupResource{
				Group:    "",
				Resource: "persistentvolumeclaims",
			},
			Namespace: namespace,
			Name:      claimName,
		})
	}

	return pvcs
}
