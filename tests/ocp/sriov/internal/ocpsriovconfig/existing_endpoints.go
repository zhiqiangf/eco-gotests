package ocpsriovconfig

import (
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"strings"

	"k8s.io/apimachinery/pkg/util/validation"
)

// ExistingEndpointsEnv opts into attachment tests against administrator-owned VFs.
const ExistingEndpointsEnv = "ECO_OCP_SRIOV_EXISTING_ENDPOINTS"

// ExistingEndpointsLabel isolates these adapted tests from policy lifecycle tests.
const ExistingEndpointsLabel = "existing-endpoints"

// ExistingEndpoint explicitly maps one existing PF resource to a worker.
type ExistingEndpoint struct {
	Node     string `json:"node"`
	PF       string `json:"pf"`
	Resource string `json:"resource"`
}

// ExistingEndpoints supports crossed physical links without renaming host PFs.
type ExistingEndpoints struct {
	MCP    string           `json:"mcp"`
	Client ExistingEndpoint `json:"client"`
	Server ExistingEndpoint `json:"server"`
}

// ParseExistingEndpoints rejects incomplete plans and shell metacharacters before API access.
func ParseExistingEndpoints(raw string) (*ExistingEndpoints, error) {
	var plan ExistingEndpoints
	decoder := json.NewDecoder(strings.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&plan); err != nil {
		return nil, fmt.Errorf("decode existing endpoints: %w", err)
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return nil, fmt.Errorf("expected a single endpoint plan")
	}
	if len(validation.IsDNS1123Subdomain(plan.MCP)) != 0 {
		return nil, fmt.Errorf("mcp must be an explicit valid pool name")
	}
	pfPattern := regexp.MustCompile(`^[a-zA-Z0-9_.:-]{1,15}$`)
	resourcePattern := regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)
	for _, endpoint := range []ExistingEndpoint{plan.Client, plan.Server} {
		if len(validation.IsDNS1123Subdomain(endpoint.Node)) != 0 ||
			!pfPattern.MatchString(endpoint.PF) || !resourcePattern.MatchString(endpoint.Resource) {
			return nil, fmt.Errorf("each endpoint requires a valid node, PF and unqualified resource name")
		}
	}
	if plan.Client.Node == plan.Server.Node && plan.Client.PF == plan.Server.PF {
		return nil, fmt.Errorf("endpoints must use distinct PFs")
	}
	return &plan, nil
}
