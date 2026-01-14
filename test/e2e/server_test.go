package e2etests

import (
	"context"
	"encoding/json"
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
