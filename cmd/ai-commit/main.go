package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/cbpetersen/ai-commit/internal"
	"github.com/cbpetersen/ai-commit/internal/ai"
	"github.com/cbpetersen/ai-commit/internal/git"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/log"
	"github.com/urfave/cli/v2"
)

func main() {
	version := "0.1.0"
	log.SetLevel(log.DebugLevel)
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
		log.Fatal(err)
	}

	config, err := internal.GetConfig(showConfig > 0)
	if err != nil {
		log.Fatal(err)
	}

	// Get the git diff
	diff, err := git.GetGitDiff()
	if err != nil {
		fmt.Printf("Error getting git diff: %v\n", err)
		return
	}

	if strings.TrimSpace(diff) == "" {
		fmt.Println("No changes to commit.")
		return
	}
	ai := ai.OpenAI{Key: config.Azure.Key, URL: config.Azure.URL}

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
					huh.NewOption("Use commit", git.UseCommit),
					huh.NewOption("edit commit", git.EditCommit),
					huh.NewOption("Do not commit", git.DontUseCommit),
				).Value(&useCommit),
		),
	)

	err = form.Run()
	if err != nil {
		log.Fatalf("Error running form: %v", err)
	}

	switch useCommit {
	case git.UseCommit:
		err = git.CreateCommit(headline, description)
	case git.EditCommit:
		err = git.EditCommitMessage(headline, description)
	case git.DontUseCommit:
		fmt.Println("Commit message not used.")
	}
}
