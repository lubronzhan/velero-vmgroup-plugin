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

package main

import (
	"github.com/sirupsen/logrus"

	"github.com/vmware-tanzu/velero/pkg/plugin/framework"

	"github.com/lubronzhan/velero-vmgroup-plugin/pkg/plugin"
)

func main() {
	framework.NewServer().
		RegisterRestoreItemAction("lubronzhan.io/vm-restore", newVMRestorePlugin).
		RegisterRestoreItemAction("lubronzhan.io/pvc-restore", newPVCRestorePlugin).
		Serve()
}

func newVMRestorePlugin(logger logrus.FieldLogger) (interface{}, error) {
	return plugin.NewVMRestoreItemAction(logger), nil
}

func newPVCRestorePlugin(logger logrus.FieldLogger) (interface{}, error) {
	return plugin.NewPVCRestoreItemAction(logger), nil
}
