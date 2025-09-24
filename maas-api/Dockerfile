FROM registry.access.redhat.com/ubi9/go-toolset:1.24 AS builder

# Temporarily switch to root for build operations
USER root

WORKDIR /app

COPY --chown=1001:0 go.mod go.sum ./

RUN go mod download

COPY --chown=1001:0 . .

RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /tmp/maas-api ./cmd/

FROM registry.access.redhat.com/ubi9/ubi-minimal:latest

# Install CA certificates for HTTPS requests
RUN microdnf install -y ca-certificates && microdnf clean all

WORKDIR /app

COPY --from=builder /tmp/maas-api ./maas-api

# Make binary executable and fix permissions for OpenShift
RUN chmod +x maas-api && \
    chgrp -R 0 /app && \
    chmod -R g=u /app

# Use a non-root user (OpenShift will assign random UID)
USER 1001

EXPOSE 8080

CMD ["./maas-api"]
