// Package main provides a sample / smoke-test client that exercises all
// Graphiti Knowledge Graph operations against a live Graphiti server endpoint.
//
// Usage:
//
//	export GRAPHITI_BASE_URL="http://localhost:8000"       # your Docker graphiti endpoint
//	export GRAPHITI_GROUP_ID="test-group"                  # group to test with
//	go run pkg/providers/memory/ioc/graphiti/graphiti_testclient/testclient.go
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/joho/godotenv"

	graphiticlient "github.com/cisco-eti/ioc-cfn-svc/pkg/providers/memory/ioc/graphiti"
)

func main() {
	fmt.Println("=== Graphiti Knowledge Graph — Test Client ===")

	if err := godotenv.Load(); err != nil {
		fmt.Printf("Note: no .env file loaded: %v\n", err)
	}

	baseURL := os.Getenv("GRAPHITI_BASE_URL")
	if baseURL == "" {
		baseURL = "http://localhost:8000"
	}

	groupID := os.Getenv("GRAPHITI_GROUP_ID")
	if groupID == "" {
		groupID = "test-group"
	}

	fmt.Printf("Endpoint : %s\n", baseURL)
	fmt.Printf("Group ID : %s\n\n", groupID)

	cfg := graphiticlient.DefaultProxyClientConfig()
	client := graphiticlient.NewProxyClient(cfg)
	fmt.Println("Client created successfully")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	// --- 1. Health Check ---
	fmt.Println("\n--- 1. Health Check ---")
	healthResp, err := client.HealthCheck(ctx, baseURL)
	if err != nil {
		log.Printf("Health check failed: %v", err)
	} else {
		printJSON("Health Check Response", healthResp)
	}

	// --- 2. Add Messages ---
	fmt.Println("\n--- 2. Add Messages ---")
	addMsgResp, err := client.AddMessages(ctx, baseURL, &graphiticlient.AddMessagesRequest{
		GroupID: groupID,
		Messages: []graphiticlient.Message{
			{
				Content:  "I need help debugging my Go application",
				RoleType: "user",
				Role:     strPtr("developer"),
			},
			{
				Content:  "I can help with Go debugging. What error are you seeing?",
				RoleType: "assistant",
				Role:     strPtr("assistant"),
			},
			{
				Content:  "I'm getting a nil pointer dereference in my HTTP handler",
				RoleType: "user",
				Role:     strPtr("developer"),
			},
		},
	})
	if err != nil {
		log.Printf("Add messages failed: %v", err)
	} else {
		printJSON("Add Messages Response", addMsgResp)
	}

	// --- 3. Add Entity Node ---
	fmt.Println("\n--- 3. Add Entity Node ---")
	entityResp, err := client.AddEntityNode(ctx, baseURL, &graphiticlient.AddEntityNodeRequest{
		UUID:    "test-entity-001",
		GroupID: groupID,
		Name:    "Go Application",
		Summary: "A Go-based HTTP server application being debugged",
	})
	if err != nil {
		log.Printf("Add entity node failed: %v", err)
	} else {
		printJSON("Add Entity Node Response", entityResp)
	}

	// --- 4. Search ---
	fmt.Println("\n--- 4. Search ---")
	searchResp, err := client.Search(ctx, baseURL, &graphiticlient.SearchQuery{
		GroupIDs: []string{groupID},
		Query:    "debugging Go application",
		MaxFacts: 5,
	})
	if err != nil {
		log.Printf("Search failed: %v", err)
	} else {
		fmt.Printf("Search results: %d facts\n", len(searchResp.Facts))
		for _, fact := range searchResp.Facts {
			fmt.Printf("  - [%s] %s: %s\n", fact.UUID[:8], fact.Name, fact.Fact)
		}
	}

	// --- 5. Get Memory ---
	fmt.Println("\n--- 5. Get Memory ---")
	memoryResp, err := client.GetMemory(ctx, baseURL, &graphiticlient.GetMemoryRequest{
		GroupID:  groupID,
		MaxFacts: 5,
		Messages: []graphiticlient.Message{
			{
				Content:  "What do you know about the Go debugging issue?",
				RoleType: "user",
				Role:     strPtr("developer"),
			},
		},
	})
	if err != nil {
		log.Printf("Get memory failed: %v", err)
	} else {
		fmt.Printf("Memory facts: %d\n", len(memoryResp.Facts))
		for _, fact := range memoryResp.Facts {
			fmt.Printf("  - [%s] %s\n", fact.UUID[:8], fact.Fact)
		}
	}

	// --- 6. Get Episodes ---
	fmt.Println("\n--- 6. Get Episodes ---")
	episodes, err := client.GetEpisodes(ctx, baseURL, groupID, 5)
	if err != nil {
		log.Printf("Get episodes failed: %v", err)
	} else {
		fmt.Printf("Episodes retrieved: %d\n", len(episodes))
		printJSON("Episodes", episodes)
	}

	// --- 7. ForwardRequest (generic proxy mode) ---
	fmt.Println("\n--- 7. ForwardRequest (generic proxy mode) ---")
	proxyBody, _ := json.Marshal(map[string]interface{}{
		"group_ids": []string{groupID},
		"query":     "nil pointer dereference",
		"max_facts": 3,
	})
	proxyResp, err := client.ForwardRequest(ctx, "POST",
		baseURL+"/search",
		proxyBody,
		nil,
	)
	if err != nil {
		log.Printf("ForwardRequest failed: %v", err)
	} else {
		printJSON("ForwardRequest Response", proxyResp)
	}

	// --- 8. Delete Group (cleanup) ---
	fmt.Println("\n--- 8. Delete Group (cleanup) ---")
	delResp, err := client.DeleteGroup(ctx, baseURL, groupID)
	if err != nil {
		log.Printf("Delete group failed: %v", err)
	} else {
		printJSON("Delete Group Response", delResp)
	}

	fmt.Println("\n=== Test Client completed ===")
}

func printJSON(label string, v interface{}) {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		fmt.Printf("%s: (marshal error: %v)\n", label, err)
		return
	}
	fmt.Printf("%s:\n%s\n", label, string(data))
}

func strPtr(s string) *string {
	return &s
}
