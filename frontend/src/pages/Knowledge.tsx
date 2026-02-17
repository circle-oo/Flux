import { useEffect, useState, useCallback } from 'react'
import ReactMarkdown from 'react-markdown'
import { useKnowledgeStore } from '../stores/knowledgeStore'
import PageHeader from '../components/PageHeader'
import LoadingState from '../components/LoadingState'
import ErrorBanner from '../components/ErrorBanner'

type KnowledgeTab = 'browse' | 'search' | 'health'

export default function Knowledge() {
  const [tab, setTab] = useState<KnowledgeTab>('browse')

  const tabs: { id: KnowledgeTab; label: string }[] = [
    { id: 'browse', label: 'Browse' },
    { id: 'search', label: 'Search' },
    { id: 'health', label: 'Health' },
  ]

  return (
    <div className="page-shell space-y-6 animate-fade-in">
      <PageHeader
        title="Knowledge"
        subtitle="Browse and search your vault notes"
      />

      {/* Tab Bar */}
      <div className="flex gap-1 p-1 bg-surface-hover rounded-lg w-fit">
        {tabs.map((t) => (
          <button
            key={t.id}
            onClick={() => setTab(t.id)}
            className={`px-4 py-2 rounded-md text-sm font-medium transition-all ${
              tab === t.id
                ? 'bg-surface text-content shadow-sm'
                : 'text-content-muted hover:text-content-secondary'
            }`}
          >
            {t.label}
          </button>
        ))}
      </div>

      {tab === 'browse' && <BrowseTab />}
      {tab === 'search' && <SearchTab />}
      {tab === 'health' && <HealthTab />}
    </div>
  )
}

