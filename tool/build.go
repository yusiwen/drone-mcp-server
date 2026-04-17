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

type GetBuildLastArgs struct {
	Owner  string `json:"owner"`
	Repo   string `json:"repo"`
	Branch string `json:"branch,omitempty"`
}

func (h *BuildHandler) HandleGetBuildLast(ctx context.Context, req *mcp.CallToolRequest, args GetBuildLastArgs) (*mcp.CallToolResult, any, error) {
	build, err := h.client.BuildLast(args.Owner, args.Repo, args.Branch)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get last build: %w", err)
	}

	content := fmt.Sprintf("Latest Build #%d\nStatus: %s\nRef: %s\nCommit: %s\nAuthor: %s\nStarted: %v\nEvent: %s\nAction: %s",
		build.Number, build.Status, build.Ref, build.After, build.Author, time.Unix(build.Timestamp, 0), build.Event, build.Action)

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: content},
		},
	}, nil, nil
}

type BuildLogsArgs struct {
	Owner string  `json:"owner"`
	Repo  string  `json:"repo"`
	Build float64 `json:"build"`
	Stage float64 `json:"stage"`
	Step  float64 `json:"step"`
}

func (h *BuildHandler) HandleBuildLogs(ctx context.Context, req *mcp.CallToolRequest, args BuildLogsArgs) (*mcp.CallToolResult, any, error) {
	buildNum := int(args.Build)
	stageNum := int(args.Stage)
	stepNum := int(args.Step)

	logs, err := h.client.Logs(args.Owner, args.Repo, buildNum, stageNum, stepNum)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get build logs: %w", err)
	}

	var logLines []string
	for _, line := range logs {
		logLines = append(logLines, fmt.Sprintf("[%d] %s", line.Number, line.Message))
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: strings.Join(logLines, "\n")},
		},
	}, nil, nil
}

type RestartBuildArgs struct {
	Owner  string            `json:"owner"`
	Repo   string            `json:"repo"`
	Build  float64           `json:"build"`
	Params map[string]string `json:"params,omitempty"`
}

func (h *BuildHandler) HandleRestartBuild(ctx context.Context, req *mcp.CallToolRequest, args RestartBuildArgs) (*mcp.CallToolResult, any, error) {
	buildNum := int(args.Build)
	build, err := h.client.BuildRestart(args.Owner, args.Repo, buildNum, args.Params)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to restart build: %w", err)
	}

	content := fmt.Sprintf("Build #%d restarted successfully\nNew build: #%d", buildNum, build.Number)

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: content},
		},
	}, nil, nil
}

type CancelBuildArgs struct {
	Owner string  `json:"owner"`
	Repo  string  `json:"repo"`
	Build float64 `json:"build"`
}

func (h *BuildHandler) HandleCancelBuild(ctx context.Context, req *mcp.CallToolRequest, args CancelBuildArgs) (*mcp.CallToolResult, any, error) {
	buildNum := int(args.Build)
	err := h.client.BuildCancel(args.Owner, args.Repo, buildNum)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to cancel build: %w", err)
	}

	content := fmt.Sprintf("Build #%d cancelled successfully", buildNum)

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: content},
		},
	}, nil, nil
}

type PromoteBuildArgs struct {
	Owner  string            `json:"owner"`
	Repo   string            `json:"repo"`
	Build  float64           `json:"build"`
	Target string            `json:"target"`
	Params map[string]string `json:"params,omitempty"`
}

func (h *BuildHandler) HandlePromoteBuild(ctx context.Context, req *mcp.CallToolRequest, args PromoteBuildArgs) (*mcp.CallToolResult, any, error) {
	buildNum := int(args.Build)
	build, err := h.client.Promote(args.Owner, args.Repo, buildNum, args.Target, args.Params)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to promote build: %w", err)
	}

	content := fmt.Sprintf("Build #%d promoted to %s successfully\nNew build: #%d", buildNum, args.Target, build.Number)

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: content},
		},
	}, nil, nil
}

type RollbackBuildArgs struct {
	Owner  string            `json:"owner"`
	Repo   string            `json:"repo"`
	Build  float64           `json:"build"`
	Target string            `json:"target"`
	Params map[string]string `json:"params,omitempty"`
}

func (h *BuildHandler) HandleRollbackBuild(ctx context.Context, req *mcp.CallToolRequest, args RollbackBuildArgs) (*mcp.CallToolResult, any, error) {
	buildNum := int(args.Build)
	build, err := h.client.Rollback(args.Owner, args.Repo, buildNum, args.Target, args.Params)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to rollback build: %w", err)
	}

	content := fmt.Sprintf("Rolled back %s to build #%d successfully\nNew build: #%d", args.Target, buildNum, build.Number)

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: content},
		},
	}, nil, nil
}

type ApproveBuildArgs struct {
	Owner string  `json:"owner"`
	Repo  string  `json:"repo"`
	Build float64 `json:"build"`
	Stage float64 `json:"stage"`
}

func (h *BuildHandler) HandleApproveBuild(ctx context.Context, req *mcp.CallToolRequest, args ApproveBuildArgs) (*mcp.CallToolResult, any, error) {
	buildNum := int(args.Build)
	stageNum := int(args.Stage)
	err := h.client.Approve(args.Owner, args.Repo, buildNum, stageNum)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to approve build stage: %w", err)
	}

	content := fmt.Sprintf("Build #%d stage %d approved successfully", buildNum, stageNum)

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: content},
		},
	}, nil, nil
}

type DeclineBuildArgs struct {
	Owner string  `json:"owner"`
	Repo  string  `json:"repo"`
	Build float64 `json:"build"`
	Stage float64 `json:"stage"`
}

func (h *BuildHandler) HandleDeclineBuild(ctx context.Context, req *mcp.CallToolRequest, args DeclineBuildArgs) (*mcp.CallToolResult, any, error) {
	buildNum := int(args.Build)
	stageNum := int(args.Stage)
	err := h.client.Decline(args.Owner, args.Repo, buildNum, stageNum)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to decline build stage: %w", err)
	}

	content := fmt.Sprintf("Build #%d stage %d declined successfully", buildNum, stageNum)

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: content},
		},
	}, nil, nil
}

type CreateBuildArgs struct {
	Owner  string            `json:"owner"`
	Repo   string            `json:"repo"`
	Commit string            `json:"commit,omitempty"`
	Branch string            `json:"branch,omitempty"`
	Params map[string]string `json:"params,omitempty"`
}

func (h *BuildHandler) HandleCreateBuild(ctx context.Context, req *mcp.CallToolRequest, args CreateBuildArgs) (*mcp.CallToolResult, any, error) {
	build, err := h.client.BuildCreate(args.Owner, args.Repo, args.Commit, args.Branch, args.Params)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create build: %w", err)
	}

	content := fmt.Sprintf("Build created successfully\nBuild #%d\nStatus: %s\nRef: %s", build.Number, build.Status, build.Ref)

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: content},
		},
	}, nil, nil
}
