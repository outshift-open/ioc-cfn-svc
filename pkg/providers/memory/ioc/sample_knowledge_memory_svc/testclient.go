package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"time"

	httpclient "github.com/cisco-eti/ioc-cfn-svc/pkg/client/http"
)

//Sample Client using the Knowledge Memory Service
//This sample shows how to use the Knowledge Memory Service directly to perform knowledge graph operations
// using the JSON requests and responses.

const (
	KNOWLEDGE_MEMORY_SVC_REST_ENDPOINT = "http://localhost:8001"
)

// go run pkg/client/http/samples/knowledge_memory_svc_client.go

// createClient creates and configures a Client instance
func createClient() *httpclient.Client {
	config := httpclient.DefaultConfig()
	config.Timeout = 30 * time.Second
	config.MaxRetries = 3

	return httpclient.NewWithConfig(config)
}

// upsertKnowledgeGraph sends a POST request to upsert knowledge graph data
func upsertKnowledgeGraph(client *httpclient.Client) error {
	ctx := context.Background()

	// Mock JSON request body matching the curl example
	jsonData := []byte(`{
    "records": {
        "concepts": [
            {
                "id": "123e4567-e89b-12d3-a456-426614174000",
                "name": "New Test Artificial Intelligence",
                "description": "The simulation of human intelligence processes by machines",
                "attributes": {
                    "category": "Technology",
                    "founded_year": 1956
                },
                "embeddings": {
                    "name": "text-embedding-ada-002",
                    "data": [
                        0.1,
                        0.2,
                        0.3,
                        0.4,
                        0.5
                    ]
                }
            },
            {
                "id": "123e4567-e89b-12d3-a456-426614174001",
                "name": "New Machine Learning",
                "description": "A subset of AI that enables systems to learn from data",
                "attributes": {
                    "category": "Computer Science",
                    "parent_field": "AI"
                },
                "embeddings": {
                    "name": "text-embedding-ada-002",
                    "data": [
                        0.2,
                        0.3,
                        0.4,
                        0.5,
                        0.6
                    ]
                }
            },
            {
                "id": "123e4567-e89b-12d3-a456-426614174002",
                "name": "Deep Learning",
                "description": "A subset of ML using neural networks with multiple layers",
                "attributes": {
                    "category": "Neural networks",
                    "parent_field": "Machine Learning"
                },
                "embeddings": {
                    "name": "text-embedding-ada-002",
                    "data": [
                        0.3,
                        0.4,
                        0.5,
                        0.6,
                        0.7
                    ]
                }
            }
        ],
        "relations": [
            {
                "id": "rel_11",
                "relation": "HAS_A_SUBFIELD",
                "node_ids": [
                    "123e4567-e89b-12d3-a456-426614174000",
                    "123e4567-e89b-12d3-a456-426614174001"
                ],
                "attributes": {
                    "since": 1956,
                    "strength": 0.9
                },
                "embeddings": {
                    "name": "relation-embedding",
                    "data": [
                        0.15,
                        0.25,
                        0.35,
                        0.45,
                        0.55
                    ]
                }
            },
            {
                "id": "rel_12",
                "relation": "HAS_SUBFIELD",
                "node_ids": [
                    "123e4567-e89b-12d3-a456-426614174001",
                    "123e4567-e89b-12d3-a456-426614174002"
                ],
                "attributes": {
                    "since": 1980,
                    "strength": 0.95
                }
            },
            {
                "id": "rel_13",
                "relation": "RELATED_TO",
                "node_ids": [
                    "123e4567-e89b-12d3-a456-426614174000",
                    "123e4567-e89b-12d3-a456-426614174002"
                ],
                "attributes": {
                    "relationship": "hierarchical",
                    "direct": "False"
                }
            }
        ]
    },
    "wksp_id": "11111111-e89b-12d3-a456-426614174000",
    "mas_id": "123e4567-e89b-12d3-a456-426614174000",
    "force_replace": "True"
}`)

	// Prepare headers
	headers := map[string]string{
		"Content-Type": "application/json",
	}

	// Make POST request
	url := KNOWLEDGE_MEMORY_SVC_REST_ENDPOINT + "/api/knowledge/graph"
	resp, err := client.Post(ctx, url, jsonData, headers)
	if err != nil {
		return fmt.Errorf("failed to send POST request: %w", err)
	}
	defer resp.Body.Close()

	// Read and print response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	fmt.Printf("POST request to %s completed\n", url)
	fmt.Printf("Response status: %s\n", resp.Status)
	fmt.Printf("Response headers: %v\n", resp.Header)
	fmt.Printf("Response body: %s\n", string(body))

	// Pretty print JSON response
	var prettyJSON interface{}
	if err := json.Unmarshal(body, &prettyJSON); err == nil {
		if formatted, err := json.MarshalIndent(prettyJSON, "", "  "); err == nil {
			fmt.Printf("Pretty JSON:\n%s\n", string(formatted))
		}
	}

	return nil
}

