package sub

import (
	"encoding/base64"
	"encoding/json"
	"slices"
	"strings"
	"testing"
)

func happLink(t *testing.T, prefix string, profile map[string]any) string {
	t.Helper()
	raw, err := json.Marshal(profile)
	if err != nil {
		t.Fatal(err)
	}
	return prefix + base64.StdEncoding.EncodeToString(raw)
}

func decodeHapp(t *testing.T, link string) map[string]any {
	t.Helper()
	i := strings.LastIndex(link, "/")
	raw, err := decodeBase64Loose(link[i+1:])
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	return out
}

func toStrings(v any) []string {
	var out []string
	for _, x := range v.([]any) {
		out = append(out, x.(string))
	}
	return out
}

func TestApplyRoutingExtrasMergesLists(t *testing.T) {
	in := happLink(t, happOnAddPrefix, map[string]any{
		"Name":        "RoscomVPN",
		"DirectSites": []string{"geosite:category-ru", "domain:wb.ru"},
		"DirectIp":    []string{"geoip:private"},
		"BlockSites":  []string{"geosite:category-ads"},
		"RouteOrder":  "block-proxy-direct",
	})

	out := applyRoutingExtras(in)
	if !strings.HasPrefix(out, happOnAddPrefix) {
		t.Fatalf("prefix lost: %q", out[:30])
	}
	p := decodeHapp(t, out)

	direct := toStrings(p["DirectSites"])
	if direct[0] != "geosite:category-ru" {
		t.Errorf("original order not kept: %v", direct[:2])
	}
	for _, want := range []string{"domain:paywb.com", "domain:ozon.com", "domain:wbcontent.net"} {
		if !slices.Contains(direct, want) {
			t.Errorf("DirectSites missing %s", want)
		}
	}
	n := 0
	for _, d := range direct {
		if d == "domain:wb.ru" {
			n++
		}
	}
	if n != 1 {
		t.Errorf("duplicate domain:wb.ru, count=%d", n)
	}
	if !slices.Contains(toStrings(p["DirectIp"]), "185.62.202.0/24") {
		t.Error("DirectIp missing WB subnet")
	}
	block := toStrings(p["BlockSites"])
	if !slices.Contains(block, "keyword:appmetrica") || !slices.Contains(block, "geosite:category-ads") {
		t.Errorf("BlockSites wrong: %v", block)
	}
	if p["RouteOrder"] != "block-proxy-direct" || p["Name"] != "RoscomVPN" {
		t.Error("unrelated fields changed")
	}
}

func TestApplyRoutingExtrasLeavesOtherInputAlone(t *testing.T) {
	for _, in := range []string{"", "https://example.com/rules", "happ://routing/onadd/%%%notbase64", "happ://routing/onadd/" + base64.StdEncoding.EncodeToString([]byte("not json"))} {
		if got := applyRoutingExtras(in); got != in {
			t.Errorf("changed %q -> %q", in, got)
		}
	}
}

func TestRoutingSourceTakesExtras(t *testing.T) {
	cases := map[string]bool{"default": true, "": true, "whitelist": true, "jsonsub": false, "custom": false, " Custom ": false}
	for src, want := range cases {
		if got := routingSourceTakesExtras(src); got != want {
			t.Errorf("%q: got %v want %v", src, got, want)
		}
	}
}

func TestBuildShadowrocketConf(t *testing.T) {
	conf := buildShadowrocketConf("Енот", "https://example.com/sub/abc/shadowrocket.conf")
	for _, want := range []string{
		"[General]", "[Rule]",
		"update-url = https://example.com/sub/abc/shadowrocket.conf",
		"DOMAIN-KEYWORD,appmetrica,REJECT",
		"DOMAIN-SUFFIX,paywb.com,DIRECT",
		"IP-CIDR6,2a03:720::/32,DIRECT,no-resolve",
		"DOMAIN-SUFFIX,ru,DIRECT",
		"GEOIP,RU,DIRECT",
	} {
		if !strings.Contains(conf, want) {
			t.Errorf("config missing %q", want)
		}
	}
	if !strings.HasSuffix(conf, "FINAL,PROXY\n") {
		t.Error("FINAL,PROXY must be the last rule")
	}
	// Trackers must be rejected before any DIRECT rule could match them.
	if strings.Index(conf, "REJECT") > strings.Index(conf, ",DIRECT") {
		t.Error("REJECT rules must come before DIRECT rules")
	}
}
