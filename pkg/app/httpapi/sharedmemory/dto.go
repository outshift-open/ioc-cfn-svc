package sharedmemory

import (
	"fmt"

	"github.com/cisco-eti/ioc-cfn-svc/pkg/client/cognitionagentclient"
	iocmemoryprovider "github.com/cisco-eti/ioc-cfn-svc/pkg/providers/memory/ioc"
)

type UpsertRequest struct {
	// optional
	AgentId   *string `json:"agent_id,omitempty"`
	RequestId *string `json:"request_id,omitempty"`

	Payload cognitionagentclient.ExtractionPayload `json:"payload"`
}

type UpsertResponse struct {
	ResponseID *string `json:"response_id,omitempty" description:"ID of the response, this gets populated from request_id"`
	Status     string  `json:"status" description:"Status of the request"`
	Message    *string `json:"message,omitempty" description:"Optional message providing additional information"`
}

type QueryRequest struct {
	AgentId        *string `json:"agent_id,omitempty"`
	RequestId      *string `json:"request_id,omitempty"`
	SearchStrategy *string `json:"search_strategy,omitempty"` // only supported search strategy: "semantic_graph_traversal"
	Intent         *string `json:"intent,omitempty" description:"user message/intent"`
	// TODO: not sure if we allow users to specify query type along with specified node IDs
	//NodeIDs           *[]string                                      `json:"node_ids,omitempty"`        // node ID(s) must be provided if query_type is "neighbor" or "path". Node ID(s) is ignored is query_type is set to be "concept"
	//QueryCriteria     *iocmemoryprovider.KnowledgeGraphQueryCriteria `json:"query_criteria,omitempty"`
	AdditionalContext []interface{} `json:"additional_context,omitempty"`
}

const (
	SearchStrategySemanticGraphTraversal = "semantic_graph_traversal"
)

// SearchStrategyConvertMap Reasoning service is using ""Semantic Graph Traversal" for its validation, hence we need a conversion here
var SearchStrategyConvertMap = map[string]string{
	SearchStrategySemanticGraphTraversal: "Semantic Graph Traversal",
}

func (r *QueryRequest) Validate() error {
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
	//	r.QueryCriteria.QueryType != iocmemoryprovider.QueryTypeNeighbor &&
	//	r.QueryCriteria.QueryType != iocmemoryprovider.QueryTypePath {
	//	return fmt.Errorf("invalid query_type, valid values are: %s, %s, %s",
	//		iocmemoryprovider.QueryTypeConcept,
	//		iocmemoryprovider.QueryTypeNeighbor,
	//		iocmemoryprovider.QueryTypePath,
	//	)
	//}
	//
	//if r.QueryCriteria.QueryType == iocmemoryprovider.QueryTypeNeighbor ||
	//	r.QueryCriteria.QueryType == iocmemoryprovider.QueryTypePath {
	//
	//	if r.NodeIDs == nil || strings.TrimSpace(*r.NodeIDs) == "" {
	//		return fmt.Errorf("node_ids must be provided when query_type is %s or %s",
	//			iocmemoryprovider.QueryTypeNeighbor,
	//			iocmemoryprovider.QueryTypePath,
	//		)
	//	}
	//}

	return nil
}

type QueryResponse struct {
	ResponseID *string                                               `json:"response_id,omitempty" description:"ID of the response, this gets populated from request_id"`
	Status     string                                                `json:"status" description:"Status of the request"`
	Message    *string                                               `json:"message,omitempty" description:"Optional message providing additional information"`
	Records    []iocmemoryprovider.KnowledgeGraphQueryResponseRecord `json:"records,omitempty" description:"Query response records (only included for success status)"`
}
