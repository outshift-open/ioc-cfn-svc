package app

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"strconv"
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

// MetricDataPoint represents a single metric in the batch
type MetricDataPoint struct {
	Timestamp  *time.Time             `json:"timestamp"`
	Name       string                 `json:"name"`
	Value      float64                `json:"value"`
	Attributes map[string]interface{} `json:"attributes"`
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
				attributesJSON = datatypes.JSON([]byte("{}"))
			} else {
				attributesJSON = datatypes.JSON(attrBytes)
			}
		} else {
			attributesJSON = datatypes.JSON([]byte("{}"))
		}

		records = append(records, metric.MASMetric{
			Time:        timestamp,
			WorkspaceID: workspaceID,
			MASID:       masID,
			AgentID:     req.AgentID,
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
				attributesJSON = datatypes.JSON([]byte("{}"))
			} else {
				attributesJSON = datatypes.JSON(attrBytes)
			}
		} else {
			attributesJSON = datatypes.JSON([]byte("{}"))
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

// MetricRecord represents a single metric data point in response
// Fields are populated based on which table the metric came from (CE or MAS)
type MetricRecord struct {
	Timestamp string                 `json:"timestamp"`

	// CE fields (populated for CE metrics)
	CEID string `json:"ce_id,omitempty"`

	// MAS fields (populated for MAS metrics)
	WorkspaceID string `json:"workspace_id,omitempty"`
	MASID       string `json:"mas_id,omitempty"`
	AgentID     string `json:"agent_id,omitempty"`

	// Common fields
	MetricName string                 `json:"metric_name"`
	Value      float64                `json:"value"`
	Attributes map[string]interface{} `json:"attributes"`
}

// MetricsQueryResponse represents the unified query result with separate CE and MAS metrics
type MetricsQueryResponse struct {
	Period     Period              `json:"period"`
	Filters    Filters             `json:"filters,omitempty"`
	CEMetrics  *MetricResultSet    `json:"ce_metrics,omitempty"`
	MASMetrics *MetricResultSet    `json:"mas_metrics,omitempty"`
}

// MetricResultSet represents metrics from a single table with pagination
type MetricResultSet struct {
	Total  int            `json:"total"`
	Limit  int            `json:"limit"`
	Offset int            `json:"offset"`
	Data   []MetricRecord `json:"data"`
}

type Period struct {
	Start string `json:"start"`
	End   string `json:"end"`
}

type Filters struct {
	CEID        *string `json:"ce_id,omitempty"`
	WorkspaceID *string `json:"workspace_id,omitempty"`
	MASID       *string `json:"mas_id,omitempty"`
	AgentID     *string `json:"agent_id,omitempty"`
	MetricName  *string `json:"metric_name,omitempty"`
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

// getMetricsHandler godoc
//
// @Summary     Query metrics within time range (CE and/or MAS)
// @Description Returns raw metric data points filtered by time and optional dimensions.
// @Description Smart routing: ce_id → CE metrics, workspace/mas/agent → MAS metrics, neither → both
//
// @Tags        cognition-engine
// @Produce     json
//
// @Param       start_time query string true "Start time (Unix timestamp, RFC3339, or date)"
// @Param       end_time query string true "End time (Unix timestamp, RFC3339, or date)"
// @Param       ce_id query string false "Filter CE metrics by instance UUID"
// @Param       workspace_id query string false "Filter MAS metrics by workspace UUID"
// @Param       mas_id query string false "Filter MAS metrics by MAS UUID"
// @Param       agent_id query string false "Filter MAS metrics by agent ID"
// @Param       metric_name query string false "Filter by metric name (supports * wildcard)"
// @Param       limit query int false "Max results per table (default 1000, max 10000)"
// @Param       offset query int false "Pagination offset per table (default 0)"
//
// @Success     200 {object} MetricsQueryResponse
// @Failure     400 {object} map[string]string "Invalid parameters"
//
// @Router      /api/cognition-engine/metrics [get]
func (a *App) getMetricsHandler(w http.ResponseWriter, r *http.Request) (int, error) {
	log := getLogger()

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

	// Parse optional filters
	ceIDStr := r.URL.Query().Get("ce_id")
	workspaceIDStr := r.URL.Query().Get("workspace_id")
	masIDStr := r.URL.Query().Get("mas_id")
	agentID := r.URL.Query().Get("agent_id")
	metricName := r.URL.Query().Get("metric_name")

	// Parse pagination
	limit := 1000
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 10000 {
			limit = l
		}
	}

	offset := 0
	if offsetStr := r.URL.Query().Get("offset"); offsetStr != "" {
		if o, err := strconv.Atoi(offsetStr); err == nil && o >= 0 {
			offset = o
		}
	}

	// Get database
	db, ok := a.db.(*database.Database)
	if !ok {
		return eh.RespondWithJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "database_error",
		})
	}

	// Smart routing: determine which tables to query
	queryCE := ceIDStr != ""
	queryMAS := workspaceIDStr != "" || masIDStr != "" || agentID != ""

	// If NEITHER specified, query BOTH (time-only query)
	if !queryCE && !queryMAS {
		queryCE = true
		queryMAS = true
	}

	// Build response
	response := MetricsQueryResponse{
		Period: Period{
			Start: startTime.Format(time.RFC3339),
			End:   endTime.Format(time.RFC3339),
		},
		Filters: Filters{},
	}

	// Query CE metrics if applicable
	if queryCE {
		ceResult, err := a.queryCEMetricsData(db.DB, startTime, endTime, ceIDStr, metricName, limit, offset)
		if err != nil {
			log.Errorf("CE query failed: %v", err)
			return eh.RespondWithJSON(w, http.StatusInternalServerError, map[string]string{
				"error": "ce_query_failed",
			})
		}
		response.CEMetrics = ceResult
		if ceIDStr != "" {
			response.Filters.CEID = &ceIDStr
		}
	}

	// Query MAS metrics if applicable
	if queryMAS {
		masResult, err := a.queryMASMetricsData(db.DB, startTime, endTime, workspaceIDStr, masIDStr, agentID, metricName, limit, offset)
		if err != nil {
			log.Errorf("MAS query failed: %v", err)
			return eh.RespondWithJSON(w, http.StatusInternalServerError, map[string]string{
				"error": "mas_query_failed",
			})
		}
		response.MASMetrics = masResult
		if workspaceIDStr != "" {
			response.Filters.WorkspaceID = &workspaceIDStr
		}
		if masIDStr != "" {
			response.Filters.MASID = &masIDStr
		}
		if agentID != "" {
			response.Filters.AgentID = &agentID
		}
	}

	// Add metric name filter if specified
	if metricName != "" {
		response.Filters.MetricName = &metricName
	}

	return eh.RespondWithJSON(w, http.StatusOK, response)
}

