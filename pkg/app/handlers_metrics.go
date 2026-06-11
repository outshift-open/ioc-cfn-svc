package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"

	"github.com/cisco-eti/ioc-cfn-svc/pkg/client/database"
	"github.com/cisco-eti/ioc-cfn-svc/pkg/common"
	"github.com/cisco-eti/ioc-cfn-svc/pkg/metric"
	eh "github.com/cisco-eti/ioc-cfn-svc/pkg/tools/easyhttp"
)

// defaultCEID is a placeholder CE ID used when real CE ID is not available from configuration.
// TODO: Replace with actual CE ID extracted from management plane config.
// This well-known UUID makes it easy to identify and query metrics during development:
//   SELECT * FROM mas_metrics WHERE ce_id = '00000000-0000-0000-0000-000000000001';
var defaultCEID = uuid.MustParse("00000000-0000-0000-0000-000000000001")

// Safety limits for time-series queries (no pagination needed)
const (
	maxDatapoints = 100000 // Max datapoints per query (prevents unbounded result sets)
)

// errTooManyDatapoints is returned when a query would exceed the datapoint safety limit.
// Callers map this to HTTP 413.
type errTooManyDatapoints struct {
	count int
}

func (e *errTooManyDatapoints) Error() string {
	return fmt.Sprintf("query would return %d datapoints, exceeds maximum of %d. narrow your time range or add more filters", e.count, maxDatapoints)
}

// getDefaultCEID returns a placeholder CE ID for metrics storage.
// TODO: Extract real CE ID from cognition agent client configuration.
func (a *App) getDefaultCEID() *uuid.UUID {
	return &defaultCEID
}

// isMetricRegisteredForCE checks if a metric name is registered in the CE's metrics list from ParsedConfig.
// Returns true if validation passes or cannot be performed (graceful degradation).
func (a *App) isMetricRegisteredForCE(ceID uuid.UUID, metricName string) bool {
	cfnConfigMutex.RLock()
	defer cfnConfigMutex.RUnlock()

	// Allow all metrics if config is not loaded yet (graceful degradation)
	if ParsedConfig == nil {
		return true
	}

	ce := ParsedConfig.FindCE(ceID.String())
	if ce == nil {
		// Allow all metrics if CE not found (may not be registered yet)
		return true
	}

	// If metrics array is empty, allow all (no restrictions configured)
	if len(ce.Metrics) == 0 {
		return true
	}

	// Check if metric is in CE's registered metrics array
	for _, registeredMetric := range ce.Metrics {
		if registeredMetric == metricName {
			return true
		}
	}
	return false
}

// MetricDataPoint represents a single metric in the batch
type MetricDataPoint struct {
	Timestamp  *time.Time             `json:"timestamp"`
	Name       string                 `json:"name"`
	Value      float64                `json:"value"`
	Attributes map[string]interface{} `json:"attributes"`
}

// IngestCEMetricsRequest represents CE infrastructure metrics payload
type IngestCEMetricsRequest struct {
	Attributes map[string]interface{} `json:"attributes"`
	Metrics    []MetricDataPoint      `json:"metrics"`
}

// ingestCEMetricsHandler godoc
//
// @Summary     Ingest CE infrastructure metrics
// @Description Accepts batch of CE infrastructure metrics (queue depth, memory, CPU, active requests) and stores in TimescaleDB asynchronously.
//
// @Tags        cognition-engine
// @Accept      json
// @Produce     json
//
// @Param       ceId path string true "Cognition Engine ID"
// @Param       body body IngestCEMetricsRequest true "Metrics batch"
//
// @Success     202 {object} map[string]interface{} "Metrics accepted"
// @Failure     400 {object} map[string]string "Validation error"
//
// @Router      /api/cognition-engines/{ceId}/metrics [post]
func (a *App) ingestCEMetricsHandler(w http.ResponseWriter, r *http.Request) (int, error) {
	ceIDStr := eh.PathParam(r, "ceId")

	// Validate ce_id
	ceID, err := uuid.Parse(ceIDStr)
	if err != nil {
		return eh.RespondWithJSON(w, http.StatusBadRequest, map[string]string{
			"error":   "validation_failed",
			"details": "ce_id must be a valid UUID v4",
		})
	}

	var req IngestCEMetricsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return eh.RespondWithJSON(w, http.StatusBadRequest, map[string]string{
			"error":   "invalid_payload",
			"details": err.Error(),
		})
	}

	// Validate metrics array
	if len(req.Metrics) == 0 {
		return eh.RespondWithJSON(w, http.StatusBadRequest, map[string]string{
			"error":   "validation_failed",
			"details": "metrics array must contain at least one metric",
		})
	}

	if len(req.Metrics) > 10000 {
		return eh.RespondWithJSON(w, http.StatusBadRequest, map[string]string{
			"error":   "batch_size_exceeded",
			"details": fmt.Sprintf("batch contains %d metrics, maximum is 10000", len(req.Metrics)),
		})
	}

	// Validate metric values are finite
	for i, m := range req.Metrics {
		if math.IsNaN(m.Value) || math.IsInf(m.Value, 0) {
			return eh.RespondWithJSON(w, http.StatusBadRequest, map[string]string{
				"error":   "validation_failed",
				"details": fmt.Sprintf("metric %d (%s): value must be finite (NaN and Infinity not allowed)", i, m.Name),
			})
		}
	}

	// Validate metric names are registered for this CE
	for i, m := range req.Metrics {
		if !a.isMetricRegisteredForCE(ceID, m.Name) {
			return eh.RespondWithJSON(w, http.StatusBadRequest, map[string]string{
				"error":   "validation_failed",
				"details": fmt.Sprintf("metric %d (%s): not registered for CE %s", i, m.Name, ceID),
			})
		}
	}

	// Store CE metrics asynchronously
	go a.storeCEMetricsBatchFromRequest(req, ceID)

	// Return 202 Accepted immediately
	return eh.RespondWithJSON(w, http.StatusAccepted, map[string]interface{}{
		"status":   "accepted",
		"received": len(req.Metrics),
	})
}

