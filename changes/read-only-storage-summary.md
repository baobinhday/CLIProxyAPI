# Read-Only Storage Feature: Plan, Analysis, and Design

This document consolidates the planning, analysis, and design specifications for the read-only storage feature.

## 1. Initial Feature Plan

# Plan for Read-Only Storage Feature

This document outlines the steps to implement a read-only storage mode for the CLIProxyAPI.

Created: 2025-12-02 09:10:00
Last Modified: 2025-12-02 09:22:32

## 1. Extend Configuration

Add a `Storage` section to the main configuration struct in `internal/config/config.go` to include `ReadOnly` (boolean) and `SyncIntervalMinutes` (integer) options.

## 2. Implement Read-Only Logic

In the `internal/store/gitstore.go` file, modify the `Save`, `Delete`, and `PersistConfig` functions to prevent pushing changes to the remote repository if `ReadOnly` mode is enabled.

## 3. Create a Sync Scheduler

Develop a background scheduler that, when `ReadOnly` mode is active, periodically pulls changes from the remote Git repository, overriding local data. The sync interval will be determined by `SyncIntervalMinutes`. The implementation in `internal/managementasset/updater.go` will be used as a reference.

## 4. Add API Endpoint

Introduce a new API endpoint, likely `PUT /v0/management/storage/readonly`, in `internal/api/server.go` to control the read-only feature.

## 5. Create API Handler

Implement the corresponding handler in `internal/api/handlers/management/handler.go`. This handler will manage the state of the `ReadOnly` flag and the sync interval, and it will also be responsible for starting or stopping the sync scheduler as needed.

## 2. Analysis of Implementation

# Analysis of Read-Only Storage Feature

**Created:** 2025-12-02
**Author:** Kilo Code

## 1. Executive Summary

This document provides a detailed analysis of the "Read-Only Storage" feature, which was designed to prevent write operations and enable periodic synchronization from a remote Git repository.

The feature is **partially implemented but non-functional** in its current state due to a critical architectural flaw. While the individual components—configuration, read-only logic, and the sync scheduler—are mostly in place, they are not correctly integrated within the main application server. This prevents the management API from controlling the scheduler at runtime, rendering the feature's dynamic capabilities inoperative.

This report outlines the specific findings and provides clear, actionable recommendations to fix the issues and complete the feature.

## 2. Key Findings

My analysis, which followed the original plan laid out in `read-only-storage-feature-plan.md`, revealed the following:

### 2.1. Critical Flaw: Disconnected Scheduler

The most significant issue is that the `GitScheduler` is never instantiated and connected to the management API handler.

- **Location:** `internal/api/server.go`
- **Impact:** The management handler (`internal/api/handlers/management/handler.go`) holds a `nil` pointer for the scheduler. As a result, API calls to `/v0/management/storage/readonly` and `/v0/management/storage/sync-interval` can modify the configuration file on disk but **cannot** start, stop, or reconfigure the scheduler at runtime. The feature can be enabled at startup via the config file, but it cannot be controlled dynamically as intended.

### 2.2. Architectural Inconsistency: Conflicting Pull Strategies

There are two different, conflicting strategies for pulling changes from the remote Git repository.

- **Scheduler:** The `GitScheduler` in `internal/store/scheduler.go` uses a **forceful pull** (`git pull --force`), which overwrites any local changes. This is the correct approach for a read-only mode where the remote is the source of truth.
- **Git Store:** The `EnsureRepository` function in `internal/store/gitstore.go` uses a **passive pull** that attempts to preserve local changes and ignores many common errors, such as unstaged changes or non-fast-forward updates.

This inconsistency creates ambiguity and could lead to unpredictable behavior depending on which part of the code triggers a pull.

### 2.3. Other Issues and Potential Improvements

- **Missing Default Configuration:** The `SyncIntervalMinutes` field in `internal/config/config.go` has a default value mentioned in a comment but does not set it programmatically. Although the scheduler defaults to 60 minutes, relying on the scheduler for this default is not robust.
- **Scheduler Doesn't Update Interval:** The `GitScheduler` does not dynamically adjust its sync interval if the value is changed while it is running. It requires a full restart to pick up the new timing.
- **Redundant Code:** The `commitAndPushLocked` function in `internal/store/gitstore.go` contains a redundant read-only check, as all its callers already perform the same check.
- **Inconsistent Handler Logic:** The management handlers for storage settings do not use the shared `persist` helper method, unlike other handlers in the same file.
- **Inconsistent Naming:** JSON tags in `internal/config/config.go` inconsistently use `snake_case` for the new storage fields, while other fields use `kebab-case`.

## 3. Standalone Configuration Design

# Standalone Read-Only Storage Configuration Design

This document outlines the design for moving the read-only storage configuration into a standalone file, allowing for dynamic updates without restarting the application.

## 1. File Location and Format

-   **Path:** `data/read_only_storage.json`
-   **Format:** JSON

