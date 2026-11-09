// Package routing holds the split-tunnelling preset "Russian sites direct" (SPEC 8).
package routing

import (
	"encoding/base64"
	"encoding/json"
)

// Source: github.com/runetfreedom/russia-v2ray-rules-dat, branch "release",
// checked 2026-11-09. In stock geo files only category-ru, geoip:ru and
// geoip:private are guaranteed, so Xray-JSON (v2rayNG may use Loyalsoldier's
// files) sticks to those; sing-box and Happ download runetfreedom lists.
const (
	releaseBase = "https://raw.githubusercontent.com/runetfreedom/russia-v2ray-rules-dat/release/"
	GeoIPURL    = releaseBase + "geoip.dat"
	GeoSiteURL  = releaseBase + "geosite.dat"
	srsGeoSite  = releaseBase + "sing-box/rule-set-geosite/"
	srsGeoIP    = releaseBase + "sing-box/rule-set-geoip/"
)

// Preset names stored in settings.routing_preset.
const (
	PresetRUDirect = "ru_direct"
	PresetNone     = "none"
)

// Russian national TLDs: .ru, .su, .рф (xn--p1ai).
var ruSuffixes = []string{"ru", "su", "xn--p1ai"}

type ruleSet struct {
	Tag string
	URL string
}

var singBoxRuleSets = []ruleSet{
	{"geosite-category-ru", srsGeoSite + "geosite-category-ru.srs"},
	{"geosite-ru-available-only-inside", srsGeoSite + "geosite-ru-available-only-inside.srs"},
	{"geoip-ru", srsGeoIP + "geoip-ru.srs"},
}

// RUDirect sends Russian domains and IPs plus private networks around the VPN.
type RUDirect struct{}

func (RUDirect) SingBox(cfg map[string]any) {
	sets := make([]any, 0, len(singBoxRuleSets))
	tags := make([]string, 0, len(singBoxRuleSets))
	for _, rs := range singBoxRuleSets {
		sets = append(sets, map[string]any{
			"tag": rs.Tag, "type": "remote", "format": "binary", "url": rs.URL,
			// GitHub may be unreachable directly; fetch the lists through the tunnel
			"download_detour": "proxy",
		})
		tags = append(tags, rs.Tag)
	}
	suffixes := make([]string, 0, len(ruSuffixes))
	for _, s := range ruSuffixes {
		suffixes = append(suffixes, "."+s)
	}
	route, _ := cfg["route"].(map[string]any)
	if route == nil {
		route = map[string]any{}
	}
	route["rule_set"] = sets
	route["rules"] = []any{
		map[string]any{"action": "sniff"},
		map[string]any{"protocol": "dns", "action": "hijack-dns"},
		map[string]any{"ip_is_private": true, "outbound": "direct"},
		map[string]any{"domain_suffix": suffixes, "outbound": "direct"},
		map[string]any{"rule_set": tags, "outbound": "direct"},
	}
	cfg["route"] = route
	cfg["dns"] = map[string]any{
		"servers": []any{
			map[string]any{"tag": "dns-remote", "address": "https://1.1.1.1/dns-query", "detour": "proxy"},
			map[string]any{"tag": "dns-direct", "address": "77.88.8.8", "detour": "direct"},
		},
		"rules": []any{
			map[string]any{"domain_suffix": suffixes, "server": "dns-direct"},
			map[string]any{"rule_set": tags[:2], "server": "dns-direct"},
		},
		"final":    "dns-remote",
		"strategy": "prefer_ipv4",
	}
}

func (RUDirect) Xray(cfg map[string]any) {
	domains := []string{"geosite:category-ru"}
	for _, s := range ruSuffixes {
		domains = append(domains, "domain:"+s)
	}
	cfg["routing"] = map[string]any{
		"domainStrategy": "IPIfNonMatch",
		"rules": []any{
			map[string]any{"domain": domains, "outboundTag": "direct"},
			map[string]any{"ip": []string{"geoip:private", "geoip:ru"}, "outboundTag": "direct"},
		},
	}
}

// happProfile is Happ's routing profile; the subscription sends it base64-encoded
// in the "routing" header (SPEC 8).
type happProfile struct {
	Name              string   `json:"Name"`
	GlobalProxy       string   `json:"GlobalProxy"`
	RemoteDNSType     string   `json:"RemoteDNSType"`
	RemoteDNSDomain   string   `json:"RemoteDNSDomain"`
	RemoteDNSIP       string   `json:"RemoteDNSIP"`
	DomesticDNSType   string   `json:"DomesticDNSType"`
	DomesticDNSDomain string   `json:"DomesticDNSDomain"`
	DomesticDNSIP     string   `json:"DomesticDNSIP"`
	Geoipurl          string   `json:"Geoipurl"`
	Geositeurl        string   `json:"Geositeurl"`
	DirectSites       []string `json:"DirectSites"`
	DirectIp          []string `json:"DirectIp"`
	ProxySites        []string `json:"ProxySites"`
	ProxyIp           []string `json:"ProxyIp"`
	BlockSites        []string `json:"BlockSites"`
	BlockIp           []string `json:"BlockIp"`
	DomainStrategy    string   `json:"DomainStrategy"`
	FakeDNS           string   `json:"FakeDNS"`
}

// HappHeader is the value of the "routing" header for Happ.
func (RUDirect) HappHeader() string {
	sites := []string{"geosite:category-ru", "geosite:ru-available-only-inside"}
	for _, s := range ruSuffixes {
		sites = append(sites, "domain:"+s)
	}
	p := happProfile{
		Name:              "Swift",
		GlobalProxy:       "true",
		RemoteDNSType:     "DoH",
		RemoteDNSDomain:   "https://cloudflare-dns.com/dns-query",
		RemoteDNSIP:       "1.1.1.1",
		DomesticDNSType:   "DoU",
		DomesticDNSDomain: "",
		DomesticDNSIP:     "77.88.8.8",
		Geoipurl:          GeoIPURL,
		Geositeurl:        GeoSiteURL,
		DirectSites:       sites,
		DirectIp:          []string{"geoip:private", "geoip:ru"},
		ProxySites:        []string{},
		ProxyIp:           []string{},
		BlockSites:        []string{},
		BlockIp:           []string{},
		DomainStrategy:    "IPIfNonMatch",
		FakeDNS:           "false",
	}
	b, _ := json.Marshal(p)
	return base64.StdEncoding.EncodeToString(b)
}
