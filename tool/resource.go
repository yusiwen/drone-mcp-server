package tool

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/drone/drone-go/drone"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type ResourceHandler struct {
	client drone.Client
}

func NewResourceHandler(client drone.Client) *ResourceHandler {
	return &ResourceHandler{client: client}
}

func (h *ResourceHandler) HandleBuildResource(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
	uri := req.Params.URI
	if !strings.HasPrefix(uri, "drone://builds/") {
		return nil, fmt.Errorf("invalid URI format")
	}

	parts := strings.Split(strings.TrimPrefix(uri, "drone://builds/"), "/")
	if len(parts) != 3 {
		return nil, fmt.Errorf("invalid URI format, expected drone://builds/owner/repo/build")
	}

	owner, repo, buildStr := parts[0], parts[1], parts[2]
	if err := firstErr(ValidateSegment("owner", owner), ValidateSegment("repo", repo)); err != nil {
		return nil, fmt.Errorf("invalid resource URI: %w", err)
	}

	buildNum, err := strconv.Atoi(buildStr)
	if err != nil {
		return nil, fmt.Errorf("invalid build number: %w", err)
	}
	if buildNum < 1 || buildNum > maxNumber {
		return nil, fmt.Errorf("invalid build number: must be between 1 and %d", maxNumber)
	}

	build, err := h.client.Build(owner, repo, buildNum)
	if err != nil {
		return nil, fmt.Errorf("failed to get build: %w", err)
	}

	content := fmt.Sprintf("Build #%d\nStatus: %s\nRef: %s\nCommit: %s\nAuthor: %s\nStarted: %v\nEvent: %s\nAction: %s",
		build.Number, build.Status, build.Ref, build.After, build.Author, time.Unix(build.Timestamp, 0), build.Event, build.Action)

	return &mcp.ReadResourceResult{
		Contents: []*mcp.ResourceContents{
			{
				URI:  uri,
				Text: content,
			},
		},
	}, nil
}
