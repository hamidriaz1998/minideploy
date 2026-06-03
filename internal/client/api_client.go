package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/hamid/minideploy/internal/shared"
)

type APIClient struct {
	BaseURL    string
	APIKey     string
	HTTPClient *http.Client
}

func NewAPIClient(host string, port int, apiKey string) *APIClient {
	baseURL := fmt.Sprintf("http://%s:%d/api/v1", host, port)
	return &APIClient{
		BaseURL: baseURL,
		APIKey:  apiKey,
		HTTPClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

func (c *APIClient) do(method, path string, body interface{}) (*shared.APIEnvelope, error) {
	var reqBody io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal request: %w", err)
		}
		reqBody = bytes.NewReader(data)
	}

	req, err := http.NewRequest(method, c.BaseURL+path, reqBody)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	var env shared.APIEnvelope
	if err := json.NewDecoder(resp.Body).Decode(&env); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &env, nil
}

func (c *APIClient) Deploy(req shared.DeployRequest) (*shared.DeployResponse, error) {
	env, err := c.do("POST", "/deploy", req)
	if err != nil {
		return nil, err
	}
	if !env.Success {
		return nil, fmt.Errorf("deploy failed: %s", env.Error)
	}

	data, err := json.Marshal(env.Data)
	if err != nil {
		return nil, fmt.Errorf("marshal response: %w", err)
	}

	var resp shared.DeployResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	return &resp, nil
}

func (c *APIClient) Rollback(req shared.RollbackRequest) (*shared.RollbackResponse, error) {
	env, err := c.do("POST", "/rollback", req)
	if err != nil {
		return nil, err
	}
	if !env.Success {
		return nil, fmt.Errorf("rollback failed: %s", env.Error)
	}

	data, _ := json.Marshal(env.Data)
	var resp shared.RollbackResponse
	json.Unmarshal(data, &resp)
	return &resp, nil
}

func (c *APIClient) Status() (*shared.StatusResponse, error) {
	env, err := c.do("GET", "/status", nil)
	if err != nil {
		return nil, err
	}

	data, _ := json.Marshal(env.Data)
	var resp shared.StatusResponse
	json.Unmarshal(data, &resp)
	return &resp, nil
}

func (c *APIClient) ListApps() ([]shared.AppSummary, error) {
	env, err := c.do("GET", "/apps", nil)
	if err != nil {
		return nil, err
	}

	data, _ := json.Marshal(env.Data)
	var apps []shared.AppSummary
	json.Unmarshal(data, &apps)
	return apps, nil
}

func (c *APIClient) AppDetail(name string) (*shared.AppDetail, error) {
	env, err := c.do("GET", "/apps/"+name, nil)
	if err != nil {
		return nil, err
	}

	data, _ := json.Marshal(env.Data)
	var app shared.AppDetail
	json.Unmarshal(data, &app)
	return &app, nil
}

func (c *APIClient) AppStatus(name string) (*shared.AppStatus, error) {
	env, err := c.do("GET", fmt.Sprintf("/apps/%s/status", name), nil)
	if err != nil {
		return nil, err
	}

	data, _ := json.Marshal(env.Data)
	var st shared.AppStatus
	json.Unmarshal(data, &st)
	return &st, nil
}

func (c *APIClient) AppReleases(name string) ([]shared.Release, error) {
	env, err := c.do("GET", fmt.Sprintf("/apps/%s/releases", name), nil)
	if err != nil {
		return nil, err
	}

	data, _ := json.Marshal(env.Data)
	var releases []shared.Release
	json.Unmarshal(data, &releases)
	return releases, nil
}

func (c *APIClient) AppLogs(name string) (string, error) {
	req, err := http.NewRequest("GET", c.BaseURL+"/apps/"+name+"/logs", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+c.APIKey)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return string(data), nil
}
