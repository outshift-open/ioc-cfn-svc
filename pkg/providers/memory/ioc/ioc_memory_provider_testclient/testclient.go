// Copyright 2026 Cisco Systems, Inc. and its affiliates
//
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"fmt"
	"os"

	"github.com/joho/godotenv"
	"go.uber.org/zap"

	iocmemoryprovider "github.com/outshift-open/ioc-cfn-svc/pkg/providers/memory/ioc"
	"github.com/outshift-open/ioc-cfn-svc/pkg/tools/logger"
)

// Sample client using the IOC Memory Provider
// This sample shows how to use the IOC Memory Provider to perform knowledge graph operations
// using schema types.

var log *zap.SugaredLogger

func main() {
	// Initialize logger first
	logger.Init()
	log = logger.Default()

	log.Info("Starting IOC Memory Provider Client Sample...")

	// Load environment variables from .env file
	if err := godotenv.Load(); err != nil {
		log.Infof("No .env file found or error loading .env file: %v", err)
	} else {
		log.Info("Loaded environment variables from .env file")
	}

	// Read environment variable for service URL
	serviceURL := os.Getenv("KNOWLEDGE_MEMORY_SVC_URL")
	if serviceURL != "" {
		log.Infof("Using Knowledge Memory Service URL from environment: %s", serviceURL)
	} else {
		log.Info("Using default Knowledge Memory Service URL")
	}

	// Create client
	client, err := iocmemoryprovider.NewClient(serviceURL)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}

	log.Info("Client established successfully")
	ctx := context.Background()

	// Test UpsertKnowledgeGraph method
	printTestSeparator()
	log.Info("Testing UpsertKnowledgeGraph...")
	if err := testUpsertKnowledgeGraph(ctx, client); err != nil {
		log.Errorf("Error in UpsertKnowledgeGraph: %v", err)
		os.Exit(1)
	}

	// Test UpsertKnowledgeGraph with InternalAttributes
	printTestSeparator()
	log.Info("Testing UpsertKnowledgeGraph with InternalAttributes...")
	if err := testUpsertKnowledgeGraphWithInternalAttributes(ctx, client); err != nil {
		log.Errorf("Error in UpsertKnowledgeGraph with InternalAttributes: %v", err)
		os.Exit(1)
	}

	// Test QueryKnowledgeGraph Relations with InternalAttributes filter
	printTestSeparator()
	log.Info("Testing QueryKnowledgeGraph Relations for InternalAttributes...")
	if err := testQueryKnowledgeGraphRelationsWithInternalAttributes(ctx, client); err != nil {
		log.Errorf("Error in QueryKnowledgeGraph Relations for InternalAttributes: %v", err)
		os.Exit(1)
	}

	// Test QueryKnowledgeGraph Concepts with InternalAttributes filter
	printTestSeparator()
	log.Info("Testing QueryKnowledgeGraph Concepts for InternalAttributes...")
	if err := testQueryKnowledgeGraphConceptsWithInternalAttributes(ctx, client); err != nil {
		log.Errorf("Error in QueryKnowledgeGraph Concepts for InternalAttributes: %v", err)
		os.Exit(1)
	}

	// Test QueryKnowledgeGraph Concepts with regular filters
	printTestSeparator()
	log.Info("Testing QueryKnowledgeGraph Concepts with Filters...")
	if err := testQueryKnowledgeGraphConceptsWithDynamicFilters(ctx, client); err != nil {
		log.Errorf("Error in QueryKnowledgeGraph Concepts with Filters: %v", err)
		os.Exit(1)
	}

	// Test QueryKnowledgeGraph Concepts with custom filters
	printTestSeparator()
	log.Info("Testing QueryKnowledgeGraph Concepts with Custom Filters...")
	if err := testQueryKnowledgeGraphConceptsWithCustomFilters(ctx, client); err != nil {
		log.Errorf("Error in QueryKnowledgeGraph Concepts with Custom Filters: %v", err)
		os.Exit(1)
	}

	// Test QueryKnowledgeGraph Relations with regular filters
	printTestSeparator()
	log.Info("Testing QueryKnowledgeGraph Relations with Filters...")
	if err := testQueryKnowledgeGraphRelationsWithDynamicFilters(ctx, client); err != nil {
		log.Errorf("Error in QueryKnowledgeGraph Relations with Filters: %v", err)
		os.Exit(1)
	}

	// Test QueryKnowledgeGraph Relations with custom filters
	printTestSeparator()
	log.Info("Testing QueryKnowledgeGraph Relations with Custom Filters...")
	if err := testQueryKnowledgeGraphConceptsWithCustomFilters(ctx, client); err != nil {
		log.Errorf("Error in QueryKnowledgeGraph Concepts with Custom Filters: %v", err)
		os.Exit(1)
	}

	// Test QueryKnowledgeGraphPath method
	printTestSeparator()
	log.Info("Testing QueryKnowledgeGraphPath...")
	if err := testQueryKnowledgeGraphPath(ctx, client); err != nil {
		log.Errorf("Error in QueryKnowledgeGraphPath: %v", err)
		os.Exit(1)
	}

	// Test QueryKnowledgeGraphNeighbor method
	printTestSeparator()
	log.Info("Testing QueryKnowledgeGraphNeighbor...")
	if err := testQueryKnowledgeGraphNeighbor(ctx, client); err != nil {
		log.Errorf("Error in QueryKnowledgeGraphNeighbor: %v", err)
		os.Exit(1)
	}

	// Test QueryKnowledgeGraphConcept method
	printTestSeparator()
	log.Info("Testing QueryKnowledgeGraphConcept...")
	if err := testQueryKnowledgeGraphConcept(ctx, client); err != nil {
		log.Errorf("Error in QueryKnowledgeGraphConcept: %v", err)
		os.Exit(1)
	}

	// Test DeleteKnowledgeGraph method
	printTestSeparator()
	log.Info("Testing DeleteKnowledgeGraph...")
	if err := testDeleteKnowledgeGraph(ctx, client); err != nil {
		log.Errorf("Error in DeleteKnowledgeGraph: %v", err)
		os.Exit(1)
	}

	// Test vector operations
	printTestSeparator()
	log.Info("\n=== Testing Vector Operations ===")

	// Test OnboardKnowledgeVectorStore method
	printTestSeparator()
	log.Info("Testing OnboardKnowledgeVectorStore...")
	if err := testOnboardKnowledgeVectorStore(ctx, client); err != nil {
		log.Errorf("Error in OnboardKnowledgeVectorStore: %v", err)
		os.Exit(1)
	}

	// Test UpsertKnowledgeVectors method
	printTestSeparator()
	log.Info("Testing UpsertKnowledgeVectors...")
	if err := testUpsertKnowledgeVectors(ctx, client); err != nil {
		log.Errorf("Error in UpsertKnowledgeVectors: %v", err)
		os.Exit(1)
	}

	// Test QueryKnowledgeVectors method (Cosine)
	printTestSeparator()
	log.Info("Testing QueryKnowledgeVectors (Cosine Distance)...")
	if err := testQueryKnowledgeVectorsCosine(ctx, client); err != nil {
		log.Errorf("Error in QueryKnowledgeVectors (Cosine): %v", err)
		os.Exit(1)
	}

	// Test QueryKnowledgeVectors method (L2)
	printTestSeparator()
	log.Info("Testing QueryKnowledgeVectors (L2 Distance)...")
	if err := testQueryKnowledgeVectorsL2(ctx, client); err != nil {
		log.Errorf("Error in QueryKnowledgeVectors (L2): %v", err)
		os.Exit(1)
	}

	// Test QueryKnowledgeVectors method (Get By ID)
	printTestSeparator()
	log.Info("Testing QueryKnowledgeVectors (Get By ID)...")
	if err := testQueryKnowledgeVectorsGetByID(ctx, client); err != nil {
		log.Errorf("Error in QueryKnowledgeVectors (Get By ID): %v", err)
		os.Exit(1)
	}

	// Test DeleteKnowledgeVectors method
	printTestSeparator()
	log.Info("Testing DeleteKnowledgeVectors...")
	if err := testDeleteKnowledgeVectors(ctx, client); err != nil {
		log.Errorf("Error in DeleteKnowledgeVectors: %v", err)
		os.Exit(1)
	}

	// Test DeleteKnowledgeVectorStore method
	printTestSeparator()
	log.Info("Testing DeleteKnowledgeVectorStore...")
	if err := testDeleteKnowledgeVectorStore(ctx, client); err != nil {
		log.Errorf("Error in DeleteKnowledgeVectorStore: %v", err)
		os.Exit(1)
	}

	log.Info("Sample completed successfully!")
}

