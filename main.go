package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/charmbracelet/huh"
	openai "github.com/sashabaranov/go-openai"
	"github.com/sashabaranov/go-openai/jsonschema"
	"github.com/urfave/cli/v2"
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
	version := "0.1.0"
	var showConfig int
	app := &cli.App{
		Flags: []cli.Flag{
			&cli.BoolFlag{Name: "config", Count: &showConfig, Usage: "Update the current configuration"},
			&cli.BoolFlag{Name: "version", Aliases: []string{"v"}, Usage: "Print the version"},
		},
		EnableBashCompletion: true,
		HideHelp:             false,
		HideVersion:          false,
		CommandNotFound: func(cCtx *cli.Context, command string) {
			fmt.Fprintf(cCtx.App.Writer, "Thar be no %q here.\n", command)
		},
		Action: func(ctx *cli.Context) error {
			if ctx.Bool("version") {
				return cli.Exit(fmt.Sprintf("Version: %s", version), 0)
			}
			return nil
		},
	}
	if err := app.Run(os.Args); err != nil {
		panic(err)
	}

	homeDir, err := os.UserHomeDir()

	if err != nil {
		fmt.Println("Failed to get home directory:", err)
		return
	}
	configPath := filepath.Join(homeDir, ".config", "ai-commit", "ai-commit.toml")
	config := Config{}
	if _, err := os.Stat(configPath); os.IsNotExist(err) || showConfig > 0 {
		configForm := huh.NewForm(
			huh.NewGroup(
				huh.NewInput().
					Title("What’s your Azure Key?").
					Value(&config.Azure.Key).
					EchoMode(huh.EchoModePassword).
					// Validating fields is easy. The form will mark erroneous fields
					// and display error messages accordingly.
					Validate(func(str string) error {
						if str == "" {
							return errors.New("Sorry, this cannot be empty")
						}
						return nil
					}),
				huh.NewInput().
					Title("What’s your Azure URL?").
					Value(&config.Azure.URL).
					// Validating fields is easy. The form will mark erroneous fields
					// and display error messages accordingly.
					Validate(func(str string) error {
						if str == "" {
							return errors.New("Sorry, this cannot be empty")
						}
						return nil
					}),
			),
		)
		configForm.Run()
		if err = os.MkdirAll(filepath.Dir(configPath), os.ModePerm); err != nil {
			fmt.Println("Failed to create config directory:", err)
			return
		}
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

	if strings.TrimSpace(diff) == "" {
		fmt.Println("No changes to commit.")
		return
	}
	ai := openAI{Key: config.Azure.Key, URL: config.Azure.URL}

	// Generate commit message
	headline, description, err := ai.GenerateCommitMessage(diff)
	if err != nil {
		fmt.Printf("Error generating commit message: %v\n", err)
		return
	}

	// Create a form using charmbracelet/huh
	var useCommit string
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewNote().
				Title("Generated Commit Message").
				Description(fmt.Sprintf("Headline: %s\n\nDescription: %s", headline, description)),
			huh.NewSelect[string]().
				Title("Do you want to use this commit message?").
				Options(
					huh.NewOption("Use commit", UseCommit),
					huh.NewOption("edit commit", EditCommit),
					huh.NewOption("Do not commit", DontUseCommit),
				).Value(&useCommit),
		),
	)

	err = form.Run()
	if err != nil {
		log.Fatalf("Error running form: %v", err)
	}

	switch useCommit {
	case UseCommit:
		err = createCommit(headline, description)
	case EditCommit:
		err = editCommit(headline, description)
	case DontUseCommit:
		fmt.Println("Commit message not used.")
	}
}

const (
	UseCommit     = "use"
	EditCommit    = "edit"
	DontUseCommit = "dont-use"
)

func getGitDiff() (string, error) {
	cmd := exec.Command("git", "diff", "--cached")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("error getting git diff: %w", err)
	}
	return string(output), nil
}

func editCommit(headline, description string) error {
	message := fmt.Sprintf("%s\n\n%s", headline, description)
	tempFile, err := os.CreateTemp("", "tempfile-*.txt")
	if err != nil {
		return fmt.Errorf("error creating temp file: %w", err)
	}
	defer os.Remove(tempFile.Name())
	tempFile.WriteString(message)
	output, err := exec.Command("git", "config", "core.editor").Output()
	if err != nil {
		return fmt.Errorf("error getting git editor: %w", err)
	}
	cmd := exec.Command(strings.TrimSpace(string(output)), tempFile.Name())
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("error running editor: %w", err)
	}
	cmd = exec.Command("git", "commit", "-F", tempFile.Name())
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("error creating commit: %w", err)
	}
	return nil
}

func createCommit(headline, description string) error {
	message := fmt.Sprintf("%s\n\n%s", headline, description)
	cmd := exec.Command("git", "commit", "-m", message)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("error creating commit: %w", err)
	}
	return nil
}
