package sub

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func happDeeplink(t *testing.T, prefix string, profile map[string]any) string {
	t.Helper()
	raw, err := json.Marshal(profile)
	if err != nil {
		t.Fatalf("marshal profile: %v", err)
	}
	return prefix + base64.StdEncoding.EncodeToString(raw)
}

func decodeHappDeeplink(t *testing.T, link string) map[string]any {
	t.Helper()
	payload := link[strings.LastIndex(link, "/")+1:]
	raw, err := decodeRoutingBase64(payload)
	if err != nil {
		t.Fatalf("decode %q: %v", link, err)
	}
	var profile map[string]any
	if err := json.Unmarshal(raw, &profile); err != nil {
		t.Fatalf("unmarshal %q: %v", raw, err)
	}
	return profile
}

func ruleList(t *testing.T, profile map[string]any, key string) []string {
	t.Helper()
	values, ok := profile[key].([]any)
	if !ok {
		t.Fatalf("%s = %#v, want a list", key, profile[key])
	}
	out := make([]string, 0, len(values))
	for _, value := range values {
		out = append(out, value.(string))
	}
	return out
}

func TestApplyMarketplaceRoutingMergesIntoProfile(t *testing.T) {
	for _, prefix := range []string{"happ://routing/onadd/", "happ://routing/add/"} {
		t.Run(prefix, func(t *testing.T) {
			in := happDeeplink(t, prefix, map[string]any{
				"Name":        "RoscomVPN",
				"RouteOrder":  "block-proxy-direct",
				"DirectSites": []string{"geosite:category-ru", "domain:wb.ru"},
				"DirectIp":    []string{"geoip:private"},
				"BlockSites":  []string{"geosite:category-ads"},
			})

			out := applyMarketplaceRouting(in)
			if !strings.HasPrefix(out, prefix) {
				t.Fatalf("prefix changed: %q", out)
			}
			profile := decodeHappDeeplink(t, out)

			direct := ruleList(t, profile, "DirectSites")
			if direct[0] != "geosite:category-ru" || direct[1] != "domain:wb.ru" {
				t.Fatalf("existing rules must keep their order, got %v", direct[:2])
			}
			for _, want := range []string{"domain:paywb.com", "domain:ozon.com", "domain:wbcontent.net"} {
				if !slices.Contains(direct, want) {
					t.Fatalf("DirectSites lacks %s: %v", want, direct)
				}
			}
			n := 0
			for _, rule := range direct {
				if rule == "domain:wb.ru" {
					n++
				}
			}
			if n != 1 {
				t.Fatalf("domain:wb.ru appears %d times, want once", n)
			}
			if ip := ruleList(t, profile, "DirectIp"); !slices.Equal(ip, []string{"geoip:private", "185.62.201.0/24", "185.62.202.0/24", "2a03:720::/32"}) {
				t.Fatalf("DirectIp = %v", ip)
			}
			block := ruleList(t, profile, "BlockSites")
			if block[0] != "geosite:category-ads" || !slices.Contains(block, "keyword:appmetrica") {
				t.Fatalf("BlockSites = %v", block)
			}
			if profile["Name"] != "RoscomVPN" || profile["RouteOrder"] != "block-proxy-direct" {
				t.Fatalf("unrelated fields changed: %v", profile)
			}
		})
	}
}

func TestApplyMarketplaceRoutingLeavesOtherValuesAlone(t *testing.T) {
	cases := map[string]string{
		"empty":            "",
		"routing off":      "happ://routing/off",
		"incy deeplink":    "incy://routing/onadd/" + base64.StdEncoding.EncodeToString([]byte(`{"Name":"x"}`)),
		"broken base64":    "happ://routing/onadd/%%%",
		"payload not json": "happ://routing/onadd/" + base64.StdEncoding.EncodeToString([]byte("not json")),
	}
	for name, in := range cases {
		t.Run(name, func(t *testing.T) {
			if got := applyMarketplaceRouting(in); got != in {
				t.Fatalf("applyMarketplaceRouting(%q) = %q, want it unchanged", in, got)
			}
		})
	}
}

func TestApplyCommonHeadersAddsMarketplaceRoutesOnlyWhenEnabled(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rules := happDeeplink(t, "happ://routing/onadd/", map[string]any{"Name": "RoscomVPN", "DirectSites": []string{"geosite:category-ru"}})

	for _, enabled := range []bool{true, false} {
		recorder := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(recorder)
		ctx.Request = httptest.NewRequest(http.MethodGet, "/sub/s1", nil)
		(&SUBController{subRoutingExtras: enabled}).ApplyCommonHeaders(ctx, "", "12", "", "", "", "", true, rules, false)

		got := recorder.Header().Get("Routing")
		direct := ruleList(t, decodeHappDeeplink(t, got), "DirectSites")
		if has := slices.Contains(direct, "domain:paywb.com"); has != enabled {
			t.Fatalf("extras=%v: marketplace domains present=%v in %v", enabled, has, direct)
		}
	}
}
