package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/BurntSushi/toml"
	"github.com/charmbracelet/huh"
	openai "github.com/sashabaranov/go-openai"
	"github.com/sashabaranov/go-openai/jsonschema"
)

type openAI struct {
	Key string
	URL string
}

func (ai *openAI) GenerateCommitMessage(diff string) (string, string, error) {
	config := openai.DefaultAzureConfig(ai.Key, ai.URL)
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

type Config struct {
	Azure struct {
		Key string `toml:"key"`
		URL string `toml:"url"`
	} `toml:"settings"`
}

func main() {
	homeDir, err := os.UserHomeDir()
	// err != nil {

	// configPath := homeDir + "/.config/myapp/config.toml"

	// Create the directories if they don't exist
	// err = os.MkdirAll(homeDir + "/.config/myapp", os.ModePerm)
	// if err != nil && !os.IsExist(err) {
	// Handle error
	// }

	// Open the file for writing, overwriting if it exists
	// f, err := os.OpenFile(configPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		// Handle error
		fmt.Println("Failed to get home directory:", err)
		return
	}
	// defer f.Close()
	configPath := filepath.Join(homeDir, ".config", "ai-commit", "ai-commit.toml")
	config := Config{}
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		configForm := huh.NewForm(
			huh.NewGroup(
				huh.NewInput().
					Title("What’s your Azure Key?").
					Value(&config.Azure.Key).
					// Validating fields is easy. The form will mark erroneous fields
					// and display error messages accordingly.
					Validate(func(str string) error {
						if str == "Frank" {
							// return errors.New("Sorry, we don’t serve customers named Frank.")
						}
						return nil
					}),
				huh.NewInput().
					Title("What’s your Azure URL?").
					Value(&config.Azure.URL).
					// Validating fields is easy. The form will mark erroneous fields
					// and display error messages accordingly.
					Validate(func(str string) error {
						if str == "Frank" {
							// return errors.New("Sorry, we don’t serve customers named Frank.")
						}
						return nil
					}),

				// huh.NewText().
				// Title("Special Instructions").
				// CharLimit(400).
				// Value(&instructions),

				// huh.NewConfirm().
				// Title("Would you like 15% off?").
				// Value(&discount),
			),
		)
		configForm.Run()
		if err = os.MkdirAll(filepath.Dir(configPath), os.ModePerm); err != nil {
			fmt.Println("Failed to create config directory:", err)
			return
		}
		// f, err := os.Create(configPath)
		// if err != nil {
		// 	panic(err)
		// }
		// Save the config as a TOML file
		f, err := os.Create(configPath)
		if err != nil {
			fmt.Println("Failed to create config file:", err)
			return
		}
		defer f.Close()

		if err := toml.NewEncoder(f).Encode(config); err != nil {
			fmt.Println("Failed to encode config:", err)
			return
		}
	} else {

		if _, err := toml.DecodeFile(configPath, &config); err != nil {
			fmt.Println("Failed to load config:", err)
			return
		}
	}

	// Get the git diff
	diff, err := getGitDiff()
	if err != nil {
		fmt.Printf("Error getting git diff: %v\n", err)
		return
	}

	ai := openAI{Key: config.Azure.Key, URL: config.Azure.URL}

	// Generate commit message
	headline, description, err := ai.GenerateCommitMessage(diff)
	if err != nil {
		fmt.Printf("Error generating commit message: %v\n", err)
		return
	}

	// Print the result
	// fmt.Printf("Commit Headline: %s\n\nDescription:\n%s\n", headline, description)

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