func testUpsertKnowledgeGraph(ctx context.Context, client *iocmemoryprovider.Client) error {
	// Create request using schema types
	request := iocmemoryprovider.NewKnowledgeGraphStoreRequest()

	// Set workspace and MAS IDs
	masID := "523e4567-e89b-12d3-a456-426614174000"
	wkspID := "11111111-e89b-12d3-a456-426614174000"
	request.MasID = &masID
	request.WkspID = &wkspID
	request.ForceReplace = true

	// Create concepts using schema types
	concepts := []iocmemoryprovider.Concept{
		{
			ID:          "923e4567-e89b-12d3-a456-426614174000",
			Name:        "New Test Artificial Intelligence",
			Description: stringPtr("The simulation of human intelligence processes by machines"),
			Attributes: map[string]interface{}{
				"category":     "Technology",
				"founded_year": 1956,
			},
			Embeddings: &iocmemoryprovider.EmbeddingConfig{
				Name: "text-embedding-ada-002",
				Data: []float64{0.1, 0.2, 0.3, 0.4, 0.5},
			},
		},
		{
			ID:          "923e4567-e89b-12d3-a456-426614174001",
			Name:        "New Machine Learning",
			Description: stringPtr("A subset of AI that enables systems to learn from data"),
			Attributes: map[string]interface{}{
				"category":     "Computer Science",
				"parent_field": "AI",
			},
			Embeddings: &iocmemoryprovider.EmbeddingConfig{
				Name: "text-embedding-ada-002",
				Data: []float64{0.2, 0.3, 0.4, 0.5, 0.6},
			},
		},
		{
			ID:          "923e4567-e89b-12d3-a456-426614174002",
			Name:        "Deep Learning",
			Description: stringPtr("A subset of ML using neural networks with multiple layers"),
			Attributes: map[string]interface{}{
				"category":     "Neural networks",
				"parent_field": "Machine Learning",
			},
			Embeddings: &iocmemoryprovider.EmbeddingConfig{
				Name: "text-embedding-ada-002",
				Data: []float64{0.3, 0.4, 0.5, 0.6, 0.7},
			},
		},
	}

	// Create relations using schema types
	relations := []iocmemoryprovider.Relation{
		{
			ID:       "823e4567-e89b-12d3-a456-426614174000",
			Relation: "HAS_A_SUBFIELD",
			NodeIDs: []string{
				"923e4567-e89b-12d3-a456-426614174000",
				"923e4567-e89b-12d3-a456-426614174001",
			},
			Attributes: map[string]interface{}{
				"since":    1956,
				"strength": 0.9,
			},
			Embeddings: &iocmemoryprovider.EmbeddingConfig{
				Name: "relation-embedding",
				Data: []float64{0.15, 0.25, 0.35, 0.45, 0.55},
			},
		},
		{
			ID:       "723e4567-e89b-12d3-a456-426614174000",
			Relation: "HAS_SUBFIELD",
			NodeIDs: []string{
				"923e4567-e89b-12d3-a456-426614174001",
				"923e4567-e89b-12d3-a456-426614174002",
			},
			Attributes: map[string]interface{}{
				"since":    1980,
				"strength": 0.95,
			},
		},
		{
			ID:       "623e4567-e89b-12d3-a456-426614174000",
			Relation: "RELATED_TO",
			NodeIDs: []string{
				"923e4567-e89b-12d3-a456-426614174000",
				"923e4567-e89b-12d3-a456-426614174002",
			},
			Attributes: map[string]interface{}{
				"relationship": "hierarchical",
				"direct":       "False",
			},
		},
	}

	// Set records
	request.Records = &iocmemoryprovider.Records{
		Concepts:  concepts,
		Relations: relations,
	}

	// Call the client method
	response, err := client.UpsertKnowledgeGraph(ctx, request)
	if err != nil {
		return err
	}

	log.Infof("Upsert response status: %s", response.Status)
	if response.Message != nil {
		log.Infof("Upsert response message: %s", *response.Message)
	}

	return nil
}

