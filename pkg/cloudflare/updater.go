package cloudflare

import (
	"context"
	"fmt"
	cf "github.com/cloudflare/cloudflare-go"
	"github.com/cromefire/fritzbox-cloudflare-dyndns/pkg/logging"
	"golang.org/x/net/publicsuffix"
	"log/slog"
	"net"
	"strings"
	"time"
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

	isInit bool
	api    *cf.API
	log    *slog.Logger

	In chan *net.IP

	lastIpv4 *net.IP
	lastIpv6 *net.IP
}

func NewUpdater(log *slog.Logger) *Updater {
	return &Updater{
		isInit:    false,
		In:        make(chan *net.IP, 10),
		log:       log.With(slog.String("module", "cloudflare")),
		ipv4Zones: make([]string, 0),
		ipv6Zones: make([]string, 0),
	}
}

func (u *Updater) SetIPv4Zones(zones string) {
	u.ipv4Zones = strings.Split(zones, ",")
}

func (u *Updater) SetIPv6Zones(zones string) {
	u.ipv6Zones = strings.Split(zones, ",")
}

func (u *Updater) InitWithToken(token string) error {
	api, err := cf.NewWithAPIToken(token)

	if err != nil {
		return err
	}

	return u.init(api)
}

func (u *Updater) InitWithKey(email string, key string) error {
	api, err := cf.New(key, email)

	if err != nil {
		return err
	}

	return u.init(api)
}

func (u *Updater) init(api *cf.API) error {
	// Create unique list of zones and fetch their Cloudflare zone IDs
	zoneIdMap := make(map[string]string)

	for _, val := range u.ipv4Zones {
		zoneIdMap[val] = ""
	}

	for _, val := range u.ipv6Zones {
		zoneIdMap[val] = ""
	}

	for val := range zoneIdMap {
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
	u.isInit = true

	return nil
}

func (u *Updater) StartWorker() {
	if !u.isInit {
		return
	}

	go u.spawnWorker()
}

func (u *Updater) spawnWorker() {
	for {
		select {
		case ip := <-u.In:
			if ip.To4() == nil {
				if u.lastIpv6 != nil && u.lastIpv6.Equal(*ip) {
					continue
				}
			} else {
				if u.lastIpv4 != nil && u.lastIpv4.Equal(*ip) {
					continue
				}
			}
			u.log.Info("Received update request", slog.Any("ip", ip))

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
				alog := u.log.With(slog.String("domain", fmt.Sprintf("%s/IPv%d", action.DnsRecord, action.IpVersion)))

				// Decide record type on ip version
				var recordType string

				if ip.To4() == nil {
					recordType = "AAAA"
				} else {
					recordType = "A"
				}

				ctx, cancel := context.WithTimeout(context.Background(), time.Minute)

				rc := cf.ZoneIdentifier(action.CfZoneId)

				// Research all current records matching the current scheme
				records, _, err := u.api.ListDNSRecords(ctx, rc, cf.ListDNSRecordsParams{
					Type: recordType,
					Name: action.DnsRecord,
				})

				if err != nil {
					alog.Error("Action failed, could not research DNS records", logging.ErrorAttr(err))
					continue
				}

				// Create record if none were found
				if len(records) == 0 {
					alog.Info("Creating DNS record")

					proxied := false

					_, err := u.api.CreateDNSRecord(ctx, rc, cf.CreateDNSRecordParams{
						Type:    recordType,
						Name:    action.DnsRecord,
						Content: ip.String(),
						Proxied: &proxied,
						TTL:     120,
						ZoneID:  action.CfZoneId,
					})

					if err != nil {
						alog.Error("Action failed, could not create DNS record", logging.ErrorAttr(err))
						continue
					}
				}

				// Update existing records
				for _, record := range records {
					alog.Info("Updating DNS record", slog.Any("record-id", record.ID))

					if record.Content == ip.String() {
						continue
					}

					// Ensure we submit all required fields even if they did not change,otherwise
					// cloudflare-go might revert them to default values.
					_, err := u.api.UpdateDNSRecord(ctx, rc, cf.UpdateDNSRecordParams{
						ID:      record.ID,
						Content: ip.String(),
						TTL:     record.TTL,
						Proxied: record.Proxied,
					})

					if err != nil {
						alog.Error("Action failed, could not update DNS record", logging.ErrorAttr(err))
						continue
					}
				}

				cancel()
			}

			if ip.To4() == nil {
				u.lastIpv6 = ip
			} else {
				u.lastIpv4 = ip
			}
		}
	}
}
