package config

// ReadOnlyStorageConfig represents the structure of the standalone read-only storage configuration file.
type ReadOnlyStorageConfig struct {
	ReadOnly            bool `json:"read_only"`
	SyncIntervalMinutes int  `json:"sync_interval_minutes,omitempty"`
}