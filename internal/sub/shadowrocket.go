package sub

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

const shadowrocketConfFile = "shadowrocket.conf"

// Russian services hosted outside the .ru/.su/.рф zones.
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

var ruDirectZones = []string{"ru", "su", "xn--p1ai", "xn--p1acf", "moscow", "xn--80adxhks"}

// buildShadowrocketConf renders routing rules only: Shadowrocket keeps taking
// servers from the regular subscription and reaches them through PROXY.
func buildShadowrocketConf(title, updateURL string) string {
	if title == "" {
		title = "VPN"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "# %s: Shadowrocket routing\n\n", title)

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

	// Trackers come first so no DIRECT rule can let them through.
	b.WriteString("\n[Rule]\n")
	for _, keyword := range trackerKeywords {
		fmt.Fprintf(&b, "DOMAIN-KEYWORD,%s,REJECT\n", keyword)
	}
	for _, domain := range trackerDomains {
		fmt.Fprintf(&b, "DOMAIN-SUFFIX,%s,REJECT\n", domain)
	}
	for _, domain := range marketplaceDirectDomains {
		fmt.Fprintf(&b, "DOMAIN-SUFFIX,%s,DIRECT\n", domain)
	}
	for _, cidr := range marketplaceDirectCIDRs {
		kind := "IP-CIDR"
		if strings.Contains(cidr, ":") {
			kind = "IP-CIDR6"
		}
		fmt.Fprintf(&b, "%s,%s,DIRECT,no-resolve\n", kind, cidr)
	}
	for _, zone := range ruDirectZones {
		fmt.Fprintf(&b, "DOMAIN-SUFFIX,%s,DIRECT\n", zone)
	}
	b.WriteString("DOMAIN-KEYWORD,yandex,DIRECT\n")
	b.WriteString("DOMAIN-KEYWORD,yastatic,DIRECT\n")
	for _, domain := range ruDirectDomains {
		fmt.Fprintf(&b, "DOMAIN-SUFFIX,%s,DIRECT\n", domain)
	}
	b.WriteString("GEOIP,RU,DIRECT\n")
	b.WriteString("FINAL,PROXY\n")
	return b.String()
}

// shadowrocketConf answers unknown subIds exactly like the subscription
// itself, so the extra path reveals nothing about which ids exist.
func (a *SUBController) shadowrocketConf(c *gin.Context) {
	subId := c.Param("subid")
	scheme, host, hostWithPort, _ := a.subService.ResolveRequest(c)
	subReq := a.subService.ForRequest(host)
	subReq.subscriptionBody = false
	subs, _, _, _, err := subReq.getSubs(subId)
	if err != nil || subs == nil {
		writeSubError(c, err)
		return
	}
	updateURL := fmt.Sprintf("%s://%s%s", scheme, hostWithPort, c.Request.URL.EscapedPath())
	setNoCacheHeaders(c)
	c.Header("Content-Disposition", `attachment; filename="`+shadowrocketConfFile+`"`)
	c.Data(http.StatusOK, "text/plain; charset=utf-8", []byte(buildShadowrocketConf(a.subTitle, updateURL)))
}

func shadowrocketConfURL(subURL string) string {
	base, _, _ := strings.Cut(subURL, "?")
	return strings.TrimRight(base, "/") + "/" + shadowrocketConfFile
}
