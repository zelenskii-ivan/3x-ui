package sub

import (
	"encoding/base64"
	"encoding/json"
	"strings"
)

// Extra routing rules layered on top of the RoscomVPN profiles.
//
// Russian marketplaces (Wildberries, Ozon) refuse to work when part of their
// traffic or their analytics SDKs leave through a foreign IP. RoscomVPN covers
// .ru zones via geosite:category-ru, but WB and Ozon also use domains outside
// .ru, and in-app trackers (AppMetrica, AppsFlyer, Adjust…) report the exit IP.
// These lists send the marketplace domains direct and block the trackers.
// The same lists feed the Shadowrocket config (see shadowrocket.go).

// extraDirectDomains are domain suffixes routed DIRECT.
var extraDirectDomains = []string{
	// Wildberries
	"wildberries.ru", "wb.ru", "wbbasket.ru", "wb-basket.ru", "geobasket.ru",
	"wbstatic.net", "wbstatic.ru", "wbcontent.net", "wb-cloud.ru",
	"paywb.com", "paywb.ru", "wbpay.ru", "wb-bank.ru", "rwb.ru", "rwbgeo.ru",
	"wibes.com", "wibes.ru", "mp.world", "warehouse.domains",
	// Ozon
	"ozon.ru", "ozone.ru", "ozon.com", "ozon.app", "ozon.tech", "ozon.dev",
	"ozon.pro", "ozon.express", "ozon.market", "ozonusercontent.com",
	"ozonru.me", "ozonru.com", "ozoncard.ru",
}

// extraDirectCIDRs are IP ranges routed DIRECT (Wildberries AS).
var extraDirectCIDRs = []string{
	"185.62.201.0/24",
	"185.62.202.0/24",
	"2a03:720::/32",
}

// trackerKeywords are blocked when the domain contains the keyword.
var trackerKeywords = []string{
	"appmetrica", "appsflyersdk", "adfox", "adriver", "criteo",
}

// trackerDomains are domain suffixes that are blocked.
var trackerDomains = []string{
	"adjust.com", "adjust.world", "adjust.net.in", "adjust.cn",
	"app-measurement.com", "doubleclick.net", "ingest.sentry.io",
	"amplitude.com", "mixpanel.com", "segment.io", "fullstory.com",
	"logrocket.com", "clarity.ms", "browser-intake-datadoghq.com",
	"an.yandex.ru", "yabs.yandex.ru", "mc.webvisor.org", "tns-counter.ru",
	"top-fwz1.mail.ru", "ad.mail.ru", "st.top100.ru", "counter.reddigital.ru",
}

const (
	happOnAddPrefix = "happ://routing/onadd/"
	happAddPrefix   = "happ://routing/add/"
)

// applyRoutingExtras merges the extra lists into a Happ routing deeplink
// (happ://routing/onadd/<base64 json>). Anything that is not such a deeplink
// (a plain URL, an empty string, malformed base64) is returned unchanged, so
// a broken upstream never breaks the subscription response.
func applyRoutingExtras(link string) string {
	link = strings.TrimSpace(link)
	prefix := ""
	switch {
	case strings.HasPrefix(link, happOnAddPrefix):
		prefix = happOnAddPrefix
	case strings.HasPrefix(link, happAddPrefix):
		prefix = happAddPrefix
	default:
		return link
	}

	payload := strings.TrimSpace(strings.TrimPrefix(link, prefix))
	raw, err := decodeBase64Loose(payload)
	if err != nil {
		return link
	}

	var profile map[string]any
	if err := json.Unmarshal(raw, &profile); err != nil {
		return link
	}

	direct := make([]string, 0, len(extraDirectDomains))
	for _, d := range extraDirectDomains {
		direct = append(direct, "domain:"+d)
	}
	block := make([]string, 0, len(trackerKeywords)+len(trackerDomains))
	for _, k := range trackerKeywords {
		block = append(block, "keyword:"+k)
	}
	for _, d := range trackerDomains {
		block = append(block, "domain:"+d)
	}

	profile["DirectSites"] = mergeUnique(profile["DirectSites"], direct)
	profile["DirectIp"] = mergeUnique(profile["DirectIp"], extraDirectCIDRs)
	profile["BlockSites"] = mergeUnique(profile["BlockSites"], block)

	out, err := json.Marshal(profile)
	if err != nil {
		return link
	}
	return prefix + base64.StdEncoding.EncodeToString(out)
}

// mergeUnique appends items to an existing JSON array value, skipping
// duplicates and preserving the original order.
func mergeUnique(existing any, items []string) []string {
	out := []string{}
	seen := map[string]bool{}
	if arr, ok := existing.([]any); ok {
		for _, v := range arr {
			if s, ok := v.(string); ok && !seen[s] {
				seen[s] = true
				out = append(out, s)
			}
		}
	}
	for _, s := range items {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}

func decodeBase64Loose(s string) ([]byte, error) {
	s = strings.TrimRight(s, "=")
	if b, err := base64.RawStdEncoding.DecodeString(s); err == nil {
		return b, nil
	}
	return base64.RawURLEncoding.DecodeString(s)
}

// routingSourceTakesExtras reports whether the extras overlay applies to a
// routing source. Custom links are the admin's own rules and JSONSUB is a
// deliberately rule-free profile, so both are left untouched.
func routingSourceTakesExtras(source string) bool {
	switch strings.ToLower(strings.TrimSpace(source)) {
	case RoscomVPNSourceCustom, RoscomVPNSourceJsonSub:
		return false
	default:
		return true
	}
}