// IngestMetricsRequest represents unified metrics ingestion payload.
// Auto-detects CE metrics (if ce_id present) vs MAS metrics (if workspace_id/mas_id present).
type IngestMetricsRequest struct {
	// CE metrics fields (mutually exclusive with MAS fields)
	CEID string `json:"ce_id,omitempty"`

	// MAS metrics fields (mutually exclusive with CE fields)
	WorkspaceID string `json:"workspace_id,omitempty"`
	MASID       string `json:"mas_id,omitempty"`
	AgentID     string `json:"agent_id,omitempty"`

	// Common fields
	Attributes map[string]interface{} `json:"attributes"`
	Metrics    []MetricDataPoint      `json:"metrics"`
}

// ingestMetricsHandler godoc
//
// @Summary     Ingest metrics batch (CE or MAS)
// @Description Accepts batch of metrics from CE and stores in TimescaleDB asynchronously.
// @Description Auto-detects CE metrics (ce_id) vs MAS metrics (workspace_id/mas_id/agent_id).
//
// @Tags        internal
// @Accept      json
// @Produce     json
//
// @Param       body body IngestMetricsRequest true "Metrics batch"
//
// @Success     202 {object} map[string]interface{} "Metrics accepted"
// @Failure     400 {object} map[string]string "Validation error"
//
// @Router      /api/internal/cognition-engine/metrics [post]
func (a *App) ingestMetricsHandler(w http.ResponseWriter, r *http.Request) (int, error) {
	var req IngestMetricsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return eh.RespondWithJSON(w, http.StatusBadRequest, map[string]string{
			"error":   "invalid_payload",
			"details": err.Error(),
		})
	}

	// Auto-detect metric type based on payload
	isCEMetrics := req.CEID != ""
	isMASMetrics := req.WorkspaceID != "" || req.MASID != "" || req.AgentID != ""

	// Validate: must be one or the other, not both
	if isCEMetrics && isMASMetrics {
		return eh.RespondWithJSON(w, http.StatusBadRequest, map[string]string{
			"error":   "validation_failed",
			"details": "cannot specify both ce_id and workspace_id/mas_id/agent_id",
		})
	}

	if !isCEMetrics && !isMASMetrics {
		return eh.RespondWithJSON(w, http.StatusBadRequest, map[string]string{
			"error":   "validation_failed",
			"details": "must specify either ce_id (CE metrics) or workspace_id/mas_id/agent_id (MAS metrics)",
		})
	}

	// Route to appropriate handler
	if isCEMetrics {
		return a.handleCEMetrics(w, req)
	}
	return a.handleMASMetrics(w, req)
}

// handleCEMetrics processes CE infrastructure metrics
func (a *App) handleCEMetrics(w http.ResponseWriter, req IngestMetricsRequest) (int, error) {
	// Validate ce_id
	ceID, err := uuid.Parse(req.CEID)
	if err != nil {
		return eh.RespondWithJSON(w, http.StatusBadRequest, map[string]string{
			"error":   "validation_failed",
			"details": "ce_id must be a valid UUID v4",
		})
	}

	// Validate metrics array
	if len(req.Metrics) == 0 {
		return eh.RespondWithJSON(w, http.StatusBadRequest, map[string]string{
			"error":   "validation_failed",
			"details": "metrics array must contain at least one metric",
		})
	}

	if len(req.Metrics) > 10000 {
		return eh.RespondWithJSON(w, http.StatusBadRequest, map[string]string{
			"error":   "batch_size_exceeded",
			"details": fmt.Sprintf("batch contains %d metrics, maximum is 10000", len(req.Metrics)),
		})
	}

	// Validate metric values are finite
	for i, m := range req.Metrics {
		if math.IsNaN(m.Value) || math.IsInf(m.Value, 0) {
			return eh.RespondWithJSON(w, http.StatusBadRequest, map[string]string{
				"error":   "validation_failed",
				"details": fmt.Sprintf("metric %d (%s): value must be finite (NaN and Infinity not allowed)", i, m.Name),
			})
		}
	}

	// Store CE metrics asynchronously
	go a.storeCEMetricsBatch(req, ceID)

	// Return 202 Accepted immediately
	return eh.RespondWithJSON(w, http.StatusAccepted, map[string]interface{}{
		"status":   "accepted",
		"received": len(req.Metrics),
	})
}

