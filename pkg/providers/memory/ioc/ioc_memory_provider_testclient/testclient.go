// Copyright 2026 Cisco Systems, Inc. and its affiliates
//
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"flag"
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
//
// Usage:
//   go build -o testclient .
//   ./testclient                 - Run all tests (default)
//   ./testclient -all            - Run all tests (explicit)
//   ./testclient -graph          - Run only Knowledge Graph tests
//   ./testclient -vector         - Run only Knowledge Vector tests
//   ./testclient -kvpmas         - Run only KVP MAS scope tests
//   ./testclient -kvpce          - Run only KVP CE scope tests

var log *zap.SugaredLogger

func main() {
	// Define command-line flags
	var all = flag.Bool("all", false, "Run all tests (explicit)")
	var graph = flag.Bool("graph", false, "Run only Knowledge Graph tests")
	var vector = flag.Bool("vector", false, "Run only Knowledge Vector tests")
	var kvpMAS = flag.Bool("kvpmas", false, "Run only KVP MAS scope tests")
	var kvpCE = flag.Bool("kvpce", false, "Run only KVP CE scope tests")
	flag.Parse()

	// Default to all tests if no flags are set
	if !*all && !*graph && !*vector && !*kvpMAS && !*kvpCE {
		*all = true
	}

	// Initialize logger first
	logger.Init()
	log = logger.Default()

	log.Info("Starting IOC Memory Provider Client Sample...")

	// Log which tests will be run
	if *all {
		log.Info("Running all tests")
	} else if *graph {
		log.Info("Running only Knowledge Graph tests")
	} else if *vector {
		log.Info("Running only Knowledge Vector tests")
	} else if *kvpMAS {
		log.Info("Running only Knowledge KeyValue MAS scope tests")
	} else if *kvpCE {
		log.Info("Running only Knowledge KeyValue CE scope tests")
	}

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

	// Run Knowledge Graph tests when -all or -graph flag is set
	if *all || *graph {
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
	}

	// Run Knowledge Vector tests when -all or -vector flag is set
	if *all || *vector {
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
	}

	// Test KVP operations based on flags
	if *all || *kvpMAS || *kvpCE {
		printTestSeparator()
		log.Info("\n=== Testing KVP Operations ===")
	}

	// Test MAS Scope KVP Operations (run if -all or -kvpmas flag is set)
	if *all || *kvpMAS {
		printTestSeparator()
		log.Info("Testing MAS Scope KVP Operations...")

		// Test OnboardKnowledgeKVPStore method (MAS)
		printTestSeparator()
		log.Info("Testing OnboardKnowledgeKVPStore (MAS)...")
		if err := testOnboardKnowledgeKVPStoreMAS(ctx, client); err != nil {
			log.Errorf("Error in OnboardKnowledgeKVPStore (MAS): %v", err)
			os.Exit(1)
		}

		// Test UpsertKnowledgeKVPs method (MAS)
		printTestSeparator()
		log.Info("Testing UpsertKnowledgeKVPs (MAS)...")
		if err := testUpsertKnowledgeKVPsMAS(ctx, client); err != nil {
			log.Errorf("Error in UpsertKnowledgeKVPs (MAS): %v", err)
			os.Exit(1)
		}

		// Test QueryKnowledgeKVPs method (MAS)
		printTestSeparator()
		log.Info("Testing QueryKnowledgeKVPs (MAS)...")
		if err := testQueryKnowledgeKVPsMAS(ctx, client); err != nil {
			log.Errorf("Error in QueryKnowledgeKVPs (MAS): %v", err)
			os.Exit(1)
		}

		// Test DeleteKnowledgeKVPs method (MAS)
		printTestSeparator()
		log.Info("Testing DeleteKnowledgeKVPs (MAS)...")
		if err := testDeleteKnowledgeKVPsMAS(ctx, client); err != nil {
			log.Errorf("Error in DeleteKnowledgeKVPs (MAS): %v", err)
			os.Exit(1)
		}

		// Test DeleteKnowledgeKVPStore method (MAS)
		printTestSeparator()
		log.Info("Testing DeleteKnowledgeKVPStore (MAS)...")
		if err := testDeleteKnowledgeKVPStoreMAS(ctx, client); err != nil {
			log.Errorf("Error in DeleteKnowledgeKVPStore (MAS): %v", err)
			os.Exit(1)
		}
	}

	// Test CE Scope KVP Operations (run if -all or -kvpce flag is set)
	if *all || *kvpCE {
		printTestSeparator()
		log.Info("Testing CE Scope KVP Operations...")

		// Test OnboardKnowledgeKVPStore method (CE)
		printTestSeparator()
		log.Info("Testing OnboardKnowledgeKVPStore (CE)...")
		if err := testOnboardKnowledgeKVPStoreCE(ctx, client); err != nil {
			log.Errorf("Error in OnboardKnowledgeKVPStore (CE): %v", err)
			os.Exit(1)
		}

		// Test UpsertKnowledgeKVPs method (CE)
		printTestSeparator()
		log.Info("Testing UpsertKnowledgeKVPs (CE)...")
		if err := testUpsertKnowledgeKVPsCE(ctx, client); err != nil {
			log.Errorf("Error in UpsertKnowledgeKVPs (CE): %v", err)
			os.Exit(1)
		}

		// Test QueryKnowledgeKVPs method (CE)
		printTestSeparator()
		log.Info("Testing QueryKnowledgeKVPs (CE)...")
		if err := testQueryKnowledgeKVPsCE(ctx, client); err != nil {
			log.Errorf("Error in QueryKnowledgeKVPs (CE): %v", err)
			os.Exit(1)
		}

		// Test DeleteKnowledgeKVPs method (CE)
		printTestSeparator()
		log.Info("Testing DeleteKnowledgeKVPs (CE)...")
		if err := testDeleteKnowledgeKVPsCE(ctx, client); err != nil {
			log.Errorf("Error in DeleteKnowledgeKVPs (CE): %v", err)
			os.Exit(1)
		}

		// Test DeleteKnowledgeKVPStore method (CE)
		printTestSeparator()
		log.Info("Testing DeleteKnowledgeKVPStore (CE)...")
		if err := testDeleteKnowledgeKVPStoreCE(ctx, client); err != nil {
			log.Errorf("Error in DeleteKnowledgeKVPStore (CE): %v", err)
			os.Exit(1)
		}
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

func testOnboardKnowledgeKVPStoreMAS(ctx context.Context, client *iocmemoryprovider.Client) error {
	// Create request using schema types
	storeID := "223e4567-e89b-12d3-a456-426614174001"
	request := iocmemoryprovider.NewKnowledgeKVPStoreOnboardRequest(iocmemoryprovider.ScopeTypeMAS, &storeID)

	// Call the client method
	response, err := client.OnboardKnowledgeKVPStore(ctx, request)
	if err != nil {
		return err
	}

	log.Infof("Onboard KVP store response status: %s", response.Status)
	if response.Message != nil {
		log.Infof("Onboard KVP store response message: %s", *response.Message)
	}

	return nil
}

func testUpsertKnowledgeKVPsMAS(ctx context.Context, client *iocmemoryprovider.Client) error {
	// Create sample KVP records
	records := []iocmemoryprovider.KnowledgeKVPRecord{
		{
			Key: map[string]interface{}{
				"episode_id": "12345",
			},
			Value: map[string]interface{}{
				"name": "John Doe",
				"preferences": map[string]interface{}{
					"theme":    "dark",
					"language": "en",
				},
				"last_login": 1640995200,
			},
		},
		{
			Key: map[string]interface{}{
				"episode_id": "67890",
			},
			Value: map[string]interface{}{
				"name": "Jane Smith",
				"preferences": map[string]interface{}{
					"theme":    "light",
					"language": "es",
				},
				"last_login": 1640995300,
			},
		},
	}

	// Create request using schema types
	request := iocmemoryprovider.NewKnowledgeKVPStoreRequest(iocmemoryprovider.ScopeTypeMAS, records)

	// Set required fields for MAS scope
	masID := "223e4567-e89b-12d3-a456-426614174001"
	wkspID := "9f136aa0-143c-46a6-82f2-249eac489e52"
	agentID := "agent-001"
	request.MasID = &masID
	request.WkspID = &wkspID
	request.AgentID = &agentID

	// Call the client method
	response, err := client.UpsertKnowledgeKVPs(ctx, request)
	if err != nil {
		return err
	}

	log.Infof("Upsert KVPs response status: %s", response.Status)
	if response.Message != nil {
		log.Infof("Upsert KVPs response message: %s", *response.Message)
	}

	return nil
}

func testQueryKnowledgeKVPsMAS(ctx context.Context, client *iocmemoryprovider.Client) error {
	// Create query criteria
	queryCriteria := iocmemoryprovider.NewKnowledgeKVPQueryCriteria(
		iocmemoryprovider.QueryTypeGetByKey,
		map[string]interface{}{
			"episode_id": "12345",
		},
		nil,
	)

	// Create request using schema types
	request := iocmemoryprovider.NewKnowledgeKVPQueryRequest(iocmemoryprovider.ScopeTypeMAS, *queryCriteria)

	// Set required fields for MAS scope
	masID := "223e4567-e89b-12d3-a456-426614174001"
	wkspID := "9f136aa0-143c-46a6-82f2-249eac489e52"
	agentID := "agent-001"
	request.MasID = &masID
	request.WkspID = &wkspID
	request.AgentID = &agentID

	// Call the client method
	response, err := client.QueryKnowledgeKVPs(ctx, request)
	if err != nil {
		return err
	}

	log.Infof("Query KVPs response status: %s", response.Status)
	if response.Message != nil {
		log.Infof("Query KVPs response message: %s", *response.Message)
	}
	log.Infof("Query KVPs response records count: %d", len(response.Records))

	return nil
}

func testDeleteKnowledgeKVPsMAS(ctx context.Context, client *iocmemoryprovider.Client) error {
	// Create delete key - use the same key that was successfully queried
	key := map[string]interface{}{
		"episode_id": "12345",
	}

	// Create request using schema types - use soft delete (false) to avoid permanent deletion
	request := iocmemoryprovider.NewKnowledgeKVPDeleteRequest(iocmemoryprovider.ScopeTypeMAS, key, false)

	// Set required fields for MAS scope - use same IDs as upsert operation
	masID := "223e4567-e89b-12d3-a456-426614174001"
	wkspID := "9f136aa0-143c-46a6-82f2-249eac489e52"
	agentID := "agent-001"
	request.MasID = &masID
	request.WkspID = &wkspID
	request.AgentID = &agentID

	// Call the client method
	response, err := client.DeleteKnowledgeKVPs(ctx, request)
	if err != nil {
		return err
	}

	log.Infof("Delete KVPs response status: %s", response.Status)
	if response.Message != nil {
		log.Infof("Delete KVPs response message: %s", *response.Message)
	}

	return nil
}

func testDeleteKnowledgeKVPStoreMAS(ctx context.Context, client *iocmemoryprovider.Client) error {
	// Create request using schema types
	storeID := "223e4567-e89b-12d3-a456-426614174001"
	request := iocmemoryprovider.NewKnowledgeKVPStoreOnboardDeleteRequest(iocmemoryprovider.ScopeTypeMAS, storeID)

	// Call the client method
	response, err := client.DeleteKnowledgeKVPStore(ctx, request)
	if err != nil {
		return err
	}

	log.Infof("Delete KVP store response status: %s", response.Status)
	if response.Message != nil {
		log.Infof("Delete KVP store response message: %s", *response.Message)
	}

	return nil
}

// CE Scope Tests

func testOnboardKnowledgeKVPStoreCE(ctx context.Context, client *iocmemoryprovider.Client) error {
	// Create request using schema types for CE scope
	storeID := "12345678-1234-1234-1234-123456789abc"
	request := iocmemoryprovider.NewKnowledgeKVPStoreOnboardRequest(iocmemoryprovider.ScopeTypeCE, &storeID)

	// Call the client method
	response, err := client.OnboardKnowledgeKVPStore(ctx, request)
	if err != nil {
		return err
	}

	log.Infof("Onboard KVP store (CE) response status: %s", response.Status)
	if response.Message != nil {
		log.Infof("Onboard KVP store (CE) response message: %s", *response.Message)
	}

	return nil
}

func testUpsertKnowledgeKVPsCE(ctx context.Context, client *iocmemoryprovider.Client) error {
	// Create sample KVP records for CE scope
	records := []iocmemoryprovider.KnowledgeKVPRecord{
		{
			Key: map[string]interface{}{
				"ce_config_key": "model_settings",
			},
			Value: map[string]interface{}{
				"temperature":   0.7,
				"max_tokens":    2048,
				"model_version": "v2.1",
			},
		},
		{
			Key: map[string]interface{}{
				"ce_config_key": "system_prompt",
			},
			Value: map[string]interface{}{
				"prompt": "You are a helpful AI assistant for cognitive engine operations.",
			},
		},
	}

	// Create request using schema types for CE scope
	request := iocmemoryprovider.NewKnowledgeKVPStoreRequest(iocmemoryprovider.ScopeTypeCE, records)
	ceID := "12345678-1234-1234-1234-123456789abc"
	request.CeID = &ceID

	// Call the client method
	response, err := client.UpsertKnowledgeKVPs(ctx, request)
	if err != nil {
		return err
	}

	log.Infof("Upsert KVPs (CE) response status: %s", response.Status)
	if response.Message != nil {
		log.Infof("Upsert KVPs (CE) response message: %s", *response.Message)
	}

	return nil
}

func testQueryKnowledgeKVPsCE(ctx context.Context, client *iocmemoryprovider.Client) error {
	// Create query criteria for CE scope
	queryCriteria := iocmemoryprovider.NewKnowledgeKVPQueryCriteria(
		iocmemoryprovider.QueryTypeGetByKey,
		map[string]interface{}{
			"ce_config_key": "model_settings",
		},
		nil,
	)

	// Create request using schema types for CE scope
	request := iocmemoryprovider.NewKnowledgeKVPQueryRequest(iocmemoryprovider.ScopeTypeCE, *queryCriteria)
	ceID := "12345678-1234-1234-1234-123456789abc"
	request.CeID = &ceID

	// Call the client method
	response, err := client.QueryKnowledgeKVPs(ctx, request)
	if err != nil {
		return err
	}

	log.Infof("Query KVPs (CE) response status: %s", response.Status)
	if response.Message != nil {
		log.Infof("Query KVPs (CE) response message: %s", *response.Message)
	}
	if len(response.Records) > 0 {
		log.Infof("Query KVPs (CE) returned %d records", len(response.Records))
	}

	return nil
}

func testDeleteKnowledgeKVPsCE(ctx context.Context, client *iocmemoryprovider.Client) error {
	// Create delete key for CE scope
	key := map[string]interface{}{
		"ce_config_key": "system_prompt",
	}

	// Create request using schema types for CE scope
	request := iocmemoryprovider.NewKnowledgeKVPDeleteRequest(iocmemoryprovider.ScopeTypeCE, key, false)
	ceID := "12345678-1234-1234-1234-123456789abc"
	request.CeID = &ceID

	// Call the client method
	response, err := client.DeleteKnowledgeKVPs(ctx, request)
	if err != nil {
		return err
	}

	log.Infof("Delete KVPs (CE) response status: %s", response.Status)
	if response.Message != nil {
		log.Infof("Delete KVPs (CE) response message: %s", *response.Message)
	}

	return nil
}

func testDeleteKnowledgeKVPStoreCE(ctx context.Context, client *iocmemoryprovider.Client) error {
	// Create request using schema types for CE scope
	storeID := "12345678-1234-1234-1234-123456789abc"
	request := iocmemoryprovider.NewKnowledgeKVPStoreOnboardDeleteRequest(iocmemoryprovider.ScopeTypeCE, storeID)

	// Call the client method
	response, err := client.DeleteKnowledgeKVPStore(ctx, request)
	if err != nil {
		return err
	}

	log.Infof("Delete KVP store (CE) response status: %s", response.Status)
	if response.Message != nil {
		log.Infof("Delete KVP store (CE) response message: %s", *response.Message)
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
