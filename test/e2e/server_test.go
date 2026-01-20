package e2etests

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"

	toolchaintests "github.com/codeready-toolchain/toolchain-e2e/testsupport/metrics"

	argocdv3 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/codeready-toolchain/argocd-mcp-server/internal/argocd"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
)

// ------------------------------------------------------------------------------------------------
// Note: make sure you ran `task install` before running this test
// ------------------------------------------------------------------------------------------------

func TestServer(t *testing.T) {

	testdata := []struct {
		name string
		init func(*testing.T) *mcp.ClientSession
	}{
		{
			name: "stdio",
			init: newStdioSession(true, "http://localhost:50084", "secure-token", true),
		},
		{
			name: "http",
			init: newHTTPSession("http://localhost:50081/mcp"),
		},
		{
			name: "http-stateless",
			init: newHTTPSession("http://localhost:50083/mcp"),
		},
	}

	// test stdio and http transports with a valid Argo CD client
	for _, td := range testdata {
		t.Run(td.name, func(t *testing.T) {
			// given
			session := td.init(t)
			defer session.Close()

			t.Run("call/unhealthyApplications/ok", func(t *testing.T) {
				// get the metrics before the call
				var mcpCallsTotalMetricBefore int64
				var mcpCallsDurationSecondsInfBucketBefore int64
				if td.name == "http" {
					mcpCallsTotalMetricBefore, mcpCallsDurationSecondsInfBucketBefore = getMetrics(t, "http://localhost:50081", map[string]string{
						"method":  "tools/call",
						"name":    "unhealthyApplications",
						"success": "true",
					})
				}

				// when
				result, err := session.CallTool(context.Background(), &mcp.CallToolParams{
					Name: "unhealthyApplications",
				})

				// then
				require.NoError(t, err)
				require.False(t, result.IsError, result.Content[0].(*mcp.TextContent).Text)
				// expected content
				expectedContent := map[string]any{
					"degraded":    []any{"a-degraded-application", "another-degraded-application"},
					"progressing": []any{"a-progressing-application", "another-progressing-application"},
					"outOfSync":   []any{"an-out-of-sync-application", "another-out-of-sync-application"},
				}
				expectedContentText, err := json.Marshal(expectedContent)
				require.NoError(t, err)
				// verify the `text` result
				resultContent, ok := result.Content[0].(*mcp.TextContent)
				require.True(t, ok)
				assert.JSONEq(t, string(expectedContentText), resultContent.Text)
				// verify the `structured` content
				require.IsType(t, map[string]any{}, result.StructuredContent)
				actualStructuredContent := map[string]any{}
				err = runtime.DefaultUnstructuredConverter.FromUnstructured(result.StructuredContent.(map[string]any), &actualStructuredContent)
				require.NoError(t, err)
				assert.Equal(t, expectedContent, actualStructuredContent)
				// also, check the metrics when the server runs on HTTP
				if td.name == "http" {
					// get the metrics after the call
					mcpCallsTotalMetricAfter, mcpCallsDurationSecondsInfBucketAfter := getMetrics(t, "http://localhost:50081", map[string]string{
						"method":  "tools/call",
						"name":    "unhealthyApplications",
						"success": "true",
					})
					assert.Equal(t, mcpCallsTotalMetricBefore+1, mcpCallsTotalMetricAfter)
					assert.Equal(t, mcpCallsDurationSecondsInfBucketBefore+1, mcpCallsDurationSecondsInfBucketAfter)
				}

			})

			t.Run("call/unhealthyApplicationResources/ok", func(t *testing.T) {
				var mcpCallsTotalMetricBefore int64
				var mcpCallsDurationSecondsInfBucketBefore int64
				if td.name == "http" {
					mcpCallsTotalMetricBefore, mcpCallsDurationSecondsInfBucketBefore = getMetrics(t, "http://localhost:50081", map[string]string{
						"method":  "tools/call",
						"name":    "unhealthyApplicationResources",
						"success": "true",
					})
				}

				// when
				result, err := session.CallTool(context.Background(), &mcp.CallToolParams{
					Name: "unhealthyApplicationResources",
					Arguments: map[string]any{
						"name": "example",
					},
				})

				// then
				require.NoError(t, err)
				expectedContent := argocd.UnhealthyResources{
					Resources: []argocdv3.ResourceStatus{
						{
							Group:     "apps",
							Version:   "v1",
							Kind:      "StatefulSet",
							Namespace: "example-ns",
							Name:      "example",
							Status:    "Synced",
							Health: &argocdv3.HealthStatus{
								Status:  "Progressing",
								Message: "Waiting for 1 pods to be ready...",
							},
						},
						{
							Group:     "external-secrets.io",
							Version:   "v1beta1",
							Kind:      "ExternalSecret",
							Namespace: "example-ns",
							Name:      "example-secret",
							Status:    "OutOfSync",
							Health: &argocdv3.HealthStatus{
								Status: "Missing",
							},
						},
						{
							Group:   "operator.tekton.dev",
							Version: "v1alpha1",
							Kind:    "TektonConfig",
							Name:    "config",
							Status:  "OutOfSync",
						},
					},
				}
				expectedResourcesText, err := json.Marshal(expectedContent)
				require.NoError(t, err)

				// verify the `text` result
				resultContent, ok := result.Content[0].(*mcp.TextContent)
				require.True(t, ok)
				assert.JSONEq(t, string(expectedResourcesText), resultContent.Text)

				// verify the `structured` content
				require.IsType(t, map[string]any{}, result.StructuredContent)
				actualStructuredContent := argocd.UnhealthyResources{}
				err = runtime.DefaultUnstructuredConverter.FromUnstructured(result.StructuredContent.(map[string]any), &actualStructuredContent)
				require.NoError(t, err)
				assert.Equal(t, expectedContent, actualStructuredContent)
				if td.name == "http" {
					// get the metrics after the call
					mcpCallsTotalMetricAfter, mcpCallsDurationSecondsInfBucketAfter := getMetrics(t, "http://localhost:50081", map[string]string{
						"method":  "tools/call",
						"name":    "unhealthyApplicationResources",
						"success": "true",
					})
					assert.Equal(t, mcpCallsTotalMetricBefore+1, mcpCallsTotalMetricAfter)
					assert.Equal(t, mcpCallsDurationSecondsInfBucketBefore+1, mcpCallsDurationSecondsInfBucketAfter)
				}
			})

			t.Run("call/unhealthyApplicationResources/argocd-error", func(t *testing.T) {
				var mcpCallsTotalMetricBefore int64
				var mcpCallsDurationSecondsInfBucketBefore int64
				if td.name == "http" {
					mcpCallsTotalMetricBefore, mcpCallsDurationSecondsInfBucketBefore = getMetrics(t, "http://localhost:50081", map[string]string{
						"method":  "tools/call",
						"name":    "unhealthyApplicationResources",
						"success": "false",
					})
				}

				// when
				result, err := session.CallTool(context.Background(), &mcp.CallToolParams{
					Name: "unhealthyApplicationResources",
					Arguments: map[string]any{
						"name": "example-error",
					},
				})

				// then
				require.NoError(t, err)
				assert.True(t, result.IsError)
				if td.name == "http" {
					// get the metrics after the call
					mcpCallsTotalMetricAfter, mcpCallsDurationSecondsInfBucketAfter := getMetrics(t, "http://localhost:50081", map[string]string{
						"method":  "tools/call",
						"name":    "unhealthyApplicationResources",
						"success": "false",
					})
					assert.Equal(t, mcpCallsTotalMetricBefore+1, mcpCallsTotalMetricAfter)
					assert.Equal(t, mcpCallsDurationSecondsInfBucketBefore+1, mcpCallsDurationSecondsInfBucketAfter)
				}
			})

			t.Run("verify/capabilities/listChanged", func(t *testing.T) {
				// Verify the ListChanged capability based on server mode
				expectedListChanged := td.name != "http-stateless"
				assertListChanged(t, session, expectedListChanged)
			})
		})
	}

	testdataUnreachable := []struct {
		name string
		init func(*testing.T) *mcp.ClientSession
	}{
		{
			name: "stdio-unreachable",
			init: newStdioSession(true, "http://localhost:50085", "another-token", true), // invalid URL and token for the Argo CD server
		},
		{
			name: "http-unreachable",
			init: newHTTPSession("http://localhost:50082/mcp"), // invalid URL and token for the Argo CD server
		},
	}

	// test stdio and http transports with an invalid Argo CD client
	for _, td := range testdataUnreachable {
		t.Run(td.name, func(t *testing.T) {
			// given
			session := td.init(t)
			defer session.Close()
			t.Run("call/unhealthyApplications/argocd-unreachable", func(t *testing.T) {
				// when
				result, err := session.CallTool(context.Background(), &mcp.CallToolParams{
					Name: "unhealthyApplications",
				})

				// then
				require.NoError(t, err)
				assert.True(t, result.IsError, "expected error, got %v", result)
			})
		})

	}
}

