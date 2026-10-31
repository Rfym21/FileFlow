package middleware

import (
	"reflect"
	"testing"
)

func TestMCPManagementRequiresCompleteTokenScopeAndCurrentRole(t *testing.T) {
	full := []string{"read", "write", "delete", "manage"}
	for _, input := range []struct {
		name, role string
		scopes     []string
		mcp        bool
		want       []string
	}{
		{"owner MCP", "owner", full, true, full},
		{"admin MCP", "admin", full, true, full},
		{"REST", "owner", full, false, full[:3]},
		{"legacy partial MCP", "owner", []string{"read", "manage"}, true, []string{"read"}},
		{"role downgraded", "member", full, true, full[:3]},
		{"viewer", "viewer", full, true, []string{"read"}},
	} {
		t.Run(input.name, func(t *testing.T) {
			if got := effectivePermissions(input.role, input.scopes, input.mcp); !reflect.DeepEqual(got, input.want) {
				t.Fatalf("permissions = %v, want %v", got, input.want)
			}
		})
	}
}
