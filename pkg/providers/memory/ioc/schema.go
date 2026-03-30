package iocmemoryprovider

import (
	"encoding/json"
	"errors"
	"fmt"

	//"github.com/cisco-eti/ioc-cfn-svc/pkg/tools/logger"
	"github.com/google/uuid"
)

//var log = logger.Default()

// ResponseStatus represents response status values used across knowledge graph endpoints
type ResponseStatus string

const (
	ResponseStatusSuccess         ResponseStatus = "success"
	ResponseStatusFailure         ResponseStatus = "failure"
	ResponseStatusValidationError ResponseStatus = "validation error"
	ResponseStatusNotFound        ResponseStatus = "not found"
)

///////////////////////// GRAPH SCHEMA /////////////////////////

// Query type constants
const (
	QueryTypeNeighbour = "neighbour"
	QueryTypePath      = "path"
	QueryTypeConcept   = "concept"
)

// Memory type constants
const (
	MemoryTypeSemantic   = "Semantic"
	MemoryTypeProcedural = "Procedural"
	MemoryTypeEpisodic   = "Episodic"
)

// EmbeddingConfig represents configuration for embeddings in the store
type EmbeddingConfig struct {
	Name string    `json:"name" description:"Name of the embedding model (e.g., huggingface model name)"`
	Data []float64 `json:"data" description:"Embedding vector data"`
}

// Concept represents a concept in the knowledge graph
type Concept struct {
	ID          string                 `json:"id" description:"Unique identifier for the concept"`
	Name        string                 `json:"name" description:"Name of the concept"`
	Description *string                `json:"description,omitempty" description:"Detailed description of the concept"`
	Attributes  map[string]interface{} `json:"attributes,omitempty" description:"Additional attributes for the concept"`
	Embeddings  *EmbeddingConfig       `json:"embeddings,omitempty" description:"Embedding configuration for the concept"`
	Tags        []string               `json:"tags,omitempty" description:"Optional list of tags for categorization"`
}

// Relation represents a relationship between concepts
type Relation struct {
	ID         string                 `json:"id" description:"Unique identifier for the relation"`
	Relation   string                 `json:"relation" description:"Type of relationship between nodes"`
	NodeIDs    []string               `json:"node_ids" description:"List of node IDs this relation connects (minimum 2)"`
	Attributes map[string]interface{} `json:"attributes,omitempty" description:"Additional attributes for the relation"`
	Embeddings *EmbeddingConfig       `json:"embeddings,omitempty" description:"Embedding configuration for the relation"`
}

// Validate validates that a relation connects at least 2 nodes
func (r *Relation) Validate() error {
	if len(r.NodeIDs) < 2 {
		return errors.New("a relation must connect at least 2 nodes")
	}
	return nil
}

// Records represents the records structure containing concepts and relations
type Records struct {
	Concepts  []Concept  `json:"concepts,omitempty"`
	Relations []Relation `json:"relations"`
}

// KnowledgeGraphStoreRequest represents a request to the Store for storing and managing knowledge graph data
type KnowledgeGraphStoreRequest struct {
	RequestID    string   `json:"request_id" description:"Auto-generated UUID for request tracking"`
	Records      *Records `json:"records,omitempty" description:"Dictionary containing concepts and relations"`
	MemoryType   *string  `json:"memory_type,omitempty" description:"Type of memory being stored"`
	MasID        *string  `json:"mas_id,omitempty" description:"ID for the Multi-Agent System (Not required for Global Knowledge)"`
	WkspID       *string  `json:"wksp_id,omitempty" description:"ID for the Multi-Agent System Workspace"`
	ForceReplace bool     `json:"force_replace" description:"Force replace existing nodes and edges"`
}

// NewKnowledgeGraphStoreRequest creates a new store request with auto-generated UUID
func NewKnowledgeGraphStoreRequest() *KnowledgeGraphStoreRequest {
	return &KnowledgeGraphStoreRequest{
		RequestID:    uuid.New().String(),
		ForceReplace: false,
	}
}

