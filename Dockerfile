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

FROM golang:1.22-alpine AS builder

WORKDIR /workspace

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY main.go ./
COPY pkg/ pkg/

# Build
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -o velero-vmgroup-plugin .

# Use a minimal base image
FROM alpine:3.19

RUN apk add --no-cache ca-certificates

WORKDIR /plugins

# Copy the plugin binary
COPY --from=builder /workspace/velero-vmgroup-plugin /plugins/

# Set the entrypoint
USER nobody:nobody
ENTRYPOINT ["/bin/sh", "-c", "cp /plugins/* /target/."]
