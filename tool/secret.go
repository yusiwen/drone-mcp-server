package tool

import (
	"context"
	"fmt"
	"strings"

	"github.com/drone/drone-go/drone"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type SecretHandler struct {
	client drone.Client
}

func NewSecretHandler(client drone.Client) *SecretHandler {
	return &SecretHandler{client: client}
}

type ListSecretsArgs struct {
	Owner string `json:"owner"`
	Repo  string `json:"repo"`
}

func (h *SecretHandler) HandleListSecrets(ctx context.Context, req *mcp.CallToolRequest, args ListSecretsArgs) (*mcp.CallToolResult, any, error) {
	secrets, err := h.client.SecretList(args.Owner, args.Repo)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to list secrets: %w", err)
	}

	var secretList []string
	for _, secret := range secrets {
		secretList = append(secretList, fmt.Sprintf("%s (PullRequest: %v, PullRequestPush: %v)",
			secret.Name, secret.PullRequest, secret.PullRequestPush))
	}

	content := fmt.Sprintf("Secrets for %s/%s (%d):\n%s", args.Owner, args.Repo, len(secrets), strings.Join(secretList, "\n"))

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: content},
		},
	}, nil, nil
}

type GetSecretArgs struct {
	Owner string `json:"owner"`
	Repo  string `json:"repo"`
	Name  string `json:"name"`
}

func (h *SecretHandler) HandleGetSecret(ctx context.Context, req *mcp.CallToolRequest, args GetSecretArgs) (*mcp.CallToolResult, any, error) {
	secret, err := h.client.Secret(args.Owner, args.Repo, args.Name)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get secret: %w", err)
	}

	content := fmt.Sprintf("Secret: %s\nPullRequest: %v\nPullRequestPush: %v\nNamespace: %s",
		secret.Name, secret.PullRequest, secret.PullRequestPush, secret.Namespace)

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: content},
		},
	}, nil, nil
}

type CreateSecretArgs struct {
	Owner           string `json:"owner"`
	Repo            string `json:"repo"`
	Name            string `json:"name"`
	Value           string `json:"value"`
	PullRequest     bool   `json:"pull_request,omitempty"`
	PullRequestPush bool   `json:"pull_request_push,omitempty"`
}

func (h *SecretHandler) HandleCreateSecret(ctx context.Context, req *mcp.CallToolRequest, args CreateSecretArgs) (*mcp.CallToolResult, any, error) {
	secret := &drone.Secret{
		Name:            args.Name,
		Data:            args.Value,
		PullRequest:     args.PullRequest,
		PullRequestPush: args.PullRequestPush,
	}

	createdSecret, err := h.client.SecretCreate(args.Owner, args.Repo, secret)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create secret: %w", err)
	}

	content := fmt.Sprintf("Secret created successfully: %s\nPullRequest: %v\nPullRequestPush: %v",
		createdSecret.Name, createdSecret.PullRequest, createdSecret.PullRequestPush)

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: content},
		},
	}, nil, nil
}

type UpdateSecretArgs struct {
	Owner           string `json:"owner"`
	Repo            string `json:"repo"`
	Name            string `json:"name"`
	Value           string `json:"value"`
	PullRequest     bool   `json:"pull_request,omitempty"`
	PullRequestPush bool   `json:"pull_request_push,omitempty"`
}

func (h *SecretHandler) HandleUpdateSecret(ctx context.Context, req *mcp.CallToolRequest, args UpdateSecretArgs) (*mcp.CallToolResult, any, error) {
	secret := &drone.Secret{
		Name:            args.Name,
		Data:            args.Value,
		PullRequest:     args.PullRequest,
		PullRequestPush: args.PullRequestPush,
	}

	updatedSecret, err := h.client.SecretUpdate(args.Owner, args.Repo, secret)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to update secret: %w", err)
	}

	content := fmt.Sprintf("Secret updated successfully: %s\nPullRequest: %v\nPullRequestPush: %v",
		updatedSecret.Name, updatedSecret.PullRequest, updatedSecret.PullRequestPush)

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: content},
		},
	}, nil, nil
}

type DeleteSecretArgs struct {
	Owner string `json:"owner"`
	Repo  string `json:"repo"`
	Name  string `json:"name"`
}