func testUpsertKnowledgeGraphWithInternalAttributes(ctx context.Context, client *iocmemoryprovider.Client) error {
	// Create request using schema types
	request := iocmemoryprovider.NewKnowledgeGraphStoreRequest()

	// Set workspace and MAS IDs (same as other graph tests)
	masID := "523e4567-e89b-12d3-a456-426614174000"
	wkspID := "11111111-e89b-12d3-a456-426614174000"
	request.MasID = &masID
	request.WkspID = &wkspID
	request.ForceReplace = true
	request.IncrementalUpdate = true

	// Create concepts with internal attributes
	concepts := []iocmemoryprovider.Concept{
		{
			ID:          "concept-with-internal-attrs-001",
			Name:        "AI Concept with Internal Attributes",
			Description: stringPtr("A concept demonstrating internal attributes functionality"),
			Attributes: map[string]interface{}{
				"category": "AI",
				"type":     "concept",
			},
			InternalAttributes: []iocmemoryprovider.InternalAttributes{
				{
					Owner: "123e4567-e89b-12d3-a456-426614174001",
					Attributes: map[string]interface{}{
						"distill_status": "completed",
					},
				},
			},
		},
	}

	// Create relations with internal attributes
	relations := []iocmemoryprovider.Relation{
		{
			ID:       "relation-with-internal-attrs-001",
			Relation: "REFERENCES",
			NodeIDs: []string{
				"concept-with-internal-attrs-001",
				"923e4567-e89b-12d3-a456-426614174002", // Reference existing concept
			},
			Attributes: map[string]interface{}{
				"relationship": "references",
				"strength":     0.8,
			},
			InternalAttributes: []iocmemoryprovider.InternalAttributes{
				{
					Owner: "123e4567-e89b-12d3-a456-426614174000",
					Attributes: map[string]interface{}{
						"session_time": 1672531207,
					},
				},
			},
		},
	}

	// Set records
	request.Records = &iocmemoryprovider.Records{
		Concepts:  concepts,
		Relations: relations,
	}

	// Call the client method
	response, err := client.UpsertKnowledgeGraph(ctx, request)
	if err != nil {
		return err
	}

	log.Infof("Upsert with InternalAttributes response status: %s", response.Status)
	if response.Message != nil {
		log.Infof("Upsert with InternalAttributes response message: %s", *response.Message)
	}

	return nil
}