function BrowseTab() {
  const {
    notes,
    folders,
    currentNote,
    isLoading,
    error,
    fetchNotes,
    fetchFolders,
    fetchNote,
    clearCurrentNote,
    createNote,
  } = useKnowledgeStore()

  const [selectedFolder, setSelectedFolder] = useState<string | null>(null)
  const [showCreateDialog, setShowCreateDialog] = useState(false)
  const [newNotePath, setNewNotePath] = useState('')
  const [newNoteContent, setNewNoteContent] = useState('')

  useEffect(() => {
    fetchFolders()
    fetchNotes()
  }, [fetchFolders, fetchNotes])

  const handleFolderClick = useCallback((folder: string) => {
    setSelectedFolder(folder)
    clearCurrentNote()
    fetchNotes(folder)
  }, [clearCurrentNote, fetchNotes])

  const handleNoteClick = useCallback((path: string) => {
    fetchNote(path)
  }, [fetchNote])

  const handleShowAll = useCallback(() => {
    setSelectedFolder(null)
    clearCurrentNote()
    fetchNotes()
  }, [clearCurrentNote, fetchNotes])

  const handleCreateNote = async () => {
    if (!newNotePath.trim()) return
    try {
      await createNote(newNotePath.trim(), newNoteContent)
      setShowCreateDialog(false)
      setNewNotePath('')
      setNewNoteContent('')
      fetchNotes(selectedFolder || undefined)
    } catch {
      // error is set in store
    }
  }

  return (
    <div className="space-y-4">
      {error && <ErrorBanner message={error} />}

      <div className="flex justify-end">
        <button
          onClick={() => setShowCreateDialog(true)}
          className="px-3 py-1.5 text-xs font-medium bg-primary-500 text-white rounded-lg hover:bg-primary-600 transition-colors"
        >
          Create Note
        </button>
      </div>

      {/* Create Note Dialog */}
      {showCreateDialog && (
        <div className="card p-5 space-y-3">
          <div className="text-[11px] font-medium text-content-faint uppercase tracking-widest">New Note</div>
          <input
            type="text"
            placeholder="Path (e.g. projects/my-note)"
            value={newNotePath}
            onChange={(e) => setNewNotePath(e.target.value)}
            className="w-full px-3 py-2 text-sm bg-surface border border-line rounded-lg text-content placeholder:text-content-faint focus:outline-none focus:border-primary-500"
          />
          <textarea
            placeholder="Content (markdown)"
            value={newNoteContent}
            onChange={(e) => setNewNoteContent(e.target.value)}
            rows={4}
            className="w-full px-3 py-2 text-sm bg-surface border border-line rounded-lg text-content placeholder:text-content-faint focus:outline-none focus:border-primary-500 resize-y"
          />
          <div className="flex gap-2">
            <button
              onClick={handleCreateNote}
              className="px-3 py-1.5 text-xs font-medium bg-primary-500 text-white rounded-lg hover:bg-primary-600 transition-colors"
            >
              Create
            </button>
            <button
              onClick={() => setShowCreateDialog(false)}
              className="px-3 py-1.5 text-xs font-medium text-content-muted hover:text-content transition-colors"
            >
              Cancel
            </button>
          </div>
        </div>
      )}

      <div className="grid grid-cols-12 gap-4">
        {/* Left panel: folder tree */}
        <div className="col-span-3">
          <div className="card p-4 space-y-1">
            <div className="text-[11px] font-medium text-content-faint uppercase tracking-widest mb-3">Folders</div>
            <button
              onClick={handleShowAll}
              className={`w-full text-left px-2 py-1.5 rounded text-xs transition-colors ${
                selectedFolder === null
                  ? 'bg-primary-50 text-primary-500 font-medium'
                  : 'text-content-muted hover:text-content hover:bg-surface-hover'
              }`}
            >
              All Notes
            </button>
            {folders.map((folder) => (
              <button
                key={folder}
                onClick={() => handleFolderClick(folder)}
                className={`w-full text-left px-2 py-1.5 rounded text-xs transition-colors ${
                  selectedFolder === folder
                    ? 'bg-primary-50 text-primary-500 font-medium'
                    : 'text-content-muted hover:text-content hover:bg-surface-hover'
                }`}
              >
                {folder}
              </button>
            ))}
            {folders.length === 0 && !isLoading && (
              <div className="text-xs text-content-faint px-2 py-1.5">No folders</div>
            )}
          </div>
        </div>

        {/* Right panel: note list or content */}
        <div className="col-span-9">
          {currentNote ? (
            <div className="card p-5 space-y-3">
              <div className="flex items-center justify-between">
                <div className="text-xs font-mono text-content-muted">{currentNote.path}</div>
                <button
                  onClick={clearCurrentNote}
                  className="text-xs text-content-faint hover:text-content transition-colors"
                >
                  Back to list
                </button>
              </div>
              <div className="prose prose-sm prose-invert max-w-none text-content-secondary">
                <ReactMarkdown>{currentNote.content}</ReactMarkdown>
              </div>
            </div>
          ) : isLoading ? (
            <LoadingState message="Loading notes..." />
          ) : (
            <div className="card p-4">
              <div className="space-y-0.5">
                {notes.map((note) => (
                  <button
                    key={note}
                    onClick={() => handleNoteClick(note)}
                    className="w-full text-left px-3 py-2 rounded-lg text-xs text-content-secondary hover:bg-surface-hover hover:text-content transition-colors font-mono"
                  >
                    {note}
                  </button>
                ))}
                {notes.length === 0 && (
                  <div className="text-xs text-content-faint px-3 py-4 text-center">
                    No notes found
                  </div>
                )}
              </div>
            </div>
          )}
        </div>
      </div>
    </div>
  )
}