The configuration will be stored in a simple JSON file. A new `data/` directory will be created at the root of the project to hold this and potentially other dynamic configuration files in the future.

Using JSON is standard, easy to read, and easy to parse in Go.

## 2. File Content

The `data/read_only_storage.json` file will have a structure containing both the read-only flag and the sync interval.

**Example:**

```json
{
    "read_only": true,
    "sync_interval_minutes": 30
}
```

A corresponding Go struct will be used for unmarshaling:

```go
// In a new file, e.g., internal/config/readonly.go
package config

type ReadOnlyStorageConfig struct {
    ReadOnly            bool `json:"read_only"`
    SyncIntervalMinutes int  `json:"sync_interval_minutes,omitempty"`
}
```

## 3. Loading Mechanism

The application will load this configuration at startup. The loading logic will be designed to be resilient to the file's absence.

**Startup Logic:**

1.  The application will attempt to read and parse `data/read_only_storage.json`.
2.  **If the file does not exist or is invalid JSON:** The application will default to `read_only: false` and log a warning. It will not prevent the application from starting.
3.  **If the file exists and is valid:** The `read_only` value will be read from the file.

This logic will likely be added to the existing configuration loading process in the `internal/config` package. The loaded value will be stored in the main application `Config` struct.

## 4. Accessing the Configuration

To make the setting easily accessible throughout the application, it will be integrated into the global `Config` struct. This maintains consistency with how other configuration values are accessed.

**Proposed Change in `internal/config/config.go`:**

The `Config` struct will use atomic types for thread-safe access to the read-only storage and sync interval settings.

```go
import "sync/atomic"

type Config struct {
    // ... other existing fields

    // Read-only storage settings
    readOnlyStorage     atomic.Bool
    syncIntervalMinutes atomic.Int64
}

// Getter for read-only storage setting
func (c *Config) IsReadOnlyStorage() bool {
    return c.readOnlyStorage.Load()
}

// Setter for read-only storage setting
func (c *Config) SetReadOnlyStorage(value bool) {
    c.readOnlyStorage.Store(value)
}

// Getter for sync interval setting
func (c *Config) SyncIntervalMinutes() int {
    return int(c.syncIntervalMinutes.Load())
}

// Setter for sync interval setting
func (c *Config) SetSyncIntervalMinutes(value int) {
    c.syncIntervalMinutes.Store(int64(value))
}
```

By using a getter method (`IsReadOnlyStorage`), we encapsulate the locking mechanism and provide a safe way for other parts of the application to check the setting.

## 5. Dynamic Reloading

To allow runtime changes without a server restart, a file watcher will be implemented.

**Mechanism:**

1.  **File Watcher:** The application will use a library like `fsnotify` to monitor `data/read_only_storage.json` for changes.
2.  **Goroutine:** A dedicated goroutine will be launched at startup to handle file events.
3.  **Event Handling:** When a `Write` event is detected on the file, the goroutine will:
    a.  Re-read and parse the `data/read_only_storage.json` file.
    b.  If successful, update the `readOnlyStorage` and `syncIntervalMinutes` values in the global `Config` object using the thread-safe `SetReadOnlyStorage` and `SetSyncIntervalMinutes` methods.
    c.  If the file is deleted or becomes invalid, the system could either revert to the default values (`read_only: false`, `sync_interval_minutes: 30`) or maintain the last known valid state. Reverting to default values is the safer option.
    d.  Log the change in the configuration.

This approach makes the read-only mode truly dynamic and manageable at runtime.

## Mermaid Diagram: Dynamic Reloading Flow

```mermaid
sequenceDiagram
    participant App as Application
    participant Watcher as File Watcher
    participant File as data/read_only_storage.json
    participant Config as Global Config

    App->>+Watcher: Start watching File
    loop On file change
        Watcher-->>App: File modified event
        App->>+File: Read content
        File-->>-App: {"read_only": true}
        App->>Config: SetReadOnlyStorage(true)
    end
## 4. Final Implemented API

This section summarizes the final API endpoints implemented for managing the read-only storage feature.

*   **GET /v0/management/storage/readonly**
    *   **Description:** Retrieves the current read-only status of the storage.

*   **PUT /v0/management/storage/readonly**
    *   **Description:** Updates the read-only status of the storage. Requires a boolean `value` in the request body. If enabling read-only mode, it checks for pending local changes and prevents the operation if changes exist.

*   **PATCH /v0/management/storage/readonly**
    *   **Description:** An alias for the `PUT` method to update the read-only status.

*   **GET /v0/management/storage/sync-interval**
    *   **Description:** Retrieves the current synchronization interval for the storage in minutes.

*   **PUT /v0/management/storage/sync-interval**
    *   **Description:** Updates the synchronization interval for the storage in minutes. Requires an integer `value` in the request body, which must be at least 1.

*   **PATCH /v0/management/storage/sync-interval**
    *   **Description:** An alias for the `PUT` method to update the synchronization interval.