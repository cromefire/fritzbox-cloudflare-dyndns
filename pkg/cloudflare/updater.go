package cloudflare

import (
	cf "github.com/cloudflare/cloudflare-go"
	"golang.org/x/net/publicsuffix"
	"strings"
)

type Action struct {
	DnsRecord      string
	CloudflareZone string
	UpdateOnIpv4   bool
	UpdateOnIpv6   bool
}

type Updater struct {
	ipv4Zones []string
	ipv6Zones []string

	actions []*Action

	init bool
	api  *cf.API
}

func NewUpdater() *Updater {
	return &Updater{
		init: false,
	}
}

func (u *Updater) SetIPv4Zones(zones string) {
	u.ipv4Zones = strings.Split(zones, ",")
}

func (u *Updater) SetIPv6Zones(zones string) {
	u.ipv6Zones = strings.Split(zones, ",")
}

func (u *Updater) Init(email string, key string) error {
	// api, err := cf.NewWithAPIToken(token)
	api, err := cf.New(key, email)

	if err != nil {
		return err
	}

	// Create unique list of zones and fetch their CloudFlare zone IDs
	zoneIdMap := make(map[string]string)

	for _, val := range u.ipv4Zones {
		zoneIdMap[val] = ""
	}

	for _, val := range u.ipv6Zones {
		zoneIdMap[val] = ""
	}

	for val, _ := range zoneIdMap {
		zone, err := publicsuffix.EffectiveTLDPlusOne(val)

		if err != nil {
			return err
		}

		id, err := api.ZoneIDByName(zone)

		if err != nil {
			return err
		}

		zoneIdMap[val] = id
	}

	// Now create an updater action list
	for _, val := range u.ipv4Zones {
		a := &Action{
			DnsRecord:      val,
			CloudflareZone: zoneIdMap[val],
			UpdateOnIpv4:   true,
			UpdateOnIpv6:   false,
		}

		u.actions = append(u.actions, a)
	}

	for _, val := range u.ipv6Zones {
		a := &Action{
			DnsRecord:      val,
			CloudflareZone: zoneIdMap[val],
			UpdateOnIpv4:   false,
			UpdateOnIpv6:   true,
		}

		u.actions = append(u.actions, a)
	}

	u.api = api
	u.init = true

	return nil
}
