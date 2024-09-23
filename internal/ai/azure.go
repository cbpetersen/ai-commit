package ai

import (
	"context"
	"encoding/json"
	"fmt"

	openai "github.com/sashabaranov/go-openai"
	"github.com/sashabaranov/go-openai/jsonschema"
)

type OpenAI struct {
	Key string
	URL string
}

func (ai *OpenAI) GenerateCommitMessage(ctx context.Context, diff string) (string, string, error) {
	config := openai.DefaultAzureConfig(ai.Key, ai.URL)
	client := openai.NewClientWithConfig(config)

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
	dependencies = addDependencies(dependencies, "Added", commitMessage.Dependencies.Added)
	dependencies = addDependencies(dependencies, "Upgraded", commitMessage.Dependencies.Upgraded)
	dependencies = addDependencies(dependencies, "Downgraded", commitMessage.Dependencies.Downgraded)
	dependencies = addDependencies(dependencies, "Removed", commitMessage.Dependencies.Removed)

	description := commitMessage.Description

	if len(dependencies) > 0 {
		description = fmt.Sprintf("%s\n\nDependency changes:%s", description, dependencies)
	}

	return commitMessage.Headline, description, nil
}
