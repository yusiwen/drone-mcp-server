package tool

import (
	"context"
	"fmt"
	"strings"

	"github.com/drone/drone-go/drone"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type TemplateHandler struct {
	client drone.Client
}

func NewTemplateHandler(client drone.Client) *TemplateHandler {
	return &TemplateHandler{client: client}
}

type ListTemplatesArgs struct {
	Namespace string `json:"namespace,omitempty"`
}

func (h *TemplateHandler) HandleListTemplates(ctx context.Context, req *mcp.CallToolRequest, args ListTemplatesArgs) (*mcp.CallToolResult, any, error) {
	var templates []*drone.Template
	var err error

	if args.Namespace == "" {
		// List all templates
		templates, err = h.client.TemplateListAll()
		if err != nil {
			return nil, nil, fmt.Errorf("failed to list all templates: %w", err)
		}
	} else {
		// List templates by namespace
		templates, err = h.client.TemplateList(args.Namespace)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to list templates for namespace %s: %w", args.Namespace, err)
		}
	}

	var templateList []string
	for _, template := range templates {
		templateList = append(templateList, fmt.Sprintf("%s (Data length: %d)", template.Name, len(template.Data)))
	}

	content := fmt.Sprintf("Templates (%d):\n%s", len(templates), strings.Join(templateList, "\n"))

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: content},
		},
	}, nil, nil
}

type GetTemplateArgs struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
}

func (h *TemplateHandler) HandleGetTemplate(ctx context.Context, req *mcp.CallToolRequest, args GetTemplateArgs) (*mcp.CallToolResult, any, error) {
	template, err := h.client.Template(args.Namespace, args.Name)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get template: %w", err)
	}

	content := fmt.Sprintf("Template: %s\nNamespace: %s\nData length: %d\n\nData:\n%s",
		template.Name, args.Namespace, len(template.Data), template.Data)

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: content},
		},
	}, nil, nil
}

type CreateTemplateArgs struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
	Data      string `json:"data"`
}

func (h *TemplateHandler) HandleCreateTemplate(ctx context.Context, req *mcp.CallToolRequest, args CreateTemplateArgs) (*mcp.CallToolResult, any, error) {
	template := &drone.Template{
		Name: args.Name,
		Data: args.Data,
	}

	createdTemplate, err := h.client.TemplateCreate(args.Namespace, template)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create template: %w", err)
	}

	content := fmt.Sprintf("Template created successfully: %s\nNamespace: %s\nData length: %d",
		createdTemplate.Name, args.Namespace, len(createdTemplate.Data))

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: content},
		},
	}, nil, nil
}

type UpdateTemplateArgs struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
	Data      string `json:"data"`
}

func (h *TemplateHandler) HandleUpdateTemplate(ctx context.Context, req *mcp.CallToolRequest, args UpdateTemplateArgs) (*mcp.CallToolResult, any, error) {
	template := &drone.Template{
		Name: args.Name,
		Data: args.Data,
	}

	updatedTemplate, err := h.client.TemplateUpdate(args.Namespace, args.Name, template)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to update template: %w", err)
	}

	content := fmt.Sprintf("Template updated successfully: %s\nNamespace: %s\nData length: %d",
		updatedTemplate.Name, args.Namespace, len(updatedTemplate.Data))

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: content},
		},
	}, nil, nil
}

type DeleteTemplateArgs struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
}

func (h *TemplateHandler) HandleDeleteTemplate(ctx context.Context, req *mcp.CallToolRequest, args DeleteTemplateArgs) (*mcp.CallToolResult, any, error) {
	err := h.client.TemplateDelete(args.Namespace, args.Name)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to delete template: %w", err)
	}

	content := fmt.Sprintf("Template deleted successfully: %s (namespace: %s)", args.Name, args.Namespace)

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: content},
		},
	}, nil, nil
}
