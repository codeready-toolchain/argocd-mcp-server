package server

import (
	"context"
	"log/slog"

	"github.com/codeready-toolchain/argocd-mcp-server/internal/argocd"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func New(logger *slog.Logger, cl *argocd.Client) *mcp.Server {
	s := mcp.NewServer(
		&mcp.Implementation{
			Name:    "argocd-mcp-server",
			Version: "0.1",
		},
		&mcp.ServerOptions{
			InitializedHandler: func(_ context.Context, ir *mcp.InitializedRequest) {
				logger.Debug("initialized", "session_id", ir.Session.ID())
			},
			Logger: logger,
		},
	)

	s.AddPrompt(argocd.UnhealthyResourcesPrompt, argocd.UnhealthyApplicationResourcesPromptHandle(logger, cl))
	mcp.AddTool(s, argocd.UnhealthyApplicationsTool, argocd.UnhealthyApplicationsToolHandle(logger, cl))
	mcp.AddTool(s, argocd.UnhealthyApplicationResourcesTool, argocd.UnhealthyApplicationResourcesToolHandle(logger, cl))
	return s
}