// TestStatelessMultipleReplicas verifies that multiple stateless server instances
// can serve requests independently without maintaining client state
func TestStatelessMultipleReplicas(t *testing.T) {
	// given - start 2 server instances with --stateless flag
	argocdURL := "http://localhost:50084"
	argocdToken := "secure-token"

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	// Start server instance 1 on port 8091
	server1 := startServer(t, newHTTPServerCmd(ctx, argocdURL, argocdToken, "localhost:8091", true, true), "localhost:8091")
	defer stopServer(t, server1)

	// Start server instance 2 on port 8092
	server2 := startServer(t, newHTTPServerCmd(ctx, argocdURL, argocdToken, "localhost:8092", true, true), "localhost:8092")
	defer stopServer(t, server2)

	// Wait for servers to be ready
	waitForServer(t, "http://localhost:8091/mcp")
	waitForServer(t, "http://localhost:8092/mcp")

	t.Run("multiple clients can connect to different replicas", func(t *testing.T) {
		session1, err := newClientSession(ctx, "http://localhost:8091/mcp", "e2e-test-client-1")
		require.NoError(t, err, "should connect to server 1")
		defer session1.Close()

		session2, err := newClientSession(ctx, "http://localhost:8092/mcp", "e2e-test-client-2")
		require.NoError(t, err, "should connect to server 2")
		defer session2.Close()

		// Verify both can list tools
		tools1, err := session1.ListTools(ctx, &mcp.ListToolsParams{})
		require.NoError(t, err)
		assert.NotEmpty(t, tools1.Tools, "server 1 should have tools")

		tools2, err := session2.ListTools(ctx, &mcp.ListToolsParams{})
		require.NoError(t, err)
		assert.NotEmpty(t, tools2.Tools, "server 2 should have tools")

		// Verify both have the same tools (stateless replicas are identical)
		assert.Len(t, tools2.Tools, len(tools1.Tools),
			"both replicas should have the same number of tools")
	})

	t.Run("round-robin requests work across replicas", func(t *testing.T) {
		// Simulate load balancing by alternating between servers
		servers := []string{
			"http://localhost:8091/mcp",
			"http://localhost:8092/mcp",
		}

		// Make 10 requests alternating between servers
		for i := 0; i < 10; i++ {
			serverURL := servers[i%len(servers)] //nolint:gosec // modulo with constant array length is safe
			t.Logf("Request %d to %s", i+1, serverURL)

			session, err := newClientSession(ctx, serverURL, fmt.Sprintf("e2e-test-client-%d", i))
			require.NoError(t, err, "should connect to %s on request %d", serverURL, i+1)

			// Make a request
			tools, err := session.ListTools(ctx, &mcp.ListToolsParams{})
			require.NoError(t, err, "should list tools on request %d", i+1)
			assert.NotEmpty(t, tools.Tools, "should have tools on request %d", i+1)

			session.Close()
		}
	})

	t.Run("verify stateless mode - no ListChanged notifications", func(t *testing.T) {
		session, err := newClientSession(ctx, "http://localhost:8091/mcp", "e2e-test-client-stateless-check")
		require.NoError(t, err)
		defer session.Close()

		assertListChanged(t, session, false)
	})

	t.Run("tools work correctly in stateless mode", func(t *testing.T) {
		session, err := newClientSession(ctx, "http://localhost:8091/mcp", "e2e-test-client-tool-check")
		require.NoError(t, err)
		defer session.Close()

		// Call a tool to verify it works in stateless mode
		result, err := session.CallTool(ctx, &mcp.CallToolParams{
			Name: "unhealthyApplications",
		})
		require.NoError(t, err)
		require.False(t, result.IsError, "tool call should succeed in stateless mode")
		assert.NotEmpty(t, result.Content, "tool should return content")
	})
}