// Validate validates the store request
func (k *KnowledgeGraphStoreRequest) Validate() error {
	// Validate that either mas_id or wksp_id is provided
	if (k.MasID == nil || *k.MasID == "") && (k.WkspID == nil || *k.WkspID == "") {
		return errors.New("either 'mas_id' or 'wksp_id' or both must be provided")
	}

	if k.Records == nil {
		return nil
	}

	// Get all concept IDs for reference
	conceptIDs := make(map[string]bool)
	for _, concept := range k.Records.Concepts {
		conceptIDs[concept.ID] = true
	}

	// Validate that all node_ids in relations exist in concepts
	for _, relation := range k.Records.Relations {
		if err := relation.Validate(); err != nil {
			return fmt.Errorf("relation %s validation failed: %w", relation.ID, err)
		}

		// Validate that edges only contain nodes specified in this request's nodes
		for _, nodeID := range relation.NodeIDs {
			if !conceptIDs[nodeID] {
				return fmt.Errorf("relation %s references non-existent node ID '%s'. Node IDs must be present in the 'concepts' list", relation.ID, nodeID)
			}
		}
	}

	return nil
}

// KnowledgeGraphStoreResponse represents a response from the Store after storing and managing knowledge graph data
type KnowledgeGraphStoreResponse struct {
	RequestID *string        `json:"request_id,omitempty" description:"UUID for request tracking"`
	Status    ResponseStatus `json:"status" description:"Status of the request"`
	Message   *string        `json:"message,omitempty" description:"Optional message providing additional information"`
}

// MarshalJSON custom marshaling
func (k *KnowledgeGraphStoreResponse) MarshalJSON() ([]byte, error) {
	type Alias KnowledgeGraphStoreResponse
	aux := &struct {
		*Alias
		RequestID *string `json:"request_id,omitempty"`
	}{
		Alias: (*Alias)(k),
	}

	if k.RequestID != nil && *k.RequestID != "" {
		aux.RequestID = k.RequestID
	}

	return json.Marshal(aux)
}

// ConceptRecord represents a concept record for delete operations
type ConceptRecord struct {
	ID   string `json:"id,omitempty" description:"Unique identifier for the concept"`
	Name string `json:"name" description:"Name of the concept"`
}

// DeleteRecords represents the records structure for delete operations
type DeleteRecords struct {
	Concepts []ConceptRecord `json:"concepts"`
}

// KnowledgeGraphDeleteRequest represents a request to delete knowledge graph data
type KnowledgeGraphDeleteRequest struct {
	RequestID string         `json:"request_id" description:"Auto-generated UUID for request tracking"`
	Records   *DeleteRecords `json:"records,omitempty" description:"Dictionary containing concepts"`
	MasID     *string        `json:"mas_id,omitempty" description:"ID for the Multi-Agent System (Not required for Global Knowledge)"`
	WkspID    *string        `json:"wksp_id,omitempty" description:"The workspace ID for the request"`
}

// NewKnowledgeGraphDeleteRequest creates a new delete request with auto-generated UUID
func NewKnowledgeGraphDeleteRequest() *KnowledgeGraphDeleteRequest {
	return &KnowledgeGraphDeleteRequest{
		RequestID: uuid.New().String(),
	}
}

// Validate validates the delete request
func (k *KnowledgeGraphDeleteRequest) Validate() error {
	// Validate that either mas_id or wksp_id is provided
	if (k.MasID == nil || *k.MasID == "") && (k.WkspID == nil || *k.WkspID == "") {
		return errors.New("either 'mas_id' or 'wksp_id' or both must be provided")
	}
	return nil
}

// KnowledgeGraphDeleteResponse represents a response from the Store after deleting knowledge graph data
type KnowledgeGraphDeleteResponse struct {
	RequestID *string        `json:"request_id,omitempty" description:"UUID for request tracking"`
	Status    ResponseStatus `json:"status" description:"Status of the request"`
	Message   *string        `json:"message,omitempty" description:"Optional message providing additional information"`
}

