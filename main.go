package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"

	openai "github.com/sashabaranov/go-openai"
	"github.com/sashabaranov/go-openai/jsonschema"
	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/ollama"
)

// LLMService interface for service-agnostic LLM interactions
type LLMService interface {
	GenerateCommitMessage(diff string) (string, string, error)
}

// OllamaService implements LLMService using Ollama
type OllamaService struct {
	ModelName string
}

func GenerateCommitMessage(diff string) (string, string, error) {
	config := openai.DefaultAzureConfig(os.Getenv("azure_openai_key"), os.Getenv("azure_openai_url"))
	client := openai.NewClientWithConfig(config)
	ctx := context.Background()

	type Result struct {
		Explanation string `json:"headline"`
		Output      string `json:"description"`
	}

	var result Result
	schema, err := jsonschema.GenerateSchemaForType(result)
	if err != nil {
		fmt.Printf("Error generating schema: %v\n", err)
		return "", "", err
	}
	jsonSchema, err := schema.MarshalJSON()
	if err != nil {
		fmt.Printf("Error marshalling schema: %v\n", err)
		return "", "", err
	}
	resp, err := client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model: openai.GPT4o,
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleSystem,
				Content: "You are a helpful git comment author. Please provide a concise headline and a brief description for the following git diff.",
			},
			{
				Role:    openai.ChatMessageRoleSystem,
				Content: "Output muSt be in the form of a JSON object with a headline and description field according to schema: " + string(jsonSchema),
			},
			{
				Role:    openai.ChatMessageRoleUser,
				Content: "my diff is:\n" + diff,
			},
		},
		ResponseFormat: &openai.ChatCompletionResponseFormat{
			Type: openai.ChatCompletionResponseFormatTypeJSONObject,
			// JSONSchema: &openai.ChatCompletionResponseFormatJSONSchema{
			// Name:   "git_commit_message",
			// Schema: schema,
			// Strict: true,
			// },
		},
	})

	if err != nil {
		fmt.Printf("ChatCompletion error: %v\n", err)
		return "", "", err
	}

	fmt.Println(resp.Choices[0].Message.Content)
	return resp.Choices[0].Message.Content, "", nil
}

func (o *OllamaService) GenerateCommitMessage(diff string) (string, string, error) {
	prompt := fmt.Sprintf("Given the following git diff, generate a concise commit message. Provide a short concise headline within 50 characters, and a brief description, all text should be plain text and no text that not part of the headline or description is wanted, headline and description should be seperated by two blank lines:\n\n%s", diff)
	llm, err := ollama.New(ollama.WithModel("gemma:2b"))
	if err != nil {
		log.Fatal(err)
	}
	ctx := context.Background()
	completion, err := llms.GenerateFromSinglePrompt(
		ctx,
		llm,
		prompt,
		llms.WithTemperature(0.8),
		llms.WithStreamingFunc(func(ctx context.Context, chunk []byte) error {
			fmt.Print(string(chunk))
			return nil
		}),
	)
	if err != nil {
		log.Fatal(err)
	}
	// fmt.Println(completion)
	_ = completion
	// cmd := exec.Command("ollama", "run", o.ModelName, prompt)
	// output, err := cmd.CombinedOutput()
	// if err != nil {
	// 	return "", "", fmt.Errorf("error running Ollama: %w", err)
	// }

	// response := string(completion)
	// fmt.Println(response)
	parts := strings.SplitN(completion, "\n\n", 2)

	if len(parts) < 2 {
		return strings.TrimSpace(completion), "", nil
	}

	return strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]), nil
}

func main() {
	// Create an instance of OllamaService
	// llm := &OllamaService{ModelName: "gemma:2b"}

	// Get the git diff
	diff, err := getGitDiff()
	if err != nil {
		fmt.Printf("Error getting git diff: %v\n", err)
		return
	}
	fmt.Println(diff)

	// Generate commit message
	// headline, description, err := llm.GenerateCommitMessage(diff)
	headline, description, err := GenerateCommitMessage(diff)
	if err != nil {
		fmt.Printf("Error generating commit message: %v\n", err)
		return
	}

	// Print the result
	fmt.Printf("Commit Headline: %s\n\nDescription:\n%s\n", headline, description)
}

func getGitDiff() (string, error) {
	cmd := exec.Command("git", "diff", "--cached")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("error getting git diff: %w", err)
	}
	return string(output), nil
}
