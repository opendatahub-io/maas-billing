FROM registry.access.redhat.com/ubi9/go-toolset:1.24 AS builder

WORKDIR /app

COPY go.mod go.sum ./

RUN go mod download

COPY . .

USER root
RUN CGO_ENABLED=1 GOEXPERIMENT=strictfipsruntime GOOS=linux go build -trimpath -ldflags="-s -w" -o maas-api ./cmd/

FROM registry.access.redhat.com/ubi9/ubi-minimal:latest

WORKDIR /app

COPY --from=builder /app/maas-api .

# Make binary executable and fix permissions for OpenShift
RUN chmod +x maas-api && \
    chgrp -R 0 /app && \
    chmod -R g=u /app

# Use a non-root user (OpenShift will assign random UID)
USER 1001

EXPOSE 8080

CMD ["./maas-api"]
