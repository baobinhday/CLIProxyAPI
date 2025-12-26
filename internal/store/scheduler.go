package store

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/go-git/go-git/v6"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/config"
	log "github.com/sirupsen/logrus"
)

// GitScheduler manages periodic synchronization of the Git repository
// when the storage is in read-only mode.
type GitScheduler struct {
	config         *config.Config
	tokenStore     *GitTokenStore
	stopCh         chan struct{}
	mu             sync.Mutex // Use regular mutex for simplicity
	running        bool
	onSyncComplete func() // Callback invoked after successful sync to reload auth tokens
}

// NewGitScheduler creates a new GitScheduler instance.
func NewGitScheduler(cfg *config.Config, store *GitTokenStore) *GitScheduler {
	return &GitScheduler{
		config:     cfg,
		tokenStore: store,
		stopCh:     make(chan struct{}),
		running:    false,
	}
}

// SetOnSyncComplete sets a callback that is invoked after successful sync.
// This is typically used to reload auth tokens from disk after git pull.
func (s *GitScheduler) SetOnSyncComplete(callback func()) {
	s.mu.Lock()
	s.onSyncComplete = callback
	s.mu.Unlock()
}

// Start begins the synchronization scheduler based on the configuration.
// It will periodically pull changes from the remote repository if read-only mode is enabled.
func (s *GitScheduler) Start() error {
	s.mu.Lock()

	// Check if scheduler is already running
	if s.running {
		// Stop the existing scheduler first
		close(s.stopCh)
		s.running = false
		// Reinitialize the stop channel
		s.stopCh = make(chan struct{})
	}

	if s.config == nil {
		s.mu.Unlock()
		return fmt.Errorf("configuration is nil")
	}

	if s.tokenStore == nil {
		s.mu.Unlock()
		return fmt.Errorf("token store is nil")
	}

	// Set running state
	s.running = true
	s.mu.Unlock()

	log.Info("Starting Git scheduler")

	// Start the scheduler goroutine
	go s.run(s.stopCh)

	return nil
}

// Stop stops the synchronization scheduler.
func (s *GitScheduler) Stop() {
	s.mu.Lock()

	if !s.running {
		s.mu.Unlock()
		return
	}

	// Close the stop channel to signal goroutine to exit
	close(s.stopCh)
	s.running = false

	s.mu.Unlock()

	log.Info("Git scheduler stopped")
}

// run is the main loop for the scheduler.
func (s *GitScheduler) run(stopCh <-chan struct{}) {
	log.Info("Git scheduler started")

	// Create a cancellable context that is cancelled when stopCh receives a value
	ctx, cancel := context.WithCancel(context.Background())

	// Goroutine to cancel context when stopCh receives a value
	go func() {
		<-stopCh
		cancel()
	}()

	for {
		// Read the current config safely
		s.mu.Lock()
		cfg := s.config
		running := s.running
		s.mu.Unlock()

		// If not running anymore, exit
		if !running {
			return
		}

		// Check if read-only mode is enabled
		if cfg != nil && cfg.IsReadOnlyStorage() {
			// Perform sync at the start of each iteration
			// This ensures immediate sync when starting and after each interval
			if err := s.sync(); err != nil {
				log.WithError(err).Error("Git scheduler sync failed")
			}

			// Calculate sync interval - default to 1 hour if not set or invalid
			syncInterval := time.Duration(cfg.SyncIntervalMinutes()) * time.Minute
			if syncInterval <= 0 {
				syncInterval = 60 * time.Minute // Default to 1 hour
			}

			// Create a timer for the sync interval
			timer := time.NewTimer(syncInterval)

			// Wait for either the timer to complete or stop signal
			select {
			case <-timer.C:
				// Timer completed, loop will continue and sync again
			case <-ctx.Done():
				// Context cancelled (stop signal received), clean up and exit
				if !timer.Stop() {
					// If timer already fired, drain the channel
					<-timer.C
				}
				log.Info("Git scheduler stopped")
				return
			}
		} else {
			// Read-only mode is disabled, wait for stop signal or config change
			// For simplicity, we'll just wait for stop signal
			select {
			case <-ctx.Done():
				log.Info("Git scheduler stopped")
				return
			case <-time.After(1 * time.Second): // Check config again after 1 second
				// Continue loop to re-read config
			}
		}
	}
}