func testQueryKnowledgeGraphRelationsWithInternalAttributes(ctx context.Context, client *iocmemoryprovider.Client) error {
	// Create filter for session_time range
	owner := "123e4567-e89b-12d3-a456-426614174000"
	filters := []iocmemoryprovider.KnowledgeGraphQueryCriteriaFilter{
		{
			Category:  "internal",
			Key:       "session_time",
			Operation: "range",
			Value:     []interface{}{1672531200, 1672531300}, // Actual array/slice
			Owner:     &owner,
		},
	}

	// Create query criteria for relations query with filters
	queryCriteria := iocmemoryprovider.NewKnowledgeGraphQueryCriteriaWithFilters(
		iocmemoryprovider.QueryTypeRelations,
		nil,
		nil,
		filters,
	)

	// Create request using schema types
	masID := "523e4567-e89b-12d3-a456-426614174000"
	wkspID := "11111111-e89b-12d3-a456-426614174000"
	request := iocmemoryprovider.NewKnowledgeGraphQueryRequest(queryCriteria)

	// Set workspace and MAS IDs
	request.MasID = &masID
	request.WkspID = &wkspID

	// Call the client method (using concept query method which should handle different query types)
	response, err := client.QueryKnowledgeGraphConcept(ctx, request)
	if err != nil {
		return err
	}

	log.Infof("Relations query response status: %s", response.Status)
	if response.Message != nil {
		log.Infof("Relations query response message: %s", *response.Message)
	}
	log.Infof("Relations query response records count: %d", len(response.Records))

	return nil
}

func testQueryKnowledgeGraphConceptsWithInternalAttributes(ctx context.Context, client *iocmemoryprovider.Client) error {
	// Create filter for distill_status
	owner := "123e4567-e89b-12d3-a456-426614174001"
	filters := []iocmemoryprovider.KnowledgeGraphQueryCriteriaFilter{
		{
			Category:  "internal",
			Key:       "distill_status",
			Operation: "eqstr",
			Value:     []interface{}{"completed"}, // Single string value in slice
			Owner:     &owner,
		},
	}

	// Create query criteria for concepts query with filters
	queryCriteria := iocmemoryprovider.NewKnowledgeGraphQueryCriteriaWithFilters(
		iocmemoryprovider.QueryTypeConcepts,
		nil,
		nil,
		filters,
	)

	// Create request using schema types
	masID := "523e4567-e89b-12d3-a456-426614174000"
	wkspID := "11111111-e89b-12d3-a456-426614174000"
	request := iocmemoryprovider.NewKnowledgeGraphQueryRequest(queryCriteria)

	// Set workspace and MAS IDs
	request.MasID = &masID
	request.WkspID = &wkspID

	// Call the client method (using concept query method which should handle different query types)
	response, err := client.QueryKnowledgeGraphConcept(ctx, request)
	if err != nil {
		return err
	}

	log.Infof("Concepts query response status: %s", response.Status)
	if response.Message != nil {
		log.Infof("Concepts query response message: %s", *response.Message)
	}
	log.Infof("Concepts query response records count: %d", len(response.Records))

	return nil
}

