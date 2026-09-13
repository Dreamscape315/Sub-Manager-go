# --- Build stage --------------------------------------------------------
# --platform=$BUILDPLATFORM + explicit GOOS/GOARCH: the compiler always runs
# natively on the build host and cross-compiles, so multi-arch builds don't
# need (slow, sometimes flaky) QEMU emulation for the Go build step itself.
FROM --platform=$BUILDPLATFORM golang:1.25-bookworm AS build
ARG TARGETOS
ARG TARGETARCH
WORKDIR /src
COPY . .
# Vendored deps (committed under vendor/) avoid needing network access to the
# Go module proxy during the image build.
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -mod=vendor -trimpath -ldflags="-s -w" \
    -o /out/submanager ./cmd/submanager
# Pre-create the data dir so it can be COPYed into the final (shell-less)
# distroless stage with the right ownership before VOLUME takes over.
RUN mkdir -p /out/data

# --- Runtime stage --------------------------------------------------------
# distroless static: no shell, no package manager, just the binary + CA certs.
FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app
COPY --from=build /out/submanager /app/submanager
COPY --from=build --chown=nonroot:nonroot /out/data /app/data
VOLUME ["/app/data"]
EXPOSE 8080
ENV SUBMANAGER_DB_PATH=/app/data/submanager.db \
    PORT=8080 \
    TZ=UTC
ENTRYPOINT ["/app/submanager"]
