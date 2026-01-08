package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/codeready-toolchain/argocd-mcp-server/internal/argocd"
	"github.com/codeready-toolchain/argocd-mcp-server/internal/server"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/spf13/cobra"
)

var transport, listen, argocdURL, argocdToken string
var argocdInsecure, debug, stateless bool

func init() {
	startServerCmd.Flags().StringVar(&argocdURL, "argocd-url", "", "Specify the URL of the Argo CD server to query (required)")
	if err := startServerCmd.MarkFlagRequired("argocd-url"); err != nil {
		panic(err)
	}
	startServerCmd.Flags().StringVar(&argocdToken, "argocd-token", "", "Specify the token to include in the Authorization header (required)")
	if err := startServerCmd.MarkFlagRequired("argocd-token"); err != nil {
		panic(err)
	}
	startServerCmd.Flags().BoolVar(&argocdInsecure, "insecure", false, "Allow insecure TLS connections to the Argo CD server")
	startServerCmd.Flags().BoolVar(&debug, "debug", false, "Enable debug mode")
	startServerCmd.Flags().BoolVar(&stateless, "stateless", false, "Enable stateless mode where the server does not send change notifications (required for multiple replicas)")
	startServerCmd.Flags().StringVar(&transport, "transport", "http", "Choose between 'stdio' or 'http' transport")
	startServerCmd.Flags().StringVar(&listen, "listen", "0.0.0.0:8080", "Specify the host and port to listen on when using the 'http' transport")
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := startServerCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

// startServerCmd the command to start the Argo CD MCP server
var startServerCmd = &cobra.Command{
	Use:   "argocd-mcp-server",
	Short: "Start the Argo CD MCP server",
	PreRunE: func(_ *cobra.Command, _ []string) error {
		if transport != "stdio" && transport != "http" {
			return fmt.Errorf("invalid transport: choose between 'http' and 'stdio'")
		}
		return nil
	},
	RunE: func(cmd *cobra.Command, _ []string) error {
		lvl := new(slog.LevelVar)
		lvl.Set(slog.LevelInfo)
		logger := slog.New(slog.NewTextHandler(cmd.ErrOrStderr(), &slog.HandlerOptions{
			Level: lvl,
		}))
		logger.Info("starting the Argo CD MCP server", "transport", transport, "url", argocdURL, "insecure", argocdInsecure, "debug", debug, "stateless", stateless)
		if debug {
			lvl.Set(slog.LevelDebug)
			logger.Debug("debug mode enabled")
		}
		cl := argocd.NewClient(argocdURL, argocdToken, argocdInsecure)
		srv := server.New(logger, cl, stateless)
		switch transport {
		case "stdio":
			if stateless {
				return fmt.Errorf("stateless mode is not supported for stdio transport")
			}
			t := &mcp.LoggingTransport{
				Transport: &mcp.StdioTransport{},
				Writer:    cmd.ErrOrStderr(),
			}
			if err := srv.Run(context.Background(), t); err != nil {
				return fmt.Errorf("failed to serve on stdio: %v", err.Error())
			}
		default:
			mux := http.NewServeMux()
			// MCP endpoint
			// Stateless mode configuration from server settings.
			// When Stateless is true, the server will not send notifications to clients
			// (e.g., tools/list_changed, prompts/list_changed). This disables dynamic
			// tool and prompt updates but is useful for container deployments, load
			// balancing, and serverless environments where maintaining client state
			// is not desired or possible.
			mux.Handle("/mcp", mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server {
				return srv
			}, &mcp.StreamableHTTPOptions{
				Stateless: stateless,
			}))
			// HealthCheck endpoint.
			mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(map[string]string{ //nolint:errcheck
					"status": "healthy",
					"time":   time.Now().Format(time.RFC3339),
				})
			})

			server := &http.Server{
				Addr:         listen,
				Handler:      mux,
				ReadTimeout:  15 * time.Second,
				WriteTimeout: 15 * time.Second,
				IdleTimeout:  60 * time.Second,
			}
			if err := server.ListenAndServe(); err != nil {
				return fmt.Errorf("failed to start server: %v", err.Error())
			}
		}
		return nil
	},
}
