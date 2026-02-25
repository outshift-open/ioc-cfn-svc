// Package main provides a sample / smoke-test client that exercises all
// mem0 Agentic Memory Client operations against a live OpenMemory endpoint.
//
// Usage:
//
//	export MEM0_API_KEY="test"                         # required (any value for self-hosted)
//	export MEM0_BASE_URL="http://localhost:8765"       # your Docker mem0 endpoint
//	export MEM0_USER_ID="shkurapa"                     # user to test with
//	go run pkg/providers/memory/ioc/mem0/mem0_testclient/testclient.go
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/joho/godotenv"

	mem0client "github.com/cisco-eti/ioc-cfn-svc/pkg/providers/memory/ioc/mem0"
)

func main() {
	fmt.Println("=== Mem0 Agentic Memory Client — Test Client ===")

	// Load .env if present (non-fatal if missing)
	if err := godotenv.Load(); err != nil {
		fmt.Printf("Note: no .env file loaded: %v\n", err)
	}

	// Read configuration from environment
	apiKey := os.Getenv("MEM0_API_KEY")
	if apiKey == "" {
		apiKey = "test" // self-hosted OpenMemory doesn't require a real key
		fmt.Println("MEM0_API_KEY not set, using default 'test' for self-hosted")
	}

	baseURL := os.Getenv("MEM0_BASE_URL")
	if baseURL == "" {
		baseURL = mem0client.DefaultBaseURL
	}

	userID := os.Getenv("MEM0_USER_ID")
	if userID == "" {
		userID = "testuser"
	}

	fmt.Printf("Endpoint : %s\n", baseURL)
	fmt.Printf("User ID  : %s\n\n", userID)

	// Create the client — use longer timeout for write ops that involve LLM inference
	cfg := mem0client.DefaultClientConfig()
	cfg.BaseURL = baseURL
	cfg.APIKey = apiKey // sourced from environment, never hardcoded
	cfg.Timeout = 120 * time.Second
	cfg.MaxRetries = 1

	client, err := mem0client.NewClient(cfg)
	if err != nil {
		log.Fatalf("Failed to create mem0 client: %v", err)
	}
	fmt.Println("Client created successfully ✓")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	// --- 1. Create Memory ---
	fmt.Println("\n--- 1. Create Memory ---")
	createResp, err := client.CreateMemory(ctx, &mem0client.CreateMemoryRequest{
		Text:   "I prefer dark mode in all my applications",
		UserID: userID,
	})
	if err != nil {
		log.Printf("Create memory failed: %v", err)
	} else {
		printJSON("Create Memory Response", createResp)
	}

	// Capture a memory ID for subsequent operations
	var memoryID string
	if createResp != nil {
		memoryID = createResp.ID
		fmt.Printf("Captured memory ID: %s\n", memoryID)
	} else if err == nil {
		fmt.Println("Memory was a duplicate — no new memory created (this is expected)")
	}

	// --- 2. List Memories ---
	fmt.Println("\n--- 2. List Memories ---")
	listResp, err := client.ListMemories(ctx, &mem0client.ListMemoriesRequest{
		UserID: userID,
	})
	if err != nil {
		log.Printf("List memories failed: %v", err)
	} else {
		fmt.Printf("Total memories: %d (page %d/%d)\n", listResp.Total, listResp.Page, listResp.Pages)
		for _, item := range listResp.Items {
			fmt.Printf("  - [%s] %s (state=%s)\n", item.ID[:8], item.Content, item.State)
		}
	}

	// --- 3. Get Single Memory ---
	if memoryID != "" {
		fmt.Println("\n--- 3. Get Single Memory ---")
		getResp, err := client.GetMemory(ctx, memoryID)
		if err != nil {
			log.Printf("Get memory failed: %v", err)
		} else {
			printJSON("Get Memory Response", getResp)
		}
	}

	// --- 4. Filter/Search Memories ---
	fmt.Println("\n--- 4. Filter Memories ---")
	filterResp, err := client.FilterMemories(ctx, &mem0client.FilterMemoriesRequest{
		Query:  "preferences",
		UserID: userID,
	})
	if err != nil {
		log.Printf("Filter memories failed: %v", err)
	} else {
		fmt.Printf("Filter results: %d matches\n", filterResp.Total)
		for _, item := range filterResp.Items {
			fmt.Printf("  - [%s] %s\n", item.ID[:8], item.Content)
		}
	}

	// --- 5. Get Categories ---
	fmt.Println("\n--- 5. Get Categories ---")
	catResp, err := client.GetCategories(ctx, userID)
	if err != nil {
		log.Printf("Get categories failed: %v", err)
	} else {
		fmt.Printf("Categories (%d):\n", catResp.Total)
		for _, cat := range catResp.Categories {
			fmt.Printf("  - %s: %s\n", cat.Name, cat.Description)
		}
	}

	// --- 6. Update Memory ---
	if memoryID != "" {
		fmt.Println("\n--- 6. Update Memory ---")
		updateResp, err := client.UpdateMemory(ctx, memoryID, &mem0client.UpdateMemoryRequest{
			MemoryContent: "Strongly prefers dark mode across all applications and IDEs",
			UserID:        userID,
		})
		if err != nil {
			log.Printf("Update memory failed: %v", err)
		} else {
			printJSON("Update Memory Response", updateResp)
		}
	}

	// --- 7. Get Stats ---
	fmt.Println("\n--- 7. Get Stats ---")
	statsResp, err := client.GetStats(ctx, userID)
	if err != nil {
		log.Printf("Get stats failed: %v", err)
	} else {
		printJSON("Stats Response", statsResp)
	}

	// --- 8. ForwardRequest (proxy mode) ---
	fmt.Println("\n--- 8. ForwardRequest (proxy mode) ---")
	proxyBody, _ := json.Marshal(map[string]interface{}{
		"text":    "I like Go programming language",
		"user_id": userID,
	})
	proxyResp, err := client.ForwardRequest(ctx, "POST",
		baseURL+"/api/v1/memories/",
		proxyBody,
		nil,
	)
	if err != nil {
		log.Printf("ForwardRequest failed: %v", err)
	} else {
		printJSON("ForwardRequest Response", proxyResp)
	}

	// Capture proxy-created memory ID for cleanup
	var proxyMemoryID string
	if proxyResp != nil && proxyResp.HTTPResponseBody != nil {
		if id, ok := proxyResp.HTTPResponseBody["id"].(string); ok {
			proxyMemoryID = id
		}
	}

	// --- 9. Delete Memories (cleanup) ---
	fmt.Println("\n--- 9. Delete Memories (cleanup) ---")
	idsToDelete := []string{}
	if memoryID != "" {
		idsToDelete = append(idsToDelete, memoryID)
	}
	if proxyMemoryID != "" {
		idsToDelete = append(idsToDelete, proxyMemoryID)
	}
	if len(idsToDelete) > 0 {
		delResp, err := client.DeleteMemories(ctx, &mem0client.DeleteMemoriesRequest{
			MemoryIDs: idsToDelete,
			UserID:    userID,
		})
		if err != nil {
			log.Printf("Delete memories failed: %v", err)
		} else {
			printJSON("Delete Memories Response", delResp)
		}
	} else {
		fmt.Println("No memories to clean up")
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
