package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"runtime"
	"strings"
	"time"

	"drone-mcp-server/tool"
	"github.com/drone/drone-go/drone"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

var (
	useSSE  = flag.Bool("sse", false, "Use SSE HTTP transport instead of stdio")
	host    = flag.String("host", "localhost", "Host to listen on (SSE mode only)")
	port    = flag.String("port", "8080", "Port to listen on (SSE mode only)")
	path    = flag.String("path", "/", "Path for SSE endpoint (SSE mode only)")
	version = flag.Bool("version", false, "Show version information")
)

// Build-time variables (set via -ldflags)
var (
	buildVersion = "dev"
	buildCommit  = "unknown"
	buildDate    = "unknown"
)

type DroneServer struct {
	client          drone.Client
	repoHandler     *tool.RepoHandler
	buildHandler    *tool.BuildHandler
	resourceHandler *tool.ResourceHandler
	cronHandler     *tool.CronHandler
	secretHandler   *tool.SecretHandler
	userHandler     *tool.UserHandler
	templateHandler *tool.TemplateHandler
}

func main() {
	flag.Parse()

	// Show version information if requested
	if *version {
		fmt.Printf("drone-mcp-server\n")
		fmt.Printf("Version: %s\n", buildVersion)
		fmt.Printf("Commit: %s\n", buildCommit)
		fmt.Printf("Build date: %s\n", buildDate)
		fmt.Printf("Go version: %s\n", runtime.Version())
		os.Exit(0)
	}

	droneServer := &DroneServer{}
	droneServer.initDroneClient()

	// Initialize handlers
	droneServer.repoHandler = tool.NewRepoHandler(droneServer.client)
	droneServer.buildHandler = tool.NewBuildHandler(droneServer.client)
	droneServer.resourceHandler = tool.NewResourceHandler(droneServer.client)
	droneServer.cronHandler = tool.NewCronHandler(droneServer.client)
	droneServer.secretHandler = tool.NewSecretHandler(droneServer.client)
	droneServer.userHandler = tool.NewUserHandler(droneServer.client)
	droneServer.templateHandler = tool.NewTemplateHandler(droneServer.client)

	server := mcp.NewServer(&mcp.Implementation{
		Name:    "drone-mcp-server",
		Version: "0.1.0",
	}, nil)

	// Add logging middleware for MCP method calls
	server.AddReceivingMiddleware(func(next mcp.MethodHandler) mcp.MethodHandler {
		return func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
			// Log the method call
			start := time.Now()

			// For tools/call method, log tool name
			if method == "tools/call" {
				if callReq, ok := req.(*mcp.CallToolRequest); ok {
					log.Printf("[TOOL] Tool called: %s", callReq.Params.Name)
				}
			} else {
				log.Printf("[MCP] Method called: %s", method)
			}

			// Call the next handler
			result, err := next(ctx, method, req)

			// Log completion
			duration := time.Since(start)
			if err != nil {
				if method == "tools/call" {
					if callReq, ok := req.(*mcp.CallToolRequest); ok {
						log.Printf("[TOOL] Tool %s failed: %v (took %v)", callReq.Params.Name, err, duration)
					} else {
						log.Printf("[MCP] Method %s failed: %v (took %v)", method, err, duration)
					}
				} else {
					log.Printf("[MCP] Method %s failed: %v (took %v)", method, err, duration)
				}
			} else {
				if method == "tools/call" {
					if callReq, ok := req.(*mcp.CallToolRequest); ok {
						log.Printf("[TOOL] Tool %s completed successfully (took %v)", callReq.Params.Name, duration)
					} else {
						log.Printf("[MCP] Method %s completed (took %v)", method, duration)
					}
				} else {
					log.Printf("[MCP] Method %s completed (took %v)", method, duration)
				}
			}

			return result, err
		}
	})

	// Register tools
	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_repos",
		Description: "List all repositories in Drone",
	}, droneServer.repoHandler.HandleListRepos)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_repo",
		Description: "Get repository details",
	}, droneServer.repoHandler.HandleGetRepo)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "enable_repo",
		Description: "Enable a repository",
	}, droneServer.repoHandler.HandleEnableRepo)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "disable_repo",
		Description: "Disable a repository",
	}, droneServer.repoHandler.HandleDisableRepo)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "repair_repo",
		Description: "Repair a repository",
	}, droneServer.repoHandler.HandleRepairRepo)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "chown_repo",
		Description: "Change repository ownership",
	}, droneServer.repoHandler.HandleChownRepo)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "sync_repos",
		Description: "Synchronize repository list",
	}, droneServer.repoHandler.HandleSyncRepos)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_incomplete",
		Description: "List repositories with incomplete builds",
	}, droneServer.repoHandler.HandleListIncomplete)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_crons",
		Description: "List cron jobs for a repository",
	}, droneServer.cronHandler.HandleListCrons)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_cron",
		Description: "Get cron job details",
	}, droneServer.cronHandler.HandleGetCron)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "create_cron",
		Description: "Create a new cron job",
	}, droneServer.cronHandler.HandleCreateCron)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "delete_cron",
		Description: "Delete a cron job",
	}, droneServer.cronHandler.HandleDeleteCron)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "execute_cron",
		Description: "Execute a cron job immediately",
	}, droneServer.cronHandler.HandleExecuteCron)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_secrets",
		Description: "List repository secrets",
	}, droneServer.secretHandler.HandleListSecrets)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_secret",
		Description: "Get repository secret details",
	}, droneServer.secretHandler.HandleGetSecret)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "create_secret",
		Description: "Create a repository secret",
	}, droneServer.secretHandler.HandleCreateSecret)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "update_secret",
		Description: "Update a repository secret",
	}, droneServer.secretHandler.HandleUpdateSecret)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "delete_secret",
		Description: "Delete a repository secret",
	}, droneServer.secretHandler.HandleDeleteSecret)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_org_secrets",
		Description: "List organization secrets",
	}, droneServer.secretHandler.HandleListOrgSecrets)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_org_secret",
		Description: "Get organization secret details",
	}, droneServer.secretHandler.HandleGetOrgSecret)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "create_org_secret",
		Description: "Create an organization secret",
	}, droneServer.secretHandler.HandleCreateOrgSecret)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "update_org_secret",
		Description: "Update an organization secret",
	}, droneServer.secretHandler.HandleUpdateOrgSecret)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "delete_org_secret",
		Description: "Delete an organization secret",
	}, droneServer.secretHandler.HandleDeleteOrgSecret)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_self",
		Description: "Get current authenticated user",
	}, droneServer.userHandler.HandleGetSelf)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_users",
		Description: "List all users",
	}, droneServer.userHandler.HandleListUsers)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_user",
		Description: "Get user details",
	}, droneServer.userHandler.HandleGetUser)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "create_user",
		Description: "Create a new user",
	}, droneServer.userHandler.HandleCreateUser)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "update_user",
		Description: "Update a user",
	}, droneServer.userHandler.HandleUpdateUser)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "delete_user",
		Description: "Delete a user",
	}, droneServer.userHandler.HandleDeleteUser)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_templates",
		Description: "List templates (optionally by namespace)",
	}, droneServer.templateHandler.HandleListTemplates)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_template",
		Description: "Get template details and data",
	}, droneServer.templateHandler.HandleGetTemplate)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "create_template",
		Description: "Create a new template",
	}, droneServer.templateHandler.HandleCreateTemplate)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "update_template",
		Description: "Update a template",
	}, droneServer.templateHandler.HandleUpdateTemplate)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "delete_template",
		Description: "Delete a template",
	}, droneServer.templateHandler.HandleDeleteTemplate)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_builds",
		Description: "List builds for a repository",
	}, droneServer.buildHandler.HandleListBuilds)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_build",
		Description: "Get build details",
	}, droneServer.buildHandler.HandleGetBuild)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_build_last",
		Description: "Get the last build for a repository (optionally by branch)",
	}, droneServer.buildHandler.HandleGetBuildLast)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_build_logs",
		Description: "Get logs for a specific build stage and step",
	}, droneServer.buildHandler.HandleBuildLogs)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "restart_build",
		Description: "Restart a build (optionally with parameters)",
	}, droneServer.buildHandler.HandleRestartBuild)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "cancel_build",
		Description: "Cancel a running build",
	}, droneServer.buildHandler.HandleCancelBuild)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "promote_build",
		Description: "Promote a build to a target environment",
	}, droneServer.buildHandler.HandlePromoteBuild)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "rollback_build",
		Description: "Rollback a deployment to a previous build",
	}, droneServer.buildHandler.HandleRollbackBuild)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "approve_build",
		Description: "Approve a build stage (for gated deployments)",
	}, droneServer.buildHandler.HandleApproveBuild)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "decline_build",
		Description: "Decline a build stage (for gated deployments)",
	}, droneServer.buildHandler.HandleDeclineBuild)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "create_build",
		Description: "Create a new build from a commit or branch",
	}, droneServer.buildHandler.HandleCreateBuild)

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

	// Get SSE authentication token from environment (optional)
	sseToken := os.Getenv("MCP_AUTH_TOKEN")

	// Create SSE handler
	sseHandler := mcp.NewSSEHandler(func(request *http.Request) *mcp.Server {
		// For now, we only have one server instance
		// Could support multiple endpoints in the future
		return server
	}, nil)

	// Wrap with authentication middleware if token is set
	var handler http.Handler = sseHandler
	if sseToken != "" {
		handler = authMiddleware(sseHandler, sseToken)
		log.Println("SSE authentication enabled (Bearer token required)")
	} else {
		log.Println("SSE authentication disabled (no MCP_AUTH_TOKEN set)")
	}

	// Wrap with logging middleware for access logging
	handler = loggingMiddleware(handler)

	// Ensure path starts with "/"
	ssePath := *path
	if !strings.HasPrefix(ssePath, "/") {
		ssePath = "/" + ssePath
	}

	// Create HTTP mux and register handler on the specified path
	mux := http.NewServeMux()
	mux.Handle(ssePath, handler)

	log.Printf("Starting MCP server with SSE transport on http://%s%s", addr, ssePath)
	log.Fatal(http.ListenAndServe(addr, mux))
}