// handleMASMetrics processes MAS operation metrics
func (a *App) handleMASMetrics(w http.ResponseWriter, req IngestMetricsRequest) (int, error) {
	// Validate required fields
	if req.WorkspaceID == "" || req.MASID == "" || req.AgentID == "" {
		return eh.RespondWithJSON(w, http.StatusBadRequest, map[string]string{
			"error":   "validation_failed",
			"details": "workspace_id, mas_id, and agent_id are required for MAS metrics",
		})
	}

	// Validate metrics array
	if len(req.Metrics) == 0 {
		return eh.RespondWithJSON(w, http.StatusBadRequest, map[string]string{
			"error":   "validation_failed",
			"details": "metrics array must contain at least one metric",
		})
	}

	if len(req.Metrics) > 10000 {
		return eh.RespondWithJSON(w, http.StatusBadRequest, map[string]string{
			"error":   "batch_size_exceeded",
			"details": fmt.Sprintf("batch contains %d metrics, maximum is 10000", len(req.Metrics)),
		})
	}

	// Validate UUIDs
	workspaceID, err := uuid.Parse(req.WorkspaceID)
	if err != nil {
		return eh.RespondWithJSON(w, http.StatusBadRequest, map[string]string{
			"error":   "validation_failed",
			"details": "workspace_id must be a valid UUID v4",
		})
	}

	masID, err := uuid.Parse(req.MASID)
	if err != nil {
		return eh.RespondWithJSON(w, http.StatusBadRequest, map[string]string{
			"error":   "validation_failed",
			"details": "mas_id must be a valid UUID v4",
		})
	}

	// Validate metric values are finite
	for i, m := range req.Metrics {
		if math.IsNaN(m.Value) || math.IsInf(m.Value, 0) {
			return eh.RespondWithJSON(w, http.StatusBadRequest, map[string]string{
				"error":   "validation_failed",
				"details": fmt.Sprintf("metric %d (%s): value must be finite (NaN and Infinity not allowed)", i, m.Name),
			})
		}
	}

	// Store MAS metrics asynchronously
	go a.storeMetricsBatch(req, workspaceID, masID)

	// Return 202 Accepted immediately
	return eh.RespondWithJSON(w, http.StatusAccepted, map[string]interface{}{
		"status":   "accepted",
		"received": len(req.Metrics),
	})
}

// storeMetricsBatch inserts metrics batch into TimescaleDB (runs async)
func (a *App) storeMetricsBatch(
	req IngestMetricsRequest,
	workspaceID, masID uuid.UUID,
) {
	log := getLogger()

	// Type-assert to concrete database.Database
	db, ok := a.db.(*database.Database)
	if !ok {
		log.Errorf("Failed to type-assert database")
		return
	}

	// Parse CE ID once if provided
	var ceIDPtr *uuid.UUID
	if req.CEID != "" {
		if ceIDParsed, err := uuid.Parse(req.CEID); err == nil {
			ceIDPtr = &ceIDParsed
		} else {
			log.Warnf("Invalid CE ID format: %s", req.CEID)
		}
	}

	// Build records for batch insert
	var records []metric.MASMetric
	now := time.Now()

	for i, m := range req.Metrics {
		// Validate metric name
		if m.Name == "" {
			log.Warnf("Skipping metric %d: empty name", i)
			continue
		}

		// Validate value is finite
		if math.IsNaN(m.Value) || math.IsInf(m.Value, 0) {
			log.Warnf("Skipping metric %s: value must be finite (got %v)", m.Name, m.Value)
			continue
		}

		// Use provided timestamp or default to now
		timestamp := now
		if m.Timestamp != nil {
			timestamp = *m.Timestamp
		}

		// Merge batch-level and metric-level attributes
		finalAttributes := make(map[string]interface{})

		// Start with batch-level attributes
		if req.Attributes != nil {
			for k, v := range req.Attributes {
				finalAttributes[k] = v
			}
		}

		// Override with metric-level attributes
		if m.Attributes != nil {
			for k, v := range m.Attributes {
				finalAttributes[k] = v
			}
		}

		// Marshal attributes to JSONB
		var attributesJSON datatypes.JSON
		if len(finalAttributes) > 0 {
			attrBytes, err := json.Marshal(finalAttributes)
			if err != nil {
				log.Warnf("Failed to marshal attributes for metric %s: %v", m.Name, err)
				attributesJSON = datatypes.JSON("{}")
			} else {
				attributesJSON = attrBytes
			}
		} else {
			attributesJSON = datatypes.JSON("{}")
		}

		records = append(records, metric.MASMetric{
			Time:        timestamp,
			WorkspaceID: workspaceID,
			MASID:       masID,
			AgentID:     req.AgentID,
			CEID:        ceIDPtr,
			MetricName:  m.Name,
			Value:       m.Value,
			Attributes:  attributesJSON,
		})
	}

	// Batch insert (GORM handles conflict on duplicate PK)
	if len(records) > 0 {
		if err := db.Create(&records).Error; err != nil {
			log.Errorf("Failed to store %d metrics: %v", len(records), err)
		} else {
			log.Infof("Stored %d metrics for agent %s", len(records), req.AgentID)
		}
	}
}

// storeCEMetricsBatch inserts CE metrics batch into TimescaleDB (runs async)
// storeCEMetricsBatchFromRequest adapts IngestCEMetricsRequest to storage
func (a *App) storeCEMetricsBatchFromRequest(req IngestCEMetricsRequest, ceID uuid.UUID) {
	a.storeCEMetricsBatch(IngestMetricsRequest{
		CEID:       ceID.String(),
		Attributes: req.Attributes,
		Metrics:    req.Metrics,
	}, ceID)
}

