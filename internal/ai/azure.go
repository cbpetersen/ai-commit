package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/charmbracelet/log"
	openai "github.com/sashabaranov/go-openai"
	"github.com/sashabaranov/go-openai/jsonschema"
)

type OpenAI struct {
	Key string
	URL string
}

type Patch struct {
	Patch                []string `json:"patch"`
	CommitMessage        string   `json:"commitMessage"`
	MorePatchesRemaining bool     `json:"morePatchesRemaining"`
	ContainsFaults       bool     `json:"containsFaults"`
}

func (ai *OpenAI) CreatePatchFromDiff(ctx context.Context, diff string, commits string) (*Patch, error) {
	config := openai.DefaultAzureConfig(ai.Key, ai.URL)
	client := openai.NewClientWithConfig(config)

	type GPTPatch struct {
		IncludedHunks        []string      `json:"included-hunks"`
		Reason               string        `json:"reason"`
		CommitMessage        CommitMessage `json:"commitMessage"`
		MorePatchesRemaining bool          `json:"morePatchesRemaining"`
		ContainsFaults       bool          `json:"containsFaults"`
	}

	var result GPTPatch
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
	// log.Debug(diff)
	log.Info("Querying AI for patch")

	numHunks := 0
	for _, line := range strings.Split(diff, "\n") {
		if strings.HasPrefix(line, "@@") {
			numHunks++
		}
	}

	resp, err := client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model: openai.GPT4o,
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleSystem,
				Content: "You are a helpful git champion and master coder. Please provide a assistance with creating patches and commit messages based on the git diff.",
			},
			{
				Role:    openai.ChatMessageRoleSystem,
				Content: "Output must be in the form of a JSON object with a patches including dependencies, headline and description field according to schema: " + string(jsonSchema),
			},
			{
				Role: openai.ChatMessageRoleSystem,
				Content: `
				I have a long diff, that probably should be split into multiple patches / commits based on what makes sense to commit together.
				A patch can span across multiple files and hunks if it makes sense to be grouped in the same commit.
				The diff is created with 'git --no-pager diff'
				For included-hunks i want a list like ["y", "n", "y", "n", "y", "n", "y"...] where y means to apply the hunk and n means to skip the patch this should be in the included-hunks just as yes and no in a 'git add -p'.
				Create the patches in the order that makes most sense for a human on how the changes are created.
				I only want to create one patch at a time.
				please set morePatchesRemaining to if everthing is committed.
				If you discover i accidentally added keys that should not be included in the patch, please do not include them in the patch and set the containsFaults flag.
				`,
			}, {
				Role:    openai.ChatMessageRoleSystem,
				Content: fmt.Sprintf(`Number of hunks in the diff: %d so the included-hunks elements should have the same count`, numHunks),
			},
			{
				Role: openai.ChatMessageRoleSystem,
				Content: `Git Commit messages
				Changes to dependencies should not be mentioned directly but should be in the structured output
				Headline should only mention the core area of the diff that is being staged in the patch.
				`,
			},
			{
				Role:    openai.ChatMessageRoleUser,
				Content: fmt.Sprintf("my diff is:\n%s\n\nFor context i have applied the following commit:\n%s", diff, commits),
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
	log.Debugf("resp: %v", resp)
	var patch GPTPatch
	err = json.Unmarshal([]byte(resp.Choices[0].Message.Content), &patch)
	if err != nil {
		return nil, fmt.Errorf("error unmarshalling response: %w", err)
	}

	out := &Patch{
		Patch:                patch.IncludedHunks,
		CommitMessage:        fmt.Sprintf("%s\n\n%s", patch.CommitMessage.Headline, patch.CommitMessage.descriptionWithDependencies()),
		MorePatchesRemaining: patch.MorePatchesRemaining,
	}

	if len(out.Patch) != numHunks {
		return nil, fmt.Errorf("expected %d hunks in the patch, got %d", numHunks, len(out.Patch))
	}

	// Validate the patch output
	for _, hunk := range out.Patch {
		if hunk != "n" && hunk != "y" {
			return nil, fmt.Errorf("invalid hunk: %s", hunk)
		}
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
