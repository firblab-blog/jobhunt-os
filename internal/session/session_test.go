package session

import (
	"strings"
	"testing"
	"time"
)

func TestPolicyValidate(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		policy  Policy
		wantErr string
	}{
		"valid": {
			policy: Policy{
				IdleTimeout:     15 * time.Minute,
				AbsoluteTimeout: 24 * time.Hour,
			},
		},
		"zero idle timeout": {
			policy: Policy{
				AbsoluteTimeout: 24 * time.Hour,
			},
			wantErr: "idle timeout must be positive",
		},
		"negative absolute timeout": {
			policy: Policy{
				IdleTimeout:     15 * time.Minute,
				AbsoluteTimeout: -time.Second,
			},
			wantErr: "absolute timeout must be positive",
		},
	} {
		name, tc := name, tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			err := tc.policy.Validate()
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("Validate() error = %v, want nil", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("Validate() error = %v, want containing %q", err, tc.wantErr)
			}
		})
	}
}
