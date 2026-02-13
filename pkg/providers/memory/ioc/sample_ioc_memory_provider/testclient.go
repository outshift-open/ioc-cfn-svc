package main

import (
	"context"
	"os"

	iocmemoryprovider "github.com/cisco-eti/ioc-cfn-svc/pkg/providers/memory/ioc"
	"github.com/cisco-eti/ioc-cfn-svc/pkg/tools/logger"
)

// Sample client using the IOC Memory Provider
// This sample shows how to use the IOC Memory Provider to perform knowledge graph operations
// using schema types.

var log = logger.Default()

func main() {
	log.Info("Starting IOC Memory Provider Client Sample...")

	// Create client
	client := iocmemoryprovider.NewClient("")
	if client == nil {
		log.Fatal("Failed to create client")
	}

	log.Info("Client established successfully")
	ctx := context.Background()

	// Test UpsertKnowledgeGraph method
	log.Info("Testing UpsertKnowledgeGraph...")
	if err := testUpsertKnowledgeGraph(ctx, client); err != nil {
		log.Errorf("Error in UpsertKnowledgeGraph: %v", err)
		os.Exit(1)
	}

	// Test QueryKnowledgeGraphPath method
	log.Info("Testing QueryKnowledgeGraphPath...")
	if err := testQueryKnowledgeGraphPath(ctx, client); err != nil {
		log.Errorf("Error in QueryKnowledgeGraphPath: %v", err)
		os.Exit(1)
	}

	// Test QueryKnowledgeGraphNeighbor method
	log.Info("Testing QueryKnowledgeGraphNeighbor...")
	if err := testQueryKnowledgeGraphNeighbor(ctx, client); err != nil {
		log.Errorf("Error in QueryKnowledgeGraphNeighbor: %v", err)
		os.Exit(1)
	}

	// Test QueryKnowledgeGraphConcept method
	log.Info("Testing QueryKnowledgeGraphConcept...")
	if err := testQueryKnowledgeGraphConcept(ctx, client); err != nil {
		log.Errorf("Error in QueryKnowledgeGraphConcept: %v", err)
		os.Exit(1)
	}

	// Test DeleteKnowledgeGraph method
	log.Info("Testing DeleteKnowledgeGraph...")
	if err := testDeleteKnowledgeGraph(ctx, client); err != nil {
		log.Errorf("Error in DeleteKnowledgeGraph: %v", err)
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

func testQueryKnowledgeGraphPath(ctx context.Context, client *iocmemoryprovider.Client) error {
	// Create request using schema types
	request := iocmemoryprovider.NewKnowledgeGraphQueryRequest()

	// Set workspace and MAS IDs
	masID := "523e4567-e89b-12d3-a456-426614174000"
	request.MasID = &masID

	// Set query criteria for path query
	request.QueryCriteria = &iocmemoryprovider.KnowledgeGraphQueryCriteria{
		QueryType: iocmemoryprovider.QueryTypePath,
	}

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
	// Create request using schema types
	request := iocmemoryprovider.NewKnowledgeGraphQueryRequest()

	// Set workspace and MAS IDs
	masID := "523e4567-e89b-12d3-a456-426614174000"
	request.MasID = &masID

	// Set query criteria for neighbor query
	request.QueryCriteria = &iocmemoryprovider.KnowledgeGraphQueryCriteria{
		QueryType: iocmemoryprovider.QueryTypeNeighbour,
	}

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
	// Create request using schema types
	request := iocmemoryprovider.NewKnowledgeGraphQueryRequest()

	// Set workspace and MAS IDs
	masID := "523e4567-e89b-12d3-a456-426614174000"
	request.MasID = &masID

	// Set query criteria for concept query
	request.QueryCriteria = &iocmemoryprovider.KnowledgeGraphQueryCriteria{
		QueryType: iocmemoryprovider.QueryTypeConcept,
	}

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

// stringPtr is a helper function to get a pointer to a string
func stringPtr(s string) *string {
	return &s
}