func testQueryKnowledgeGraphConceptsWithDynamicFilters(ctx context.Context, client *iocmemoryprovider.Client) error {
	// Create filter for category attribute
	filters := []iocmemoryprovider.KnowledgeGraphQueryCriteriaFilter{
		{
			Category:  "dynamic",
			Key:       "category",
			Operation: "eqstr",
			Value:     []interface{}{"AI"}, // Single string value in slice
		},
	}

	// Create query criteria for concepts query with filters
	queryCriteria := iocmemoryprovider.NewKnowledgeGraphQueryCriteriaWithFilters(
		iocmemoryprovider.QueryTypeConcepts,
		nil,
		nil,
		filters,
	)

	// Create request using schema types
	masID := "523e4567-e89b-12d3-a456-426614174000"
	wkspID := "11111111-e89b-12d3-a456-426614174000"
	request := iocmemoryprovider.NewKnowledgeGraphQueryRequest(queryCriteria)

	// Set workspace and MAS IDs
	request.MasID = &masID
	request.WkspID = &wkspID

	// Call the client method (using concept query method which should handle different query types)
	response, err := client.QueryKnowledgeGraphConcept(ctx, request)
	if err != nil {
		return err
	}

	log.Infof("Concepts with Filters query response status: %s", response.Status)
	if response.Message != nil {
		log.Infof("Concepts with Filters query response message: %s", *response.Message)
	}
	log.Infof("Concepts with Filters query response records count: %d", len(response.Records))

	return nil
}

func testQueryKnowledgeGraphRelationsWithDynamicFilters(ctx context.Context, client *iocmemoryprovider.Client) error {
	// Create filter for relationship attribute
	filters := []iocmemoryprovider.KnowledgeGraphQueryCriteriaFilter{
		{
			Category:  "dynamic",
			Key:       "relationship",
			Operation: "eqstr",
			Value:     []interface{}{"references"}, // Single string value in slice
		},
	}

	// Create query criteria for relations query with filters
	queryCriteria := iocmemoryprovider.NewKnowledgeGraphQueryCriteriaWithFilters(
		iocmemoryprovider.QueryTypeRelations,
		nil,
		nil,
		filters,
	)

	// Create request using schema types
	masID := "523e4567-e89b-12d3-a456-426614174000"
	wkspID := "11111111-e89b-12d3-a456-426614174000"
	request := iocmemoryprovider.NewKnowledgeGraphQueryRequest(queryCriteria)

	// Set workspace and MAS IDs
	request.MasID = &masID
	request.WkspID = &wkspID

	// Call the client method (using concept query method which should handle different query types)
	response, err := client.QueryKnowledgeGraphConcept(ctx, request)
	if err != nil {
		return err
	}

	log.Infof("Relations with Filters query response status: %s", response.Status)
	if response.Message != nil {
		log.Infof("Relations with Filters query response message: %s", *response.Message)
	}
	log.Infof("Relations with Filters query response records count: %d", len(response.Records))

	return nil
}

func testQueryKnowledgeGraphConceptsWithCustomFilters(ctx context.Context, client *iocmemoryprovider.Client) error {
	// Create filter for relations_cnt custom attribute
	filters := []iocmemoryprovider.KnowledgeGraphQueryCriteriaFilter{
		{
			Category:  "custom",
			Key:       "relations_cnt",
			Operation: "lte",
			Value:     []interface{}{3}, // Single number value in slice
		},
	}

	// Create query criteria for relations query with filters
	queryCriteria := iocmemoryprovider.NewKnowledgeGraphQueryCriteriaWithFilters(
		iocmemoryprovider.QueryTypeConcepts,
		nil,
		nil,
		filters,
	)

	// Create request using schema types
	masID := "523e4567-e89b-12d3-a456-426614174000"
	wkspID := "11111111-e89b-12d3-a456-426614174000"
	request := iocmemoryprovider.NewKnowledgeGraphQueryRequest(queryCriteria)

	// Set workspace and MAS IDs
	request.MasID = &masID
	request.WkspID = &wkspID

	// Call the client method (using concept query method which should handle different query types)
	response, err := client.QueryKnowledgeGraphConcept(ctx, request)
	if err != nil {
		return err
	}

	log.Infof("Concepts with Custom Filters(relations_cnt) query response status: %s", response.Status)
	if response.Message != nil {
		log.Infof("Concepts with Custom Filters(relations_cnt) query response message: %s", *response.Message)
	}
	log.Infof("Concepts with Custom Filters(relations_cnt) query response records count: %d", len(response.Records))

	return nil
}

