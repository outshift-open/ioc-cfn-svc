package memoryoperations

// MemoryOperationRequest represents the request to interact with remote agent memory
type MemoryOperationRequest struct {
	Header  *MemoryOperationHeader `json:"header,omitempty"`
	Payload MemoryOperationPayload `json:"payload"`
}

// MemoryOperationHeader is an optional header element
// In the future this may become the SSTP header
type MemoryOperationHeader struct {
	// Optional fields for future use
}

// MemoryOperationPayload contains the HTTP request details to forward to the memory provider
type MemoryOperationPayload struct {
	HTTPRequestType string                 `json:"http-request-type"` // POST, PUT, GET, DELETE, etc.
	HTTPURL         string                 `json:"http-url"`          // URL with query parameters (URL encoded)
	HTTPRequestBody map[string]interface{} `json:"http-request-body"` // JSON payload
	HTTPHeaders     map[string]string      `json:"http-headers"`      // Custom headers
}

// MemoryOperationResponse represents the response from the memory provider
type MemoryOperationResponse struct {
	HTTPStatus       int                    `json:"http-status"`
	HTTPHeaders      map[string]string      `json:"http-headers"`
	HTTPResponseBody map[string]interface{} `json:"http-response-body"`
}