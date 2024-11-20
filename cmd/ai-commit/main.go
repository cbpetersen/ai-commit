package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/cbpetersen/ai-commit/internal"
	"github.com/cbpetersen/ai-commit/internal/ai"
	"github.com/cbpetersen/ai-commit/internal/git"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/huh/spinner"
	"github.com/charmbracelet/log"
	"github.com/urfave/cli/v2"
)

func SplitIntoPatchesFlow(aiEngine *ai.OpenAI, dryRun bool) error {
	commits := ""
	for {
		diff, err := git.GetGitDiff()
		if err != nil {
			return err
		}
		if strings.TrimSpace(diff) == "" {
			return nil
		}

		patch, err := aiEngine.CreatePatchFromDiff(context.Background(), diff, commits)
		if err != nil {
			return err
		}

		if patch.ContainsFaults {
			return fmt.Errorf("AI detected faults in the patch")
		}

		if len(patch.Patch) == 0 {
			break
		}
		ok := false
		for _, hunk := range patch.Patch {
			if hunk == "y" {
				ok = true
				break
			}
		}
		if !ok {
			log.Info("No more hunks to stage.")
			return nil
		}

		if err := git.StageHunksFromPatch(patch); err != nil {
			return err
		}

		if dryRun {
			break
		}

		if err := git.Commit(patch.CommitMessage); err != nil {
			return err
		}

		if !patch.MorePatchesRemaining {
			break
		}

		lastCommit, err := git.GetLastCommit()
		if err != nil {
			return err
		}
		commits += lastCommit + "\n"
	}
	return nil
}

func main() {
	version := "0.1.0"
	log.SetLevel(log.DebugLevel)
	var showConfig int
	var doPatches int
	app := &cli.App{
		Flags: []cli.Flag{
			&cli.BoolFlag{Name: "config", Count: &showConfig, Usage: "Update the current configuration"},
			&cli.BoolFlag{Name: "version", Aliases: []string{"v"}, Usage: "Print the version"},
			&cli.BoolFlag{Name: "patch", Aliases: []string{"p"}, Count: &doPatches, Usage: "Create a patch file instead of committing"},
		},
		EnableBashCompletion: true,
		HideHelp:             false,
		HideVersion:          false,
		CommandNotFound: func(cCtx *cli.Context, command string) {
			fmt.Fprintf(cCtx.App.Writer, "No Options with %q here.\n", command)
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

	ai := ai.OpenAI{Key: config.Azure.Key, URL: config.Azure.URL}

	if doPatches > 0 {
		err := SplitIntoPatchesFlow(&ai, dryRun > 0)
		if err != nil {
			log.Fatalf("Error creating patches: %v", err)
		}
		return
	}

	err = CommitFlow(&ai)
	if err != nil {
		log.Fatalf("Error in commit flow: %v", err)
	}
}

func CommitFlow(ai *ai.OpenAI) error {
	diff, err := git.GetGitDiffCached()
	if err != nil {
		return err
	}

	if strings.TrimSpace(diff) == "" {
		log.Error("No changes to commit.")
		return err
	}

	// Generate commit message
	var headline, description string
	var aiError error
	err = spinner.New().Action(func() {
		ctx, cancelled := context.WithTimeout(context.Background(), time.Second*30)
		headline, description, aiError = ai.GenerateCommitMessage(ctx, diff)
		cancelled()
	}).Title("Generating commit message...").Run()

	if err != nil {
		return err
	}

	if aiError != nil {
		return aiError
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
		return err
	}

	switch useCommit {
	case git.UseCommit:
		err = git.CreateCommit(headline, description)
	case git.EditCommit:
		err = git.EditCommitMessage(headline, description)
	case git.DontUseCommit:
		log.Error("Commit message not used.")
	}

	return err

}
