package gemini

import (
	domainSession "chakam/internal/domain/session"
	"context"
	"fmt"

	"google.golang.org/genai"
)

type Client struct {
	client *genai.Client
	model  string
}

type ConversationResponse struct {
	TextResponse  string
	FunctionCalls []FunctionCall
	FinishReason  string
}

type FunctionCall struct {
	Name      string
	Arguments map[string]interface{}
}
type GenerateContentRequest struct {
	Contents             []*genai.Content
	SystemInstruction    string
	FunctionDeclarations []*genai.FunctionDeclaration
}

func NewClient(ctx context.Context, apiKey, model string) (*Client, error) {
	geminiClient, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return nil, fmt.Errorf("gemini: new client: %w", err)
	}
	return &Client{client: geminiClient, model: model}, nil
}

func (c *Client) GenerateResponse(ctx context.Context, messages domainSession.ConversationHistory) (string, error) {

	contents := make([]*genai.Content, 0, len(messages))

	for _, message := range messages {
		role := genai.RoleUser
		if message.Role == "assistant" {
			role = genai.RoleModel
		}
		contents = append(contents, genai.NewContentFromText(message.Content, genai.Role(role)))
	}

	config := &genai.GenerateContentConfig{
		SystemInstruction: genai.NewContentFromText("You are a helpful assistant that can answer questions and help with tasks.", genai.RoleUser),
	}
	response, err := c.client.Models.GenerateContent(ctx, c.model, contents, config)
	if err != nil {
		return "", err
	}
	return response.Text(), nil
}

func (c *Client) Generate(ctx context.Context, req GenerateContentRequest) (*genai.GenerateContentResponse, error) {
	tools := make([]*genai.Tool, 0)
	tools = append(tools, &genai.Tool{
		FunctionDeclarations: req.FunctionDeclarations,
	})

	config := &genai.GenerateContentConfig{
		SystemInstruction: genai.NewContentFromText(req.SystemInstruction, genai.RoleUser),
		Tools:             tools,
	}
	response, err := c.client.Models.GenerateContent(ctx, c.model, req.Contents, config)
	if err != nil {
		return nil, err
	}
	return response, nil
}
