package ai

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/charmbracelet/log"
	openai "github.com/sashabaranov/go-openai"
	"github.com/sashabaranov/go-openai/jsonschema"
)

type OpenAI struct {
	Key string
	URL string
}

type Patch struct {
	Patch         string `json:"patch"`
	CommitMessage string `json:"commitMessage"`
}

func (ai *OpenAI) CreatePatchFromDiff(ctx context.Context, diff string) ([]Patch, error) {

	config := openai.DefaultAzureConfig(ai.Key, ai.URL)
	client := openai.NewClientWithConfig(config)

	type Patches struct {
		Patches []struct {
			Patch         string        `json:"patch"`
			Reason        string        `json:"reason"`
			CommitMessage CommitMessage `json:"commitMessage"`
		} `json:"patches"`
	}

	var result Patches
	schema, err := jsonschema.GenerateSchemaForType(result)
	if err != nil {
		fmt.Printf("Error generating schema: %v\n", err)
		return nil, err
	}
	jsonSchema, err := schema.MarshalJSON()
	if err != nil {
		fmt.Printf("Error marshalling schema: %v\n", err)
		return nil, err
	}
	resp, err := client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model: openai.GPT4o,
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleSystem,
				Content: "You are a helpful git champion and master coder. Please provide a assistance with creating patches and commit messages based on the git diff.",
			},
			{
				Role: openai.ChatMessageRoleSystem,
				Content: `
				git patches shoud be in a format that can be used with git apply.
				if it makes sense to split the diff into multiple patches please do so.
				`,
			},
			{
				Role: openai.ChatMessageRoleSystem,
				Content: `Git Commit messages
	Changes to dependencies should not be mentioned directly but should be in the structured output
				Headline should only mention the core area of the diff
				`,
			},
			{
				Role:    openai.ChatMessageRoleSystem,
				Content: "Output must be in the form of a JSON object with a patches including dependencies, headline and description field according to schema: " + string(jsonSchema),
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
		return nil, err
	}
	log.Infof("resp: %v", resp)
	var patches Patches
	err = json.Unmarshal([]byte(resp.Choices[0].Message.Content), &patches)
	if err != nil {
		return nil, fmt.Errorf("error unmarshalling response: %w", err)
	}

	out := []Patch{}
	for _, patch := range patches.Patches {
		out = append(out, Patch{
			Patch:         patch.Patch,
			CommitMessage: fmt.Sprintf("%s\n\n%s", patch.CommitMessage.Headline, patch.CommitMessage.descriptionWithDependencies()),
		})
	}

	return out, nil
}

type Dependency struct {
	Version string `json:"version"`
	Name    string `json:"name"`
}

type CommitMessage struct {
	Headline     string `json:"headline"`
	Description  string `json:"description"`
	Dependencies struct {
		Added      []Dependency `json:"added"`
		Upgraded   []Dependency `json:"upgraded"`
		Downgraded []Dependency `json:"downgraded"`
		Removed    []Dependency `json:"removed"`
	} `json:"dependencies"`
}

func (ai *OpenAI) GenerateCommitMessage(ctx context.Context, diff string) (string, string, error) {
	config := openai.DefaultAzureConfig(ai.Key, ai.URL)
	client := openai.NewClientWithConfig(config)

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
				Content: "Headline should only mention the core area of the diff",
			},
			{
				Role:    openai.ChatMessageRoleSystem,
				Content: "Changes to dependencies should not be mentioned directly but should be in the structured output",
			},
			{
				Role:    openai.ChatMessageRoleSystem,
				Content: "Output must be in the form of a JSON object with a dependencies, headline and description field according to schema: " + string(jsonSchema),
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
	// addDependencies := func(dependencyStr, typeOfDependency string, deps []Dependency) string {
	// 	if len(deps) == 0 {
	// 		return dependencyStr
	// 	}
	// 	d := fmt.Sprintf("- %s:\n", typeOfDependency)
	// 	for _, dep := range deps {
	// 		d += fmt.Sprintf("  - %s (%s)\n", dep.Name, dep.Version)
	// 	}
	//
	// 	return fmt.Sprintf("%s\n%s", dependencyStr, d)
	// }
	//
	// dependencies := ""
	// dependencies = addDependencies(dependencies, "Added", commitMessage.Dependencies.Added)
	// dependencies = addDependencies(dependencies, "Upgraded", commitMessage.Dependencies.Upgraded)
	// dependencies = addDependencies(dependencies, "Downgraded", commitMessage.Dependencies.Downgraded)
	// dependencies = addDependencies(dependencies, "Removed", commitMessage.Dependencies.Removed)
	description := commitMessage.descriptionWithDependencies()

	return commitMessage.Headline, description, nil
}

func (c *CommitMessage) descriptionWithDependencies() string {
	addDependencies := func(dependencyStr, typeOfDependency string, deps []Dependency) string {
		if len(deps) == 0 {
			return dependencyStr
		}
		d := fmt.Sprintf("- %s:\n", typeOfDependency)
		for _, dep := range deps {
			d += fmt.Sprintf("  - %s (%s)\n", dep.Name, dep.Version)
		}

		return fmt.Sprintf("%s\n%s", dependencyStr, d)
	}

	dependencies := ""
	dependencies = addDependencies(dependencies, "Added", c.Dependencies.Added)
	dependencies = addDependencies(dependencies, "Upgraded", c.Dependencies.Upgraded)
	dependencies = addDependencies(dependencies, "Downgraded", c.Dependencies.Downgraded)
	dependencies = addDependencies(dependencies, "Removed", c.Dependencies.Removed)

	description := c.Description

	if len(dependencies) > 0 {
		description = fmt.Sprintf("%s\n\nDependency changes:%s", description, dependencies)
	}

	return description
}
