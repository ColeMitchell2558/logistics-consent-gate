package shipment

import "testing"

func TestDecideShipmentEvent(t *testing.T) {
	tests := []struct {
		name    string
		event   Event
		granted bool
		scope   string
		accept  bool
		wantErr bool
	}{
		{name: "tracking event accepted", event: Event{ShipmentID: "shp_101", Kind: "in_transit"}, granted: true, scope: ScopeEvents, accept: true},
		{name: "proof denied without its grant", event: Event{ShipmentID: "shp_102", Kind: "proof_of_delivery", ProofObjectKey: "pod/shp_102.jpg"}, granted: false, scope: ScopeProof},
		{name: "exception accepted", event: Event{ShipmentID: "shp_103", Kind: "exception", ExceptionReason: "address_review"}, granted: true, scope: ScopeException, accept: true},
		{name: "proof requires an object key", event: Event{ShipmentID: "shp_104", Kind: "proof_of_delivery"}, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			decision, err := Decide(tt.event, tt.granted)
			if (err != nil) != tt.wantErr {
				t.Fatalf("Decide() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err == nil && (decision.Accepted != tt.accept || decision.RequiredScope != tt.scope) {
				t.Fatalf("Decide() = %+v, want accepted=%v scope=%q", decision, tt.accept, tt.scope)
			}
		})
	}
}
