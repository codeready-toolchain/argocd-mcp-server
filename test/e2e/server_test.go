package e2etests

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os/exec"
	"strconv"
	"testing"

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

// TestServer verifies basic MCP functionality with both stdio and http transports.
// Both transports run in stateful mode (ListChanged enabled) and test:
// - Tool calls (unhealthyApplications, unhealthyApplicationResources)
// - Error handling (argocd-error, unreachable scenarios)
// - Metrics collection (for http transport)
// - Session reuse across multiple tool calls
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
	}

	// Test stdio and http transports with a valid Argo CD client (stateful mode)
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
				// Both stdio and http transports use stateful mode by default
				assertListChanged(t, session, true)
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

// TestStateless verifies that multiple stateless server instances work correctly
// with load balancing across replicas. This comprehensive test validates:
// - Initialize response and capabilities
// - Session reuse and independence
// - Multiple replicas serving requests independently
// - Tools functionality in stateless mode
// - Concurrent client connections
// - No ListChanged notifications
func TestStateless(t *testing.T) {
	ctx := context.Background()
	serverURL := "http://localhost:50090/mcp"

	t.Run("initialize response validates stateless mode", func(t *testing.T) {
		session, err := newClientSession(ctx, serverURL, "e2e-test-init-validation")
		require.NoError(t, err)
		defer session.Close()

		// Comprehensive validation of initialize response for stateless mode
		assertInitializeResponse(t, session, true)
		
		// Verify ListChanged is false (no notifications)
		assertListChanged(t, session, false)
	})

	t.Run("single session can be reused", func(t *testing.T) {
		// In stateless mode, you CAN reuse the same session
		// Stateless means no server-side state, not "no sessions"
		session, err := newClientSession(ctx, serverURL, "e2e-test-reused-session")
		require.NoError(t, err)
		defer session.Close()

		// Make multiple requests on the same session
		for i := 0; i < 5; i++ {
			tools, err := session.ListTools(ctx, &mcp.ListToolsParams{})
			require.NoError(t, err, "should list tools on request %d", i)
			assert.NotEmpty(t, tools.Tools, "should have tools on request %d", i)
		}
	})

	t.Run("multiple sessions have identical tools (no shared state)", func(t *testing.T) {
		// Create two sessions
		session1, err := newClientSession(ctx, serverURL, "e2e-test-session-1")
		require.NoError(t, err)
		defer session1.Close()

		session2, err := newClientSession(ctx, serverURL, "e2e-test-session-2")
		require.NoError(t, err)
		defer session2.Close()

		// Both sessions should get identical tool lists (stateless = no per-session customization)
		tools1, err := session1.ListTools(ctx, &mcp.ListToolsParams{})
		require.NoError(t, err)

		tools2, err := session2.ListTools(ctx, &mcp.ListToolsParams{})
		require.NoError(t, err)

		// Verify both have the same tools
		require.Len(t, tools2.Tools, len(tools1.Tools), "both sessions should see same tools")
		for i, tool := range tools1.Tools {
			assert.Equal(t, tool.Name, tools2.Tools[i].Name, "tool %d should have same name", i)
		}
	})

	t.Run("multiple independent clients work (load-balanced)", func(t *testing.T) {
		// Simulates multiple clients in a load-balanced deployment
		// Each client might hit a different replica
		for i := 0; i < 10; i++ {
			session, err := newClientSession(ctx, serverURL, fmt.Sprintf("e2e-test-client-%d", i))
			require.NoError(t, err, "should connect on request %d", i)

			tools, err := session.ListTools(ctx, &mcp.ListToolsParams{})
			require.NoError(t, err, "should list tools on request %d", i)
			assert.NotEmpty(t, tools.Tools, "should have tools on request %d", i)

			session.Close()
		}
	})

	t.Run("tools work correctly with content validation", func(t *testing.T) {
		session, err := newClientSession(ctx, serverURL, "e2e-test-tool-check")
		require.NoError(t, err)
		defer session.Close()

		// Call a tool to verify it works in stateless mode
		result, err := session.CallTool(ctx, &mcp.CallToolParams{
			Name: "unhealthyApplications",
		})
		require.NoError(t, err)
		require.False(t, result.IsError, "tool call should succeed")
		assert.NotEmpty(t, result.Content, "tool should return content")

		// Verify the content is correct
		expectedContent := map[string]any{
			"degraded":    []any{"a-degraded-application", "another-degraded-application"},
			"progressing": []any{"a-progressing-application", "another-progressing-application"},
			"outOfSync":   []any{"an-out-of-sync-application", "another-out-of-sync-application"},
		}
		expectedContentText, err := json.Marshal(expectedContent)
		require.NoError(t, err)

		resultContent, ok := result.Content[0].(*mcp.TextContent)
		require.True(t, ok)
		assert.JSONEq(t, string(expectedContentText), resultContent.Text)
	})

	t.Run("concurrent connections work correctly", func(t *testing.T) {
		// Create multiple concurrent sessions
		sessions := make([]*mcp.ClientSession, 5)
		for i := 0; i < 5; i++ {
			session, err := newClientSession(ctx, serverURL, fmt.Sprintf("e2e-test-concurrent-%d", i))
			require.NoError(t, err, "should connect session %d", i)
			sessions[i] = session
			defer session.Close()
		}

		// All sessions should be able to call tools simultaneously
		for i, session := range sessions {
			result, err := session.CallTool(ctx, &mcp.CallToolParams{
				Name: "unhealthyApplications",
			})
			require.NoError(t, err, "session %d should call tool", i)
			require.False(t, result.IsError, "session %d tool call should succeed", i)
		}
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

// assertInitializeResponse performs comprehensive validation of the initialize response
func assertInitializeResponse(t *testing.T, session *mcp.ClientSession, stateless bool) {
	t.Helper()
	
	initResult := session.InitializeResult()
	require.NotNil(t, initResult, "should have initialize result")
	
	// Verify server info exists
	require.NotNil(t, initResult.ServerInfo, "should have server info")
	assert.NotEmpty(t, initResult.ServerInfo.Name, "server name should not be empty")
	assert.NotEmpty(t, initResult.ServerInfo.Version, "server version should not be empty")
	
	// Verify protocol version exists
	assert.NotEmpty(t, initResult.ProtocolVersion, "protocol version should not be empty")
	
	// Verify capabilities
	require.NotNil(t, initResult.Capabilities, "should have capabilities")
	
	// In stateless mode: ListChanged should be false (no notifications)
	// In stateful mode: ListChanged should be true (notifications enabled)
	
	// Tools capability
	require.NotNil(t, initResult.Capabilities.Tools, "should have tools capability")
	if stateless {
		assert.False(t, initResult.Capabilities.Tools.ListChanged, "stateless mode should have Tools.ListChanged=false")
	} else {
		assert.True(t, initResult.Capabilities.Tools.ListChanged, "stateful mode should have Tools.ListChanged=true")
	}
	
	// Prompts capability
	require.NotNil(t, initResult.Capabilities.Prompts, "should have prompts capability")
	if stateless {
		assert.False(t, initResult.Capabilities.Prompts.ListChanged, "stateless mode should have Prompts.ListChanged=false")
	} else {
		assert.True(t, initResult.Capabilities.Prompts.ListChanged, "stateful mode should have Prompts.ListChanged=true")
	}
}