// MarshalJSON custom marshaling
func (k *KnowledgeGraphDeleteResponse) MarshalJSON() ([]byte, error) {
	type Alias KnowledgeGraphDeleteResponse
	aux := &struct {
		*Alias
		RequestID *string `json:"request_id,omitempty"`
	}{
		Alias: (*Alias)(k),
	}

	if k.RequestID != nil && *k.RequestID != "" {
		aux.RequestID = k.RequestID
	}

	return json.Marshal(aux)
}

// KnowledgeGraphQueryCriteria represents query criteria for knowledge graph queries
type KnowledgeGraphQueryCriteria struct {
	Depth        *int   `json:"depth,omitempty" description:"Depth of the query (number of hops) to be used for path queries"`
	UseDirection *bool  `json:"use_direction,omitempty" description:"Whether to use directed relationships in path queries"`
	QueryType    string `json:"query_type" description:"Type of query to execute"`
}

// NewKnowledgeGraphQueryCriteria creates new query criteria with specified values
func NewKnowledgeGraphQueryCriteria(queryType string, depth *int, useDirection *bool) *KnowledgeGraphQueryCriteria {
	return &KnowledgeGraphQueryCriteria{
		Depth:        depth,
		UseDirection: useDirection,
		QueryType:    queryType,
	}
}

// QueryRecords represents the records structure for query operations
type QueryRecords struct {
	Concepts []ConceptRecord `json:"concepts"`
}

// KnowledgeGraphQueryRequest represents a request to query the store
type KnowledgeGraphQueryRequest struct {
	RequestID     string                       `json:"request_id" description:"Auto-generated UUID for request tracking"`
	Records       QueryRecords                 `json:"records" description:"Dictionary containing 'concepts' keys"`
	MemoryType    *string                      `json:"memory_type,omitempty" description:"Memory type"`
	MasID         *string                      `json:"mas_id,omitempty" description:"ID for the Multi-Agent System"`
	WkspID        *string                      `json:"wksp_id,omitempty" description:"ID for the Workspace"`
	QueryCriteria *KnowledgeGraphQueryCriteria `json:"query_criteria,omitempty" description:"Query criteria"`
}

// NewKnowledgeGraphQueryRequest creates a new query request with auto-generated UUID
func NewKnowledgeGraphQueryRequest(queryCriteria *KnowledgeGraphQueryCriteria) *KnowledgeGraphQueryRequest {
	return &KnowledgeGraphQueryRequest{
		RequestID:     uuid.New().String(),
		QueryCriteria: queryCriteria,
	}
}

// Validate validates the query request
func (k *KnowledgeGraphQueryRequest) Validate() error {
	// Validate that either mas_id or wksp_id is provided
	if (k.MasID == nil || *k.MasID == "") && (k.WkspID == nil || *k.WkspID == "") {
		return errors.New("either 'mas_id' or 'wksp_id' or both must be provided")
	}

	conceptsCount := len(k.Records.Concepts)
	queryType := QueryTypeNeighbour
	if k.QueryCriteria != nil {
		queryType = k.QueryCriteria.QueryType
	}

	switch queryType {
	case QueryTypePath:
		if conceptsCount != 2 {
			return errors.New("path queries require exactly 2 concepts (source and destination)")
		}
	case QueryTypeNeighbour:
		if conceptsCount != 1 {
			return errors.New("neighbor queries require exactly 1 concept")
		}
	case QueryTypeConcept:
		if conceptsCount != 1 {
			return errors.New("concept queries require exactly 1 concept")
		}
	default:
		// Default to neighbor query validation
		if conceptsCount != 1 {
			return errors.New("neighbor queries require exactly 1 concept")
		}
	}

	return nil
}

// KnowledgeGraphQueryResponseRecord represents a single record in the query response
type KnowledgeGraphQueryResponseRecord struct {
	Relationships []Relation `json:"relationships"`
	Concepts      []Concept  `json:"concepts"`
}

