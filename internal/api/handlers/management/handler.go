// Package management provides the management API handlers and middleware
// for configuring the server and managing auth files.
package management

import (
	"crypto/subtle"
	"fmt"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/buildinfo"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/config"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/store"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/usage"
	sdkAuth "github.com/router-for-me/CLIProxyAPI/v6/sdk/auth"
	coreauth "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/auth"
	log "github.com/sirupsen/logrus"
	"golang.org/x/crypto/bcrypt"
)

type attemptInfo struct {
	count        int
	blockedUntil time.Time
}

// Handler aggregates config reference, persistence path and helpers.
type Handler struct {
	cfg                 *config.Config
	configFilePath      string
	mu                  sync.Mutex
	attemptsMu          sync.Mutex
	failedAttempts      map[string]*attemptInfo // keyed by client IP
	authManager         *coreauth.Manager
	usageStats          *usage.RequestStatistics
	tokenStore          coreauth.Store
	localPassword       string
	allowRemoteOverride bool
	envSecret           string
	logDir              string
	scheduler           *store.GitScheduler
}

// NewHandler creates a new management handler instance.
func NewHandler(cfg *config.Config, configFilePath string, manager *coreauth.Manager) *Handler {
	envSecret, _ := os.LookupEnv("MANAGEMENT_PASSWORD")
	envSecret = strings.TrimSpace(envSecret)

	return &Handler{
		cfg:                 cfg,
		configFilePath:      configFilePath,
		failedAttempts:      make(map[string]*attemptInfo),
		authManager:         manager,
		usageStats:          usage.GetRequestStatistics(),
		tokenStore:          sdkAuth.GetTokenStore(),
		allowRemoteOverride: envSecret != "",
		envSecret:           envSecret,
	}
}

// SetConfig updates the in-memory config reference when the server hot-reloads.
func (h *Handler) SetConfig(cfg *config.Config) { h.cfg = cfg }

// SetAuthManager updates the auth manager reference used by management endpoints.
func (h *Handler) SetAuthManager(manager *coreauth.Manager) { h.authManager = manager }

// SetUsageStatistics allows replacing the usage statistics reference.
func (h *Handler) SetUsageStatistics(stats *usage.RequestStatistics) { h.usageStats = stats }

// SetLocalPassword configures the runtime-local password accepted for localhost requests.
func (h *Handler) SetLocalPassword(password string) { h.localPassword = password }

// SetLogDirectory updates the directory where main.log should be looked up.
func (h *Handler) SetLogDirectory(dir string) {
	if dir == "" {
		return
	}
	if !filepath.IsAbs(dir) {
		if abs, err := filepath.Abs(dir); err == nil {
			dir = abs
		}
	}
	h.logDir = dir
}

// SetScheduler sets the Git scheduler for the handler.
func (h *Handler) SetScheduler(scheduler *store.GitScheduler) {
	h.scheduler = scheduler
}

// Scheduler returns the Git scheduler for the handler.
func (h *Handler) Scheduler() *store.GitScheduler {
	return h.scheduler
}

