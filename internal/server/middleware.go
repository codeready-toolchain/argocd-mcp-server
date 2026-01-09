package server

import (
	"context"
	"log/slog"
	"strconv"
	"time"

	"github.com/codeready-toolchain/argocd-mcp-server/internal/metrics"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func NewMetricsMiddleware(logger *slog.Logger) mcp.Middleware {
	return func(next mcp.MethodHandler) mcp.MethodHandler {
		return func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
			logger.Debug("metrics-middleware: received request", "method", method, "params", req.GetParams())
			// measure the duration of the request
			start := time.Now()
			// call the next middleware
			result, err := next(ctx, method, req)
			// measure the duration of the request
			duration := time.Since(start)
			var tool string
			if p, ok := req.GetParams().(*mcp.CallToolParamsRaw); ok {
				tool = p.Name
			}
			success := err == nil
			if r, ok := result.(*mcp.CallToolResult); ok {
				logger.Debug("metrics-middleware: call tool result", "is-error", r.IsError)
				success = success && !r.IsError
			}
			// increment/update the metrics
			metrics.MCPCallsTotal.WithLabelValues(method, tool, strconv.FormatBool(success)).Inc()
			metrics.MCPCallDuration.WithLabelValues(method, tool, strconv.FormatBool(success)).Observe(float64(duration.Seconds()))
			return result, err
		}
	}
}