// TestStatefulSingleReplica verifies that without --stateless flag, the server
// operates in stateful mode (for comparison)
func TestStatefulSingleReplica(t *testing.T) {
	argocdURL := "http://localhost:50084"
	argocdToken := "secure-token"

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Minute)
	defer cancel()

	// Start server WITHOUT --stateless flag
	server := startServer(t, newHTTPServerCmd(ctx, argocdURL, argocdToken, "localhost:8093", false, true), "localhost:8093")
	defer stopServer(t, server)

	waitForServer(t, "http://localhost:8093/mcp")

	t.Run("verify stateful mode - ListChanged enabled", func(t *testing.T) {
		session, err := newClientSession(ctx, "http://localhost:8093/mcp", "e2e-test-client-stateful-check")
		require.NoError(t, err)
		defer session.Close()

		assertListChanged(t, session, true)
	})

	t.Run("tools work correctly in stateful mode", func(t *testing.T) {
		session, err := newClientSession(ctx, "http://localhost:8093/mcp", "e2e-test-client-tool-check-stateful")
		require.NoError(t, err)
		defer session.Close()

		// Call a tool to verify it works in stateful mode
		result, err := session.CallTool(ctx, &mcp.CallToolParams{
			Name: "unhealthyApplications",
		})
		require.NoError(t, err)
		require.False(t, result.IsError, "tool call should succeed in stateful mode")
		assert.NotEmpty(t, result.Content, "tool should return content")
	})
}

