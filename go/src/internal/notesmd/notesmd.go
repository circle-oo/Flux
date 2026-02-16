package notesmd

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

// Client wraps the notesmd-cli binary for vault operations.
type Client struct {
	vaultName string
	binPath   string
}

// NewClient creates a Client that targets the given vault name.
// The vault must already be registered with notesmd-cli (via "notesmd-cli set-default").
func NewClient(vaultName string) *Client {
	return &Client{
		vaultName: vaultName,
		binPath:   "notesmd-cli",
	}
}

// Print reads and returns the content of a note.
func (c *Client) Print(notePath string) (string, error) {
	return c.run("print", notePath)
}

// Create creates a new note with the given content.
func (c *Client) Create(notePath, content string) error {
	_, err := c.run("create", notePath, "--content", content)
	return err
}

// Overwrite creates or overwrites a note with the given content.
func (c *Client) Overwrite(notePath, content string) error {
	_, err := c.run("create", notePath, "--content", content, "--overwrite")
	return err
}

// Append appends content to an existing note.
func (c *Client) Append(notePath, content string) error {
	_, err := c.run("create", notePath, "--content", content, "--append")
	return err
}

// Delete removes a note from the vault.
func (c *Client) Delete(notePath string) error {
	_, err := c.run("delete", notePath)
	return err
}

// Move renames or moves a note within the vault.
func (c *Client) Move(from, to string) error {
	_, err := c.run("move", from, to)
	return err
}

// List returns the notes in a folder (or all notes if folder is empty).
func (c *Client) List(folder string) ([]string, error) {
	args := []string{"list"}
	if folder != "" {
		args = append(args, folder)
	}
	out, err := c.runArgs(args...)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(out) == "" {
		return nil, nil
	}
	return strings.Split(strings.TrimRight(out, "\n"), "\n"), nil
}

// SearchContent performs a full-text search across vault notes.
func (c *Client) SearchContent(query string) (string, error) {
	return c.run("search-content", query)
}

// Frontmatter returns the frontmatter of a note as a string.
func (c *Client) Frontmatter(notePath string) (string, error) {
	return c.run("frontmatter", notePath, "--print")
}

// SetFrontmatter sets a single frontmatter key-value pair on a note.
func (c *Client) SetFrontmatter(notePath, key, val string) error {
	_, err := c.run("frontmatter", notePath, "--edit", "--key", key, "--value", val)
	return err
}

// Daily creates or opens the daily note.
func (c *Client) Daily() error {
	_, err := c.run("daily")
	return err
}

// run executes a notesmd-cli subcommand with the vault flag.
func (c *Client) run(args ...string) (string, error) {
	return c.runArgs(args...)
}

// runArgs builds and executes the full notesmd-cli command.
func (c *Client) runArgs(args ...string) (string, error) {
	full := append(args, "--vault", c.vaultName)
	cmd := exec.Command(c.binPath, full...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("notesmd-cli %s: %w: %s", args[0], err, strings.TrimSpace(stderr.String()))
	}
	return stdout.String(), nil
}
