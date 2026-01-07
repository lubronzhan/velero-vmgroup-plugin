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

// Package plugin implements Velero restore item action for PVC resources.
// It removes volume health annotations that shouldn't be restored.
package plugin

import (
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	veleroplugin "github.com/vmware-tanzu/velero/pkg/plugin/velero"
)

// PVCRestoreItemAction is a restore item action plugin for PersistentVolumeClaims
type PVCRestoreItemAction struct {
	log logrus.FieldLogger
}

// NewPVCRestoreItemAction creates a new PVCRestoreItemAction
func NewPVCRestoreItemAction(log logrus.FieldLogger) *PVCRestoreItemAction {
	return &PVCRestoreItemAction{
		log: log,
	}
}

// AppliesTo returns the resources this plugin applies to
func (p *PVCRestoreItemAction) AppliesTo() (veleroplugin.ResourceSelector, error) {
	return veleroplugin.ResourceSelector{
		IncludedResources: []string{"persistentvolumeclaims"},
	}, nil
}

// Execute performs the restore action
// Removes volume health annotations that shouldn't be restored
func (p *PVCRestoreItemAction) Execute(input *veleroplugin.RestoreItemActionExecuteInput) (*veleroplugin.RestoreItemActionExecuteOutput, error) {
	p.log.Info("Executing PVCRestoreItemAction")

	// Convert unstructured to PVC
	pvc := &corev1.PersistentVolumeClaim{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(input.Item.UnstructuredContent(), pvc); err != nil {
		return nil, errors.Wrap(err, "failed to convert item to PersistentVolumeClaim")
	}

	p.log.Infof("Processing PVC %s/%s", pvc.Namespace, pvc.Name)

	// Remove volume health annotation
	// This annotation is cluster-specific and shouldn't be restored
	if pvc.Annotations != nil {
		if _, exists := pvc.Annotations["volumehealth.storage.kubernetes.io/health"]; exists {
			p.log.Infof("Removing volumehealth annotation from PVC %s/%s", pvc.Namespace, pvc.Name)
			delete(pvc.Annotations, "volumehealth.storage.kubernetes.io/health")
		}
	}

	// Convert back to unstructured
	unstructuredPVC, err := runtime.DefaultUnstructuredConverter.ToUnstructured(pvc)
	if err != nil {
		return nil, errors.Wrap(err, "failed to convert PVC to unstructured")
	}

	return &veleroplugin.RestoreItemActionExecuteOutput{
		UpdatedItem: &unstructured.Unstructured{Object: unstructuredPVC},
	}, nil
}