func (h *SecretHandler) HandleDeleteSecret(ctx context.Context, req *mcp.CallToolRequest, args DeleteSecretArgs) (*mcp.CallToolResult, any, error) {
	err := h.client.SecretDelete(args.Owner, args.Repo, args.Name)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to delete secret: %w", err)
	}

	content := fmt.Sprintf("Secret deleted successfully: %s", args.Name)

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: content},
		},
	}, nil, nil
}

// Organization secrets
type ListOrgSecretsArgs struct {
	Namespace string `json:"namespace"`
}

func (h *SecretHandler) HandleListOrgSecrets(ctx context.Context, req *mcp.CallToolRequest, args ListOrgSecretsArgs) (*mcp.CallToolResult, any, error) {
	secrets, err := h.client.OrgSecretList(args.Namespace)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to list org secrets: %w", err)
	}

	var secretList []string
	for _, secret := range secrets {
		secretList = append(secretList, fmt.Sprintf("%s (PullRequest: %v, PullRequestPush: %v)",
			secret.Name, secret.PullRequest, secret.PullRequestPush))
	}

	content := fmt.Sprintf("Organization secrets for %s (%d):\n%s", args.Namespace, len(secrets), strings.Join(secretList, "\n"))

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: content},
		},
	}, nil, nil
}

type GetOrgSecretArgs struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
}

func (h *SecretHandler) HandleGetOrgSecret(ctx context.Context, req *mcp.CallToolRequest, args GetOrgSecretArgs) (*mcp.CallToolResult, any, error) {
	secret, err := h.client.OrgSecret(args.Namespace, args.Name)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get org secret: %w", err)
	}

	content := fmt.Sprintf("Organization secret: %s\nPullRequest: %v\nPullRequestPush: %v",
		secret.Name, secret.PullRequest, secret.PullRequestPush)

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: content},
		},
	}, nil, nil
}

type CreateOrgSecretArgs struct {
	Namespace       string `json:"namespace"`
	Name            string `json:"name"`
	Value           string `json:"value"`
	PullRequest     bool   `json:"pull_request,omitempty"`
	PullRequestPush bool   `json:"pull_request_push,omitempty"`
}

func (h *SecretHandler) HandleCreateOrgSecret(ctx context.Context, req *mcp.CallToolRequest, args CreateOrgSecretArgs) (*mcp.CallToolResult, any, error) {
	secret := &drone.Secret{
		Name:            args.Name,
		Data:            args.Value,
		PullRequest:     args.PullRequest,
		PullRequestPush: args.PullRequestPush,
	}

	createdSecret, err := h.client.OrgSecretCreate(args.Namespace, secret)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create org secret: %w", err)
	}

	content := fmt.Sprintf("Organization secret created successfully: %s\nPullRequest: %v\nPullRequestPush: %v",
		createdSecret.Name, createdSecret.PullRequest, createdSecret.PullRequestPush)

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: content},
		},
	}, nil, nil
}

type UpdateOrgSecretArgs struct {
	Namespace       string `json:"namespace"`
	Name            string `json:"name"`
	Value           string `json:"value"`
	PullRequest     bool   `json:"pull_request,omitempty"`
	PullRequestPush bool   `json:"pull_request_push,omitempty"`
}

func (h *SecretHandler) HandleUpdateOrgSecret(ctx context.Context, req *mcp.CallToolRequest, args UpdateOrgSecretArgs) (*mcp.CallToolResult, any, error) {
	secret := &drone.Secret{
		Name:            args.Name,
		Data:            args.Value,
		PullRequest:     args.PullRequest,
		PullRequestPush: args.PullRequestPush,
	}

	updatedSecret, err := h.client.OrgSecretUpdate(args.Namespace, secret)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to update org secret: %w", err)
	}

	content := fmt.Sprintf("Organization secret updated successfully: %s\nPullRequest: %v\nPullRequestPush: %v",
		updatedSecret.Name, updatedSecret.PullRequest, updatedSecret.PullRequestPush)

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: content},
		},
	}, nil, nil
}

type DeleteOrgSecretArgs struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
}

func (h *SecretHandler) HandleDeleteOrgSecret(ctx context.Context, req *mcp.CallToolRequest, args DeleteOrgSecretArgs) (*mcp.CallToolResult, any, error) {
	err := h.client.OrgSecretDelete(args.Namespace, args.Name)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to delete org secret: %w", err)
	}

	content := fmt.Sprintf("Organization secret deleted successfully: %s", args.Name)

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: content},
		},
	}, nil, nil
}
