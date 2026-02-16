package vault

// VaultReader provides read-only access to the vault.
// This interface decouples consumers from the concrete VaultFacade implementation.
type VaultReader interface {
	// Read returns the content of a note at the given path.
	Read(path string) (string, error)

	// List returns note paths under the given folder.
	List(folder string) ([]string, error)

	// Search performs full-text search across vault notes.
	Search(query string) (string, error)

	// Frontmatter returns the frontmatter of a note.
	Frontmatter(path string) (string, error)

	// IsHealthy returns true if the vault backend is operational.
	IsHealthy() bool

	// Mode returns the current operational mode (e.g., "full", "fallback", "degraded").
	Mode() string
}

// VaultWriter provides write access to the vault.
// Extends VaultReader with mutation operations.
type VaultWriter interface {
	VaultReader

	// Write writes content to a vault note with the given mode.
	Write(path, content string, mode WriteMode) error

	// Delete removes a note from the vault.
	Delete(path string) error

	// DailyAppend appends content to today's daily note (thread-safe).
	DailyAppend(content string) error

	// Daily returns today's daily note content.
	Daily() (string, error)

	// Close shuts down the writer and flushes pending operations.
	Close()
}