// KnowledgeGraphQueryResponse represents a response from the Store after querying knowledge graph data
type KnowledgeGraphQueryResponse struct {
	RequestID *string                             `json:"request_id,omitempty" description:"UUID for request tracking"`
	Status    ResponseStatus                      `json:"status" description:"Status of the request"`
	Message   *string                             `json:"message,omitempty" description:"Optional message providing additional information"`
	Records   []KnowledgeGraphQueryResponseRecord `json:"records,omitempty" description:"Query response records (only included for success status)"`
}

// MarshalJSON custom marshaling
func (k *KnowledgeGraphQueryResponse) MarshalJSON() ([]byte, error) {
	type Alias KnowledgeGraphQueryResponse
	aux := &struct {
		*Alias
		RequestID *string                             `json:"request_id,omitempty"`
		Records   []KnowledgeGraphQueryResponseRecord `json:"records,omitempty"`
	}{
		Alias: (*Alias)(k),
	}

	if k.RequestID != nil && *k.RequestID != "" {
		aux.RequestID = k.RequestID
	}

	if len(k.Records) > 0 {
		aux.Records = k.Records
	}

	return json.Marshal(aux)
}

///////////////////////// VECTOR SCHEMA /////////////////////////

// Vector query type constants
const (
	// Internal queries
	QueryTypeInternalListByWkspID = "list_by_wksp_id"
	QueryTypeInternalListByMasID  = "list_by_mas_id"
	// External queries
	QueryTypeGetByID        = "get_by_id"
	QueryTypeDistanceL2     = "distance_l2"
	QueryTypeDistanceCosine = "distance_cosine"
)

// KnowledgeVectorStoreOnboardRequest represents a request to setup the Vector store
type KnowledgeVectorStoreOnboardRequest struct {
	RequestID string `json:"request_id" description:"Auto-generated UUID for request tracking"`
	WkspID    string `json:"wksp_id" description:"ID for the Workspace"`
}

// NewKnowledgeVectorStoreOnboardRequest creates a new onboard request with auto-generated UUID
func NewKnowledgeVectorStoreOnboardRequest(wkspID string) *KnowledgeVectorStoreOnboardRequest {
	return &KnowledgeVectorStoreOnboardRequest{
		RequestID: uuid.New().String(),
		WkspID:    wkspID,
	}
}

// Validate validates the onboard request
func (k *KnowledgeVectorStoreOnboardRequest) Validate() error {
	if k.WkspID == "" {
		return fmt.Errorf("wksp_id is required")
	}
	return nil
}

// KnowledgeVectorStoreOnboardResponse represents a response from the create Container request
type KnowledgeVectorStoreOnboardResponse struct {
	RequestID *string        `json:"request_id,omitempty" description:"UUID for request tracking"`
	Status    ResponseStatus `json:"status" description:"Status of the request"`
	Message   *string        `json:"message,omitempty" description:"Optional message providing additional information"`
}

// MarshalJSON custom marshaling
func (k *KnowledgeVectorStoreOnboardResponse) MarshalJSON() ([]byte, error) {
	type Alias KnowledgeVectorStoreOnboardResponse
	aux := &struct {
		*Alias
	}{
		Alias: (*Alias)(k),
	}

	if k.RequestID != nil && *k.RequestID == "" {
		aux.RequestID = nil
	}

	return json.Marshal(aux)
}

// KnowledgeVectorStoreOnboardDeleteRequest represents a request to delete the Container
type KnowledgeVectorStoreOnboardDeleteRequest struct {
	RequestID string `json:"request_id" description:"Auto-generated UUID for request tracking"`
	WkspID    string `json:"wksp_id" description:"ID for the Workspace"`
}

// NewKnowledgeVectorStoreOnboardDeleteRequest creates a new onboard delete request with auto-generated UUID
func NewKnowledgeVectorStoreOnboardDeleteRequest(wkspID string) *KnowledgeVectorStoreOnboardDeleteRequest {
	return &KnowledgeVectorStoreOnboardDeleteRequest{
		RequestID: uuid.New().String(),
		WkspID:    wkspID,
	}
}