func (a *App) storeCEMetricsBatch(req IngestMetricsRequest, ceID uuid.UUID) {
	log := getLogger()

	db, ok := a.db.(*database.Database)
	if !ok {
		log.Errorf("Failed to type-assert database")
		return
	}

	var records []metric.CEMetric
	now := time.Now()

	for i, m := range req.Metrics {
		if m.Name == "" {
			log.Warnf("Skipping metric %d: empty name", i)
			continue
		}

		if math.IsNaN(m.Value) || math.IsInf(m.Value, 0) {
			log.Warnf("Skipping metric %s: non-finite value", m.Name)
			continue
		}

		timestamp := now
		if m.Timestamp != nil {
			timestamp = *m.Timestamp
		}

		// Merge batch-level and metric-level attributes
		finalAttributes := make(map[string]interface{})
		if req.Attributes != nil {
			for k, v := range req.Attributes {
				finalAttributes[k] = v
			}
		}
		if m.Attributes != nil {
			for k, v := range m.Attributes {
				finalAttributes[k] = v
			}
		}

		// Marshal attributes to JSONB
		var attributesJSON datatypes.JSON
		if len(finalAttributes) > 0 {
			attrBytes, err := json.Marshal(finalAttributes)
			if err != nil {
				log.Warnf("Failed to marshal attributes for metric %s: %v", m.Name, err)
				attributesJSON = datatypes.JSON("{}")
			} else {
				attributesJSON = attrBytes
			}
		} else {
			attributesJSON = datatypes.JSON("{}")
		}

		records = append(records, metric.CEMetric{
			Time:       timestamp,
			CEID:       ceID,
			MetricName: m.Name,
			Value:      m.Value,
			Attributes: attributesJSON,
		})
	}

	// Batch insert
	if len(records) > 0 {
		if err := db.Create(&records).Error; err != nil {
			log.Errorf("Failed to store %d CE metrics: %v", len(records), err)
		} else {
			log.Infof("Stored %d CE metrics for ce_id=%s", len(records), ceID)
		}
	}
}

// MetricSeries represents a grouped time-series (metric name + entity IDs + datapoints)
// Reduces verbosity by avoiding repeated metadata in each datapoint
type MetricSeries struct {
	MetricName string `json:"metric_name"`

	// MAS fields (populated for MAS metrics)
	WorkspaceID string `json:"workspace_id,omitempty"`
	MASID       string `json:"mas_id,omitempty"`
	CEID        string `json:"ce_id,omitempty"`
	AgentID     string `json:"agent_id,omitempty"`

	// Attributes (shared across all datapoints in this series)
	Attributes map[string]interface{} `json:"attributes"`

	// Datapoints: array of [timestamp, value] pairs
	// Format: [["2026-05-27T10:00:00Z", 123.45], ["2026-05-27T10:01:00Z", 456.78]]
	Datapoints [][]interface{} `json:"datapoints"`
}

type MASMetricSeries struct {
	MetricName string                 `json:"metric_name"`
	AgentID    string                 `json:"agent_id,omitempty"`
	Attributes map[string]interface{} `json:"attributes"`
	Datapoints [][]interface{}        `json:"datapoints"`
}

// CEMetricGroup groups metric series by CE within a MAS metrics response.
type CEMetricGroup struct {
	CEID   string            `json:"ce_id"`
	Series []MASMetricSeries `json:"series"`
}

// MASMetricsQueryResponse represents the MAS-scoped metrics query result
type MASMetricsQueryResponse struct {
	MASID       string          `json:"mas_id"`
	WorkspaceID string          `json:"workspace_id"`
	StartTime   string          `json:"start_time"`
	EndTime     string          `json:"end_time"`
	CEs         []CEMetricGroup `json:"ces"`
}

// MetricsQueryResponse represents the unified query result with separate CE and MAS metrics
type MetricsQueryResponse struct {
	CEID       string           `json:"ce_id"`
	CEMetrics  *MetricResultSet `json:"ce_metrics,omitempty"`
	MASMetrics *MetricResultSet `json:"mas_metrics,omitempty"`
}

// MetricResultSet represents metrics from a single table
type MetricResultSet struct {
	Series []MetricSeries `json:"series"`
}

// parseFlexibleTime parses time from multiple formats:
// - Unix timestamp (seconds): "1716076800"
// - Unix timestamp (milliseconds): "1716076800000"
// - RFC3339: "2026-05-19T00:00:00Z"
// - Date-only (assumes UTC midnight): "2026-05-19"
func parseFlexibleTime(s string) (time.Time, error) {
	// Try Unix timestamp (seconds or milliseconds)
	if unix, err := strconv.ParseInt(s, 10, 64); err == nil {
		// Milliseconds have 13 digits, seconds have 10 digits
		if unix > 9999999999 { // > 10 digits, assume milliseconds
			return time.Unix(0, unix*int64(time.Millisecond)).UTC(), nil
		}
		return time.Unix(unix, 0).UTC(), nil
	}

	// Try RFC3339 (explicit timezone)
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t.UTC(), nil
	}

	// Fall back to date-only (assume UTC midnight)
	if t, err := time.Parse("2006-01-02", s); err == nil {
		return t.UTC(), nil
	}

	return time.Time{}, fmt.Errorf("invalid format (use Unix timestamp, RFC3339 '2026-05-19T00:00:00Z', or date '2026-05-19')")
}

// validateTimeRange validates time range constraints
func validateTimeRange(startTime, endTime time.Time) error {
	// End time must be after start time
	if endTime.Before(startTime) {
		return fmt.Errorf("end_time must be after start_time")
	}

	// Reject unreasonable years (before 2000 or after 2100)
	if startTime.Year() < 2000 || startTime.Year() > 2100 {
		return fmt.Errorf("start_time year must be between 2000 and 2100")
	}
	if endTime.Year() < 2000 || endTime.Year() > 2100 {
		return fmt.Errorf("end_time year must be between 2000 and 2100")
	}

	// Reject queries into far future (more than 1 day ahead)
	now := time.Now().UTC()
	maxFuture := now.Add(24 * time.Hour)
	if startTime.After(maxFuture) {
		return fmt.Errorf("start_time cannot be more than 1 day in the future")
	}

	// Maximum time range: 366 days (1 year + leap day)
	maxDuration := 366 * 24 * time.Hour
	duration := endTime.Sub(startTime)
	if duration > maxDuration {
		return fmt.Errorf("time range cannot exceed 366 days (requested: %d days)", int(duration.Hours()/24))
	}

	return nil
}

