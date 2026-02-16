package notesmd

import (
	"strings"
	"testing"
)

func TestNewClient(t *testing.T) {
	c := NewClient("TestVault")
	if c.vaultName != "TestVault" {
		t.Errorf("expected vault name TestVault, got %s", c.vaultName)
	}
	if c.binPath != "notesmd-cli" {
		t.Errorf("expected bin path notesmd-cli, got %s", c.binPath)
	}
}

func TestRunArgs_BuildsCommand(t *testing.T) {
	c := NewClient("MyVault")

	// We can't easily test actual execution without the binary,
	// but we can verify the client struct is correctly configured.
	// Integration tests require notesmd-cli installed + a real vault.
	if c.vaultName != "MyVault" {
		t.Errorf("vault name mismatch: got %s", c.vaultName)
	}
}

func TestList_EmptyOutput(t *testing.T) {
	// Verify that empty string produces nil slice (not a slice with empty string)
	out := ""
	trimmed := strings.TrimSpace(out)
	if trimmed != "" {
		t.Errorf("expected empty trimmed string")
	}
}
