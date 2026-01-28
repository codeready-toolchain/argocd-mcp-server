################################################################################################
# Builder image
# See https://hub.docker.com/_/golang/
################################################################################################
FROM --platform=$BUILDPLATFORM golang:1.24 AS builder

WORKDIR /usr/src/app

ARG TARGETOS
ARG TARGETARCH

RUN echo "Building the 'argocd-mcp-server' binary for $TARGETOS/$TARGETARCH"

# pre-copy/cache parent go.mod for pre-downloading dependencies and only redownloading them in subsequent builds if they change
COPY go.mod go.sum ./
RUN go mod download && go mod verify

COPY . .

RUN GOOS=$TARGETOS GOARCH=$TARGETARCH go build -o /usr/src/app/argocd-mcp-server main.go
RUN ls -la /usr/src/app/argocd-mcp-server

################################################################################################
# image to be deployed to the target platform
################################################################################################
FROM --platform=$TARGETPLATFORM registry.access.redhat.com/ubi9/ubi-minimal:latest AS argocd-mcp-server

# Copy the generated binary into the $PATH so it can be invoked
COPY --from=builder /usr/src/app/argocd-mcp-server /usr/local/bin/

ENV ARGOCD_URL=https://argocd-server
ENV ARGOCD_TOKEN=secure-token
ENV ARGOCD_MCP_SERVER_INSECURE=false
ENV ARGOCD_MCP_SERVER_DEBUG=false
ENV ARGOCD_MCP_SERVER_STATELESS=false
ENV ARGOCD_MCP_SERVER_LISTEN_HOST=127.0.0.1
ENV ARGOCD_MCP_SERVER_LISTEN_PORT=8080

# Run as non-root user
USER 1001

EXPOSE ${ARGOCD_MCP_SERVER_LISTEN_PORT}

CMD /usr/local/bin/argocd-mcp-server --transport http --argocd-url $ARGOCD_URL --argocd-token $ARGOCD_TOKEN --insecure=$ARGOCD_MCP_SERVER_INSECURE --debug=$ARGOCD_MCP_SERVER_DEBUG --stateless=$ARGOCD_MCP_SERVER_STATELESS --listen $ARGOCD_MCP_SERVER_LISTEN_HOST:$ARGOCD_MCP_SERVER_LISTEN_PORT