// storeTokenMetricsAsync extracts token metadata from CE response and stores to TimescaleDB
// This is fire-and-forget - runs in background goroutine
func (a *App) storeTokenMetricsAsync(
	workspaceID, masID uuid.UUID,
	agentID, service, requestID string,
	tokenMeta *common.TokenUsageMeta,
) {
	if tokenMeta == nil || tokenMeta.Tokens.Total == 0 {
		return // No tokens to record
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

	// Store asynchronously (fire-and-forget)
	go a.storeMetricsBatch(metricsReq, workspaceID, masID)
}

// queryCEMetricsData queries ce_metrics table with filters
func (a *App) queryCEMetricsData(
	db *gorm.DB,
	startTime, endTime time.Time,
	ceIDStr, metricName string,
	limit, offset int,
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

	// Get total count
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, err
	}

	// Execute query with pagination
	var records []metric.CEMetric
	if err := query.Order("time DESC").Limit(limit).Offset(offset).Find(&records).Error; err != nil {
		return nil, err
	}

	// Build result set
	data := make([]MetricRecord, len(records))
	for i, r := range records {
		var attrs map[string]interface{}
		json.Unmarshal([]byte(r.Attributes), &attrs)

		data[i] = MetricRecord{
			Timestamp:  r.Time.Format(time.RFC3339Nano),
			CEID:       r.CEID.String(),
			MetricName: r.MetricName,
			Value:      r.Value,
			Attributes: attrs,
		}
	}

	return &MetricResultSet{
		Total:  int(total),
		Limit:  limit,
		Offset: offset,
		Data:   data,
	}, nil
}

// queryMASMetricsData queries mas_metrics table with filters
func (a *App) queryMASMetricsData(
	db *gorm.DB,
	startTime, endTime time.Time,
	workspaceIDStr, masIDStr, agentID, metricName string,
	limit, offset int,
) (*MetricResultSet, error) {
	query := db.Model(&metric.MASMetric{}).
		Where("time >= ? AND time < ?", startTime, endTime)

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

	// Get total count
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, err
	}

	// Execute query with pagination
	var records []metric.MASMetric
	if err := query.Order("time DESC").Limit(limit).Offset(offset).Find(&records).Error; err != nil {
		return nil, err
	}

	// Build result set
	data := make([]MetricRecord, len(records))
	for i, r := range records {
		var attrs map[string]interface{}
		json.Unmarshal([]byte(r.Attributes), &attrs)

		data[i] = MetricRecord{
			Timestamp:   r.Time.Format(time.RFC3339Nano),
			WorkspaceID: r.WorkspaceID.String(),
			MASID:       r.MASID.String(),
			AgentID:     r.AgentID,
			MetricName:  r.MetricName,
			Value:       r.Value,
			Attributes:  attrs,
		}
	}

	return &MetricResultSet{
		Total:  int(total),
		Limit:  limit,
		Offset: offset,
		Data:   data,
	}, nil
}
