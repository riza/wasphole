package ai

import (
	"context"
	"fmt"
	"os"

	anthropic "github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	openai "github.com/sashabaranov/go-openai"

	"github.com/riza/wasphole/internal/config"
)

// Client is the minimal AI generation interface used by identity and response cache.
type Client interface {
	Generate(ctx context.Context, system, user string) (string, error)
}

// NewClient constructs the appropriate Client from config.
func NewClient(cfg config.AIConfig) (Client, error) {
	switch cfg.APIType {
	case "anthropic":
		return newAnthropicClient(cfg)
	case "openai":
		return newOpenAIClient(cfg)
	default:
		return nil, fmt.Errorf("unsupported api_type %q: must be anthropic or openai", cfg.APIType)
	}
}

// --- Anthropic ---------------------------------------------------------------

type anthropicClient struct {
	c     anthropic.Client
	model anthropic.Model
}

func newAnthropicClient(cfg config.AIConfig) (Client, error) {
	opts := []option.RequestOption{option.WithAPIKey(cfg.APIKey)}
	// Anthropic SDK auto-reads ANTHROPIC_API_KEY from env when APIKey is "".
	if cfg.BaseURL != "" {
		opts = append(opts, option.WithBaseURL(cfg.BaseURL))
	}
	return &anthropicClient{
		c:     anthropic.NewClient(opts...),
		model: anthropic.Model(cfg.Model),
	}, nil
}

func (c *anthropicClient) Generate(ctx context.Context, system, user string) (string, error) {
	msg, err := c.c.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     c.model,
		MaxTokens: 4096,
		System: []anthropic.TextBlockParam{
			{Text: system},
		},
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(user)),
		},
	})
	if err != nil {
		return "", err
	}
	if len(msg.Content) == 0 {
		return "", fmt.Errorf("anthropic: empty response")
	}
	block := msg.Content[0]
	if block.Type != "text" {
		return "", fmt.Errorf("anthropic: unexpected content type %q", block.Type)
	}
	return block.Text, nil
}

// --- OpenAI-compatible -------------------------------------------------------

type openaiClient struct {
	c     *openai.Client
	model string
}

func newOpenAIClient(cfg config.AIConfig) (Client, error) {
	apiKey := cfg.APIKey
	if apiKey == "" {
		// go-openai does NOT auto-read env vars; must pass explicitly.
		apiKey = os.Getenv("OPENAI_API_KEY")
	}
	ocfg := openai.DefaultConfig(apiKey)
	if cfg.BaseURL != "" {
		ocfg.BaseURL = cfg.BaseURL
	}
	return &openaiClient{c: openai.NewClientWithConfig(ocfg), model: cfg.Model}, nil
}

func (c *openaiClient) Generate(ctx context.Context, system, user string) (string, error) {
	resp, err := c.c.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model: c.model,
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleSystem, Content: system},
			{Role: openai.ChatMessageRoleUser, Content: user},
		},
	})
	if err != nil {
		return "", err
	}
	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("openai: empty response")
	}
	return resp.Choices[0].Message.Content, nil
}
