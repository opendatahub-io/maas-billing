FROM registry.access.redhat.com/ubi9/go-toolset:1.24 AS builder

WORKDIR /app

COPY go.mod go.sum ./

RUN go mod download

COPY . .

USER root
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o maas-api ./cmd/

FROM registry.access.redhat.com/ubi9/ubi-minimal:latest

# Install CA certificates for HTTPS requests
RUN microdnf update -y && microdnf install -y ca-certificates curl-minimal && microdnf clean all

WORKDIR /app

COPY --from=builder /app/maas-api .

# Make binary executable and fix permissions for OpenShift
RUN chmod +x maas-api && \
    chgrp -R 0 /app && \
    chmod -R g=u /app

# Use a non-root user (OpenShift will assign random UID)
USER 1001

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
  CMD curl -f http://localhost:8080/health || exit 1

CMD ["./maas-api"]
