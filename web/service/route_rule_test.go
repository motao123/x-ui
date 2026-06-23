package service

import (
	"encoding/json"
	"strings"
	"testing"
	"x-ui/database/model"
	"x-ui/util/json_util"
)

func TestCompileRoutingAppendsEnabledRules(t *testing.T) {
	service := &RouteRuleService{}
	template := json_util.RawMessage(`{"rules":[{"type":"field","outboundTag":"api","inboundTag":["api"]}]}`)
	rules := []*model.RouteRule{{Enable: true, Domain: "geosite:google,domain:example.com", OutboundTag: "blocked"}}
	compiled, err := compileRouting(template, rules)
	if err != nil {
		t.Fatal(err)
	}
	text := string(compiled)
	if !strings.Contains(text, "geosite:google") || !strings.Contains(text, "domain:example.com") {
		t.Fatalf("compiled routing missing domain rules: %s", text)
	}
	var routing struct {
		Rules []map[string]interface{} `json:"rules"`
	}
	if err := json.Unmarshal(compiled, &routing); err != nil {
		t.Fatal(err)
	}
	if len(routing.Rules) != 2 {
		t.Fatalf("expected 2 rules, got %d", len(routing.Rules))
	}
	_ = service
}
