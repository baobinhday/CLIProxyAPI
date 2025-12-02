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