// getMetricsHandler godoc
//
// @Summary     Query metrics within time range (CE and/or MAS)
// @Description Returns grouped time-series data filtered by time and entity dimensions.
// @Description Returns both CE infrastructure metrics (queue, memory, CPU) and MAS operation metrics (tokens, latency, cost) processed by this CE.
// @Description Response format groups datapoints by metric name to reduce verbosity (60-70% size reduction).
// @Description No pagination - queries return all matching datapoints up to safety limit (100K max).
//
// @Tags        cognition-engine
// @Produce     json
//
// @Param       ceId path string true "Cognition Engine UUID"
// @Param       start_time query string true "Start time (Unix timestamp, RFC3339, or date)"
// @Param       end_time query string true "End time (Unix timestamp, RFC3339, or date)"
// @Param       workspace_id query string false "Filter MAS metrics by workspace UUID"
// @Param       mas_id query string false "Filter MAS metrics by MAS UUID"
// @Param       agent_id query string false "Filter MAS metrics by agent ID"
// @Param       metric_name query string false "Filter by metric name (supports * wildcard)"
//
// @Success     200 {object} MetricsQueryResponse
// @Failure     400 {object} map[string]string "Invalid parameters"
// @Failure     413 {object} map[string]string "Too many datapoints (exceeds 100K limit)"
//
// @Router      /api/cognition-engines/{ceId}/metrics [get]
func (a *App) getMetricsHandler(w http.ResponseWriter, r *http.Request) (int, error) {
	log := getLogger()

	// Extract CE ID from path parameter (required)
	ceIDStr := eh.PathParam(r, "ceId")
	if ceIDStr == "" {
		return eh.RespondWithJSON(w, http.StatusBadRequest, map[string]string{
			"error": "ce_id is required in path",
		})
	}

	// Validate CE ID format
	if _, err := uuid.Parse(ceIDStr); err != nil {
		return eh.RespondWithJSON(w, http.StatusBadRequest, map[string]string{
			"error": "ce_id must be a valid UUID",
		})
	}

	// Parse required time range
	startTimeStr := r.URL.Query().Get("start_time")
	endTimeStr := r.URL.Query().Get("end_time")

	if startTimeStr == "" || endTimeStr == "" {
		return eh.RespondWithJSON(w, http.StatusBadRequest, map[string]string{
			"error": "start_time and end_time are required",
		})
	}

	startTime, err := parseFlexibleTime(startTimeStr)
	if err != nil {
		return eh.RespondWithJSON(w, http.StatusBadRequest, map[string]string{
			"error": fmt.Sprintf("start_time: %v", err),
		})
	}

	endTime, err := parseFlexibleTime(endTimeStr)
	if err != nil {
		return eh.RespondWithJSON(w, http.StatusBadRequest, map[string]string{
			"error": fmt.Sprintf("end_time: %v", err),
		})
	}

	// Validate time range
	if err := validateTimeRange(startTime, endTime); err != nil {
		log.Warnf("Time range validation failed: %v", err)
		return eh.RespondWithJSON(w, http.StatusBadRequest, map[string]string{
			"error": err.Error(),
		})
	}

	// Parse optional filters (to narrow MAS metrics)
	workspaceIDStr := r.URL.Query().Get("workspace_id")
	masIDStr := r.URL.Query().Get("mas_id")
	agentID := r.URL.Query().Get("agent_id")
	metricName := r.URL.Query().Get("metric_name")

	// Validate optional UUID filters before hitting database
	if workspaceIDStr != "" {
		if _, err := uuid.Parse(workspaceIDStr); err != nil {
			return eh.RespondWithJSON(w, http.StatusBadRequest, map[string]string{
				"error": "workspace_id must be a valid UUID",
			})
		}
	}
	if masIDStr != "" {
		if _, err := uuid.Parse(masIDStr); err != nil {
			return eh.RespondWithJSON(w, http.StatusBadRequest, map[string]string{
				"error": "mas_id must be a valid UUID",
			})
		}
	}

	// Get database
	db, ok := a.db.(*database.Database)
	if !ok {
		return eh.RespondWithJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "database_error",
		})
	}

	// Build response
	response := MetricsQueryResponse{
		CEID: ceIDStr,
	}

	// Query CE infrastructure metrics
	ceResult, err := a.queryCEMetricsData(db.DB, startTime, endTime, ceIDStr, metricName)
	if err != nil {
		log.Errorf("CE query failed: %v", err)
		// Check if error is due to too many datapoints
		if errors.As(err, new(*errTooManyDatapoints)) {
			return eh.RespondWithJSON(w, http.StatusRequestEntityTooLarge, map[string]string{
				"error": err.Error(),
			})
		}
		return eh.RespondWithJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "ce_query_failed",
		})
	}
	response.CEMetrics = ceResult

	// Query MAS operations processed by this CE
	// Optional filters (workspace_id, mas_id, agent_id) can narrow results
	masResult, err := a.queryMASMetricsData(db.DB, startTime, endTime, workspaceIDStr, masIDStr, agentID, metricName, ceIDStr)
	if err != nil {
		log.Errorf("MAS query (by CE ID) failed: %v", err)
		// Check if error is due to too many datapoints
		if errors.As(err, new(*errTooManyDatapoints)) {
			return eh.RespondWithJSON(w, http.StatusRequestEntityTooLarge, map[string]string{
				"error": err.Error(),
			})
		}
		return eh.RespondWithJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "mas_query_failed",
		})
	}
	response.MASMetrics = masResult

	return eh.RespondWithJSON(w, http.StatusOK, response)
}

