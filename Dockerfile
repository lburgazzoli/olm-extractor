# Build stage
FROM registry.access.redhat.com/ubi9/go-toolset:1.25 AS builder

# Switch to root to install dependencies
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

# Build using Makefile
RUN make build VERSION=${VERSION} COMMIT=${COMMIT} DATE=${DATE}

# Runtime stage
FROM registry.access.redhat.com/ubi9/ubi-minimal:latest

# Copy binary from builder (built by Makefile in bin/ directory)
COPY --from=builder /workspace/bin/bundle-extract /usr/local/bin/bundle-extract

# Set entrypoint and default command
ENTRYPOINT ["/usr/local/bin/bundle-extract"]
CMD ["krm"]

