package tool

import (
	"context"
	"fmt"
	"strings"

	"github.com/drone/drone-go/drone"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type RepoHandler struct {
	client drone.Client
}

func NewRepoHandler(client drone.Client) *RepoHandler {
	return &RepoHandler{client: client}
}

type ListReposArgs struct{}

func (h *RepoHandler) HandleListRepos(ctx context.Context, req *mcp.CallToolRequest, args ListReposArgs) (*mcp.CallToolResult, any, error) {
	repos, err := h.client.RepoList()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to list repos: %w", err)
	}

	var repoList []string
	for _, repo := range repos {
		repoList = append(repoList, fmt.Sprintf("%s/%s (Active: %v)", repo.Namespace, repo.Name, repo.Active))
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: strings.Join(repoList, "\n")},
		},
	}, nil, nil
}

type GetRepoArgs struct {
	Owner string `json:"owner"`
	Repo  string `json:"repo"`
}

func (h *RepoHandler) HandleGetRepo(ctx context.Context, req *mcp.CallToolRequest, args GetRepoArgs) (*mcp.CallToolResult, any, error) {
	repo, err := h.client.Repo(args.Owner, args.Repo)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get repo: %w", err)
	}

	content := fmt.Sprintf("Repository: %s/%s\nActive: %v\nVisibility: %s\nConfig: %s\nTrusted: %v\nProtected: %v\nTimeout: %d\nCounter: %d",
		repo.Namespace, repo.Name, repo.Active, repo.Visibility, repo.Config, repo.Trusted, repo.Protected, repo.Timeout, repo.Counter)

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: content},
		},
	}, nil, nil
}

type EnableRepoArgs struct {
	Owner string `json:"owner"`
	Repo  string `json:"repo"`
}

func (h *RepoHandler) HandleEnableRepo(ctx context.Context, req *mcp.CallToolRequest, args EnableRepoArgs) (*mcp.CallToolResult, any, error) {
	repo, err := h.client.RepoEnable(args.Owner, args.Repo)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to enable repo: %w", err)
	}

	content := fmt.Sprintf("Repository enabled successfully: %s/%s\nActive: %v", repo.Namespace, repo.Name, repo.Active)

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: content},
		},
	}, nil, nil
}

type DisableRepoArgs struct {
	Owner string `json:"owner"`
	Repo  string `json:"repo"`
}

func (h *RepoHandler) HandleDisableRepo(ctx context.Context, req *mcp.CallToolRequest, args DisableRepoArgs) (*mcp.CallToolResult, any, error) {
	err := h.client.RepoDisable(args.Owner, args.Repo)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to disable repo: %w", err)
	}

	content := fmt.Sprintf("Repository disabled successfully: %s/%s", args.Owner, args.Repo)

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: content},
		},
	}, nil, nil
}

type RepairRepoArgs struct {
	Owner string `json:"owner"`
	Repo  string `json:"repo"`
}

func (h *RepoHandler) HandleRepairRepo(ctx context.Context, req *mcp.CallToolRequest, args RepairRepoArgs) (*mcp.CallToolResult, any, error) {
	err := h.client.RepoRepair(args.Owner, args.Repo)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to repair repo: %w", err)
	}

	content := fmt.Sprintf("Repository repair initiated: %s/%s", args.Owner, args.Repo)

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: content},
		},
	}, nil, nil
}

type ChownRepoArgs struct {
	Owner string `json:"owner"`
	Repo  string `json:"repo"`
}

func (h *RepoHandler) HandleChownRepo(ctx context.Context, req *mcp.CallToolRequest, args ChownRepoArgs) (*mcp.CallToolResult, any, error) {
	repo, err := h.client.RepoChown(args.Owner, args.Repo)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to change repo ownership: %w", err)
	}

	content := fmt.Sprintf("Repository ownership changed: %s/%s", repo.Namespace, repo.Name)

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: content},
		},
	}, nil, nil
}

type SyncReposArgs struct{}

func (h *RepoHandler) HandleSyncRepos(ctx context.Context, req *mcp.CallToolRequest, args SyncReposArgs) (*mcp.CallToolResult, any, error) {
	repos, err := h.client.RepoListSync()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to sync repos: %w", err)
	}

	var repoList []string
	for _, repo := range repos {
		repoList = append(repoList, fmt.Sprintf("%s/%s (Active: %v)", repo.Namespace, repo.Name, repo.Active))
	}

	content := fmt.Sprintf("Synced %d repositories:\n%s", len(repos), strings.Join(repoList, "\n"))

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: content},
		},
	}, nil, nil
}

type ListIncompleteArgs struct{}

func (h *RepoHandler) HandleListIncomplete(ctx context.Context, req *mcp.CallToolRequest, args ListIncompleteArgs) (*mcp.CallToolResult, any, error) {
	repos, err := h.client.Incomplete()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to list incomplete builds: %w", err)
	}

	var repoList []string
	for _, repo := range repos {
		repoList = append(repoList, fmt.Sprintf("%s/%s", repo.Namespace, repo.Name))
	}

	content := fmt.Sprintf("Repositories with incomplete builds (%d):\n%s", len(repos), strings.Join(repoList, "\n"))

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: content},
		},
	}, nil, nil
}
