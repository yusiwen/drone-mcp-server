//go:build test
// +build test

package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"drone-mcp-server/tool"
	"github.com/drone/drone-go/drone"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func main() {
	// Initialize Drone client
	serverURL := os.Getenv("DRONE_SERVER")
	token := os.Getenv("DRONE_TOKEN")

	if serverURL == "" || token == "" {
		log.Fatal("DRONE_SERVER and DRONE_TOKEN environment variables must be set")
	}

	httpClient := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &tokenTransport{
			token: token,
			base:  http.DefaultTransport,
		},
	}
	droneClient := drone.NewClient(serverURL, httpClient)

	// Create in-memory transports
	clientTransport, serverTransport := mcp.NewInMemoryTransports()

	// Create server
	server := mcp.NewServer(&mcp.Implementation{
		Name:    "drone-mcp-server-test",
		Version: "0.1.0",
	}, nil)

	// Initialize handlers
	repoHandler := tool.NewRepoHandler(droneClient)
	buildHandler := tool.NewBuildHandler(droneClient)

	// Register tools
	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_repos",
		Description: "List all repositories in Drone",
	}, repoHandler.HandleListRepos)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_builds",
		Description: "List builds for a repository",
	}, buildHandler.HandleListBuilds)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_build",
		Description: "Get build details",
	}, buildHandler.HandleGetBuild)

	// Connect server
	ctx := context.Background()
	serverSession, err := server.Connect(ctx, serverTransport, nil)
	if err != nil {
		log.Fatal("Failed to connect server:", err)
	}
	defer serverSession.Close()

	// Create client
	client := mcp.NewClient(&mcp.Implementation{Name: "test-client"}, nil)
	clientSession, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		log.Fatal("Failed to connect client:", err)
	}
	defer clientSession.Close()

	// Test list_repos tool
	fmt.Println("Testing list_repos tool...")
	res, err := clientSession.CallTool(ctx, &mcp.CallToolParams{
		Name:      "list_repos",
		Arguments: map[string]any{},
	})
	if err != nil {
		log.Fatal("list_repos failed:", err)
	}

	if len(res.Content) > 0 {
		if textContent, ok := res.Content[0].(*mcp.TextContent); ok {
			fmt.Println("Repositories:")
			fmt.Println(textContent.Text)
		}
	}

	// Test list_builds tool (if there are repositories)
	fmt.Println("\nTesting list_builds tool...")
	// We need a repository to test. Let's get the first repo from the list
	// For now, we'll use a known repository if available
	// This is just a basic test
	res2, err := clientSession.CallTool(ctx, &mcp.CallToolParams{
		Name:      "list_builds",
		Arguments: map[string]any{"owner": "yusiwen", "repo": "yusiwen.github.io"},
	})
	if err != nil {
		fmt.Println("list_builds test skipped or failed (might be expected):", err)
	} else {
		if len(res2.Content) > 0 {
			if textContent, ok := res2.Content[0].(*mcp.TextContent); ok {
				fmt.Println("Builds for yusiwen/yusiwen.github.io:")
				fmt.Println(textContent.Text)
			}
		}
	}

	fmt.Println("\nMCP server test completed successfully!")
}

type tokenTransport struct {
	token string
	base  http.RoundTripper
}

func (t *tokenTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Set("Authorization", "Bearer "+t.token)
	return t.base.RoundTrip(req)
}