// sync performs a single synchronization by pulling changes from the remote repository.
func (s *GitScheduler) sync() error {
	log.Info("Git scheduler: starting sync operation")

	if s.config.IsReadOnlyStorage() {
		// Ensure repository is initialized
		if err := s.tokenStore.EnsureRepository(); err != nil {
			return fmt.Errorf("failed to ensure repository: %w", err)
		}

		// Pull changes from remote
		if err := s.pullChanges(); err != nil {
			return fmt.Errorf("failed to pull changes: %w", err)
		}

		// Invoke the callback to reload auth tokens from disk
		s.mu.Lock()
		callback := s.onSyncComplete
		s.mu.Unlock()
		if callback != nil {
			log.Info("Git scheduler: invoking sync complete callback to reload auth tokens")
			callback()
		}

		log.Info("Git scheduler: sync completed successfully")
	} else {
		log.Info("Git scheduler: read-only mode disabled, skipping sync")
	}

	return nil
}

// pullChanges pulls the latest changes from the remote repository.
// It uses the GitTokenStore's repository information and authentication.
// In read-only mode, this uses fetch + hard reset to ensure local files
// exactly match the remote, discarding any local changes.
func (s *GitScheduler) pullChanges() error {
	repoDir := s.tokenStore.repoDirSnapshot()
	if repoDir == "" {
		return fmt.Errorf("repository directory not configured")
	}

	log.Infof("Git scheduler: syncing from remote to %s", repoDir)

	// Open the repository
	repo, err := git.PlainOpen(repoDir)
	if err != nil {
		return fmt.Errorf("failed to open repository: %w", err)
	}

	// Prepare authentication
	authMethod := s.tokenStore.gitAuth()

	// First, fetch the latest changes from remote
	log.Info("Git scheduler: fetching from remote...")
	err = repo.Fetch(&git.FetchOptions{
		Auth:       authMethod,
		RemoteName: "origin",
		Force:      true,
	})
	if err != nil && err != git.NoErrAlreadyUpToDate {
		return fmt.Errorf("failed to fetch changes: %w", err)
	}

	// Get the remote HEAD reference
	remoteRef, err := repo.Reference("refs/remotes/origin/HEAD", true)
	if err != nil {
		// Try to get origin/main or origin/master if origin/HEAD doesn't exist
		remoteRef, err = repo.Reference("refs/remotes/origin/main", true)
		if err != nil {
			remoteRef, err = repo.Reference("refs/remotes/origin/master", true)
			if err != nil {
				return fmt.Errorf("failed to find remote branch reference: %w", err)
			}
		}
	}

	log.Infof("Git scheduler: resetting to remote commit %s", remoteRef.Hash().String()[:8])

	// Get the worktree
	worktree, err := repo.Worktree()
	if err != nil {
		return fmt.Errorf("failed to get worktree: %w", err)
	}

	// Hard reset to the remote HEAD - this discards all local changes
	// and makes local files exactly match the remote
	err = worktree.Reset(&git.ResetOptions{
		Commit: remoteRef.Hash(),
		Mode:   git.HardReset,
	})
	if err != nil {
		return fmt.Errorf("failed to reset to remote: %w", err)
	}

	log.Info("Git scheduler: successfully synced with remote (hard reset)")
	return nil
}

// UpdateConfig updates the scheduler's configuration.
// This can be called when the configuration changes to adjust behavior.
func (s *GitScheduler) UpdateConfig(cfg *config.Config) error {
	if cfg == nil {
		return fmt.Errorf("configuration is nil")
	}

	s.mu.Lock()

	newReadOnly := cfg.IsReadOnlyStorage()

	// Update the config while holding the lock
	s.config = cfg

	// Determine if scheduler should be running based on new config
	shouldRun := newReadOnly
	isRunning := s.running

	s.mu.Unlock()

	// Start or stop based on desired state vs current state
	if shouldRun && !isRunning {
		// Need to start scheduler
		return s.Start()
	} else if !shouldRun && isRunning {
		// Need to stop scheduler
		s.Stop()
	}

	return nil
}

// HasPendingLocalChanges checks if there are any uncommitted changes in the git repository
func (s *GitScheduler) HasPendingLocalChanges() (bool, error) {
	if s.tokenStore == nil {
		return false, fmt.Errorf("token store is nil")
	}
	return s.tokenStore.HasPendingLocalChanges()
}

// CheckForPendingGitChanges checks for pending local Git changes (uncommitted or unpushed).
// This function is designed to be called from outside the store package, e.g. during application startup.
func CheckForPendingGitChanges(scheduler *GitScheduler) (bool, error) {
	if scheduler == nil {
		return false, fmt.Errorf("git scheduler is nil")
	}
	return scheduler.HasPendingLocalChanges()
}