func getMetrics(t *testing.T, mcpServerURL string, labels map[string]string) (int64, int64) { //nolint:unparam
	labelStrings := make([]string, 0, 2*len(labels))
	for k, v := range labels {
		labelStrings = append(labelStrings, k)
		labelStrings = append(labelStrings, v)
	}
	var mcpCallsTotalMetric int64
	var mcpCallsDurationSecondsInf int64

	if value, err := toolchaintests.GetMetricValue(&rest.Config{}, mcpServerURL, `mcp_calls_total`, labelStrings); err == nil {
		mcpCallsTotalMetric = int64(value)
	} else {
		t.Logf("failed to get mcp_calls_total metric, assuming 0: %v", err)
		mcpCallsTotalMetric = 0
	}
	if buckets, err := toolchaintests.GetHistogramBuckets(&rest.Config{}, mcpServerURL, `mcp_call_duration_seconds`, labelStrings); err == nil {
		for _, bucket := range buckets {
			if bucket.GetUpperBound() == math.Inf(1) {
				mcpCallsDurationSecondsInf = int64(bucket.GetCumulativeCount()) //nolint:gosec
				break
			}
		}
	}
	return mcpCallsTotalMetric, mcpCallsDurationSecondsInf
}

func newStdioSession(mcpServerDebug bool, argocdURL string, argocdToken string, argocdInsecureURL bool) func(*testing.T) *mcp.ClientSession {
	return func(t *testing.T) *mcp.ClientSession {
		ctx := context.Background()
		cmd := newStdioServerCmd(ctx, mcpServerDebug, argocdURL, argocdToken, argocdInsecureURL)
		cl := mcp.NewClient(&mcp.Implementation{Name: "e2e-test-client", Version: "v1.0.0"}, nil)
		session, err := cl.Connect(ctx, &mcp.CommandTransport{Command: cmd}, nil)
		require.NoError(t, err)
		return session
	}
}