// queryKnowledgeGraphPath sends a POST request to query knowledge graph path
func queryKnowledgeGraphPath(client *httpclient.Client) error {
	ctx := context.Background()

	// Mock JSON request body matching the curl example
	jsonData := []byte(`{ 
	"request_id":"123e4567-e89b-12d3-a456-426614174000",
	"mas_id":"123e4567-e89b-12d3-a456-426614174000",
	"records": {
		"concepts": [
			{
				"id": "123e4567-e89b-12d3-a456-426614174000"
			},
			{
				"id": "123e4567-e89b-12d3-a456-426614174001"
			}
		]
	},
	"query_criteria": {
		"query_type":"path"
	}
}`)

	// Prepare headers
	headers := map[string]string{
		"Content-Type": "application/json",
	}

	// Make POST request
	url := KNOWLEDGE_MEMORY_SVC_REST_ENDPOINT + "/api/knowledge/graph/query"
	resp, err := client.Post(ctx, url, jsonData, headers)
	if err != nil {
		return fmt.Errorf("failed to send POST request: %w", err)
	}
	defer resp.Body.Close()

	// Read and print response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	fmt.Printf("POST request to %s completed\n", url)
	fmt.Printf("Response status: %s\n", resp.Status)
	fmt.Printf("Response headers: %v\n", resp.Header)
	fmt.Printf("Response body: %s\n", string(body))

	// Pretty print JSON response
	var prettyJSON interface{}
	if err := json.Unmarshal(body, &prettyJSON); err == nil {
		if formatted, err := json.MarshalIndent(prettyJSON, "", "  "); err == nil {
			fmt.Printf("Pretty JSON:\n%s\n", string(formatted))
		}
	}

	return nil
}

// queryKnowledgeGraphNeighbor sends a POST request to query knowledge graph neighbors
func queryKnowledgeGraphNeighbor(client *httpclient.Client) error {
	ctx := context.Background()

	// Mock JSON request body matching the curl example
	jsonData := []byte(`{ 
	"request_id":"123e4567-e89b-12d3-a456-426614174000",
	"mas_id":"123e4567-e89b-12d3-a456-426614174000",
	"records": {
		"concepts": [
			{
				"id": "123e4567-e89b-12d3-a456-426614174000"
			}
		]
	},
	"query_criteria": {
		"query_type":"neighbor"
	}
}`)

	// Prepare headers
	headers := map[string]string{
		"Content-Type": "application/json",
	}

	// Make POST request
	url := KNOWLEDGE_MEMORY_SVC_REST_ENDPOINT + "/api/knowledge/graph/query"
	resp, err := client.Post(ctx, url, jsonData, headers)
	if err != nil {
		return fmt.Errorf("failed to send POST request: %w", err)
	}
	defer resp.Body.Close()

	// Read and print response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	fmt.Printf("POST request to %s completed\n", url)
	fmt.Printf("Response status: %s\n", resp.Status)
	fmt.Printf("Response headers: %v\n", resp.Header)
	fmt.Printf("Response body: %s\n", string(body))

	// Pretty print JSON response
	var prettyJSON interface{}
	if err := json.Unmarshal(body, &prettyJSON); err == nil {
		if formatted, err := json.MarshalIndent(prettyJSON, "", "  "); err == nil {
			fmt.Printf("Pretty JSON:\n%s\n", string(formatted))
		}
	}

	return nil
}