// storeTokenMetricsAsync extracts token metadata from CE response and stores to TimescaleDB
// This is fire-and-forget - runs in background goroutine
func (a *App) storeTokenMetricsAsync(
	workspaceID, masID uuid.UUID,
	agentID, service, requestID string,
	ceID *uuid.UUID,
	tokenMeta *common.TokenUsageMeta,
) {
	if tokenMeta == nil || tokenMeta.Tokens.Total == 0 {
		return // No tokens to record
	}

	// Use default CE ID if not provided
	if ceID == nil {
		ceID = a.getDefaultCEID()
	}

	// Build metrics batch
	timestamp := time.Now()
	if tokenMeta.Timestamp != "" {
		if t, err := time.Parse(time.RFC3339, tokenMeta.Timestamp); err == nil {
			timestamp = t
		}
	}

	// Prepare attributes
	attributes := map[string]interface{}{
		"service":    service, // "semantic_negotiation", "ingestion", "evidence"
		"model":      tokenMeta.Tokens.Model,
		"request_id": requestID,
	}

	metricsReq := IngestMetricsRequest{
		WorkspaceID: workspaceID.String(),
		MASID:       masID.String(),
		AgentID:     agentID,
		CEID:        "", // Set below if ceID provided
		Attributes:  attributes,
		Metrics: []MetricDataPoint{
			{
				Timestamp:  &timestamp,
				Name:       "llm.tokens.prompt",
				Value:      float64(tokenMeta.Tokens.Prompt),
				Attributes: nil, // Use batch-level attributes
			},
			{
				Timestamp:  &timestamp,
				Name:       "llm.tokens.completion",
				Value:      float64(tokenMeta.Tokens.Completion),
				Attributes: nil,
			},
			{
				Timestamp:  &timestamp,
				Name:       "llm.tokens.total",
				Value:      float64(tokenMeta.Tokens.Total),
				Attributes: nil,
			},
			{
				Timestamp:  &timestamp,
				Name:       "llm.latency_ms",
				Value:      tokenMeta.LatencyMs,
				Attributes: nil,
			},
		},
	}

	// Add cost metric if available
	if tokenMeta.CostUsd != nil && *tokenMeta.CostUsd > 0 {
		metricsReq.Metrics = append(metricsReq.Metrics, MetricDataPoint{
			Timestamp:  &timestamp,
			Name:       "llm.cost_usd",
			Value:      *tokenMeta.CostUsd,
			Attributes: nil,
		})
	}

	// Set CE ID if provided
	if ceID != nil {
		metricsReq.CEID = ceID.String()
	}

	// Store asynchronously (fire-and-forget)
	go a.storeMetricsBatch(metricsReq, workspaceID, masID)
}

// queryCEMetricsData queries ce_metrics table with filters
func (a *App) queryCEMetricsData(
	db *gorm.DB,
	startTime, endTime time.Time,
	ceIDStr, metricName string,
) (*MetricResultSet, error) {
	query := db.Model(&metric.CEMetric{}).
		Where("time >= ? AND time < ?", startTime, endTime)

	// Apply CE ID filter
	if ceIDStr != "" {
		ceID, err := uuid.Parse(ceIDStr)
		if err != nil {
			return nil, fmt.Errorf("ce_id must be valid UUID")
		}
		query = query.Where("ce_id = ?", ceID)
	}

	// Apply metric name filter
	if metricName != "" {
		if strings.Contains(metricName, "*") {
			pattern := strings.ReplaceAll(metricName, "*", "%")
			query = query.Where("metric_name LIKE ?", pattern)
		} else {
			query = query.Where("metric_name = ?", metricName)
		}
	}

	// Check count before fetching (safety limit)
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, err
	}

	if total > maxDatapoints {
		return nil, &errTooManyDatapoints{count: int(total)}
	}

	// Execute query (no pagination)
	var records []metric.CEMetric
	if err := query.Order("time DESC").Find(&records).Error; err != nil {
		return nil, err
	}

	// Group by (metric_name, attributes) to build series
	// Note: ce_id is already at top level of response, no need to repeat per series
	type seriesKey struct {
		metricName string
		attrsJSON  string
	}
	seriesMap := make(map[seriesKey]*MetricSeries)

	for _, r := range records {
		var attrs map[string]interface{}
		json.Unmarshal(r.Attributes, &attrs)

		key := seriesKey{
			metricName: r.MetricName,
			attrsJSON:  string(r.Attributes),
		}

		if _, exists := seriesMap[key]; !exists {
			seriesMap[key] = &MetricSeries{
				MetricName: r.MetricName,
				Attributes: attrs,
				Datapoints: [][]interface{}{},
			}
		}

		// Add datapoint as [timestamp, value]
		seriesMap[key].Datapoints = append(seriesMap[key].Datapoints, []interface{}{
			r.Time.Format(time.RFC3339Nano),
			r.Value,
		})
	}

	// Sort for stable response ordering — Go maps iterate in random order
	series := make([]MetricSeries, 0, len(seriesMap))
	for _, s := range seriesMap {
		series = append(series, *s)
	}
	sort.Slice(series, func(i, j int) bool {
		a, b := series[i], series[j]
		if a.MetricName != b.MetricName {
			return a.MetricName < b.MetricName
		}
		if a.CEID != b.CEID {
			return a.CEID < b.CEID
		}
		if a.AgentID != b.AgentID {
			return a.AgentID < b.AgentID
		}
		return a.WorkspaceID < b.WorkspaceID
	})

	return &MetricResultSet{
		Series: series,
	}, nil
}

