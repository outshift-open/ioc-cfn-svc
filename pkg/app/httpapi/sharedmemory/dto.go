package sharedmemory

import (
	"fmt"

	"github.com/cisco-eti/ioc-cfn-svc/pkg/client/cognitionagentclient"
	"github.com/cisco-eti/ioc-cfn-svc/pkg/common"
)

type Header struct {
	// ID that represents the agent, optional
	AgentID *string `json:"agent_id,omitempty"`
}

type CreateOrUpdateRequest struct {
	// Header(s) of the request, optional.
	Header *Header `json:"header,omitempty"`
	// ID of the request, optional.
	// If not provided, a random UUID is used to represent the request.
	RequestId *string `json:"request_id,omitempty"`

	// Payload contains the extraction metadata and the raw data to be processed.
	// The structure of the payload data is defined by Payload.Metadata.Format.
	Payload cognitionagentclient.ExtractionPayload `json:"payload"`
}

type CreateOrUpdateResponse struct {
	ResponseID *string `json:"response_id,omitempty" description:"ID of the response, this gets populated from request_id"`
	Status     string  `json:"status" description:"Status of the request"`
	Message    *string `json:"message,omitempty" description:"Optional message providing additional information"`
}

type QueryRequest struct {
	// Header(s) of the request, optional.
	Header *Header `json:"header,omitempty"`
	// ID of the request, optional.
	// If not provided, a random UUID is used to represent the request.
	RequestId *string `json:"request_id,omitempty"`
	// Search strategy to be used when executing the query.
	// Currently supported values:
	//   - "semantic_graph_traversal"
	//
	// If not specified, the service will use the default search strategy.
	SearchStrategy *string `json:"search_strategy,omitempty"`

	// User intent or natural-language query describing what information is being requested.
	// This field is the primary signal used to construct and execute the query.
	Intent *string `json:"intent,omitempty"`

	// TODO: not sure if we allow users to specify query type along with specified node IDs
	//NodeIDs           *[]string                                      `json:"node_ids,omitempty"`        // node ID(s) must be provided if query_type is "neighbor" or "path". Node ID(s) is ignored is query_type is set to be "concept"
	//QueryCriteria     *iocmemoryprovider.KnowledgeGraphQueryCriteria `json:"query_criteria,omitempty"`

	// AdditionalContext provides optional contextual information to refine query execution.
	// This may include prior conversation state, structured hints, or domain-specific metadata.
	// The contents are treated as opaque by the API and interpreted by downstream components.
	AdditionalContext []interface{} `json:"additional_context,omitempty"`
}

const (
	SearchStrategySemanticGraphTraversal = "semantic_graph_traversal"
)

// SearchStrategyConvertMap Reasoning service is using ""Semantic Graph Traversal" for its validation, hence we need a conversion here
var SearchStrategyConvertMap = map[string]string{
	SearchStrategySemanticGraphTraversal: "Semantic Graph Traversal",
}

func (r *QueryRequest) ValidateAndApplyDefault() error {
	if r.SearchStrategy == nil {
		r.SearchStrategy = common.StrToPtr(SearchStrategySemanticGraphTraversal)
	}

	if r.SearchStrategy != nil && *r.SearchStrategy != SearchStrategySemanticGraphTraversal {
		return fmt.Errorf("invalid search_strategy, valid value is %s", SearchStrategySemanticGraphTraversal)
	}
	//
	//if r.QueryCriteria == nil {
	//	useDirection := false // false for undirected path, true for directed path
	//	r.QueryCriteria = iocmemoryprovider.NewKnowledgeGraphQueryCriteria(
	//		iocmemoryprovider.QueryTypeConcept,
	//		nil, // unspecified depth will return paths of any length
	//		&useDirection,
	//	)
	//}
	//
	//if r.QueryCriteria.QueryType != iocmemoryprovider.QueryTypeConcept &&
	//	r.QueryCriteria.QueryType != iocmemoryprovider.QueryTypeNeighbour &&
	//	r.QueryCriteria.QueryType != iocmemoryprovider.QueryTypePath {
	//	return fmt.Errorf("invalid query_type, valid values are: %s, %s, %s",
	//		iocmemoryprovider.QueryTypeConcept,
	//		iocmemoryprovider.QueryTypeNeighbour,
	//		iocmemoryprovider.QueryTypePath,
	//	)
	//}
	//
	//if r.QueryCriteria.QueryType == iocmemoryprovider.QueryTypeNeighbour ||
	//	r.QueryCriteria.QueryType == iocmemoryprovider.QueryTypePath {
	//
	//	if r.NodeIDs == nil || strings.TrimSpace(*r.NodeIDs) == "" {
	//		return fmt.Errorf("node_ids must be provided when query_type is %s or %s",
	//			iocmemoryprovider.QueryTypeNeighbour,
	//			iocmemoryprovider.QueryTypePath,
	//		)
	//	}
	//}

	return nil
}

type QueryResponse struct {
	ResponseID *string               `json:"response_id,omitempty" description:"ID of the response, this gets populated from request_id"`
	Status     string                `json:"status" description:"Status of the request"`
	Message    *string               `json:"message,omitempty" description:"Optional message providing additional information"`
	Records    []QueryResponseRecord `json:"records,omitempty" description:"Query response records (only included for success status)"`
}

type QueryResponseRecord struct {
	Relationships []QueryRelation `json:"relationships,omitempty"`
	Concepts      []QueryConcept  `json:"concepts,omitempty"`
}

type QueryConcept struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Description *string                `json:"description,omitempty"`
	Attributes  map[string]interface{} `json:"attributes,omitempty"`
	Tags        []string               `json:"tags,omitempty"`
}

type QueryRelation struct {
	ID         string                 `json:"id"`
	Relation   string                 `json:"relation"`
	NodeIDs    []string               `json:"node_ids"`
	Attributes map[string]interface{} `json:"attributes,omitempty"`
}
