// Copyright 2026 Cisco Systems, Inc. and its affiliates
//
// SPDX-License-Identifier: Apache-2.0

package app

import (
	"net/http"

	"github.com/outshift-open/ioc-cfn-svc/pkg/tools/easyhttp"
	httpSwagger "github.com/swaggo/http-swagger"
)

// docsFS serves static files from the docs/ directory.
var docsFS = http.FileServer(http.Dir("docs"))

const (
	apiPrefix      = "/api"
	internalPrefix = "/api/internal"
)

func (a *App) initializeRoutes() http.Handler {
	log := getLogger()

	rtr := easyhttp.NewRouter()

	// custom middleware - log all incoming requests
	rtr.Use(func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			log.Infof("incoming request: %s %s", r.Method, r.URL.Path)
			h.ServeHTTP(w, r)
		})
	})

	// standard diagnostic endpoints
	rtr.Get(internalPrefix+"/diagnostics/health", a.diagnosticsHealthHandler)
	rtr.Get(internalPrefix+"/diagnostics/info", a.diagnosticsInfoHandler)
	rtr.Get(internalPrefix+"/diagnostics/metrics", a.diagnosticsMetricsHandler)
	rtr.Get(internalPrefix+"/diagnostics/loggers", a.diagnosticsLoggersHandler)
	rtr.Put(internalPrefix+"/diagnostics/loggers", a.diagnosticsSetLoggersHandler)

	// shared memories (consumed by mgmt-plane-svc)
	rtr.Post(internalPrefix+"/workspaces/{workspaceId}/multi-agentic-systems/{masId}/shared-memories/vector-store", withWorkspaceAndMasValidation(a.onboardSharedMemoriesVectorStoreHandler))
	rtr.Delete(internalPrefix+"/workspaces/{workspaceId}/shared-memories/vector-store/{store_id}", withWorkspaceValidation(a.deleteSharedMemoriesVectorStoreHandler))

	// shared memories (northbound APIs, consumed by MAS clients)
	rtr.Post(apiPrefix+"/workspaces/{workspaceId}/multi-agentic-systems/{masId}/shared-memories", withWorkspaceAndMasValidation(a.createOrUpdateSharedMemoriesHandler))
	rtr.Post(apiPrefix+"/workspaces/{workspaceId}/multi-agentic-systems/{masId}/shared-memories/query", withWorkspaceAndMasValidation(a.fetchSharedMemoriesHandler))

	// graph DB APIs (eastbound APIs, consumed by Evidence Gathering engine)
	rtr.Get(internalPrefix+"/workspaces/{workspaceId}/multi-agentic-systems/{masId}/graph/neighbors/{conceptId}", withWorkspaceAndMasValidation(a.getNeighborsByIdHandler))
	rtr.Post(internalPrefix+"/workspaces/{workspaceId}/multi-agentic-systems/{masId}/graph/concepts/by_ids", withWorkspaceAndMasValidation(a.fetchConceptsByIdsHandler))
	rtr.Post(internalPrefix+"/workspaces/{workspaceId}/multi-agentic-systems/{masId}/graph/paths", withWorkspaceAndMasValidation(a.fetchPathsByIdsHandler))
	rtr.Put(internalPrefix+"/workspaces/{workspaceId}/multi-agentic-systems/{masId}/graph/update", withWorkspaceAndMasValidation(a.updateGraphHandler))
	rtr.Post(internalPrefix+"/workspaces/{workspaceId}/multi-agentic-systems/{masId}/graph/distillation/read", withWorkspaceAndMasValidation(a.distillationGraphHandler))
	// similarity search (eastbound APIs, consumed by Evidence Gathering engine)
	rtr.Post(internalPrefix+"/workspaces/{workspaceId}/multi-agentic-systems/{masId}/concepts/similarity-search", withWorkspaceAndMasValidation(a.conceptSimilaritySearchHandler))
	rtr.Post(internalPrefix+"/workspaces/{workspaceId}/multi-agentic-systems/{masId}/rag/similarity-search", withWorkspaceAndMasValidation(a.vectorSimilaritySearchHandler))

	// semantic alignment
	rtr.Post(apiPrefix+"/workspaces/{workspaceId}/multi-agentic-systems/{masId}/semantic-alignment/start", withWorkspaceAndMasValidation(a.startSemanticAlignmentHandler))
	rtr.Post(apiPrefix+"/workspaces/{workspaceId}/multi-agentic-systems/{masId}/semantic-alignment/decide", withWorkspaceAndMasValidation(a.decideSemanticAlignmentHandler))

	// remote agent memory operations
	rtr.Post(apiPrefix+"/workspaces/{workspaceId}/multi-agentic-systems/{masId}/agents/{agentId}/memory-operations", withWorkspaceAndMasValidation(a.memoryOperationsHandler))

	// agent-level vector store operations (northbound APIs, consumed by MAS clients)
	rtr.Post(apiPrefix+"/workspaces/{workspaceId}/multi-agentic-systems/{masId}/agents/{agentId}/rag/vectors", withWorkspaceAndMasValidation(a.agentVectorUpsertHandler))
	rtr.Delete(apiPrefix+"/workspaces/{workspaceId}/multi-agentic-systems/{masId}/agents/{agentId}/rag/vectors", withWorkspaceAndMasValidation(a.agentVectorDeleteHandler))
	rtr.Post(apiPrefix+"/workspaces/{workspaceId}/multi-agentic-systems/{masId}/agents/{agentId}/rag/similarity-search", withWorkspaceAndMasValidation(a.agentVectorSimilaritySearchHandler))

	rtr.Post(apiPrefix+"/internal/cognition-fabric-node/{cfnId}/shared-memories/vectors", a.cognitionAgentsSharedMemoriesVectorsUpsertHandler)
	rtr.Post(apiPrefix+"/internal/cognition-fabric-node/{cfnId}/shared-memories/vectors/search", a.cognitionAgentsSharedMemoriesVectorsSearchHandler)

	// task scheduler callback (internal API)
	rtr.Post(internalPrefix+"/tasks/callback", a.handleTaskCallback)

	// audit events (internal API)
	rtr.Get(internalPrefix+"/mgmt/audit", a.listAuditEventsHandler)
	rtr.Get(internalPrefix+"/mgmt/audit/{eventId}", a.getAuditEventHandler)

	// knowledge graph (internal API)
	rtr.Get(internalPrefix+"/mgmt/workspaces/{workspaceId}/multi-agentic-systems/{masId}/knowledge-graph", a.fetchKnowledgeGraphHandler)

	// L9 protocol endpoint (MAS <-> CFN <-> CE communication)
	// Content-based routing: CE is selected based on message kind/subkind/subprotocol
	// Workspace/MAS extracted from L9 message participants.groups
	// L9 audit events are recorded to the existing audit table with L9_* types
	// (use GET /api/internal/mgmt/audit?audit_type=L9_COMMIT to query)
	rtr.Post(apiPrefix+"/l9/messages", a.l9Handler)

	// OTLP trace ingestion (canonical route follows API guidelines)
	rtr.Post(apiPrefix+"/traces", a.otelReceiver.HandleTraces)
	// /v1/traces is kept as an alias: the otel-trace plugin hardcodes
	// this suffix onto whatever base endpoint is configured, so we cannot change it without
	// a plugin-side code change.
	rtr.Post("/v1/traces", a.otelReceiver.HandleTraces)

	// metrics API - MAS-scoped query (internal, consumed by mgmt-plane)
	// GET: Query token usage metrics for a MAS across all attached CEs (optional ce_id filter)
	rtr.Get(internalPrefix+"/mgmt/workspaces/{workspaceId}/multi-agentic-systems/{masId}/metrics", a.getMASMetricsHandler)

	// metrics API - Cognition Engine integration
	// POST: CE pushes infrastructure metrics (queue depth, memory, CPU, etc.)
	// GET: Query CE infrastructure + MAS operations for a specific CE
	rtr.Post(apiPrefix+"/cognition-engines/{ceId}/metrics", a.ingestCEMetricsHandler)
	rtr.Get(apiPrefix+"/cognition-engines/{ceId}/metrics", a.getMetricsHandler)

	// Cognition Engine management proxy endpoints
	rtr.Post(apiPrefix+"/cognition-engines", a.registerCognitionEngineHandler)
	rtr.Get(apiPrefix+"/cognition-engines", a.listCognitionEnginesHandler)
	rtr.Get(apiPrefix+"/cognition-engines/{ceId}", a.getCognitionEngineHandler)
	rtr.Patch(apiPrefix+"/cognition-engines/{ceId}", a.patchCognitionEngineHandler)
	rtr.Put(apiPrefix+"/cognition-engines/{ceId}/heartbeat", a.cognitionEngineHeartbeatHandler)
	rtr.Delete(apiPrefix+"/cognition-engines/{ceId}", a.deleteCognitionEngineHandler)

	// Public Swagger UI — points to post-split swagger.json (public endpoints only)
	rtr.HandleHTTP("/docs/swagger.json", http.StripPrefix("/docs/", docsFS))
	rtr.HandleHTTP("/docs/", httpSwagger.Handler(
		httpSwagger.URL("/docs/swagger.json"),
	))

	// Internal Swagger UI — points to swagger-internal.json (internal endpoints only)
	rtr.HandleHTTP("/docs/internal/swagger-internal.json", http.StripPrefix("/docs/internal/", docsFS))
	rtr.HandleHTTP("/docs/internal/", httpSwagger.Handler(
		httpSwagger.URL("/docs/internal/swagger-internal.json"),
	))

	return rtr
}
