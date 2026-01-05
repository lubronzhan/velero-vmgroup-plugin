# Copyright 2026 the Velero contributors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# Image URL to use all building/pushing image targets
IMAGE ?= lubronzhan/velero-vmgroup-plugin
VERSION ?= latest

# Build the plugin binary
.PHONY: build
build:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -o velero-vmgroup-plugin .

# Build the docker image
.PHONY: container
container:
	docker build -t $(IMAGE):$(VERSION) .

# Push the docker image
.PHONY: push
push:
	docker push $(IMAGE):$(VERSION)

# Run go mod tidy
.PHONY: modules
modules:
	go mod tidy

# Run go fmt
.PHONY: fmt
fmt:
	go fmt ./...

# Run go vet
.PHONY: vet
vet:
	go vet ./...

# Clean build artifacts
.PHONY: clean
clean:
	rm -f velero-vmgroup-plugin

# Run all checks
.PHONY: check
check: fmt vet

# Build and push
.PHONY: all
all: check build container push