// Validate validates the onboard delete request
func (k *KnowledgeVectorStoreOnboardDeleteRequest) Validate() error {
	if k.WkspID == "" {
		return fmt.Errorf("wksp_id is required")
	}
	return nil
}

// KnowledgeVectorStoreOnboardDeleteResponse represents a response from the delete Container request
type KnowledgeVectorStoreOnboardDeleteResponse struct {
	RequestID *string        `json:"request_id,omitempty" description:"UUID for request tracking"`
	Status    ResponseStatus `json:"status" description:"Status of the request"`
	Message   *string        `json:"message,omitempty" description:"Optional message providing additional information"`
}

// MarshalJSON custom marshaling
func (k *KnowledgeVectorStoreOnboardDeleteResponse) MarshalJSON() ([]byte, error) {
	type Alias KnowledgeVectorStoreOnboardDeleteResponse
	aux := &struct {
		*Alias
	}{
		Alias: (*Alias)(k),
	}

	if k.RequestID != nil && *k.RequestID == "" {
		aux.RequestID = nil
	}

	return json.Marshal(aux)
}

// VectorEmbeddingConfig represents configuration for embeddings in the vector store
type VectorEmbeddingConfig struct {
	Data []float64 `json:"data" description:"Embedding vector data"`
}

// KnowledgeVectorStoreRequestRecord represents a vector in the knowledge vector store
type KnowledgeVectorStoreRequestRecord struct {
	ID        string                 `json:"id" description:"Unique identifier"`
	Content   string                 `json:"content" description:"content in plain text"`
	Embedding *VectorEmbeddingConfig `json:"embedding" description:"Embedding"`
}

// Validate validates the vector store request record
func (k *KnowledgeVectorStoreRequestRecord) Validate() error {
	if k.ID == "" {
		return fmt.Errorf("id is required")
	}
	if k.Content == "" {
		return fmt.Errorf("content is required")
	}
	if k.Embedding == nil {
		return fmt.Errorf("embedding is required")
	}
	return nil
}

// KnowledgeVectorStoreRequest represents a request to the Store for storing and managing knowledge vector data
type KnowledgeVectorStoreRequest struct {
	RequestID string                              `json:"request_id" description:"Auto-generated UUID for request tracking"`
	WkspID    string                              `json:"wksp_id" description:"ID for the Multi-Agent System Workspace"`
	MasID     string                              `json:"mas_id" description:"ID for the Multi-Agent System"`
	Records   []KnowledgeVectorStoreRequestRecord `json:"records" description:"List of vector records"`
}

// NewKnowledgeVectorStoreRequest creates a new vector store request with auto-generated UUID
func NewKnowledgeVectorStoreRequest(wkspID, masID string, records []KnowledgeVectorStoreRequestRecord) *KnowledgeVectorStoreRequest {
	return &KnowledgeVectorStoreRequest{
		RequestID: uuid.New().String(),
		WkspID:    wkspID,
		MasID:     masID,
		Records:   records,
	}
}

// Validate validates the vector store request
func (k *KnowledgeVectorStoreRequest) Validate() error {
	if k.MasID == "" {
		return fmt.Errorf("mas_id is required")
	}
	if k.WkspID == "" {
		return fmt.Errorf("wksp_id is required")
	}

	for i, record := range k.Records {
		if err := record.Validate(); err != nil {
			return fmt.Errorf("record at index %d validation failed: %w", i, err)
		}
	}

	return nil
}

// KnowledgeVectorStoreResponse represents a response from the Store after storing and managing knowledge vector data
type KnowledgeVectorStoreResponse struct {
	RequestID *string        `json:"request_id,omitempty" description:"UUID for request tracking"`
	Status    ResponseStatus `json:"status" description:"Status of the request"`
	Message   *string        `json:"message,omitempty" description:"Optional message providing additional information"`
}

// MarshalJSON custom marshaling
func (k *KnowledgeVectorStoreResponse) MarshalJSON() ([]byte, error) {
	type Alias KnowledgeVectorStoreResponse
	aux := &struct {
		*Alias
	}{
		Alias: (*Alias)(k),
	}

	if k.RequestID != nil && *k.RequestID == "" {
		aux.RequestID = nil
	}

	return json.Marshal(aux)
}

