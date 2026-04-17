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
		repoList = append(repoList, fmt.Sprintf("%s/%s", repo.Namespace, repo.Name))
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: strings.Join(repoList, "\n")},
		},
	}, nil, nil
}