function SearchTab() {
  const { searchResults, searchQuery, isLoading, error, searchNotes, fetchNote, currentNote, clearCurrentNote } =
    useKnowledgeStore()
  const [query, setQuery] = useState(searchQuery)

  const handleSearch = () => {
    if (query.trim()) {
      searchNotes(query.trim())
      clearCurrentNote()
    }
  }

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter') handleSearch()
  }

  return (
    <div className="space-y-4">
      {error && <ErrorBanner message={error} />}

      <div className="flex gap-2">
        <input
          type="text"
          placeholder="Search notes..."
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          onKeyDown={handleKeyDown}
          className="flex-1 px-3 py-2 text-sm bg-surface border border-line rounded-lg text-content placeholder:text-content-faint focus:outline-none focus:border-primary-500"
        />
        <button
          onClick={handleSearch}
          disabled={isLoading}
          className="px-4 py-2 text-xs font-medium bg-primary-500 text-white rounded-lg hover:bg-primary-600 transition-colors disabled:opacity-50"
        >
          Search
        </button>
      </div>

      {isLoading && <LoadingState message="Searching..." />}

      {currentNote ? (
        <div className="card p-5 space-y-3">
          <div className="flex items-center justify-between">
            <div className="text-xs font-mono text-content-muted">{currentNote.path}</div>
            <button
              onClick={clearCurrentNote}
              className="text-xs text-content-faint hover:text-content transition-colors"
            >
              Back to results
            </button>
          </div>
          <div className="prose prose-sm prose-invert max-w-none text-content-secondary">
            <ReactMarkdown>{currentNote.content}</ReactMarkdown>
          </div>
        </div>
      ) : (
        !isLoading && searchResults.length > 0 && (
          <div className="card p-4">
            <div className="text-[11px] font-medium text-content-faint uppercase tracking-widest mb-3">
              {searchResults.length} result{searchResults.length !== 1 ? 's' : ''}
            </div>
            <div className="space-y-0.5">
              {searchResults.map((result) => (
                <button
                  key={result}
                  onClick={() => fetchNote(result)}
                  className="w-full text-left px-3 py-2 rounded-lg text-xs text-content-secondary hover:bg-surface-hover hover:text-content transition-colors font-mono"
                >
                  {result}
                </button>
              ))}
            </div>
          </div>
        )
      )}

      {!isLoading && searchQuery && searchResults.length === 0 && (
        <div className="text-xs text-content-faint text-center py-8">
          No results for &quot;{searchQuery}&quot;
        </div>
      )}
    </div>
  )
}

function HealthTab() {
  const { stats, health, orphans, error, fetchStats, fetchHealth, fetchOrphans } =
    useKnowledgeStore()

  useEffect(() => {
    fetchStats()
    fetchHealth()
    fetchOrphans()
  }, [fetchStats, fetchHealth, fetchOrphans])

  const modeColor = (mode: string) => {
    switch (mode) {
      case 'full':
        return 'text-emerald-500'
      case 'fallback':
        return 'text-amber-500'
      case 'degraded':
        return 'text-rose-500'
      default:
        return 'text-content-faint'
    }
  }

  return (
    <div className="space-y-4">
      {error && <ErrorBanner message={error} />}

      {/* Vault Mode */}
      {health && (
        <div className="card p-5">
          <div className="text-[11px] font-medium text-content-faint uppercase tracking-widest mb-3">Vault Status</div>
          <div className="flex items-center gap-3">
            <div className={`w-2.5 h-2.5 rounded-full ${health.healthy ? 'bg-emerald-400' : 'bg-rose-400'}`} />
            <span className="text-sm text-content">
              Mode: <span className={`font-medium capitalize ${modeColor(health.mode)}`}>{health.mode}</span>
            </span>
          </div>
        </div>
      )}

      {/* Stats */}
      {stats && (
        <div className="card p-5">
          <div className="text-[11px] font-medium text-content-faint uppercase tracking-widest mb-3">Statistics</div>
          <div className="grid grid-cols-3 gap-4">
            <div>
              <div className="text-2xl font-semibold text-content">{stats.note_count}</div>
              <div className="text-xs text-content-faint">Notes</div>
            </div>
            <div>
              <div className="text-2xl font-semibold text-content">{stats.folder_count}</div>
              <div className="text-xs text-content-faint">Folders</div>
            </div>
            <div>
              <div className="text-2xl font-semibold text-content">{formatBytes(stats.total_size)}</div>
              <div className="text-xs text-content-faint">Total Size</div>
            </div>
          </div>
        </div>
      )}

      {/* Orphan Notes */}
      <div className="card p-5">
        <div className="text-[11px] font-medium text-content-faint uppercase tracking-widest mb-3">
          Orphan Notes ({orphans.length})
        </div>
        {orphans.length === 0 ? (
          <div className="text-xs text-content-faint">No orphan notes found</div>
        ) : (
          <div className="space-y-0.5 max-h-64 overflow-y-auto">
            {orphans.map((orphan) => (
              <div key={orphan} className="px-2 py-1.5 text-xs font-mono text-content-muted">
                {orphan}
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  )
}

function formatBytes(bytes: number): string {
  if (bytes === 0) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return `${parseFloat((bytes / Math.pow(k, i)).toFixed(1))} ${sizes[i]}`
}
