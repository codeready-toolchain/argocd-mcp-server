package argocd

import (
	"context"
	"log/slog"
	"os"
	"testing"

	testresources "github.com/codeready-toolchain/argocd-mcp-server/test/resources"

	argocdv3 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/h2non/gock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListUnhealthyApplicationResources(t *testing.T) {

	t.Run("ok", func(t *testing.T) {
		// given
		cl := NewClient("http://argocd.example.com", "secure-token", false)
		logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
			Level: slog.LevelDebug,
		}))
		gock.New("http://argocd.example.com").
			Get("/api/v1/applications").
			MatchHeader("Authorization", "Bearer secure-token").
			Reply(200).
			BodyString(testresources.ExampleApplicationStr)
		defer gock.Off() // disable HTTP interceptor after test execution

		// when
		unhealthyResources, err := listUnhealthyApplicationResources(context.Background(), logger, cl, "example")

		// then
		require.NoError(t, err)
		assert.Equal(t, UnhealthyResources{
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
		}, unhealthyResources)
	})

	t.Run("failure", func(t *testing.T) {
		// given
		cl := NewClient("http://argocd.example.com", "secure-token", false)
		logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
			Level: slog.LevelDebug,
		}))

		gock.New("http://argocd.example.com").
			Get("/api/v1/applications").
			MatchHeader("Authorization", "Bearer secure-token").
			Reply(500).
			BodyString("mock error!")
		defer gock.Off() // disable HTTP interceptor after test execution

		// when
		_, err := listUnhealthyApplicationResources(context.Background(), logger, cl, "example")

		// then
		require.Error(t, err)
		assert.EqualError(t, err, "unexpected 500 response for GET http://argocd.example.com/api/v1/applications?name=example: mock error!")
	})
}
