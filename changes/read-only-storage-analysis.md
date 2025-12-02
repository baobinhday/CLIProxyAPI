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

