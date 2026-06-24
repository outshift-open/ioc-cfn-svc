// Copyright 2026 Cisco Systems, Inc. and its affiliates
//
// SPDX-License-Identifier: Apache-2.0

package app

import (
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/outshift-open/ioc-cfn-svc/pkg/client"
	"github.com/outshift-open/ioc-cfn-svc/pkg/model"
	"github.com/outshift-open/ioc-cfn-svc/pkg/task"
)

// otelTraceTracker wraps the database TraceTracker and automatically creates
// an extraction task the first time traces arrive for a workspace/MAS pair.
// It uses an in-memory cache (seen) to avoid repeated DB calls after the
// first successful upsert per pair within a process lifetime.
type otelTraceTracker struct {
	db   client.Database
	seen sync.Map // key: "workspaceID|masID", value: true
}

// UpsertPendingOtelTrace persists trace ingestion state and then ensures
// a corresponding extraction task exists for the scheduler to pick up.
func (t *otelTraceTracker) UpsertPendingOtelTrace(workspaceID, masID, traceID string, lastSpanTime time.Time) error {
	if err := t.db.UpsertPendingOtelTrace(workspaceID, masID, traceID, lastSpanTime); err != nil {
		return err
	}
	t.autoCreateTask(workspaceID, masID)
	return nil
}

// autoCreateTask idempotently creates a task row for the extraction CE
// associated with the given workspace/MAS pair. It skips the DB call if
// the pair has already been successfully processed this session. On failure
// it clears the cache entry so the next trace retries.
func (t *otelTraceTracker) autoCreateTask(workspaceID, masID string) {
	key := workspaceID + "|" + masID
	if _, loaded := t.seen.LoadOrStore(key, true); loaded {
		return
	}

	log := getLogger()

	cfnConfigMutex.RLock()
	cfg := ParsedConfig
	cfnConfigMutex.RUnlock()

	mas := cfg.FindMAS(workspaceID, masID)
	if mas == nil {
		return
	}

	for _, ce := range mas.CognitionEngines {
		ceDef := cfg.FindCE(ce.ID)
		if ceDef == nil {
			continue
		}
		if task.GetEndpointForCE(ceDef.Name) == task.EndpointExtraction {
			newTask := &model.Task{
				ID:          uuid.New().String(),
				Name:        ceDef.Name,
				WorkspaceID: workspaceID,
				MASID:       masID,
				CEID:        ce.ID,
				Status:      "scheduled",
				NextRunTime: time.Now(),
			}
			if err := t.db.UpsertTask(newTask); err != nil {
				log.Warnf("autoCreateTask: upsert failed ws=%s mas=%s: %v", workspaceID, masID, err)
				t.seen.Delete(key)
			}
			return
		}
	}
}
