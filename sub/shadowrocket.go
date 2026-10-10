package sub

import (
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"
)

// Shadowrocket cannot take routing from subscription headers the way Happ
// does, so the subscription also serves a Shadowrocket config file with the
// rules. The client imports it once (shadowrocket://config/add/<url>) and
// keeps it updated through update-url; servers still come from the regular
// subscription, and rules send traffic to the selected server via PROXY.

// ruDirectDomains are Russian services living outside the .ru/.su/.рф zones.
var ruDirectDomains = []string{
	"vk.com", "vk.me", "vkontakte.com", "vk-cdn.net", "vk-portal.net",
	"vkuseraudio.net", "vkuservideo.net", "userapi.com", "mycdn.me", "mradx.net",
	"avito.st", "lamoda.com", "lamcdn.net", "flowwow.com", "cdek.shopping",
	"lenta.com", "x5.com", "x5.tech", "magnit.tech", "samokat.tech", "kuper.tech",
	"dodopizza.com", "dodostatic.net", "dodo.is", "goldapple.com", "sunlight.net",
	"sportmaster.com", "detmir.com", "detmir.st", "citilink.com", "mvideo.com",
	"2gis.com", "doublegis.com", "okko.tv", "premier.one", "habr.com",
	"sberdevices.com", "zvuk.com",
}

// ruDirectZones are Russian top-level zones routed DIRECT.
var ruDirectZones = []string{"ru", "su", "xn--p1ai", "xn--p1acf", "moscow", "xn--80adxhks"}

// buildShadowrocketConf renders the Shadowrocket config. updateURL is where the
// client re-downloads the file from; title goes into the header comment.
func buildShadowrocketConf(title, updateURL string) string {
	var b strings.Builder
	if title == "" {
		title = "VPN"
	}
	fmt.Fprintf(&b, "# %s — правила маршрутизации для Shadowrocket\n", title)
	b.WriteString("# Российские сервисы, WB и Ozon напрямую, трекеры заблокированы, остальное через VPN.\n\n")

	b.WriteString("[General]\n")
	b.WriteString("bypass-system = true\n")
	b.WriteString("skip-proxy = 127.0.0.1, 192.168.0.0/16, 10.0.0.0/8, 172.16.0.0/12, 100.64.0.0/10, localhost, *.local\n")
	b.WriteString("bypass-tun = 10.0.0.0/8, 100.64.0.0/10, 127.0.0.0/8, 169.254.0.0/16, 172.16.0.0/12, 192.0.0.0/24, 192.168.0.0/16, 198.18.0.0/15, 224.0.0.0/4, 255.255.255.255/32\n")
	b.WriteString("dns-server = https://dns.google/dns-query, 8.8.8.8\n")
	b.WriteString("fallback-dns-server = system\n")
	b.WriteString("ipv6 = false\n")
	if updateURL != "" {
		fmt.Fprintf(&b, "update-url = %s\n", updateURL)
	}

	b.WriteString("\n[Rule]\n")
	b.WriteString("# Трекеры: через них приложения видят зарубежный IP\n")
	for _, k := range trackerKeywords {
		fmt.Fprintf(&b, "DOMAIN-KEYWORD,%s,REJECT\n", k)
	}
	for _, d := range trackerDomains {
		fmt.Fprintf(&b, "DOMAIN-SUFFIX,%s,REJECT\n", d)
	}

	b.WriteString("# Wildberries и Ozon\n")
	for _, d := range extraDirectDomains {
		fmt.Fprintf(&b, "DOMAIN-SUFFIX,%s,DIRECT\n", d)
	}
	for _, c := range extraDirectCIDRs {
		kind := "IP-CIDR"
		if strings.Contains(c, ":") {
			kind = "IP-CIDR6"
		}
		fmt.Fprintf(&b, "%s,%s,DIRECT,no-resolve\n", kind, c)
	}

	b.WriteString("# Российские зоны и сервисы\n")
	for _, z := range ruDirectZones {
		fmt.Fprintf(&b, "DOMAIN-SUFFIX,%s,DIRECT\n", z)
	}
	b.WriteString("DOMAIN-KEYWORD,yandex,DIRECT\n")
	b.WriteString("DOMAIN-KEYWORD,yastatic,DIRECT\n")
	for _, d := range ruDirectDomains {
		fmt.Fprintf(&b, "DOMAIN-SUFFIX,%s,DIRECT\n", d)
	}

	b.WriteString("GEOIP,RU,DIRECT\n")
	b.WriteString("FINAL,PROXY\n")
	return b.String()
}

// shadowrocketConf serves the Shadowrocket config for a valid subscription ID.
// Unknown IDs get the same 400 as the main subscription endpoint so the path
// cannot be used to probe for valid IDs any differently.
func (a *SUBController) shadowrocketConf(c *gin.Context) {
	subId := c.Param("subid")
	scheme, host, hostWithPort, _ := a.subService.ResolveRequest(c)
	subs, _, _, err := a.subService.GetSubs(subId, host)
	if err != nil || len(subs) == 0 {
		c.String(400, "Error!")
		return
	}
	updateURL := fmt.Sprintf("%s://%s%s", scheme, hostWithPort, c.Request.URL.Path)
	c.Header("Content-Disposition", `attachment; filename="shadowrocket.conf"`)
	c.Data(200, "text/plain; charset=utf-8", []byte(buildShadowrocketConf(a.subTitle, updateURL)))
}