// queryKnowledgeGraphConcept sends a POST request to query knowledge graph concept
func queryKnowledgeGraphConcept(client *httpclient.Client) error {
	ctx := context.Background()

	// Mock JSON request body matching the curl example
	jsonData := []byte(`{ 
	"request_id":"123e4567-e89b-12d3-a456-426614174000",
	"mas_id":"123e4567-e89b-12d3-a456-426614174000",
	"records": {
		"concepts": [
			{
				"id": "123e4567-e89b-12d3-a456-426614174000"
			}
		]
	},
	"query_criteria": {
		"query_type":"concept"
	}
}`)

	// Prepare headers
	headers := map[string]string{
		"Content-Type": "application/json",
	}

	// Make POST request
	url := KNOWLEDGE_MEMORY_SVC_REST_ENDPOINT + "/api/knowledge/graph/query"
	resp, err := client.Post(ctx, url, jsonData, headers)
	if err != nil {
		return fmt.Errorf("failed to send POST request: %w", err)
	}
	defer resp.Body.Close()

	// Read and print response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	fmt.Printf("POST request to %s completed\n", url)
	fmt.Printf("Response status: %s\n", resp.Status)
	fmt.Printf("Response headers: %v\n", resp.Header)
	fmt.Printf("Response body: %s\n", string(body))

	// Pretty print JSON response
	var prettyJSON interface{}
	if err := json.Unmarshal(body, &prettyJSON); err == nil {
		if formatted, err := json.MarshalIndent(prettyJSON, "", "  "); err == nil {
			fmt.Printf("Pretty JSON:\n%s\n", string(formatted))
		}
	}

	return nil
}

// deleteKnowledgeGraph sends a DELETE request to delete knowledge graph data
func deleteKnowledgeGraph(client *httpclient.Client) error {
	ctx := context.Background()

	// Mock JSON request body matching the curl example
	jsonData := []byte(`{
	"request_id": "bbc2fea0-5e6c-4cf9-b7b4-fe6418c041a0",
	"wksp_id":"wksp_1",
	"mas_id":"123e4567-e89b-12d3-a456-426614174000",
	"records": {
		"concepts": [
			{
				"id": "123e4567-e89b-12d3-a456-426614174000"
			},
			{
				"id": "123e4567-e89b-12d3-a456-426614174001"
			},
			{
				"id": "123e4567-e89b-12d3-a456-426614174002"
			}
		]
	}
}`)

	// Prepare headers
	headers := map[string]string{
		"Content-Type": "application/json",
	}

	// Make DELETE request
	url := KNOWLEDGE_MEMORY_SVC_REST_ENDPOINT + "/api/knowledge/graph"
	resp, err := client.Delete(ctx, url, jsonData, headers)
	if err != nil {
		return fmt.Errorf("failed to send DELETE request: %w", err)
	}
	defer resp.Body.Close()

	// Read and print response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	fmt.Printf("DELETE request to %s completed\n", url)
	fmt.Printf("Response status: %s\n", resp.Status)
	fmt.Printf("Response headers: %v\n", resp.Header)
	fmt.Printf("Response body: %s\n", string(body))

	// Pretty print JSON response
	var prettyJSON interface{}
	if err := json.Unmarshal(body, &prettyJSON); err == nil {
		if formatted, err := json.MarshalIndent(prettyJSON, "", "  "); err == nil {
			fmt.Printf("Pretty JSON:\n%s\n", string(formatted))
		}
	}

	return nil
}

func main() {
	fmt.Println("Starting HTTP Client Sample...")

	// Create client using helper function
	client := createClient()
	if client == nil {
		log.Fatal("Failed to create client")
	}

	fmt.Println("Client established successfully")

	// Test upsert_knowledge_graph method
	fmt.Println("\nTesting upsert_knowledge_graph...")
	if err := upsertKnowledgeGraph(client); err != nil {
		log.Printf("Error in upsert_knowledge_graph: %v", err)
		os.Exit(1)
	}

	// Test queryKnowledgeGraphPath method
	fmt.Println("\nTesting queryKnowledgeGraphPath...")
	if err := queryKnowledgeGraphPath(client); err != nil {
		log.Printf("Error in queryKnowledgeGraphPath: %v", err)
		os.Exit(1)
	}

	// Test queryKnowledgeGraphNeighbor method
	fmt.Println("\nTesting queryKnowledgeGraphNeighbor...")
	if err := queryKnowledgeGraphNeighbor(client); err != nil {
		log.Printf("Error in queryKnowledgeGraphNeighbor: %v", err)
		os.Exit(1)
	}

	// Test queryKnowledgeGraphConcept method
	fmt.Println("\nTesting queryKnowledgeGraphConcept...")
	if err := queryKnowledgeGraphConcept(client); err != nil {
		log.Printf("Error in queryKnowledgeGraphConcept: %v", err)
		os.Exit(1)
	}

	// Test deleteKnowledgeGraph method
	fmt.Println("\nTesting deleteKnowledgeGraph...")
	if err := deleteKnowledgeGraph(client); err != nil {
		log.Printf("Error in deleteKnowledgeGraph: %v", err)
		os.Exit(1)
	}

	fmt.Println("Sample completed successfully!")
}
