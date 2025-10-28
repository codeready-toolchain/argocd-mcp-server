package argocd

import (
	"context"
	"log/slog"
	"os"
	"testing"

	testresources "github.com/codeready-toolchain/argocd-mcp-server/test/resources"

	"github.com/h2non/gock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListUnhealthyApplications(t *testing.T) {

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
			BodyString(testresources.ApplicationsStr)
		defer gock.Off() // disable HTTP interceptor after test execution

		// when
		unhealthyApps, err := listUnhealthyApplications(context.Background(), logger, cl)

		// then
		require.NoError(t, err)
		assert.Equal(t, UnhealthyApplications{
			Degraded:    []string{"a-degraded-application", "another-degraded-application"},
			Progressing: []string{"a-progressing-application", "another-progressing-application"},
			OutOfSync:   []string{"an-out-of-sync-application", "another-out-of-sync-application"},
			Missing:     nil, // TODO: add missing applications
			Unknown:     nil, // TODO: add unknown applications
			Suspended:   nil, // TODO: add suspended applications
		}, unhealthyApps)
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
		_, err := listUnhealthyApplications(context.Background(), logger, cl)

		// then
		require.Error(t, err)
		assert.EqualError(t, err, "unexpected 500 response for GET http://argocd.example.com/api/v1/applications: mock error!")
	})
}