func testQueryKnowledgeGraphPath(ctx context.Context, client *iocmemoryprovider.Client) error {
	// Create query criteria for path query
	// depth := 2
	useDirection := false // false for undirected path, true for directed path
	queryCriteria := iocmemoryprovider.NewKnowledgeGraphQueryCriteria(
		iocmemoryprovider.QueryTypePath,
		nil, // unspecified depth will return paths of any length, set depth to limit it
		&useDirection,
	)

	// Create request using schema types
	masID := "523e4567-e89b-12d3-a456-426614174000"
	request := iocmemoryprovider.NewKnowledgeGraphQueryRequest(queryCriteria)

	// Set workspace and MAS IDs
	request.MasID = &masID

	// Set concepts for path query (requires exactly 2)
	request.Records = iocmemoryprovider.QueryRecords{
		Concepts: []iocmemoryprovider.ConceptRecord{
			{ID: "923e4567-e89b-12d3-a456-426614174000"},
			{ID: "923e4567-e89b-12d3-a456-426614174001"},
		},
	}

	// Call the client method
	response, err := client.QueryKnowledgeGraphPath(ctx, request)
	if err != nil {
		return err
	}

	log.Infof("Path query response status: %s", response.Status)
	if response.Message != nil {
		log.Infof("Path query response message: %s", *response.Message)
	}
	log.Infof("Path query response records count: %d", len(response.Records))

	return nil
}

func testQueryKnowledgeGraphNeighbor(ctx context.Context, client *iocmemoryprovider.Client) error {
	// Create query criteria for neighbor query
	useDirection := true
	queryCriteria := iocmemoryprovider.NewKnowledgeGraphQueryCriteria(
		iocmemoryprovider.QueryTypeNeighbour,
		nil,
		&useDirection,
	)

	// Create request using schema types
	masID := "523e4567-e89b-12d3-a456-426614174000"
	request := iocmemoryprovider.NewKnowledgeGraphQueryRequest(queryCriteria)

	// Set workspace and MAS IDs
	request.MasID = &masID

	// Set concepts for neighbor query (requires exactly 1)
	request.Records = iocmemoryprovider.QueryRecords{
		Concepts: []iocmemoryprovider.ConceptRecord{
			{ID: "923e4567-e89b-12d3-a456-426614174000"},
		},
	}

	// Call the client method
	response, err := client.QueryKnowledgeGraphNeighbor(ctx, request)
	if err != nil {
		return err
	}

	log.Infof("Neighbor query response status: %s", response.Status)
	if response.Message != nil {
		log.Infof("Neighbor query response message: %s", *response.Message)
	}
	log.Infof("Neighbor query response records count: %d", len(response.Records))

	return nil
}

func testQueryKnowledgeGraphConcept(ctx context.Context, client *iocmemoryprovider.Client) error {
	// Create query criteria for concept query
	queryCriteria := iocmemoryprovider.NewKnowledgeGraphQueryCriteria(
		iocmemoryprovider.QueryTypeConcept,
		nil,
		nil,
	)

	// Create request using schema types
	masID := "523e4567-e89b-12d3-a456-426614174000"
	request := iocmemoryprovider.NewKnowledgeGraphQueryRequest(queryCriteria)

	// Set workspace and MAS IDs
	request.MasID = &masID

	// Set concepts for concept query (requires exactly 1)
	request.Records = iocmemoryprovider.QueryRecords{
		Concepts: []iocmemoryprovider.ConceptRecord{
			{ID: "923e4567-e89b-12d3-a456-426614174000"},
		},
	}

	// Call the client method
	response, err := client.QueryKnowledgeGraphConcept(ctx, request)
	if err != nil {
		return err
	}

	log.Infof("Concept query response status: %s", response.Status)
	if response.Message != nil {
		log.Infof("Concept query response message: %s", *response.Message)
	}
	log.Infof("Concept query response records count: %d", len(response.Records))

	return nil
}