// authMiddleware creates an HTTP middleware that validates Bearer token authentication
func authMiddleware(next http.Handler, expectedToken string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Extract client IP for logging
		clientIP := getClientIP(r)

		// Extract Authorization header
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			log.Printf("[AUTH] Token missing from %s %s (client: %s)", r.Method, r.URL.Path, clientIP)
			http.Error(w, "Authorization header required", http.StatusUnauthorized)
			return
		}

		// Check Bearer token format
		const bearerPrefix = "Bearer "
		if !strings.HasPrefix(authHeader, bearerPrefix) {
			log.Printf("[AUTH] Invalid token format from %s %s (client: %s)", r.Method, r.URL.Path, clientIP)
			http.Error(w, "Invalid authorization format, expected Bearer token", http.StatusUnauthorized)
			return
		}

		// Validate token
		token := strings.TrimPrefix(authHeader, bearerPrefix)
		if token != expectedToken {
			log.Printf("[AUTH] Invalid token from %s %s (client: %s)", r.Method, r.URL.Path, clientIP)
			http.Error(w, "Invalid token", http.StatusUnauthorized)
			return
		}

		log.Printf("[AUTH] Token validated for %s %s (client: %s)", r.Method, r.URL.Path, clientIP)
		// Token is valid, proceed to next handler
		next.ServeHTTP(w, r)
	})
}

