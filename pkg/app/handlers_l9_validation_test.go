package app

import (
	"testing"

	l9 "github.com/outshift-open/ioc-protocols-models/SSTP/language_bindings/golang"
)

func TestValidateKindSubkindCombination(t *testing.T) {
	tests := []struct {
		name      string
		kind      l9.Kind
		subkind   interface{}
		expectErr bool
	}{
		// Valid combinations
		{name: "knowledge+query", kind: l9.KindKnowledge, subkind: "query", expectErr: false},
		{name: "knowledge+distillation", kind: l9.KindKnowledge, subkind: "distillation", expectErr: false},
		{name: "knowledge+extraction", kind: l9.KindKnowledge, subkind: "extraction", expectErr: false},
		{name: "knowledge+feedback", kind: l9.KindKnowledge, subkind: "feedback", expectErr: false},
		{name: "commit+converged", kind: l9.KindCommit, subkind: "converged", expectErr: false},
		{name: "commit+resolved", kind: l9.KindCommit, subkind: "resolved", expectErr: false},
		{name: "commit+abort", kind: l9.KindCommit, subkind: "abort", expectErr: false},
		{name: "intent+coordinator-assignment", kind: l9.KindIntent, subkind: "coordinator-assignment", expectErr: false},
		{name: "intent+mission", kind: l9.KindIntent, subkind: "mission", expectErr: false},
		{name: "exchange+team-formation", kind: l9.KindExchange, subkind: "team-formation", expectErr: false},
		{name: "contingency+negotiation", kind: l9.KindContingency, subkind: "negotiation", expectErr: false},
		{name: "knowledge+no-subkind", kind: l9.KindKnowledge, subkind: nil, expectErr: false},

		// Invalid combinations
		{name: "knowledge+invalid", kind: l9.KindKnowledge, subkind: "invalid", expectErr: true},
		{name: "commit+query", kind: l9.KindCommit, subkind: "query", expectErr: true},
		{name: "intent+negotiation", kind: l9.KindIntent, subkind: "negotiation", expectErr: true},
		{name: "exchange+mission", kind: l9.KindExchange, subkind: "mission", expectErr: true},
		{name: "contingency+query", kind: l9.KindContingency, subkind: "query", expectErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := &l9.L9{
				Header: l9.L9Header{
					Kind:    tt.kind,
					Subkind: tt.subkind,
				},
			}

			err := validateKindSubkindCombination(msg)

			if tt.expectErr {
				if err == nil {
					t.Errorf("expected error for %s+%v, got nil", tt.kind, tt.subkind)
				} else {
					t.Logf("✓ Correctly rejected %s+%v: %v", tt.kind, tt.subkind, err)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error for valid %s+%v: %v", tt.kind, tt.subkind, err)
				} else {
					t.Logf("✓ Correctly accepted %s+%v", tt.kind, tt.subkind)
				}
			}
		})
	}
}
