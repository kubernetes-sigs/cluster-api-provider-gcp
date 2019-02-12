# Build the manager binary
FROM golang:1.11.5 as builder

# Copy in the go src
WORKDIR /go/src/sigs.k8s.io/cluster-api-provider-gcp
COPY pkg/    pkg/
COPY cmd/    cmd/
COPY vendor/ vendor/

# Build
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -ldflags '-extldflags "-static"' -o manager sigs.k8s.io/cluster-api-provider-gcp/cmd/manager

FROM ubuntu:latest as kubeadm
RUN apt-get update
RUN apt-get install -y curl
RUN curl -fsSL https://dl.k8s.io/release/v1.11.2/bin/linux/amd64/kubeadm > /usr/bin/kubeadm
RUN chmod a+rx /usr/bin/kubeadm

# Copy the controller-manager into a thin image
FROM gcr.io/distroless/static:latest
WORKDIR /
COPY --from=builder /go/src/sigs.k8s.io/cluster-api-provider-gcp/manager .
COPY --from=kubeadm /usr/bin/kubeadm /usr/bin/kubeadm
ENTRYPOINT ["/manager"]
