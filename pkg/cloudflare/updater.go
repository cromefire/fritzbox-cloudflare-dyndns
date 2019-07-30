package cloudflare

import (
	"fmt"
	cf "github.com/cloudflare/cloudflare-go"
	log "github.com/sirupsen/logrus"
	"golang.org/x/net/publicsuffix"
	"net"
	"strings"
)

type Action struct {
	DnsRecord string
	CfZoneId  string
	IpVersion int
}

type Updater struct {
	ipv4Zones []string
	ipv6Zones []string

	actions []*Action

	init bool
	api  *cf.API

	In chan *net.IP
}

func NewUpdater() *Updater {
	return &Updater{
		init: false,
		In:   make(chan *net.IP, 10),
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
			DnsRecord: val,
			CfZoneId:  zoneIdMap[val],
			IpVersion: 4,
		}

		u.actions = append(u.actions, a)
	}

	for _, val := range u.ipv6Zones {
		a := &Action{
			DnsRecord: val,
			CfZoneId:  zoneIdMap[val],
			IpVersion: 6,
		}

		u.actions = append(u.actions, a)
	}

	u.api = api
	u.init = true

	return nil
}

func (u *Updater) StartWorker() {
	go u.spawnWorker()
}

func (u *Updater) spawnWorker() {
	for {
		select {
		case ip := <-u.In:
			log.WithField("ip", ip).Info("Received DynDns update request")

			for _, action := range u.actions {
				// Skip IPv6 action mismatching IP version
				if ip.To4() == nil && action.IpVersion != 6 {
					continue
				}

				// Skip IPv4 action mismatching IP version
				if ip.To4() != nil && action.IpVersion == 6 {
					continue
				}

				// Create detailed sub-logger for this action
				alog := log.WithField("action", fmt.Sprintf("%s/IPv%d", action.DnsRecord, action.IpVersion))

				// Decide record type on ip version
				var recordType string

				if ip.To4() == nil {
					recordType = "AAAA"
				} else {
					recordType = "A"
				}

				// Research all current records matching the current scheme
				records, err := u.api.DNSRecords(action.CfZoneId, cf.DNSRecord{
					Type: recordType,
					Name: action.DnsRecord,
				})

				if err != nil {
					alog.WithError(err).Error("Action failed, could not research DNS records")
					continue
				}

				// Create record if none were found
				if len(records) == 0 {
					alog.Info("Creating DNS record")

					_, err := u.api.CreateDNSRecord(action.CfZoneId, cf.DNSRecord{
						Type:      recordType,
						Name:      action.DnsRecord,
						Content:   ip.String(),
						Proxied:   false,
						TTL:       120,
						ZoneID:    action.CfZoneId,
					})

					if err != nil {
						alog.WithError(err).Error("Action failed, could not create DNS record")
						continue
					}
				}

				// Update existing records
				for _, record := range records {
					alog.WithField("record-id", record.ID).Info("Updating DNS record")

					err := u.api.UpdateDNSRecord(action.CfZoneId, record.ID, cf.DNSRecord{Content: ip.String()})

					if err != nil {
						alog.WithError(err).Error("Action failed, could not update DNS record")
						continue
					}
				}
			}

		}
	}
}
