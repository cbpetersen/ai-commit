package git

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/cbpetersen/ai-commit/internal/ai"
	"github.com/charmbracelet/log"
)

const (
	UseCommit     = "use"
	EditCommit    = "edit"
	DontUseCommit = "dont-use"
)

func GetGitDiff() (string, error) {
	cmd := exec.Command("git", "diff")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("error getting git diff: %w", err)
	}
	return string(output), nil
}

func GetGitDiffCached() (string, error) {
	cmd := exec.Command("git", "diff", "--cached")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("error getting git diff: %w", err)
	}
	return string(output), nil
}

func EditCommitMessage(headline, description string) error {
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

func CreateCommitFromPatch(msg ai.Patch) error {
	f, err := os.Create("patch.diff")
	defer f.Close()
	// defer os.Remove("patch.diff")
	if err != nil {
		return fmt.Errorf("error creating patch file: %w", err)
	}
	log.Debug("patch file created")
	log.Debug(msg.Patch)
	if _, err := f.WriteString(msg.Patch); err != nil {
		return fmt.Errorf("error writing patch file: %w", err)
	}
	cmd := exec.Command("git", "apply", "--verbose", "patch.diff")
	output, err := cmd.CombinedOutput()

	if err != nil {
		return fmt.Errorf("error applying patch: %w\n%s", err, output)
	}
	log.Infof("patch applied:\n%s", output)

	return nil
}

func Commit(msg string) error {
	cmd := exec.Command("git", "commit", "-m", msg)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("error creating commit: %w", err)
	}
	return nil
}

func CreateCommit(headline, description string) error {
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