func newHTTPSession(mcpServerURL string) func(*testing.T) *mcp.ClientSession {
	return func(t *testing.T) *mcp.ClientSession {
		ctx := context.Background()
		cl := mcp.NewClient(&mcp.Implementation{Name: "e2e-test-client", Version: "v1.0.0"}, nil)
		session, err := cl.Connect(ctx, &mcp.StreamableClientTransport{
			MaxRetries: 5,
			Endpoint:   mcpServerURL,
		}, nil)
		require.NoError(t, err)
		return session
	}
}

func newStdioServerCmd(ctx context.Context, mcpServerDebug bool, argocdURL string, argocdToken string, argocdInsecureURL bool) *exec.Cmd {
	return exec.CommandContext(ctx, //nolint:gosec
		"argocd-mcp-server",
		"--transport", "stdio",
		"--debug", strconv.FormatBool(mcpServerDebug),
		"--argocd-url", argocdURL,
		"--argocd-token", argocdToken,
		"--insecure", strconv.FormatBool(argocdInsecureURL),
	)
}

// Helper functions for stateless tests

func newHTTPServerCmd(ctx context.Context, argocdURL, argocdToken, listen string, stateless, insecure bool) *exec.Cmd {
	args := []string{
		"--argocd-url", argocdURL,
		"--argocd-token", argocdToken,
		"--transport", "http",
		"--listen", listen,
		"--debug",
	}
	if insecure {
		args = append(args, "--insecure")
	}
	if stateless {
		args = append(args, "--stateless")
	}
	return exec.CommandContext(ctx, "argocd-mcp-server", args...) //nolint:gosec
}

func startServer(t *testing.T, cmd *exec.Cmd, listen string) *exec.Cmd {
	t.Helper()
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Start()
	require.NoError(t, err, "should start server on %s", listen)
	t.Logf("Started server on %s (PID: %d)", listen, cmd.Process.Pid)
	return cmd
}

func stopServer(t *testing.T, cmd *exec.Cmd) {
	t.Helper()
	if cmd != nil && cmd.Process != nil {
		t.Logf("Stopping server PID %d", cmd.Process.Pid)
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	}
}

func waitForServer(t *testing.T, mcpURL string) {
	t.Helper()
	// Convert MCP URL to health endpoint URL (e.g., http://localhost:8091/mcp -> http://localhost:8091/health)
	healthURL := strings.Replace(mcpURL, "/mcp", "/health", 1)
	t.Logf("Waiting for server at %s to be ready", healthURL)

	client := &http.Client{Timeout: 2 * time.Second}
	deadline := time.Now().Add(30 * time.Second)

	for time.Now().Before(deadline) {
		resp, err := client.Get(healthURL)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				t.Logf("Server at %s is ready", healthURL)
				return
			}
		}
		time.Sleep(500 * time.Millisecond)
	}
	t.Fatalf("Timeout waiting for server at %s", healthURL)
}

func newClientSession(ctx context.Context, endpoint, clientName string) (*mcp.ClientSession, error) {
	client := mcp.NewClient(&mcp.Implementation{
		Name:    clientName,
		Version: "1.0.0",
	}, nil)
	return client.Connect(ctx, &mcp.StreamableClientTransport{
		MaxRetries: 5,
		Endpoint:   endpoint,
	}, nil)
}

func assertListChanged(t *testing.T, session *mcp.ClientSession, expected bool) {
	t.Helper()
	initResult := session.InitializeResult()
	require.NotNil(t, initResult, "should have initialize result")
	require.NotNil(t, initResult.Capabilities, "should have capabilities")

	if initResult.Capabilities.Tools != nil {
		assert.Equal(t, expected, initResult.Capabilities.Tools.ListChanged,
			"Tools.ListChanged should be %t", expected)
	}
	if initResult.Capabilities.Prompts != nil {
		assert.Equal(t, expected, initResult.Capabilities.Prompts.ListChanged,
			"Prompts.ListChanged should be %t", expected)
	}
}