// KnowledgeVectorDeleteRequest represents a request to delete a vector
type KnowledgeVectorDeleteRequest struct {
	RequestID  string `json:"request_id" description:"Auto-generated UUID for request tracking"`
	WkspID     string `json:"wksp_id" description:"The workspace ID for the request"`
	MasID      string `json:"mas_id" description:"ID for the Multi-Agent System"`
	ID         string `json:"id" description:"ID of vector to delete"`
	SoftDelete bool   `json:"soft_delete" description:"Soft delete the vector"`
}

// NewKnowledgeVectorDeleteRequest creates a new vector delete request with auto-generated UUID
func NewKnowledgeVectorDeleteRequest(wkspID, masID, id string, softDelete bool) *KnowledgeVectorDeleteRequest {
	return &KnowledgeVectorDeleteRequest{
		RequestID:  uuid.New().String(),
		WkspID:     wkspID,
		MasID:      masID,
		ID:         id,
		SoftDelete: softDelete,
	}
}

// Validate validates the vector delete request
func (k *KnowledgeVectorDeleteRequest) Validate() error {
	if k.MasID == "" {
		return fmt.Errorf("mas_id is required")
	}
	if k.WkspID == "" {
		return fmt.Errorf("wksp_id is required")
	}
	if k.ID == "" {
		return fmt.Errorf("id is required")
	}
	return nil
}

// KnowledgeVectorDeleteResponse represents a response from the Store after deleting knowledge vector data
type KnowledgeVectorDeleteResponse struct {
	RequestID *string        `json:"request_id,omitempty" description:"UUID for request tracking"`
	Status    ResponseStatus `json:"status" description:"Status of the request"`
	Message   *string        `json:"message,omitempty" description:"Optional message providing additional information"`
}

// MarshalJSON custom marshaling
func (k *KnowledgeVectorDeleteResponse) MarshalJSON() ([]byte, error) {
	type Alias KnowledgeVectorDeleteResponse
	aux := &struct {
		*Alias
	}{
		Alias: (*Alias)(k),
	}

	if k.RequestID != nil && *k.RequestID == "" {
		aux.RequestID = nil
	}

	return json.Marshal(aux)
}

// KnowledgeVectorQueryCriteria represents query criteria for knowledge vector queries
type KnowledgeVectorQueryCriteria struct {
	QueryType *string                `json:"query_type,omitempty" description:"Type of query to execute"`
	ID        *string                `json:"id,omitempty" description:"ID of vector to query"`
	Embedding *VectorEmbeddingConfig `json:"embedding,omitempty" description:"Embedding for query"`
	Limit     *int                   `json:"limit,omitempty" description:"limit used by queries"`
}

// NewKnowledgeVectorQueryCriteria creates new query criteria with specified values
func NewKnowledgeVectorQueryCriteria(queryType string, options ...interface{}) *KnowledgeVectorQueryCriteria {
	criteria := &KnowledgeVectorQueryCriteria{
		QueryType: &queryType,
	}

	// Process optional parameters
	for _, option := range options {
		switch v := option.(type) {
		case *string:
			criteria.ID = v
		case string:
			criteria.ID = &v
		case *VectorEmbeddingConfig:
			criteria.Embedding = v
		case *int:
			criteria.Limit = v
		case int:
			criteria.Limit = &v
		}
	}

	return criteria
}

// KnowledgeVectorQueryRequest represents a request to query the store
type KnowledgeVectorQueryRequest struct {
	RequestID     string                        `json:"request_id" description:"Auto-generated UUID for request tracking"`
	WkspID        string                        `json:"wksp_id" description:"ID for the Workspace"`
	MasID         string                        `json:"mas_id" description:"ID for the Multi-Agent System"`
	QueryCriteria *KnowledgeVectorQueryCriteria `json:"query_criteria,omitempty" description:"Query criteria"`
}

