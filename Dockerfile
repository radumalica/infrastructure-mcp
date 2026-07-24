# syntax=docker/dockerfile:1

# --- build stage -------------------------------------------------------
FROM golang:1.25-alpine AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# CGO disabled: the SSH/Telnet clients are pure Go (golang.org/x/crypto/ssh),
# so a fully static binary is possible and keeps the final image minimal.
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w" \
    -o /out/infrastructure-mcp \
    ./cmd/server

# --- final stage ---------------------------------------------------------
# distroless static: no shell, no package manager, minimal CVE surface —
# nothing here talks to a local shell, everything is done over SSH/Telnet/
# HTTP from within the Go binary itself. Runs as the built-in nonroot user.
FROM gcr.io/distroless/static-debian12:nonroot

WORKDIR /app

COPY --from=build /out/infrastructure-mcp /app/infrastructure-mcp
COPY configs/inventory.example.yaml /app/configs/inventory.example.yaml

USER nonroot:nonroot

# Only used when -transport=http; ignored for the default stdio transport.
EXPOSE 8080

ENTRYPOINT ["/app/infrastructure-mcp"]
CMD ["-inventory", "/app/configs/inventory.yaml"]
