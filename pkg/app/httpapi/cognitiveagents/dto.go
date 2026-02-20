// Package cognitiveagents provides DTOs for the Cognitive Agents API handler.
//
// NOTE: Struct fields and JSON tags may change as the API evolves.
package cognitiveagents

// MemoryQueryRequest is the request body for POST /api/memory/.
// TODO: Fields and JSON tags may change based on core logic implementation and final API route.
type MemoryQueryRequest struct {
	MASID       string    `json:"mas_id"`       // Multi-Agent System identifier
	WorkspaceID string    `json:"workspace_id"` // Workspace identifier
	Queries     []string  `json:"queries"`      // Natural-language queries
	Embedding   []float64 `json:"embedding"`    // Query embedding vector
	K           int       `json:"k"`            // Number of results to return
}

// MemoryQueryResponse is the response body for POST /api/memory/.
// TODO: Fields and JSON tags may change based on core logic implementation and final API route.
type MemoryQueryResponse struct {
	Results []QueryResult `json:"results"`
}

// QueryResult holds the results for a single query.
// TODO: Hits structure may change based on core logic implementation.
type QueryResult struct {
	Query string                   `json:"query"`
	Hits  []map[string]interface{} `json:"hits"`
}