// NewKnowledgeVectorQueryRequest creates a new query request with auto-generated UUID
func NewKnowledgeVectorQueryRequest(wkspID, masID string, queryCriteria *KnowledgeVectorQueryCriteria) *KnowledgeVectorQueryRequest {
	return &KnowledgeVectorQueryRequest{
		RequestID:     uuid.New().String(),
		WkspID:        wkspID,
		MasID:         masID,
		QueryCriteria: queryCriteria,
	}
}

// Validate validates the vector query request
func (k *KnowledgeVectorQueryRequest) Validate() error {
	if k.MasID == "" {
		return fmt.Errorf("mas_id is required")
	}
	if k.WkspID == "" {
		return fmt.Errorf("wksp_id is required")
	}

	if k.QueryCriteria == nil || k.QueryCriteria.QueryType == nil {
		return fmt.Errorf("query_criteria with query_type is required")
	}

	queryType := *k.QueryCriteria.QueryType

	// Similarity search queries require embedding
	if queryType == QueryTypeDistanceL2 || queryType == QueryTypeDistanceCosine {
		if k.QueryCriteria.Embedding == nil || len(k.QueryCriteria.Embedding.Data) == 0 {
			return fmt.Errorf("query type '%s' requires embedding data", queryType)
		}
	}

	// Get by ID query requires id
	if queryType == QueryTypeGetByID {
		if k.QueryCriteria.ID == nil || *k.QueryCriteria.ID == "" {
			return fmt.Errorf("query type '%s' requires id field", queryType)
		}
	}

	// Validate query_type is valid
	validTypes := []string{
		QueryTypeInternalListByWkspID,
		QueryTypeInternalListByMasID,
		QueryTypeGetByID,
		QueryTypeDistanceL2,
		QueryTypeDistanceCosine,
	}

	valid := false
	for _, validType := range validTypes {
		if queryType == validType {
			valid = true
			break
		}
	}

	if !valid {
		return fmt.Errorf("invalid query_type: '%s'", queryType)
	}

	return nil
}

// KnowledgeVectorQueryResponseRecord represents a record in the query response
type KnowledgeVectorQueryResponseRecord struct {
	ID        string                 `json:"id" description:"Unique identifier"`
	Content   string                 `json:"content" description:"content in plain text"`
	Embedding *VectorEmbeddingConfig `json:"embedding" description:"Embedding configuration"`
	Distance  *float64               `json:"distance,omitempty" description:"Distance between query and record"`
	CreatedAt *int64                 `json:"created_at,omitempty" description:"Timestamp of record creation in epoch time"`
	UpdatedAt *int64                 `json:"updated_at,omitempty" description:"Timestamp of record update in epoch time"`
}

// MarshalJSON custom marshaling to exclude None fields
func (k *KnowledgeVectorQueryResponseRecord) MarshalJSON() ([]byte, error) {
	type Alias KnowledgeVectorQueryResponseRecord
	aux := &struct {
		*Alias
	}{
		Alias: (*Alias)(k),
	}

	return json.Marshal(aux)
}

// KnowledgeVectorQueryResponse represents a response from the Store after querying knowledge vector data
type KnowledgeVectorQueryResponse struct {
	RequestID *string                              `json:"request_id,omitempty" description:"UUID for request tracking"`
	Status    ResponseStatus                       `json:"status" description:"Status of the request"`
	Message   *string                              `json:"message,omitempty" description:"Optional message providing additional information"`
	Records   []KnowledgeVectorQueryResponseRecord `json:"records,omitempty" description:"Query response records (only included for success status)"`
}

// MarshalJSON custom marshaling
func (k *KnowledgeVectorQueryResponse) MarshalJSON() ([]byte, error) {
	type Alias KnowledgeVectorQueryResponse
	aux := &struct {
		*Alias
	}{
		Alias: (*Alias)(k),
	}

	if k.RequestID != nil && *k.RequestID == "" {
		aux.RequestID = nil
	}

	return json.Marshal(aux)
}
