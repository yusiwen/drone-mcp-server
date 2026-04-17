package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"drone-mcp-server/tool"
	"github.com/drone/drone-go/drone"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

var (
	useSSE = flag.Bool("sse", false, "Use SSE HTTP transport instead of stdio")
	host   = flag.String("host", "localhost", "Host to listen on (SSE mode only)")
	port   = flag.String("port", "8080", "Port to listen on (SSE mode only)")
)

type DroneServer struct {
	client          drone.Client
	repoHandler     *tool.RepoHandler
	buildHandler    *tool.BuildHandler
	resourceHandler *tool.ResourceHandler
}

func main() {
	flag.Parse()

	droneServer := &DroneServer{}
	droneServer.initDroneClient()

	// Initialize handlers
	droneServer.repoHandler = tool.NewRepoHandler(droneServer.client)
	droneServer.buildHandler = tool.NewBuildHandler(droneServer.client)
	droneServer.resourceHandler = tool.NewResourceHandler(droneServer.client)

	server := mcp.NewServer(&mcp.Implementation{
		Name:    "drone-mcp-server",
		Version: "0.1.0",
	}, nil)

	// Register tools
	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_repos",
		Description: "List all repositories in Drone",
	}, droneServer.repoHandler.HandleListRepos)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_builds",
		Description: "List builds for a repository",
	}, droneServer.buildHandler.HandleListBuilds)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_build",
		Description: "Get build details",
	}, droneServer.buildHandler.HandleGetBuild)

	// Register resource template
	server.AddResourceTemplate(&mcp.ResourceTemplate{
		Name:        "Drone build details",
		Description: "Drone build details",
		MIMEType:    "text/plain",
		URITemplate: "drone://builds/{owner}/{repo}/{build}",
	}, droneServer.resourceHandler.HandleBuildResource)

	// Start server with selected transport
	ctx := context.Background()
	if *useSSE {
		startSSEServer(ctx, server)
	} else {
		startStdioServer(ctx, server)
	}
}

func startStdioServer(ctx context.Context, server *mcp.Server) {
	stdio := &mcp.StdioTransport{}
	log.Println("Starting MCP server with stdio transport...")
	if err := server.Run(ctx, stdio); err != nil {
		log.Fatal(err)
	}
}

func startSSEServer(ctx context.Context, server *mcp.Server) {
	addr := fmt.Sprintf("%s:%s", *host, *port)

	handler := mcp.NewSSEHandler(func(request *http.Request) *mcp.Server {
		// For now, we only have one server instance
		// Could support multiple endpoints in the future
		return server
	}, nil)

	log.Printf("Starting MCP server with SSE transport on http://%s", addr)
	log.Fatal(http.ListenAndServe(addr, handler))
}

func (s *DroneServer) initDroneClient() {
	serverURL := os.Getenv("DRONE_SERVER")
	token := os.Getenv("DRONE_TOKEN")

	if serverURL == "" || token == "" {
		log.Fatal("DRONE_SERVER and DRONE_TOKEN environment variables must be set")
	}

	// Create HTTP client with token authentication
	httpClient := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &tokenTransport{
			token: token,
			base:  http.DefaultTransport,
		},
	}

	s.client = drone.NewClient(serverURL, httpClient)
}

// tokenTransport adds Authorization header to requests
type tokenTransport struct {
	token string
	base  http.RoundTripper
}

func (t *tokenTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Set("Authorization", "Bearer "+t.token)
	return t.base.RoundTrip(req)
}
