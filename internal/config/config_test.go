package config

import (
	"os"
	"testing"
)

func TestEnvVarDefaults(t *testing.T) {
	// Clear environment variables to test defaults
	os.Unsetenv("READ_ONLY")
	os.Unsetenv("SYNC_INTERVAL_MINUTES")

	cfg, err := LoadConfigOptional("", true)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if cfg.IsReadOnlyStorage() != true {
		t.Errorf("Expected default READ_ONLY to be true, got %v", cfg.IsReadOnlyStorage())
	}

	if cfg.SyncIntervalMinutes() != 5 {
		t.Errorf("Expected default SYNC_INTERVAL_MINUTES to be 5, got %v", cfg.SyncIntervalMinutes())
	}
}

func TestEnvVarReadOnlyTrue(t *testing.T) {
	os.Setenv("READ_ONLY", "true")
	os.Unsetenv("SYNC_INTERVAL_MINUTES")
	defer os.Unsetenv("READ_ONLY")
	
	cfg, err := LoadConfigOptional("", true)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}
	
	if cfg.IsReadOnlyStorage() != true {
		t.Errorf("Expected READ_ONLY to be true, got %v", cfg.IsReadOnlyStorage())
	}
	
	if cfg.SyncIntervalMinutes() != 5 {
		t.Errorf("Expected default SYNC_INTERVAL_MINUTES to be 5, got %v", cfg.SyncIntervalMinutes())
	}
}

func TestEnvVarReadOnlyFalse(t *testing.T) {
	os.Setenv("READ_ONLY", "false")
	os.Unsetenv("SYNC_INTERVAL_MINUTES")
	defer os.Unsetenv("READ_ONLY")
	
	cfg, err := LoadConfigOptional("", true)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}
	
	if cfg.IsReadOnlyStorage() != false {
		t.Errorf("Expected READ_ONLY to be false, got %v", cfg.IsReadOnlyStorage())
	}
	
	if cfg.SyncIntervalMinutes() != 5 {
		t.Errorf("Expected default SYNC_INTERVAL_MINUTES to be 5, got %v", cfg.SyncIntervalMinutes())
	}
}

func TestEnvVarSyncInterval(t *testing.T) {
	os.Unsetenv("READ_ONLY")
	os.Setenv("SYNC_INTERVAL_MINUTES", "10")
	defer os.Unsetenv("SYNC_INTERVAL_MINUTES")

	cfg, err := LoadConfigOptional("", true)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if cfg.IsReadOnlyStorage() != true {
		t.Errorf("Expected default READ_ONLY to be true, got %v", cfg.IsReadOnlyStorage())
	}

	if cfg.SyncIntervalMinutes() != 10 {
		t.Errorf("Expected SYNC_INTERVAL_MINUTES to be 10, got %v", cfg.SyncIntervalMinutes())
	}
}

func TestEnvVarBoth(t *testing.T) {
	os.Setenv("READ_ONLY", "true")
	os.Setenv("SYNC_INTERVAL_MINUTES", "15")
	defer os.Unsetenv("READ_ONLY")
	defer os.Unsetenv("SYNC_INTERVAL_MINUTES")
	
	cfg, err := LoadConfigOptional("", true)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}
	
	if cfg.IsReadOnlyStorage() != true {
		t.Errorf("Expected READ_ONLY to be true, got %v", cfg.IsReadOnlyStorage())
	}
	
	if cfg.SyncIntervalMinutes() != 15 {
		t.Errorf("Expected SYNC_INTERVAL_MINUTES to be 15, got %v", cfg.SyncIntervalMinutes())
	}
}

func TestEnvVarInvalidSyncInterval(t *testing.T) {
	os.Unsetenv("READ_ONLY")
	os.Setenv("SYNC_INTERVAL_MINUTES", "invalid")
	defer os.Unsetenv("SYNC_INTERVAL_MINUTES")

	cfg, err := LoadConfigOptional("", true)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if cfg.IsReadOnlyStorage() != true {
		t.Errorf("Expected default READ_ONLY to be true, got %v", cfg.IsReadOnlyStorage())
	}

	if cfg.SyncIntervalMinutes() != 5 {
		t.Errorf("Expected default SYNC_INTERVAL_MINUTES to be 5 (fallback from invalid value), got %v", cfg.SyncIntervalMinutes())
	}
}

func TestEnvVarSyncIntervalZero(t *testing.T) {
	os.Unsetenv("READ_ONLY")
	os.Setenv("SYNC_INTERVAL_MINUTES", "0")
	defer os.Unsetenv("SYNC_INTERVAL_MINUTES")

	cfg, err := LoadConfigOptional("", true)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if cfg.IsReadOnlyStorage() != true {
		t.Errorf("Expected default READ_ONLY to be true, got %v", cfg.IsReadOnlyStorage())
	}

	if cfg.SyncIntervalMinutes() != 5 {
		t.Errorf("Expected default SYNC_INTERVAL_MINUTES to be 5 (fallback from zero value), got %v", cfg.SyncIntervalMinutes())
	}
}