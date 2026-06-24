package service

import (
	"encoding/json"
	"testing"
	"x-ui/database/model"
	"x-ui/util/json_util"
)

func TestCompileOutboundsAppendsEnabledWireGuardEndpoint(t *testing.T) {
	template := json_util.RawMessage(`[{"tag":"direct","protocol":"freedom"}]`)
	endpoints := []*model.Endpoint{
		{Enable: true, Type: "wireguard", Tag: "warp", Address: "172.16.0.2/32,2606:4700:110::1/128", Endpoint: "engage.cloudflareclient.com:2408", SecretKey: "secret", PublicKey: "public", Reserved: "1,2,3", Mtu: 1420},
		{Enable: false, Type: "wireguard", Tag: "disabled", Address: "172.16.0.3/32", SecretKey: "secret", Mtu: 1420},
	}
	compiled, err := compileOutbounds(template, endpoints)
	if err != nil {
		t.Fatal(err)
	}
	var outbounds []map[string]interface{}
	if err := json.Unmarshal(compiled, &outbounds); err != nil {
		t.Fatal(err)
	}
	if len(outbounds) != 2 {
		t.Fatalf("expected 2 outbounds, got %d", len(outbounds))
	}
	if outbounds[1]["tag"] != "warp" || outbounds[1]["protocol"] != "wireguard" {
		t.Fatalf("unexpected endpoint outbound: %#v", outbounds[1])
	}
	settings, ok := outbounds[1]["settings"].(map[string]interface{})
	if !ok || settings["secretKey"] != "secret" {
		t.Fatalf("unexpected wireguard settings: %#v", outbounds[1]["settings"])
	}
}