// getMASMetricsHandler godoc
//
// @Summary     Query token usage metrics for a MAS
// @Description Returns time-series token usage and LLM operation metrics for a MAS across all attached CEs.
// @Description Optionally filter by ce_id, agent_id, or metric_name.
// @Description Response groups datapoints by (metric_name, ce_id, agent_id, attributes) to distinguish contributions from different CEs.
//
// @Tags        mas
// @Produce     json
//
// @Param       workspaceId  path   string true  "Workspace UUID"
// @Param       masId        path   string true  "MAS UUID"
// @Param       start_time   query  string true  "Start time (Unix timestamp, RFC3339, or date)"
// @Param       end_time     query  string true  "End time (Unix timestamp, RFC3339, or date)"
// @Param       ce_id        query  string false "Filter by Cognition Engine UUID"
// @Param       agent_id     query  string false "Filter by agent ID"
// @Param       metric_name  query  string false "Filter by metric name (supports * wildcard)"
//
// @Success     200 {object} MASMetricsQueryResponse
// @Failure     400 {object} map[string]string "Invalid parameters"
// @Failure     413 {object} map[string]string "Too many datapoints (exceeds 100K limit)"
// @Failure     500 {object} map[string]string "Internal server error"
//
// @Router      /api/internal/mgmt/workspaces/{workspaceId}/multi-agentic-systems/{masId}/metrics [get]
func (a *App) getMASMetricsHandler(w http.ResponseWriter, r *http.Request) (int, error) {
	log := getLogger()

	workspaceIDStr := eh.PathParam(r, "workspaceId")
	masIDStr := eh.PathParam(r, "masId")

	if _, err := uuid.Parse(workspaceIDStr); err != nil {
		return eh.RespondWithJSON(w, http.StatusBadRequest, map[string]string{
			"error": "workspaceId must be a valid UUID",
		})
	}
	if _, err := uuid.Parse(masIDStr); err != nil {
		return eh.RespondWithJSON(w, http.StatusBadRequest, map[string]string{
			"error": "masId must be a valid UUID",
		})
	}

	startTimeStr := r.URL.Query().Get("start_time")
	endTimeStr := r.URL.Query().Get("end_time")

	if startTimeStr == "" || endTimeStr == "" {
		return eh.RespondWithJSON(w, http.StatusBadRequest, map[string]string{
			"error": "start_time and end_time are required",
		})
	}

	startTime, err := parseFlexibleTime(startTimeStr)
	if err != nil {
		return eh.RespondWithJSON(w, http.StatusBadRequest, map[string]string{
			"error": fmt.Sprintf("start_time: %v", err),
		})
	}

	endTime, err := parseFlexibleTime(endTimeStr)
	if err != nil {
		return eh.RespondWithJSON(w, http.StatusBadRequest, map[string]string{
			"error": fmt.Sprintf("end_time: %v", err),
		})
	}

	if err := validateTimeRange(startTime, endTime); err != nil {
		log.Warnf("Time range validation failed: %v", err)
		return eh.RespondWithJSON(w, http.StatusBadRequest, map[string]string{
			"error": err.Error(),
		})
	}

	ceIDStr := r.URL.Query().Get("ce_id")
	agentID := r.URL.Query().Get("agent_id")
	metricName := r.URL.Query().Get("metric_name")

	if ceIDStr != "" {
		if _, err := uuid.Parse(ceIDStr); err != nil {
			return eh.RespondWithJSON(w, http.StatusBadRequest, map[string]string{
				"error": "ce_id must be a valid UUID",
			})
		}
	}

	if err := validateWorkspaceAndMAS(workspaceIDStr, masIDStr); err != nil {
		return eh.RespondWithJSON(w, http.StatusNotFound, map[string]string{
			"error": err.Error(),
		})
	}

	if ceIDStr != "" {
		cfnConfigMutex.RLock()
		ceExists := ParsedConfig != nil && ParsedConfig.FindCE(ceIDStr) != nil
		cfnConfigMutex.RUnlock()
		if !ceExists {
			return eh.RespondWithJSON(w, http.StatusNotFound, map[string]string{
				"error": fmt.Sprintf("cognition engine %s not found", ceIDStr),
			})
		}
	}

	db, ok := a.db.(*database.Database)
	if !ok {
		return eh.RespondWithJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "database_error",
		})
	}

	result, err := a.queryMASMetricsData(db.DB, startTime, endTime, workspaceIDStr, masIDStr, agentID, metricName, ceIDStr)
	if err != nil {
		log.Errorf("MAS metrics query failed: %v", err)
		if errors.As(err, new(*errTooManyDatapoints)) {
			return eh.RespondWithJSON(w, http.StatusRequestEntityTooLarge, map[string]string{
				"error": err.Error(),
			})
		}
		return eh.RespondWithJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "query_failed",
		})
	}

	// Group flat series by CE ID; order follows the sorted series (alphabetical by CEID).
	ceMap := make(map[string]*CEMetricGroup)
	ceOrder := []string{}
	for _, s := range result.Series {
		if _, exists := ceMap[s.CEID]; !exists {
			ceMap[s.CEID] = &CEMetricGroup{
				CEID:   s.CEID,
				Series: []MASMetricSeries{},
			}
			ceOrder = append(ceOrder, s.CEID)
		}
		ceMap[s.CEID].Series = append(ceMap[s.CEID].Series, MASMetricSeries{
			MetricName: s.MetricName,
			AgentID:    s.AgentID,
			Attributes: s.Attributes,
			Datapoints: s.Datapoints,
		})
	}
	ces := make([]CEMetricGroup, 0, len(ceOrder))
	for _, ceID := range ceOrder {
		ces = append(ces, *ceMap[ceID])
	}

	response := MASMetricsQueryResponse{
		MASID:       masIDStr,
		WorkspaceID: workspaceIDStr,
		StartTime:   startTime.Format(time.RFC3339),
		EndTime:     endTime.Format(time.RFC3339),
		CEs:         ces,
	}

	return eh.RespondWithJSON(w, http.StatusOK, response)
}

