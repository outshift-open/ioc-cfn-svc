package sharedmemory

type SharedMemoryUpsertRequest struct {
	Memories      []map[string]any `json:"memories"`
	Relationships []map[string]any `json:"relationships"`
}

type SharedMemoryQueryRequest struct {
	// TODO: define fields
}

type SharedMemoryUpsertResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

type SharedMemoryQueryResponse struct {
	// TODO: define fields
}