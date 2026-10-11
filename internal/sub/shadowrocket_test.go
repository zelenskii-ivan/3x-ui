package sub

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestBuildShadowrocketConfRuleOrder(t *testing.T) {
	conf := buildShadowrocketConf("Enot", "https://sub.example.com/sub/s1/shadowrocket.conf")

	for _, want := range []string{
		"update-url = https://sub.example.com/sub/s1/shadowrocket.conf\n",
		"DOMAIN-KEYWORD,appmetrica,REJECT\n",
		"DOMAIN-SUFFIX,paywb.com,DIRECT\n",
		"IP-CIDR,185.62.202.0/24,DIRECT,no-resolve\n",
		"IP-CIDR6,2a03:720::/32,DIRECT,no-resolve\n",
		"DOMAIN-SUFFIX,ru,DIRECT\n",
	} {
		if !strings.Contains(conf, want) {
			t.Fatalf("config lacks %q:\n%s", want, conf)
		}
	}
	// A tracker on a .ru host must hit REJECT before the zone-wide DIRECT rule.
	if strings.Index(conf, "DOMAIN-SUFFIX,an.yandex.ru,REJECT") > strings.Index(conf, "DOMAIN-SUFFIX,ru,DIRECT") {
		t.Fatal("tracker REJECT rules must precede the .ru DIRECT rule")
	}
	if !strings.HasSuffix(conf, "GEOIP,RU,DIRECT\nFINAL,PROXY\n") {
		t.Fatalf("config must end with GEOIP,RU then FINAL,PROXY:\n%s", conf)
	}
}

func TestShadowrocketConfEndpoint(t *testing.T) {
	gin.SetMode(gin.TestMode)
	initSubDB(t)
	seedInfoEndpointSub(t, "rocket-sub", "rocket@x")

	get := func(t *testing.T, enabled bool, path string) *httptest.ResponseRecorder {
		t.Helper()
		router := gin.New()
		NewSUBController(router.Group("/"), WithSUBShadowrocket(enabled), WithSUBTitle("Enot"))
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.Host = "sub.example.com"
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		return w
	}

	t.Run("known subscription gets the config", func(t *testing.T) {
		w := get(t, true, "/sub/rocket-sub/shadowrocket.conf")
		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
		}
		body := w.Body.String()
		if !strings.Contains(body, "update-url = http://sub.example.com/sub/rocket-sub/shadowrocket.conf\n") {
			t.Fatalf("update-url must point back at this endpoint:\n%s", body)
		}
		if !strings.HasPrefix(body, "# Enot: ") {
			t.Fatalf("config must carry the subscription title:\n%s", body)
		}
	})

	t.Run("unknown subscription is 404 like the subscription itself", func(t *testing.T) {
		if w := get(t, true, "/sub/missing/shadowrocket.conf"); w.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want 404", w.Code)
		}
	})

	t.Run("disabled setting does not expose the route", func(t *testing.T) {
		if w := get(t, false, "/sub/rocket-sub/shadowrocket.conf"); w.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want 404", w.Code)
		}
	})
}

func TestShadowrocketRulesURLInPageContext(t *testing.T) {
	cases := []struct {
		name    string
		enabled bool
		subURL  string
		want    string
	}{
		{name: "enabled", enabled: true, subURL: "https://h.example/sub/s1", want: "https://h.example/sub/s1/shadowrocket.conf"},
		{name: "query dropped", enabled: true, subURL: "https://h.example/sub/s1?x=1", want: "https://h.example/sub/s1/shadowrocket.conf"},
		{name: "disabled", enabled: false, subURL: "https://h.example/sub/s1", want: ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := (&SUBController{subShadowrocket: tc.enabled}).shadowrocketRulesURL(tc.subURL); got != tc.want {
				t.Fatalf("shadowrocketRulesURL = %q, want %q", got, tc.want)
			}
		})
	}
}
