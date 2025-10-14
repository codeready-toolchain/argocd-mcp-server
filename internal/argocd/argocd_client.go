package argocd

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	argocdv3 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
)

type Client struct {
	*http.Client
	host  string
	token string
}

func NewClient(host string, token string, insecure bool) *Client {
	cl := http.DefaultClient
	if insecure {
		cl.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: insecure, //nolint:gosec
			},
		}
	}
	host = strings.TrimSuffix(host, "/")
	return &Client{
		Client: cl,
		host:   host,
		token:  token,
	}
}

func (c *Client) GetApplicationsWithContext(ctx context.Context) (*argocdv3.ApplicationList, error) {
	body, err := c.doRequestWithContext(ctx, "GET", "api/v1/applications")
	if err != nil {
		return nil, err
	}
	apps := &argocdv3.ApplicationList{}
	if err = json.Unmarshal(body, apps); err != nil {
		return nil, fmt.Errorf("failed to unmarshal application list: %w", err)
	}
	return apps, nil
}

func (c *Client) GetApplicationWithContext(ctx context.Context, name string) (*argocdv3.Application, error) {
	body, err := c.doRequestWithContext(ctx, "GET", fmt.Sprintf("api/v1/applications?name=%s", url.QueryEscape(name)))
	if err != nil {
		return nil, err
	}
	apps := &argocdv3.ApplicationList{}
	if err = json.Unmarshal(body, apps); err != nil {
		return nil, fmt.Errorf("failed to unmarshal application '%s': %w", name, err)
	}
	if len(apps.Items) == 0 {
		return nil, fmt.Errorf("no application found with name '%s'", name)
	}
	return &apps.Items[0], nil
}

func (c *Client) doRequestWithContext(ctx context.Context, method, path string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, method, fmt.Sprintf("%s/%s", c.host, path), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.token))
	resp, err := c.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to %s %s: %w", method, path, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read HTTP response body: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("unexpected %d response for %s %s/%s: %s", resp.StatusCode, method, c.host, path, string(body))
	}
	return body, nil
}