// Middleware enforces access control for management endpoints.
// All requests (local and remote) require a valid management key.
// Additionally, remote access requires allow-remote-management=true.
func (h *Handler) Middleware() gin.HandlerFunc {
	const maxFailures = 5
	const banDuration = 30 * time.Minute

	return func(c *gin.Context) {
		c.Header("X-CPA-VERSION", buildinfo.Version)
		c.Header("X-CPA-COMMIT", buildinfo.Commit)
		c.Header("X-CPA-BUILD-DATE", buildinfo.BuildDate)

		clientIP := c.ClientIP()
		localClient := clientIP == "127.0.0.1" || clientIP == "::1"
		cfg := h.cfg
		var (
			allowRemote bool
			secretHash  string
		)
		if cfg != nil {
			allowRemote = cfg.RemoteManagement.AllowRemote
			secretHash = cfg.RemoteManagement.SecretKey
		}
		if h.allowRemoteOverride {
			allowRemote = true
		}
		envSecret := h.envSecret

		fail := func() {}
		if !localClient {
			h.attemptsMu.Lock()
			ai := h.failedAttempts[clientIP]
			if ai != nil {
				if !ai.blockedUntil.IsZero() {
					if time.Now().Before(ai.blockedUntil) {
						remaining := time.Until(ai.blockedUntil).Round(time.Second)
						h.attemptsMu.Unlock()
						c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": fmt.Sprintf("IP banned due to too many failed attempts. Try again in %s", remaining)})
						return
					}
					// Ban expired, reset state
					ai.blockedUntil = time.Time{}
					ai.count = 0
				}
			}
			h.attemptsMu.Unlock()

			if !allowRemote {
				c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "remote management disabled"})
				return
			}

			fail = func() {
				h.attemptsMu.Lock()
				aip := h.failedAttempts[clientIP]
				if aip == nil {
					aip = &attemptInfo{}
					h.failedAttempts[clientIP] = aip
				}
				aip.count++
				if aip.count >= maxFailures {
					aip.blockedUntil = time.Now().Add(banDuration)
					aip.count = 0
				}
				h.attemptsMu.Unlock()
			}
		}
		if secretHash == "" && envSecret == "" {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "remote management key not set"})
			return
		}

		// Accept either Authorization: Bearer <key> or X-Management-Key
		var provided string
		if ah := c.GetHeader("Authorization"); ah != "" {
			parts := strings.SplitN(ah, " ", 2)
			if len(parts) == 2 && strings.ToLower(parts[0]) == "bearer" {
				provided = parts[1]
			} else {
				provided = ah
			}
		}
		if provided == "" {
			provided = c.GetHeader("X-Management-Key")
		}

		if provided == "" {
			if !localClient {
				fail()
			}
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing management key"})
			return
		}

		if localClient {
			if lp := h.localPassword; lp != "" {
				if subtle.ConstantTimeCompare([]byte(provided), []byte(lp)) == 1 {
					c.Next()
					return
				}
			}
		}

		if envSecret != "" && subtle.ConstantTimeCompare([]byte(provided), []byte(envSecret)) == 1 {
			if !localClient {
				h.attemptsMu.Lock()
				if ai := h.failedAttempts[clientIP]; ai != nil {
					ai.count = 0
					ai.blockedUntil = time.Time{}
				}
				h.attemptsMu.Unlock()
			}
			c.Next()
			return
		}

		if secretHash == "" || bcrypt.CompareHashAndPassword([]byte(secretHash), []byte(provided)) != nil {
			if !localClient {
				fail()
			}
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid management key"})
			return
		}

		if !localClient {
			h.attemptsMu.Lock()
			if ai := h.failedAttempts[clientIP]; ai != nil {
				ai.count = 0
				ai.blockedUntil = time.Time{}
			}
			h.attemptsMu.Unlock()
		}

		c.Next()
	}
}

// persist saves the current in-memory config to disk.
func (h *Handler) persist(c *gin.Context) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	// Preserve comments when writing
	if err := config.SaveConfigPreserveComments(h.configFilePath, h.cfg); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("failed to save config: %v", err)})
		return false
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
	return true
}

// Helper methods for simple types
func (h *Handler) updateBoolField(c *gin.Context, set func(bool)) {
	var body struct {
		Value *bool `json:"value"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || body.Value == nil {
		var m map[string]any
		if err2 := c.ShouldBindJSON(&m); err2 == nil {
			for _, v := range m {
				if b, ok := v.(bool); ok {
					set(b)
					h.persist(c)
					return
				}
			}
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return
	}
	set(*body.Value)
	h.persist(c)
}

func (h *Handler) updateIntField(c *gin.Context, set func(int)) {
	var body struct {
		Value *int `json:"value"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || body.Value == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return
	}
	set(*body.Value)
	h.persist(c)
}

