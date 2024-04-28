package llmservice

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	_ "embed"

	"github.com/samber/lo"
	"github.com/tyloafer/langchaingo/llms"
	"github.com/tyloafer/langchaingo/llms/openai"
	"github.com/tyloafer/langchaingo/schema"
)

var secrets struct {
	OpenAIAPIKey string
}

type Service struct {
	client *openai.Chat
}

//go:embed triage_message_func.json
var triageMessageFuncSchema string

//go:embed triage_message_prompt.txt
var triageMessagePrompt string

//go:embed tag_forum_post_prompt.txt
var tagForumPostPrompt string

//go:embed set_message_title_func_schema.json
var setMessageTitleFuncSchema string

func NewService() (*Service, error) {
	client, err := openai.NewChat(openai.WithModel("gpt-3.5-turbo-0613"), openai.WithToken(secrets.OpenAIAPIKey))
	if err != nil {
		return nil, fmt.Errorf("couldn't create openai client: %w", err)
	}

	return &Service{client: client}, nil
}

func (s *Service) DetermineForumPostTags(
	ctx context.Context,
	availableTags []string,
	forumPostTitle, forumPostContents string,
) ([]string, error) {
	tagOptions := strings.Join(lo.Map(availableTags, func(tag string, _ int) string {
		return fmt.Sprintf(`"%s"`, tag)
	}), ", ")
	_ = tagOptions
	var llmFunctions = []llms.FunctionDefinition{
		{
			Name:        "setTags",
			Description: "Sets the tags of the forum post",
			Parameters: json.RawMessage(fmt.Sprintf(`
				{
				  "type": "object",
				  "properties": {
					"tags": { 
					  "type": "array", 
					  "items": { 
					    "type": "string", 
						"enum": [%s] 
					  } 
					}
				  },
				  "required": ["topic"]
				}
			`, tagOptions)),
		},
	}

	completion, err := s.client.Call(ctx, []schema.ChatMessage{
		schema.HumanChatMessage{Content: tagForumPostPrompt},
		schema.HumanChatMessage{Content: "What follow is details of the forum post."},
		schema.HumanChatMessage{Content: fmt.Sprintf("Title: %s", forumPostTitle)},
		schema.HumanChatMessage{Content: fmt.Sprintf("Contents:\n%s", forumPostContents)},
	}, llms.WithFunctions(llmFunctions))
	if err != nil {
		return nil, fmt.Errorf("couldn't call openai: %w", err)
	}

	var result struct {
		Tags []string `json:"tags"`
	}
	if err := json.Unmarshal([]byte(completion.FunctionCall.Arguments), &result); err != nil {
		return nil, fmt.Errorf("couldn't unmarshal function call arguments: %w", err)
	}

	return result.Tags, nil
}

func (s *Service) TriageMessageTopic(ctx context.Context, messageContents string) (string, error) {
	var llmFunctions = []llms.FunctionDefinition{
		{
			Name:        "setMessageTopic",
			Description: "Sets the topic of the message",
			Parameters:  json.RawMessage(triageMessageFuncSchema),
		},
	}

	completion, err := s.client.Call(ctx, []schema.ChatMessage{
		schema.HumanChatMessage{Content: triageMessagePrompt},
		schema.HumanChatMessage{Content: "Here's the message: " + messageContents},
	}, llms.WithFunctions(llmFunctions))
	if err != nil {
		return "", fmt.Errorf("couldn't call openai: %w", err)
	} else if completion.FunctionCall == nil {
		return "", errors.New("No function call found in completion")
	}

	var result struct {
		Topic string `json:"topic"`
	}
	if err := json.Unmarshal([]byte(completion.FunctionCall.Arguments), &result); err != nil {
		return "", fmt.Errorf("couldn't unmarshal function call arguments: %w", err)
	}

	return result.Topic, nil
}

func (s *Service) SuggestTitleForMessage(ctx context.Context, messageContents string) (string, error) {
	var llmFunctions = []llms.FunctionDefinition{
		{
			Name:        "setMessageTitle",
			Description: "Sets the title of the message",
			Parameters:  json.RawMessage(setMessageTitleFuncSchema),
		},
	}

	completion, err := s.client.Call(ctx, []schema.ChatMessage{
		schema.HumanChatMessage{
			Content: "This is a message by a user of our product and we want you to suggest a title for that message, as if it's a forum post, made by the same user"},
		schema.HumanChatMessage{Content: messageContents},
	}, llms.WithFunctions(llmFunctions))
	if err != nil {
		return "", fmt.Errorf("couldn't call openai: %w", err)
	}

	var result struct {
		Title string `json:"title"`
	}
	if err := json.Unmarshal([]byte(completion.FunctionCall.Arguments), &result); err != nil {
		return "", fmt.Errorf("couldn't unmarshal function call arguments: %w", err)
	}

	return result.Title, nil
}
