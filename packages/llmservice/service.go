package llmservice

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	_ "embed"

	"encore.app/models"
	"encore.dev/rlog"
	"github.com/samber/lo"
	"github.com/tyloafer/langchaingo/llms"
	"github.com/tyloafer/langchaingo/llms/openai"
	"github.com/tyloafer/langchaingo/schema"
)

var secrets struct {
	OpenAIAPIKey string
}

type Service struct {
	chatGpt35Client *openai.Chat
	chatGpt4Client  *openai.Chat
	llmClient       *openai.LLM
}

//go:embed triage_message_func.json
var triageMessageFuncSchema string

//go:embed triage_message_prompt.txt
var triageMessagePrompt string

//go:embed tag_forum_post_prompt.txt
var tagForumPostPrompt string

//go:embed set_message_title_func_schema.json
var setMessageTitleFuncSchema string

//go:embed find_messages_matching_topic_prompt.txt
var findMessagesMatchingTopicPrompt string

//go:embed answer_forum_post_prompt.txt
var answerForumPostPrompt string

func NewService() (*Service, error) {
	chatGpt35Client, err := openai.NewChat(openai.WithModel("gpt-3.5-turbo-0613"), openai.WithToken(secrets.OpenAIAPIKey))
	if err != nil {
		return nil, fmt.Errorf("couldn't create openai chat client: %w", err)
	}

	chatGpt4Client, err := openai.NewChat(openai.WithModel("gpt-4-turbo-2024-04-09"), openai.WithToken(secrets.OpenAIAPIKey))
	if err != nil {
		return nil, fmt.Errorf("couldn't create openai chat client: %w", err)
	}

	llmClient, err := openai.New(
		openai.WithModel("text-embedding-3-large"),
		openai.WithToken(secrets.OpenAIAPIKey))
	if err != nil {
		return nil, fmt.Errorf("couldn't create openai llm client: %w", err)
	}

	return &Service{
		chatGpt35Client: chatGpt35Client,
		chatGpt4Client:  chatGpt4Client,
		llmClient:       llmClient,
	}, nil
}

func (s *Service) CreateEmbeddings(ctx context.Context, messages []string) ([][]float32, error) {
	embeddings, err := s.llmClient.CreateEmbedding(ctx, messages)
	if err != nil {
		return nil, fmt.Errorf("couldn't create embeddings: %w", err)
	}

	return embeddings, nil
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

	completion, err := s.chatGpt35Client.Call(ctx, []schema.ChatMessage{
		schema.HumanChatMessage{Content: tagForumPostPrompt},
		schema.HumanChatMessage{Content: "What follow is details of the forum post."},
		schema.HumanChatMessage{Content: fmt.Sprintf("Title: %s", forumPostTitle)},
		schema.HumanChatMessage{Content: fmt.Sprintf("Contents:\n%s", forumPostContents)},
	}, llms.WithFunctions(llmFunctions))
	if err != nil {
		return nil, fmt.Errorf("couldn't call openai: %w", err)
	} else if completion.FunctionCall == nil {
		rlog.Warn("No function call found in completion")
		return []string{}, nil
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

	completion, err := s.chatGpt35Client.Call(ctx, []schema.ChatMessage{
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

	completion, err := s.chatGpt35Client.Call(ctx, []schema.ChatMessage{
		schema.HumanChatMessage{
			Content: `
			This is a message by a user of our product and we want you to suggest a title for that message, 
			as if it's a forum post, made by the same user. 
			The title should sound like a question from the user and shouldn't have exclamations or 
			be too long or sound like a tutorial name.
			Treat it as the title of a support request towards a customer support agent.
			`},
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

func (s *Service) FindMessagesMatchingTopic(
	ctx context.Context,
	messages []*models.DiscordRawMessage,
	topics []string,
) ([]*models.DiscordRawMessage, error) {
	if len(messages) == 0 {
		return []*models.DiscordRawMessage{}, nil
	}

	var llmFunctions = []llms.FunctionDefinition{
		{
			Name:        "setMessagesMatchingTopics",
			Description: "Sets the message IDs, matching the given topics",
			Parameters: json.RawMessage(fmt.Sprintf(`
				{
				  "type": "object",
				  "properties": {
					"matchingMessages": { 
					  "type": "array", 
					  "items": { 
					    "type": "integer"
					  } 
					}
				  },
				  "required": ["matchingMessages"]
				}
			`)),
		},
	}

	messagesInput := strings.Join(lo.Map(messages, func(message *models.DiscordRawMessage, i int) string {
		return fmt.Sprintf("\nmessage %d:\n---\n%s\n---\n", i, message.CleanContent)
	}), "")

	completion, err := s.chatGpt35Client.Call(ctx, []schema.ChatMessage{
		schema.HumanChatMessage{Content: fmt.Sprintf(findMessagesMatchingTopicPrompt, strings.Join(topics, ", "))},
		schema.HumanChatMessage{Content: "Here's the messages you have to match:"},
		schema.HumanChatMessage{Content: messagesInput},
	}, llms.WithFunctions(llmFunctions))
	if err != nil {
		return nil, fmt.Errorf("couldn't call openai: %w", err)
	}

	var result struct {
		MatchingMessages []int `json:"matchingMessages"`
	}

	if err := json.Unmarshal([]byte(completion.FunctionCall.Arguments), &result); err != nil {
		return nil, fmt.Errorf("couldn't unmarshal function call arguments: %w", err)
	}

	return lo.Map(result.MatchingMessages, func(id int, _ int) *models.DiscordRawMessage {
		if (id < 0) || (id >= len(messages)) {
			panic(fmt.Sprintf("invalid message ID: %d", id))
		}

		return messages[id]
	}), nil
}

func (s *Service) AnswerForumPost(
	ctx context.Context,
	forumPostContents string,
	knowledgeBase []*models.KnowledgeBaseArticle,
) (string, error) {
	// var llmFunctions = []llms.FunctionDefinition{
	// 	{
	// 		Name:        "provideAnswer",
	// 		Description: "Sets the answer provided to the user",
	// 		Parameters: json.RawMessage(fmt.Sprintf(`
	// 			{
	// 			  "type": "object",
	// 			  "properties": {
	// 				"yourAnswerOrSolution": {
	// 				  "type": "string"
	// 				}
	// 			  },
	// 			  "required": ["yourAnswerOrSolution"]
	// 			}
	// 		`)),
	// 	},
	// }

	knowledgeBaseInput := strings.Join(lo.Map(knowledgeBase, func(article *models.KnowledgeBaseArticle, i int) string {
		return fmt.Sprintf("\nArticle %d:\n---\n%s\n---\n\n", i, article.Text)
	}), "\n")

	prompt := fmt.Sprintf(answerForumPostPrompt, knowledgeBaseInput, forumPostContents)
	completion, err := s.chatGpt4Client.Call(ctx, []schema.ChatMessage{
		schema.HumanChatMessage{Content: prompt},
	})
	if err != nil {
		return "", fmt.Errorf("couldn't call openai: %w", err)
	}
	// } else if completion.FunctionCall == nil {
	// 	return "", errors.New("No function call found in completion")
	// }

	// var result struct {
	// 	YourAnswerOrSolution string `json:"answer"`
	// }

	// if err := json.Unmarshal([]byte(completion.FunctionCall.Arguments), &result); err != nil {
	// 	return "", fmt.Errorf("couldn't unmarshal function call arguments: %w", err)
	// }

	return completion.Content, nil
	// return result.YourAnswerOrSolution, nil
}
