package shipment

import "fmt"

const (
	ScopeEvents    = "shipment_events"
	ScopeProof     = "third_party"
	ScopeException = "shipment_exceptions"
)

type Event struct {
	UserID          string `json:"user_id"`
	ShipmentID      string `json:"shipment_id"`
	Kind            string `json:"kind"`
	OccurredAt      string `json:"occurred_at"`
	ProofObjectKey  string `json:"proof_object_key,omitempty"`
	ExceptionReason string `json:"exception_reason,omitempty"`
}

type Decision struct {
	Accepted      bool   `json:"accepted"`
	RequiredScope string `json:"required_scope"`
	ShipmentID    string `json:"shipment_id"`
}

func RequiredScope(event Event) (string, error) {
	switch event.Kind {
	case "in_transit", "delivered":
		return ScopeEvents, nil
	case "proof_of_delivery":
		if event.ProofObjectKey == "" {
			return "", fmt.Errorf("proof_object_key is required")
		}
		return ScopeProof, nil
	case "exception":
		if event.ExceptionReason == "" {
			return "", fmt.Errorf("exception_reason is required")
		}
		return ScopeException, nil
	default:
		return "", fmt.Errorf("unsupported shipment event kind %q", event.Kind)
	}
}

func Decide(event Event, granted bool) (Decision, error) {
	scope, err := RequiredScope(event)
	if err != nil {
		return Decision{}, err
	}
	return Decision{Accepted: granted, RequiredScope: scope, ShipmentID: event.ShipmentID}, nil
}
