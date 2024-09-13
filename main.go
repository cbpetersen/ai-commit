package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/joho/godotenv"
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

	type CommitMessage struct {
		Headline    string `json:"headline"`
		Description string `json:"description"`
	}

	var result CommitMessage
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
				Content: "Output must be in the form of a JSON object with a headline and description field according to schema: " + string(jsonSchema),
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

	var commitMessage CommitMessage
	err = json.Unmarshal([]byte(resp.Choices[0].Message.Content), &commitMessage)
	if err != nil {
		return "", "", fmt.Errorf("error unmarshalling response: %w", err)
	}

	return commitMessage.Headline, commitMessage.Description, nil
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
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}
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

	// headline, description, err := llm.GenerateCommitMessage(diff)
	// if err != nil {
	// log.Fatalf("Error generating commit message: %v", err)
	// }

	// Create a form using charmbracelet/huh
	var useCommit bool
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewNote().
				Title("Generated Commit Message").
				Description(fmt.Sprintf("Headline: %s\n\nDescription: %s", headline, description)),
			huh.NewConfirm().
				Title("Do you want to use this commit message?").
				Value(&useCommit),
		),
	)

	err = form.Run()
	if err != nil {
		log.Fatalf("Error running form: %v", err)
	}

	if useCommit {
		err = createCommit(headline, description)
		if err != nil {
			log.Fatalf("Error creating commit: %v", err)
		}
		fmt.Println("Commit created successfully!")
	} else {
		fmt.Println("Commit message not used.")
	}
}

func getGitDiff() (string, error) {
	cmd := exec.Command("git", "diff", "--cached")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("error getting git diff: %w", err)
	}
	return string(output), nil
}

func createCommit(headline, description string) error {
	message := fmt.Sprintf("%s\n\n%s", headline, description)
	cmd := exec.Command("git", "commit", "-m", message)
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("error creating commit: %w", err)
	}
	return nil
}
