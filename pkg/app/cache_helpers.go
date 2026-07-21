// Copyright 2026 Cisco Systems, Inc. and its affiliates
//
// SPDX-License-Identifier: Apache-2.0

package app

import (
	"fmt"

	l9cache "github.com/outshift-open/ioc-cfn-svc/pkg/cache/l9"
)

// getCacheForWorkspaceMAS returns the L9 cache for the given workspace+MAS pair.
// Creates a new cache if one doesn't exist yet.
func (a *App) getCacheForWorkspaceMAS(workspaceID, masID string) *l9cache.MessageCache {
	key := cacheKey(workspaceID, masID)

	// Try to load existing cache
	if cached, ok := a.l9Caches.Load(key); ok {
		return cached.(*l9cache.MessageCache)
	}

	// Create new cache
	newCache := l9cache.New()

	// Store it (LoadOrStore in case another goroutine created it concurrently)
	actual, _ := a.l9Caches.LoadOrStore(key, newCache)
	return actual.(*l9cache.MessageCache)
}

// cacheKey creates a cache key from workspace and MAS IDs
func cacheKey(workspaceID, masID string) string {
	return fmt.Sprintf("%s:%s", workspaceID, masID)
}
