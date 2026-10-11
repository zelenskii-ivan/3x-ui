package sub

import (
	"encoding/base64"
	"encoding/json"
	"strings"
)

// Wildberries and Ozon refuse service when part of their traffic or their
// analytics SDKs exit abroad; these lists keep both on the client's own network.
var marketplaceDirectDomains = []string{
	"wildberries.ru", "wb.ru", "wbbasket.ru", "wb-basket.ru", "geobasket.ru",
	"wbstatic.net", "wbstatic.ru", "wbcontent.net", "wb-cloud.ru",
	"paywb.com", "paywb.ru", "wbpay.ru", "wb-bank.ru", "rwb.ru", "rwbgeo.ru",
	"wibes.com", "wibes.ru", "mp.world", "warehouse.domains",
	"ozon.ru", "ozone.ru", "ozon.com", "ozon.app", "ozon.tech", "ozon.dev",
	"ozon.pro", "ozon.express", "ozon.market", "ozonusercontent.com",
	"ozonru.me", "ozonru.com", "ozoncard.ru",
}

// Wildberries' own address space.
var marketplaceDirectCIDRs = []string{
	"185.62.201.0/24",
	"185.62.202.0/24",
	"2a03:720::/32",
}

var trackerKeywords = []string{
	"appmetrica", "appsflyersdk", "adfox", "adriver", "criteo",
}

var trackerDomains = []string{
	"adjust.com", "adjust.world", "adjust.net.in", "adjust.cn",
	"app-measurement.com", "doubleclick.net", "ingest.sentry.io",
	"amplitude.com", "mixpanel.com", "segment.io", "fullstory.com",
	"logrocket.com", "clarity.ms", "browser-intake-datadoghq.com",
	"an.yandex.ru", "yabs.yandex.ru", "mc.webvisor.org", "tns-counter.ru",
	"top-fwz1.mail.ru", "ad.mail.ru", "st.top100.ru", "counter.reddigital.ru",
}

// applyMarketplaceRouting merges the marketplace and tracker lists into a Happ
// routing deeplink; any other value (off, incy://, malformed) passes unchanged.
func applyMarketplaceRouting(link string) string {
	trimmed := strings.TrimSpace(link)
	var prefix string
	for _, candidate := range []string{"happ://routing/onadd/", "happ://routing/add/"} {
		if strings.HasPrefix(trimmed, candidate) {
			prefix = candidate
			break
		}
	}
	if prefix == "" {
		return link
	}

	raw, err := decodeRoutingBase64(strings.TrimPrefix(trimmed, prefix))
	if err != nil {
		return link
	}
	var profile map[string]any
	if err := json.Unmarshal(raw, &profile); err != nil || profile == nil {
		return link
	}

	direct := make([]string, 0, len(marketplaceDirectDomains))
	for _, domain := range marketplaceDirectDomains {
		direct = append(direct, "domain:"+domain)
	}
	block := make([]string, 0, len(trackerKeywords)+len(trackerDomains))
	for _, keyword := range trackerKeywords {
		block = append(block, "keyword:"+keyword)
	}
	for _, domain := range trackerDomains {
		block = append(block, "domain:"+domain)
	}

	profile["DirectSites"] = appendUniqueRules(profile["DirectSites"], direct)
	profile["DirectIp"] = appendUniqueRules(profile["DirectIp"], marketplaceDirectCIDRs)
	profile["BlockSites"] = appendUniqueRules(profile["BlockSites"], block)

	out, err := json.Marshal(profile)
	if err != nil {
		return link
	}
	return prefix + base64.StdEncoding.EncodeToString(out)
}

func appendUniqueRules(existing any, additions []string) []string {
	out := []string{}
	seen := map[string]bool{}
	if values, ok := existing.([]any); ok {
		for _, value := range values {
			if rule, ok := value.(string); ok && !seen[rule] {
				seen[rule] = true
				out = append(out, rule)
			}
		}
	}
	for _, rule := range additions {
		if !seen[rule] {
			seen[rule] = true
			out = append(out, rule)
		}
	}
	return out
}
