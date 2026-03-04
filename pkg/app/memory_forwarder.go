package app

import (
	"context"

	graphiticlient "github.com/cisco-eti/ioc-cfn-svc/pkg/providers/memory/ioc/graphiti"
	mem0client "github.com/cisco-eti/ioc-cfn-svc/pkg/providers/memory/ioc/mem0"
)

// Memory provider type constants.
const (
	ProviderMem0     = "mem0"
	ProviderGraphiti = "graphiti"
)

// memoryProxyResponse is the provider-agnostic response from a forwarded memory request.
type memoryProxyResponse struct {
	HTTPStatus       int
	HTTPHeaders      map[string]string
	HTTPResponseBody map[string]interface{}
}

// memoryForwarder proxies HTTP requests to a memory provider.
type memoryForwarder interface {
	ForwardRequest(ctx context.Context, method, targetURL string, body []byte, headers map[string]string) (*memoryProxyResponse, error)
}

// mem0Forwarder adapts the mem0 ProxyClient to the memoryForwarder interface.
type mem0Forwarder struct {
	client *mem0client.ProxyClient
}

func (f *mem0Forwarder) ForwardRequest(ctx context.Context, method, targetURL string, body []byte, headers map[string]string) (*memoryProxyResponse, error) {
	resp, err := f.client.ForwardRequest(ctx, method, targetURL, body, headers)
	if err != nil {
		return nil, err
	}
	return &memoryProxyResponse{
		HTTPStatus:       resp.HTTPStatus,
		HTTPHeaders:      resp.HTTPHeaders,
		HTTPResponseBody: resp.HTTPResponseBody,
	}, nil
}

// graphitiForwarder adapts the Graphiti ProxyClient to the memoryForwarder interface.
type graphitiForwarder struct {
	client *graphiticlient.ProxyClient
}

func (f *graphitiForwarder) ForwardRequest(ctx context.Context, method, targetURL string, body []byte, headers map[string]string) (*memoryProxyResponse, error) {
	resp, err := f.client.ForwardRequest(ctx, method, targetURL, body, headers)
	if err != nil {
		return nil, err
	}
	return &memoryProxyResponse{
		HTTPStatus:       resp.HTTPStatus,
		HTTPHeaders:      resp.HTTPHeaders,
		HTTPResponseBody: resp.HTTPResponseBody,
	}, nil
}
