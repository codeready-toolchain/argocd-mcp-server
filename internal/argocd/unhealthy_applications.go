package argocd

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	argocdv3 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	argocdhealth "github.com/argoproj/gitops-engine/pkg/health"
)

var UnhealthyApplicationsTool = &mcp.Tool{
	Name:         "argocd_list_unhealthy_applications",
	Description:  "list the unhealthy ('degraded' and 'progressing') Applications in Argo CD",
	InputSchema:  UnhealthyApplicationsInputSchema,
	OutputSchema: UnhealthyApplicationsOutputSchema,
}

type UnhealthyApplicationsInput struct {
}

var UnhealthyApplicationsInputSchema, _ = jsonschema.For[UnhealthyApplicationsInput](&jsonschema.ForOptions{})

type UnhealthyApplicationsOutput UnhealthyApplications

var UnhealthyApplicationsOutputSchema, _ = jsonschema.For[UnhealthyApplicationsOutput](&jsonschema.ForOptions{})

func UnhealthyApplicationsToolHandle(logger *slog.Logger, cl *Client) mcp.ToolHandlerFor[UnhealthyApplicationsInput, UnhealthyApplicationsOutput] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, _ UnhealthyApplicationsInput) (*mcp.CallToolResult, UnhealthyApplicationsOutput, error) {
		apps, err := listUnhealthyApplications(ctx, logger, cl)
		if err != nil {
			return nil, UnhealthyApplicationsOutput{}, err
		}
		return nil, UnhealthyApplicationsOutput(apps), nil
	}
}

type UnhealthyApplications struct {
	Degraded    []string `json:"degraded,omitempty"`
	Progressing []string `json:"progressing,omitempty"`
	Missing     []string `json:"missing,omitempty"`
	Unknown     []string `json:"unknown,omitempty"`
	Suspended   []string `json:"suspended,omitempty"`
	OutOfSync   []string `json:"outOfSync,omitempty"`
}

// returns the name of the applications grouped by their health status
func listUnhealthyApplications(ctx context.Context, logger *slog.Logger, cl *Client) (UnhealthyApplications, error) {
	apps, err := cl.GetApplicationsWithContext(ctx)
	if err != nil {
		return UnhealthyApplications{}, err
	}
	unhealthyApps := UnhealthyApplications{
		Degraded:    []string{},
		Progressing: []string{},
		OutOfSync:   []string{},
	}
	for _, app := range apps.Items {
		switch app.Status.Health.Status {
		case argocdhealth.HealthStatusDegraded:
			unhealthyApps.Degraded = append(unhealthyApps.Degraded, app.Name)
		case argocdhealth.HealthStatusProgressing:
			unhealthyApps.Progressing = append(unhealthyApps.Progressing, app.Name)
		case argocdhealth.HealthStatusMissing:
			unhealthyApps.Missing = append(unhealthyApps.Missing, app.Name)
		case argocdhealth.HealthStatusUnknown:
			unhealthyApps.Unknown = append(unhealthyApps.Unknown, app.Name)
		case argocdhealth.HealthStatusSuspended:
			unhealthyApps.Suspended = append(unhealthyApps.Suspended, app.Name)
		case argocdhealth.HealthStatusHealthy:
			if app.Status.Sync.Status == argocdv3.SyncStatusCodeOutOfSync {
				unhealthyApps.OutOfSync = append(unhealthyApps.OutOfSync, app.Name)
			}
		default:
			// skip healthy/synced apps
		}
	}

	if logger.Enabled(ctx, slog.LevelDebug) {
		unhealthyAppsStr, err := json.Marshal(unhealthyApps)
		if err != nil {
			logger.Error("failed to convert unhealthy resources to text", "error", err.Error())
		}
		logger.DebugContext(ctx, "returned 'tools/call' response", "tool", "argocd_list_unhealthy_applications", "result", string(unhealthyAppsStr))
	}
	return unhealthyApps, nil
}
