package tool

import (
	"context"
	"fmt"
	"strings"

	"github.com/drone/drone-go/drone"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type CronHandler struct {
	client drone.Client
}

func NewCronHandler(client drone.Client) *CronHandler {
	return &CronHandler{client: client}
}

type ListCronsArgs struct {
	Owner string `json:"owner"`
	Repo  string `json:"repo"`
}

func (h *CronHandler) HandleListCrons(ctx context.Context, req *mcp.CallToolRequest, args ListCronsArgs) (*mcp.CallToolResult, any, error) {
	crons, err := h.client.CronList(args.Owner, args.Repo)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to list crons: %w", err)
	}

	var cronList []string
	for _, cron := range crons {
		cronList = append(cronList, fmt.Sprintf("%s: %d (Schedule: %s, Branch: %s, Disabled: %v)",
			cron.Name, cron.ID, cron.Expr, cron.Branch, cron.Disabled))
	}

	content := fmt.Sprintf("Cron jobs for %s/%s (%d):\n%s", args.Owner, args.Repo, len(crons), strings.Join(cronList, "\n"))

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: content},
		},
	}, nil, nil
}

type GetCronArgs struct {
	Owner string `json:"owner"`
	Repo  string `json:"repo"`
	Cron  string `json:"cron"`
}

func (h *CronHandler) HandleGetCron(ctx context.Context, req *mcp.CallToolRequest, args GetCronArgs) (*mcp.CallToolResult, any, error) {
	cron, err := h.client.Cron(args.Owner, args.Repo, args.Cron)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get cron: %w", err)
	}

	content := fmt.Sprintf("Cron job: %s\nID: %d\nSchedule: %s\nBranch: %s\nDisabled: %v\nNext execution: %v\nCreated: %v",
		cron.Name, cron.ID, cron.Expr, cron.Branch, cron.Disabled, cron.Next, cron.Created)

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: content},
		},
	}, nil, nil
}

type CreateCronArgs struct {
	Owner   string `json:"owner"`
	Repo    string `json:"repo"`
	Name    string `json:"name"`
	Expr    string `json:"expr"`
	Branch  string `json:"branch"`
	Disable bool   `json:"disable,omitempty"`
}

func (h *CronHandler) HandleCreateCron(ctx context.Context, req *mcp.CallToolRequest, args CreateCronArgs) (*mcp.CallToolResult, any, error) {
	cron := &drone.Cron{
		Name:     args.Name,
		Expr:     args.Expr,
		Branch:   args.Branch,
		Disabled: args.Disable,
	}

	createdCron, err := h.client.CronCreate(args.Owner, args.Repo, cron)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create cron: %w", err)
	}

	content := fmt.Sprintf("Cron job created successfully: %s\nID: %d\nSchedule: %s\nBranch: %s\nDisabled: %v",
		createdCron.Name, createdCron.ID, createdCron.Expr, createdCron.Branch, createdCron.Disabled)

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: content},
		},
	}, nil, nil
}

type DeleteCronArgs struct {
	Owner string `json:"owner"`
	Repo  string `json:"repo"`
	Cron  string `json:"cron"`
}

func (h *CronHandler) HandleDeleteCron(ctx context.Context, req *mcp.CallToolRequest, args DeleteCronArgs) (*mcp.CallToolResult, any, error) {
	err := h.client.CronDelete(args.Owner, args.Repo, args.Cron)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to delete cron: %w", err)
	}

	content := fmt.Sprintf("Cron job deleted successfully: %s", args.Cron)

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: content},
		},
	}, nil, nil
}

type ExecuteCronArgs struct {
	Owner string `json:"owner"`
	Repo  string `json:"repo"`
	Cron  string `json:"cron"`
}

func (h *CronHandler) HandleExecuteCron(ctx context.Context, req *mcp.CallToolRequest, args ExecuteCronArgs) (*mcp.CallToolResult, any, error) {
	err := h.client.CronExec(args.Owner, args.Repo, args.Cron)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to execute cron: %w", err)
	}

	content := fmt.Sprintf("Cron job execution triggered: %s", args.Cron)

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: content},
		},
	}, nil, nil
}
