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
	config     *config.Config
	tokenStore *GitTokenStore
	stopCh     chan struct{}
	mu         sync.Mutex // Use regular mutex for simplicity
	running    bool
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

	// Check if read-only mode is enabled
	if !s.config.Storage.ReadOnly {
		log.Info("Git scheduler: read-only mode is disabled, not starting scheduler")
		s.mu.Unlock()
		return nil
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
		if cfg != nil && cfg.Storage.ReadOnly {
			// Calculate sync interval - default to 1 hour if not set or invalid
			syncInterval := time.Duration(cfg.Storage.SyncIntervalMinutes) * time.Minute
			if syncInterval <= 0 {
				syncInterval = 60 * time.Minute // Default to 1 hour
			}

			// Create a timer for the sync interval
			timer := time.NewTimer(syncInterval)
			
			// Wait for either the timer to complete, context cancellation (stop), or stop signal
			select {
			case <-timer.C:
				if err := s.sync(); err != nil {
					log.WithError(err).Error("Git scheduler sync failed")
				}
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

	if s.config.Storage.ReadOnly {
		// Ensure repository is initialized
		if err := s.tokenStore.EnsureRepository(); err != nil {
			return fmt.Errorf("failed to ensure repository: %w", err)
		}

		// Pull changes from remote
		if err := s.pullChanges(); err != nil {
			return fmt.Errorf("failed to pull changes: %w", err)
		}

		log.Info("Git scheduler: sync completed successfully")
	} else {
		log.Info("Git scheduler: read-only mode disabled, skipping sync")
	}

	return nil
}

// pullChanges pulls the latest changes from the remote repository.
// It uses the GitTokenStore's repository information and authentication.
func (s *GitScheduler) pullChanges() error {
	repoDir := s.tokenStore.repoDirSnapshot()
	if repoDir == "" {
		return fmt.Errorf("repository directory not configured")
	}

	// Open the repository
	repo, err := git.PlainOpen(repoDir)
	if err != nil {
		return fmt.Errorf("failed to open repository: %w", err)
	}

	// Get the worktree
	worktree, err := repo.Worktree()
	if err != nil {
		return fmt.Errorf("failed to get worktree: %w", err)
	}

	// Prepare authentication
	authMethod := s.tokenStore.gitAuth()

	// Pull changes - avoid Force: true to prevent data loss
	err = worktree.Pull(&git.PullOptions{
		Auth:       authMethod,
		RemoteName: "origin",
		// Removed Force: true to prevent overwriting local changes
	})

	if err != nil && err != git.NoErrAlreadyUpToDate {
		return fmt.Errorf("failed to pull changes: %w", err)
	}

	if err == git.NoErrAlreadyUpToDate {
		log.Info("Git scheduler: repository is already up to date")
	} else {
		log.Info("Git scheduler: successfully pulled changes from remote")
	}

	return nil
}

// UpdateConfig updates the scheduler's configuration.
// This can be called when the configuration changes to adjust behavior.
func (s *GitScheduler) UpdateConfig(cfg *config.Config) error {
	if cfg == nil {
		return fmt.Errorf("configuration is nil")
	}

	s.mu.Lock()
	
	newReadOnly := cfg.Storage.ReadOnly
	
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
