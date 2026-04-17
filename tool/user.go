package tool

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/drone/drone-go/drone"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type UserHandler struct {
	client drone.Client
}

func NewUserHandler(client drone.Client) *UserHandler {
	return &UserHandler{client: client}
}

type GetSelfArgs struct{}

func (h *UserHandler) HandleGetSelf(ctx context.Context, req *mcp.CallToolRequest, args GetSelfArgs) (*mcp.CallToolResult, any, error) {
	user, err := h.client.Self()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get current user: %w", err)
	}

	content := fmt.Sprintf("Current user:\nLogin: %s\nEmail: %s\nAdmin: %v\nActive: %v\nMachine: %v\nLast login: %v",
		user.Login, user.Email, user.Admin, user.Active, user.Machine,
		time.Unix(user.LastLogin, 0).Format(time.RFC3339))

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: content},
		},
	}, nil, nil
}

type ListUsersArgs struct{}

func (h *UserHandler) HandleListUsers(ctx context.Context, req *mcp.CallToolRequest, args ListUsersArgs) (*mcp.CallToolResult, any, error) {
	users, err := h.client.UserList()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to list users: %w", err)
	}

	var userList []string
	for _, user := range users {
		userList = append(userList, fmt.Sprintf("%s (Email: %s, Admin: %v, Active: %v)",
			user.Login, user.Email, user.Admin, user.Active))
	}

	content := fmt.Sprintf("Users (%d):\n%s", len(users), strings.Join(userList, "\n"))

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: content},
		},
	}, nil, nil
}

type GetUserArgs struct {
	Login string `json:"login"`
}

func (h *UserHandler) HandleGetUser(ctx context.Context, req *mcp.CallToolRequest, args GetUserArgs) (*mcp.CallToolResult, any, error) {
	user, err := h.client.User(args.Login)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get user: %w", err)
	}

	content := fmt.Sprintf("User: %s\nEmail: %s\nAdmin: %v\nActive: %v\nMachine: %v\nCreated: %v\nLast login: %v",
		user.Login, user.Email, user.Admin, user.Active, user.Machine,
		time.Unix(user.Created, 0).Format(time.RFC3339),
		time.Unix(user.LastLogin, 0).Format(time.RFC3339))

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: content},
		},
	}, nil, nil
}

type CreateUserArgs struct {
	Login  string `json:"login"`
	Email  string `json:"email,omitempty"`
	Admin  bool   `json:"admin,omitempty"`
	Active bool   `json:"active,omitempty"`
	Token  string `json:"token,omitempty"`
}

func (h *UserHandler) HandleCreateUser(ctx context.Context, req *mcp.CallToolRequest, args CreateUserArgs) (*mcp.CallToolResult, any, error) {
	user := &drone.User{
		Login:  args.Login,
		Email:  args.Email,
		Admin:  args.Admin,
		Active: args.Active,
		Token:  args.Token,
	}

	createdUser, err := h.client.UserCreate(user)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create user: %w", err)
	}

	content := fmt.Sprintf("User created successfully: %s\nEmail: %s\nAdmin: %v\nActive: %v",
		createdUser.Login, createdUser.Email, createdUser.Admin, createdUser.Active)

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: content},
		},
	}, nil, nil
}

type UpdateUserArgs struct {
	Login  string `json:"login"`
	Email  string `json:"email,omitempty"`
	Admin  *bool  `json:"admin,omitempty"`
	Active *bool  `json:"active,omitempty"`
}

func (h *UserHandler) HandleUpdateUser(ctx context.Context, req *mcp.CallToolRequest, args UpdateUserArgs) (*mcp.CallToolResult, any, error) {
	// Create a UserPatch for updating
	userPatch := &drone.UserPatch{}

	// Set fields only if they are provided (not nil)
	if args.Admin != nil {
		userPatch.Admin = args.Admin
	}
	if args.Active != nil {
		userPatch.Active = args.Active
	}
	// Note: UserPatch doesn't have Email field, only Active, Admin, Machine, Token

	updatedUser, err := h.client.UserUpdate(args.Login, userPatch)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to update user: %w", err)
	}

	content := fmt.Sprintf("User updated successfully: %s\nEmail: %s\nAdmin: %v\nActive: %v",
		updatedUser.Login, updatedUser.Email, updatedUser.Admin, updatedUser.Active)

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: content},
		},
	}, nil, nil
}

type DeleteUserArgs struct {
	Login string `json:"login"`
}

func (h *UserHandler) HandleDeleteUser(ctx context.Context, req *mcp.CallToolRequest, args DeleteUserArgs) (*mcp.CallToolResult, any, error) {
	err := h.client.UserDelete(args.Login)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to delete user: %w", err)
	}

	content := fmt.Sprintf("User deleted successfully: %s", args.Login)

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: content},
		},
	}, nil, nil
}