func testDeleteKnowledgeGraph(ctx context.Context, client *iocmemoryprovider.Client) error {
	// Create request using schema types
	request := iocmemoryprovider.NewKnowledgeGraphDeleteRequest()

	// Set workspace and MAS IDs
	masID := "523e4567-e89b-12d3-a456-426614174000"
	wkspID := "wksp_1"
	request.MasID = &masID
	request.WkspID = &wkspID

	// Set concepts to delete
	request.Records = &iocmemoryprovider.DeleteRecords{
		Concepts: []iocmemoryprovider.ConceptRecord{
			{ID: "923e4567-e89b-12d3-a456-426614174000"},
			{ID: "923e4567-e89b-12d3-a456-426614174001"},
			{ID: "923e4567-e89b-12d3-a456-426614174002"},
		},
	}

	// Call the client method
	response, err := client.DeleteKnowledgeGraph(ctx, request)
	if err != nil {
		return err
	}

	log.Infof("Delete response status: %s", response.Status)
	if response.Message != nil {
		log.Infof("Delete response message: %s", *response.Message)
	}

	return nil
}

///////////////////////// VECTOR OPERATIONS /////////////////////////

func testOnboardKnowledgeVectorStore(ctx context.Context, client *iocmemoryprovider.Client) error {
	// Create request using schema types
	wkspID := "7f136aa0-143c-46a6-82f2-249eac489e52"
	request := iocmemoryprovider.NewKnowledgeVectorStoreOnboardRequest(wkspID)

	// Call the client method
	response, err := client.OnboardKnowledgeVectorStore(ctx, request)
	if err != nil {
		return err
	}

	log.Infof("Onboard vector store response status: %s", response.Status)
	if response.Message != nil {
		log.Infof("Onboard vector store response message: %s", *response.Message)
	}

	return nil
}

func testUpsertKnowledgeVectors(ctx context.Context, client *iocmemoryprovider.Client) error {
	// Create vector records using schema types
	records := []iocmemoryprovider.KnowledgeVectorStoreRequestRecord{
		{
			ID:      "123e4567-e89b-12d3-a456-426614174001",
			Content: "content in plain text",
			Embedding: &iocmemoryprovider.VectorEmbeddingConfig{
				Data: []float64{0.1, 0.2, 0.3},
			},
		},
		{
			ID:      "223e4567-e89b-12d3-a456-426614174001",
			Content: "content in plain text",
			Embedding: &iocmemoryprovider.VectorEmbeddingConfig{
				Data: []float64{0.4, 0.5, 0.6},
			},
		},
		{
			ID:      "323e4567-e89b-12d3-a456-426614174001",
			Content: "content in plain text",
			Embedding: &iocmemoryprovider.VectorEmbeddingConfig{
				Data: []float64{0.7, 0.8, 0.9},
			},
		},
	}

	// Create request using schema types
	wkspID := "7f136aa0-143c-46a6-82f2-249eac489e52"
	masID := "223e4567-e89b-12d3-a456-426614174001"
	request := iocmemoryprovider.NewKnowledgeVectorStoreRequest(wkspID, masID, records)

	// Call the client method
	response, err := client.UpsertKnowledgeVectors(ctx, request)
	if err != nil {
		return err
	}

	log.Infof("Upsert vectors response status: %s", response.Status)
	if response.Message != nil {
		log.Infof("Upsert vectors response message: %s", *response.Message)
	}

	return nil
}

func testQueryKnowledgeVectorsCosine(ctx context.Context, client *iocmemoryprovider.Client) error {
	// Create query criteria for cosine distance
	embedding := &iocmemoryprovider.VectorEmbeddingConfig{
		Data: []float64{0.4, 0.5, 0.6},
	}
	limit := 2
	queryCriteria := iocmemoryprovider.NewKnowledgeVectorQueryCriteria(
		iocmemoryprovider.QueryTypeDistanceCosine,
		embedding,
		limit,
	)

	// Create request using schema types
	wkspID := "7f136aa0-143c-46a6-82f2-249eac489e52"
	masID := "223e4567-e89b-12d3-a456-426614174001"
	request := iocmemoryprovider.NewKnowledgeVectorQueryRequest(wkspID, masID, queryCriteria)

	// Call the client method
	response, err := client.QueryKnowledgeVectors(ctx, request)
	if err != nil {
		return err
	}

	log.Infof("Query vectors (cosine) response status: %s", response.Status)
	if response.Message != nil {
		log.Infof("Query vectors (cosine) response message: %s", *response.Message)
	}
	log.Infof("Query vectors (cosine) response records count: %d", len(response.Records))

	return nil
}

