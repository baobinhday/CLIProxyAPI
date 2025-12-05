package config

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/fsnotify/fsnotify"
)

// ReadOnlyStorageSetter is an interface for setting read-only storage state
type ReadOnlyStorageSetter interface {
	SetReadOnlyStorage(bool)
	SetSyncIntervalMinutes(int)
}

// LoadReadOnlyStorageConfig loads the standalone read-only storage configuration from the specified file path.
// If the file doesn't exist, it defaults to false.
func LoadReadOnlyStorageConfig(setter ReadOnlyStorageSetter, path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			// File doesn't exist, default to false
			setter.SetReadOnlyStorage(false)
			return nil
		}
		return fmt.Errorf("failed to read read-only storage config file: %w", err)
	}

	var readOnlyConfig ReadOnlyStorageConfig
	if err := json.Unmarshal(data, &readOnlyConfig); err != nil {
		// If JSON is invalid, default to false
		setter.SetReadOnlyStorage(false)
		return fmt.Errorf("failed to parse read-only storage config: %w", err)
	}

	setter.SetReadOnlyStorage(readOnlyConfig.ReadOnly)

	if readOnlyConfig.SyncIntervalMinutes > 0 {
		setter.SetSyncIntervalMinutes(readOnlyConfig.SyncIntervalMinutes)
	}

	return nil
}

// StartReadOnlyStorageWatcher starts a file watcher to monitor changes to the read-only storage configuration file.
// It runs in a separate goroutine and updates the configuration when the file changes.
func StartReadOnlyStorageWatcher(ctx context.Context, setter ReadOnlyStorageSetter, path string) error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("failed to create file watcher: %w", err)
	}

	// Add the directory to watch (not the file directly to handle file replacement scenarios)
	configDir := getDirFromPath(path)
	if err := watcher.Add(configDir); err != nil {
		watcher.Close()
		return fmt.Errorf("failed to add directory to watcher: %w", err)
	}

	// Start the watching goroutine
	go func() {
		defer watcher.Close()
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return // Channel closed
				}
				
				// Check if the event is for our specific file
				if event.Name == path && (event.Op&fsnotify.Write == fsnotify.Write || event.Op&fsnotify.Create == fsnotify.Create) {
					// File was modified or created, reload the configuration
					if err := LoadReadOnlyStorageConfig(setter, path); err != nil {
						// Log the error but continue watching
						log.Printf("Warning: failed to reload read-only storage config: %v", err)
					} else {
						// Successfully reloaded, log the new value
						// We can't directly check the new value since we only have the interface,
						// but we know it was updated by the LoadReadOnlyStorageConfig call
						log.Printf("Info: read-only storage configuration reloaded from %s", path)
					}
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					return // Channel closed
				}
				log.Printf("File watcher error: %v", err)
			case <-ctx.Done():
				log.Printf("Shutting down read-only storage watcher for %s", path)
				return // Context canceled, exit the goroutine
			}
		}
	}()

	return nil
}

// getDirFromPath extracts the directory from a file path
func getDirFromPath(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' || path[i] == os.PathSeparator {
			return path[:i]
		}
	}
	return "."
}

// PersistReadOnlyState persists the read-only and sync interval state to the specified JSON file.
// This function is designed to be called from outside the config package, e.g. during application startup.
func PersistReadOnlyState(readOnly bool, syncIntervalMinutes int, path string) error {
	// Read the current data/read_only_storage.json file
	data, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("failed to read read-only storage config file: %w", err)
		}
		// If file doesn't exist, we'll create it with the current values
		data = []byte(`{"read_only": false}`)
	}

	// Unmarshal into ReadOnlyStorageConfig struct
	var readOnlyConfig ReadOnlyStorageConfig
	if err := json.Unmarshal(data, &readOnlyConfig); err != nil {
		return fmt.Errorf("failed to parse read-only storage config: %w", err)
	}

	// Update both fields
	readOnlyConfig.ReadOnly = readOnly
	readOnlyConfig.SyncIntervalMinutes = syncIntervalMinutes

	// Marshal the updated struct back into JSON
	updatedData, err := json.MarshalIndent(readOnlyConfig, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal updated read-only storage config: %w", err)
	}

	// Write the new JSON content back to the file
	if err := os.WriteFile(path, updatedData, 0644); err != nil {
		return fmt.Errorf("failed to write updated read-only storage config: %w", err)
	}

	return nil
}