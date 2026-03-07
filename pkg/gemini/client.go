package gemini

import (
	"context"
	"fmt"

	"google.golang.org/genai"
)

// FunctionCallResult represents the extracted function call arguments from LLM response.
type FunctionCallResult struct {
	Name string
	Args map[string]any
}

// Client wraps the Google Generative AI SDK for Gemini API access.
type Client struct {
	client *genai.Client
	model  string
}

// NewClient creates a new Gemini API client.
func NewClient(ctx context.Context, apiKey, model string) (*Client, error) {
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return nil, fmt.Errorf("create gemini client: %w", err)
	}
	return &Client{
		client: client,
		model:  model,
	}, nil
}

// GenerateWithFunctionCall sends a prompt with function calling enabled and returns the function call result.
func (c *Client) GenerateWithFunctionCall(
	ctx context.Context,
	systemPrompt string,
	userPrompt string,
	funcDecl *genai.FunctionDeclaration,
) (*FunctionCallResult, error) {
	config := &genai.GenerateContentConfig{
		SystemInstruction: &genai.Content{
			Parts: []*genai.Part{genai.NewPartFromText(systemPrompt)},
		},
		Tools: []*genai.Tool{{
			FunctionDeclarations: []*genai.FunctionDeclaration{funcDecl},
		}},
		ToolConfig: &genai.ToolConfig{
			FunctionCallingConfig: &genai.FunctionCallingConfig{
				Mode:                 genai.FunctionCallingConfigModeAny,
				AllowedFunctionNames: []string{funcDecl.Name},
			},
		},
	}

	resp, err := c.client.Models.GenerateContent(ctx, c.model, []*genai.Content{{
		Role:  "user",
		Parts: []*genai.Part{genai.NewPartFromText(userPrompt)},
	}}, config)
	if err != nil {
		return nil, fmt.Errorf("generate content: %w", err)
	}

	if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
		return nil, fmt.Errorf("empty response from LLM")
	}

	for _, part := range resp.Candidates[0].Content.Parts {
		if part.FunctionCall != nil {
			return &FunctionCallResult{
				Name: part.FunctionCall.Name,
				Args: part.FunctionCall.Args,
			}, nil
		}
	}

	return nil, fmt.Errorf("no function call in response")
}
