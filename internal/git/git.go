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

func GetLastCommit() (string, error) {
	cmd := exec.Command("git", "format-patch", "-1", "--stdout")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("error getting last commit: %w", err)
	}
	return string(output), nil
}

func GetGitDiff() (string, error) {
	cmd := exec.Command("git", "--no-pager", "diff")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("error getting git diff: %w", err)
	}
	return string(output), nil
}
func ResetAndStash() (string, error) {
	cmd := exec.Command("git", "reset")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("error reseting: %w", err)
	}

	cmd = exec.Command("git", "stash")
	output, err = cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("error stashing: %w", err)
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

func StageHunksFromPatch(msg *ai.Patch) error {
	str := "EOF" + "\n"
	for i := 0; i < len(msg.Patch); i++ {
		str += string(msg.Patch[i])
		str += "\n"
	}
	str += "EOF"

	cmd := exec.Command("bash", "-c", fmt.Sprintf("git add --patch << %s", str))

	out, err := cmd.CombinedOutput()
	if err != nil {
		return err
	}
	log.Debug(string(out))

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
