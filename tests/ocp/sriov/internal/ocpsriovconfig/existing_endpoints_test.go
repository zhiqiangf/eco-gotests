package ocpsriovconfig

import (
	"strings"
	"testing"
)

func TestParseExistingEndpoints(t *testing.T) {
	valid := `{"mcp":"arm-pool","client":{"node":"worker-a.example.test","pf":"net_r0_p0","resource":"plane_0"},"server":{"node":"worker-b.example.test","pf":"net_r0_p1","resource":"plane_1"}}`
	for _, test := range []struct {
		name, raw string
		wantError bool
	}{
		{"crossed link", valid, false},
		{"empty", "", true},
		{"missing pool", strings.Replace(valid, `"arm-pool"`, `""`, 1), true},
		{"missing server", `{"mcp":"arm-pool","client":{"node":"worker-a","pf":"eth1","resource":"plane0"}}`, true},
		{"shell input", strings.Replace(valid, "net_r0_p0", "eth1;id", 1), true},
		{"qualified resource", strings.Replace(valid, "plane_0", "openshift.io/plane_0", 1), true},
		{"unknown field", strings.Replace(valid, `"mcp"`, `"pool"`, 1), true},
		{"trailing object", valid + `{}`, true},
		{"same PF", strings.ReplaceAll(strings.ReplaceAll(valid, "worker-b", "worker-a"), "net_r0_p1", "net_r0_p0"), true},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := ParseExistingEndpoints(test.raw)
			if (err != nil) != test.wantError {
				t.Fatalf("error=%v, wantError=%v", err, test.wantError)
			}
		})
	}
}