// loggingMiddleware creates an HTTP middleware that logs access information
func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		clientIP := getClientIP(r)
		userAgent := r.Header.Get("User-Agent")
		if userAgent == "" {
			userAgent = "-"
		}

		// Create a response writer wrapper to capture status code
		rw := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

		// Process request
		next.ServeHTTP(rw, r)

		duration := time.Since(start)

		// Log the access
		log.Printf("[ACCESS] %s %s %s %d %s %s %s",
			clientIP,
			r.Method,
			r.URL.Path,
			rw.statusCode,
			r.Header.Get("Content-Type"),
			userAgent,
			duration,
		)
	})
}

// responseWriter wraps http.ResponseWriter to capture status code and support streaming interfaces
type responseWriter struct {
	http.ResponseWriter
	statusCode int
	hijacked   bool
}

func (rw *responseWriter) WriteHeader(code int) {
	if !rw.hijacked {
		rw.statusCode = code
		rw.ResponseWriter.WriteHeader(code)
	}
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	if rw.hijacked {
		return 0, http.ErrHijacked
	}
	if rw.statusCode == 0 {
		rw.statusCode = http.StatusOK
	}
	return rw.ResponseWriter.Write(b)
}

// Flush implements http.Flusher for SSE streaming support
func (rw *responseWriter) Flush() {
	if f, ok := rw.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// Hijack implements http.Hijacker for connection upgrades
func (rw *responseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if h, ok := rw.ResponseWriter.(http.Hijacker); ok {
		rw.hijacked = true
		return h.Hijack()
	}
	return nil, nil, fmt.Errorf("hijacking not supported")
}

// Push implements http.Pusher for HTTP/2 server push
func (rw *responseWriter) Push(target string, opts *http.PushOptions) error {
	if p, ok := rw.ResponseWriter.(http.Pusher); ok {
		return p.Push(target, opts)
	}
	return http.ErrNotSupported
}

// ReadFrom implements io.ReaderFrom for efficient copying
func (rw *responseWriter) ReadFrom(r io.Reader) (n int64, err error) {
	if rf, ok := rw.ResponseWriter.(io.ReaderFrom); ok {
		return rf.ReadFrom(r)
	}
	return io.Copy(rw.ResponseWriter, r)
}

// getClientIP extracts the client IP address from the request
func getClientIP(r *http.Request) string {
	// Check X-Forwarded-For header (for proxies)
	if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
		// Take the first IP in the list
		if commaIndex := strings.Index(forwarded, ","); commaIndex != -1 {
			return strings.TrimSpace(forwarded[:commaIndex])
		}
		return forwarded
	}

	// Check X-Real-IP header
	if realIP := r.Header.Get("X-Real-IP"); realIP != "" {
		return realIP
	}

	// Fall back to remote address
	return strings.Split(r.RemoteAddr, ":")[0]
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