func (h *Handler) updateStringField(c *gin.Context, set func(string)) {
	var body struct {
		Value *string `json:"value"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || body.Value == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return
	}
	set(*body.Value)
	h.persist(c)
}

// GetStorageReadOnly returns the current read-only status of the storage.
func (h *Handler) GetStorageReadOnly(c *gin.Context) {
	if h.cfg == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "configuration not available"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"read_only": h.cfg.Storage.ReadOnly,
	})
}

// PutStorageReadOnly updates the read-only status of the storage.
func (h *Handler) PutStorageReadOnly(c *gin.Context) {
	// Parse the boolean field from the request body
	value, found := h.parseBoolField(c, "read_only", "value")
	if !found {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return
	}

	if value {
		// Check if we can enable read-only mode
		if !h.ensureCanEnableReadOnly(c) {
			return
		}
	}

	h.updateStorageReadOnly(value)
	c.JSON(http.StatusOK, gin.H{"read_only": value})
}

// GetStorageSyncInterval returns the current sync interval in minutes.
func (h *Handler) GetStorageSyncInterval(c *gin.Context) {
	if h.cfg == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "configuration not available"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"sync_interval_minutes": h.cfg.Storage.SyncIntervalMinutes,
	})
}

const minSyncIntervalMinutes = 1

// PutStorageSyncInterval updates the sync interval in minutes.
func (h *Handler) PutStorageSyncInterval(c *gin.Context) {
	// Parse the integer field from the request body
	value, found := h.parseIntField(c, "sync_interval_minutes", "value", minSyncIntervalMinutes)
	if !found {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return
	}

	h.updateStorageSyncInterval(value)
	c.JSON(http.StatusOK, gin.H{"sync_interval_minutes": value})
}

// updateStorageReadOnly updates the storage read-only flag and manages the scheduler.
func (h *Handler) updateStorageReadOnly(readOnly bool) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.cfg != nil {
		oldReadOnly := h.cfg.Storage.ReadOnly
		h.cfg.Storage.ReadOnly = readOnly

		// If the scheduler exists, update it with the new configuration
		if h.scheduler != nil {
			_ = h.scheduler.UpdateConfig(h.cfg)
		}

		// Log the change
		if oldReadOnly != readOnly {
			if readOnly {
				log.Info("Storage read-only mode enabled")
			} else {
				log.Info("Storage read-only mode disabled")
			}
		}
	}

	h.persistConfig() // Use the shared persist method to save config and handle response
}

// updateStorageSyncInterval updates the sync interval and manages the scheduler.
func (h *Handler) updateStorageSyncInterval(syncIntervalMinutes int) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.cfg != nil {
		h.cfg.Storage.SyncIntervalMinutes = syncIntervalMinutes

		// If the scheduler exists, update it with the new configuration
		if h.scheduler != nil {
			_ = h.scheduler.UpdateConfig(h.cfg)
		}

		// Log the change
		log.Infof("Storage sync interval updated to %d minutes", syncIntervalMinutes)
	}

	h.persistConfig() // Use the shared persist method to save config and handle response
}

// parseBoolField extracts a boolean field from the request body with primary and alternative keys.
func (h *Handler) parseBoolField(c *gin.Context, primaryKey, altKey string) (bool, bool) {
	// Bind to map once to avoid double binding
	var m map[string]any
	if err := c.ShouldBindJSON(&m); err != nil {
		return false, false
	}

	// Check primary key first
	if val, exists := m[primaryKey]; exists {
		if b, ok := val.(bool); ok {
			return b, true
		}
	}

	// Check alternative key
	if val, exists := m[altKey]; exists {
		if b, ok := val.(bool); ok {
			return b, true
		}
	}

	return false, false
}

// parseIntField extracts an integer field from the request body with primary and alternative keys.
func (h *Handler) parseIntField(c *gin.Context, primaryKey, altKey string, min int) (int, bool) {
	// Bind to map once to avoid double binding
	var m map[string]any
	if err := c.ShouldBindJSON(&m); err != nil {
		return 0, false
	}

	// Check primary key first
	if val, exists := m[primaryKey]; exists {
		if f, ok := val.(float64); ok { // JSON numbers are float64
			// Verify that the float64 value is a whole number before casting to int
			if f != math.Floor(f) {
				c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("%s must be a whole number", primaryKey)})
				return 0, false
			}
			intVal := int(f)
			if intVal < min {
				c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("%s must be at least %d", primaryKey, min)})
				return 0, false
			}
			return intVal, true
		}
		if i, ok := val.(int); ok {
			if i < min {
				c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("%s must be at least %d", primaryKey, min)})
				return 0, false
			}
			return i, true
		}
	}

	// Check alternative key
	if val, exists := m[altKey]; exists {
		if f, ok := val.(float64); ok { // JSON numbers are float64
			// Verify that the float64 value is a whole number before casting to int
			if f != math.Floor(f) {
				c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("%s must be a whole number", altKey)})
				return 0, false
			}
			intVal := int(f)
			if intVal < min {
				c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("%s must be at least %d", altKey, min)})
				return 0, false
			}
			return intVal, true
		}
		if i, ok := val.(int); ok {
			if i < min {
				c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("%s must be at least %d", altKey, min)})
				return 0, false
			}
			return i, true
		}
	}

	return 0, false
}

// ensureCanEnableReadOnly checks if read-only mode can be enabled by checking for pending changes.
func (h *Handler) ensureCanEnableReadOnly(c *gin.Context) bool {
	if h.scheduler != nil {
		hasChanges, err := h.scheduler.HasPendingLocalChanges()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": fmt.Sprintf("Cannot check for pending changes: %v", err),
			})
			return false
		}
		if hasChanges {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "Cannot enable read-only mode while there are pending local changes. Please sync changes first.",
			})
			return false
		}
	}
	return true
}

// persistConfig calls the persist method with a nil context to save the configuration.
func (h *Handler) persistConfig() bool {
	return h.persist(nil)
}
