package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"

	"github.com/charmbracelet/huh"
	"github.com/joho/godotenv"
	openai "github.com/sashabaranov/go-openai"
	"github.com/sashabaranov/go-openai/jsonschema"
)

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

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}
	// Get the git diff
	diff, err := getGitDiff()
	if err != nil {
		fmt.Printf("Error getting git diff: %v\n", err)
		return
	}
	fmt.Println(diff)

	// Generate commit message
	headline, description, err := GenerateCommitMessage(diff)
	if err != nil {
		fmt.Printf("Error generating commit message: %v\n", err)
		return
	}

	// Print the result
	fmt.Printf("Commit Headline: %s\n\nDescription:\n%s\n", headline, description)

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