// queryMASMetricsData queries mas_metrics table with optional filters.
// Used by both getMASMetricsHandler (MAS-scoped) and getMetricsHandler (CE-scoped).
func (a *App) queryMASMetricsData(
	db *gorm.DB,
	startTime, endTime time.Time,
	workspaceIDStr, masIDStr, agentID, metricName, ceIDStr string,
) (*MetricResultSet, error) {
	query := db.Model(&metric.MASMetric{}).
		Where("time >= ? AND time < ?", startTime, endTime)

	// Apply CE ID filter (for CE-centric queries)
	if ceIDStr != "" {
		ceID, err := uuid.Parse(ceIDStr)
		if err != nil {
			return nil, fmt.Errorf("ce_id must be valid UUID")
		}
		query = query.Where("ce_id = ?", ceID)
	}

	// Apply workspace ID filter
	if workspaceIDStr != "" {
		workspaceID, err := uuid.Parse(workspaceIDStr)
		if err != nil {
			return nil, fmt.Errorf("workspace_id must be valid UUID")
		}
		query = query.Where("workspace_id = ?", workspaceID)
	}

	// Apply MAS ID filter
	if masIDStr != "" {
		masID, err := uuid.Parse(masIDStr)
		if err != nil {
			return nil, fmt.Errorf("mas_id must be valid UUID")
		}
		query = query.Where("mas_id = ?", masID)
	}

	// Apply agent ID filter
	if agentID != "" {
		query = query.Where("agent_id = ?", agentID)
	}

	// Apply metric name filter
	if metricName != "" {
		if strings.Contains(metricName, "*") {
			pattern := strings.ReplaceAll(metricName, "*", "%")
			query = query.Where("metric_name LIKE ?", pattern)
		} else {
			query = query.Where("metric_name = ?", metricName)
		}
	}

	// Check count before fetching (safety limit)
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, err
	}

	if total > maxDatapoints {
		return nil, &errTooManyDatapoints{count: int(total)}
	}

	// Execute query (no pagination)
	var records []metric.MASMetric
	if err := query.Order("time DESC").Find(&records).Error; err != nil {
		return nil, err
	}

	// Group by (workspace_id, mas_id, ce_id, agent_id, metric_name, attributes) to build series.
	// ce_id is included so metrics from different CEs within the same MAS are not merged.
	type seriesKey struct {
		workspaceID string
		masID       string
		ceID        string
		agentID     string
		metricName  string
		attrsJSON   string
	}
	seriesMap := make(map[seriesKey]*MetricSeries)

	for _, r := range records {
		var attrs map[string]interface{}
		json.Unmarshal(r.Attributes, &attrs)

		ceIDStr := ""
		if r.CEID != nil {
			ceIDStr = r.CEID.String()
		}

		key := seriesKey{
			workspaceID: r.WorkspaceID.String(),
			masID:       r.MASID.String(),
			ceID:        ceIDStr,
			agentID:     r.AgentID,
			metricName:  r.MetricName,
			attrsJSON:   string(r.Attributes),
		}

		if _, exists := seriesMap[key]; !exists {
			seriesMap[key] = &MetricSeries{
				MetricName:  r.MetricName,
				WorkspaceID: r.WorkspaceID.String(),
				MASID:       r.MASID.String(),
				CEID:        ceIDStr,
				AgentID:     r.AgentID,
				Attributes:  attrs,
				Datapoints:  [][]interface{}{},
			}
		}

		// Add datapoint as [timestamp, value]
		seriesMap[key].Datapoints = append(seriesMap[key].Datapoints, []interface{}{
			r.Time.Format(time.RFC3339Nano),
			r.Value,
		})
	}

	// Sort for stable response ordering — Go map iterates in random order
	series := make([]MetricSeries, 0, len(seriesMap))
	for _, s := range seriesMap {
		series = append(series, *s)
	}
	sort.Slice(series, func(i, j int) bool {
		a, b := series[i], series[j]
		if a.MetricName != b.MetricName {
			return a.MetricName < b.MetricName
		}
		if a.CEID != b.CEID {
			return a.CEID < b.CEID
		}
		if a.AgentID != b.AgentID {
			return a.AgentID < b.AgentID
		}
		return a.WorkspaceID < b.WorkspaceID
	})

	return &MetricResultSet{
		Series: series,
	}, nil
}
