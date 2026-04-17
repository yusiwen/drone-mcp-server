package tool

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/drone/drone-go/drone"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type BuildHandler struct {
	client drone.Client
}

func NewBuildHandler(client drone.Client) *BuildHandler {
	return &BuildHandler{client: client}
}

type ListBuildsArgs struct {
	Owner string `json:"owner"`
	Repo  string `json:"repo"`
}

func (h *BuildHandler) HandleListBuilds(ctx context.Context, req *mcp.CallToolRequest, args ListBuildsArgs) (*mcp.CallToolResult, any, error) {
	builds, err := h.client.BuildList(args.Owner, args.Repo, drone.ListOptions{})
	if err != nil {
		return nil, nil, fmt.Errorf("failed to list builds: %w", err)
	}

	var buildList []string
	for _, build := range builds {
		buildList = append(buildList, fmt.Sprintf("#%d %s %s", build.Number, build.Status, build.Ref))
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: strings.Join(buildList, "\n")},
		},
	}, nil, nil
}

type GetBuildArgs struct {
	Owner string  `json:"owner"`
	Repo  string  `json:"repo"`
	Build float64 `json:"build"`
}

func (h *BuildHandler) HandleGetBuild(ctx context.Context, req *mcp.CallToolRequest, args GetBuildArgs) (*mcp.CallToolResult, any, error) {
	buildNum := int(args.Build)
	build, err := h.client.Build(args.Owner, args.Repo, buildNum)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get build: %w", err)
	}

	content := fmt.Sprintf("Build #%d\nStatus: %s\nRef: %s\nCommit: %s\nAuthor: %s\nStarted: %v\nEvent: %s\nAction: %s",
		build.Number, build.Status, build.Ref, build.After, build.Author, time.Unix(build.Timestamp, 0), build.Event, build.Action)

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: content},
		},
	}, nil, nil
}
