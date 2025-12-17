# Build stage - use native platform for builder to avoid emulation
FROM --platform=$BUILDPLATFORM registry.access.redhat.com/ubi9/go-toolset:1.25 AS builder

# Build arguments for cross-compilation
ARG TARGETOS
ARG TARGETARCH

# Switch to root for installation
USER root

# Install make (using yum for go-toolset image)
RUN yum install -y make && yum clean all

WORKDIR /workspace

# Copy go mod files first for better layer caching
COPY go.mod go.sum ./

# Enable Go toolchain auto-download to match go.mod version requirement
ENV GOTOOLCHAIN=auto
RUN go mod download

# Copy source code and Makefile
COPY . .

# Build arguments for version information
ARG VERSION=dev
ARG COMMIT=unknown
ARG DATE=unknown

# Build using Makefile with cross-compilation
RUN make build \
    GOOS=${TARGETOS} \
    GOARCH=${TARGETARCH} \
    VERSION=${VERSION} \
    COMMIT=${COMMIT} \
    DATE=${DATE}

# Runtime stage
FROM registry.access.redhat.com/ubi9/ubi-minimal:latest

# Copy binary from builder (cross-compiled for target platform)
COPY --from=builder /workspace/bin/bundle-extract /usr/local/bin/bundle-extract

# Set entrypoint and default command
ENTRYPOINT ["/usr/local/bin/bundle-extract"]
CMD ["krm"]