func testQueryKnowledgeVectorsL2(ctx context.Context, client *iocmemoryprovider.Client) error {
	// Create query criteria for L2 distance
	embedding := &iocmemoryprovider.VectorEmbeddingConfig{
		Data: []float64{0.4, 0.5, 0.6},
	}
	limit := 2
	queryCriteria := iocmemoryprovider.NewKnowledgeVectorQueryCriteria(
		iocmemoryprovider.QueryTypeDistanceL2,
		embedding,
		limit,
	)

	// Create request using schema types
	wkspID := "7f136aa0-143c-46a6-82f2-249eac489e52"
	masID := "223e4567-e89b-12d3-a456-426614174001"
	request := iocmemoryprovider.NewKnowledgeVectorQueryRequest(wkspID, masID, queryCriteria)

	// Call the client method
	response, err := client.QueryKnowledgeVectors(ctx, request)
	if err != nil {
		return err
	}

	log.Infof("Query vectors (L2) response status: %s", response.Status)
	if response.Message != nil {
		log.Infof("Query vectors (L2) response message: %s", *response.Message)
	}
	log.Infof("Query vectors (L2) response records count: %d", len(response.Records))

	return nil
}

func testQueryKnowledgeVectorsGetByID(ctx context.Context, client *iocmemoryprovider.Client) error {
	// Create query criteria for get by ID
	vectorID := "223e4567-e89b-12d3-a456-426614174001"
	queryCriteria := iocmemoryprovider.NewKnowledgeVectorQueryCriteria(
		iocmemoryprovider.QueryTypeGetByID,
		vectorID,
	)

	// Create request using schema types
	wkspID := "7f136aa0-143c-46a6-82f2-249eac489e52"
	masID := "223e4567-e89b-12d3-a456-426614174001"
	request := iocmemoryprovider.NewKnowledgeVectorQueryRequest(wkspID, masID, queryCriteria)

	// Call the client method
	response, err := client.QueryKnowledgeVectors(ctx, request)
	if err != nil {
		return err
	}

	log.Infof("Query vectors (Get By ID) response status: %s", response.Status)
	if response.Message != nil {
		log.Infof("Query vectors (Get By ID) response message: %s", *response.Message)
	}
	log.Infof("Query vectors (Get By ID) response records count: %d", len(response.Records))

	return nil
}

func testDeleteKnowledgeVectors(ctx context.Context, client *iocmemoryprovider.Client) error {
	// Create request using schema types
	wkspID := "7f136aa0-143c-46a6-82f2-249eac489e52"
	masID := "223e4567-e89b-12d3-a456-426614174001"
	vectorID := "223e4567-e89b-12d3-a456-426614174001"
	softDelete := false
	request := iocmemoryprovider.NewKnowledgeVectorDeleteRequest(wkspID, masID, vectorID, softDelete)

	// Call the client method
	response, err := client.DeleteKnowledgeVectors(ctx, request)
	if err != nil {
		return err
	}

	log.Infof("Delete vectors response status: %s", response.Status)
	if response.Message != nil {
		log.Infof("Delete vectors response message: %s", *response.Message)
	}

	return nil
}

func testDeleteKnowledgeVectorStore(ctx context.Context, client *iocmemoryprovider.Client) error {
	// Create request using schema types
	wkspID := "7f136aa0-143c-46a6-82f2-249eac489e52"
	request := iocmemoryprovider.NewKnowledgeVectorStoreOnboardDeleteRequest(wkspID)

	// Call the client method
	response, err := client.DeleteKnowledgeVectorStore(ctx, request)
	if err != nil {
		return err
	}

	log.Infof("Delete vector store response status: %s", response.Status)
	if response.Message != nil {
		log.Infof("Delete vector store response message: %s", *response.Message)
	}

	return nil
}

// printTestSeparator prints a separator line between tests
func printTestSeparator() {
	fmt.Println()
	fmt.Println("================================================================================")
	fmt.Println()
}

// stringPtr is a helper function to get a pointer to a string
func stringPtr(s string) *string {
	return &s
}
