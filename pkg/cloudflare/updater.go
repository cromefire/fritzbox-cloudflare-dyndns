package cloudflare

import (
	"context"
	"fmt"
	cf "github.com/cloudflare/cloudflare-go"
	"github.com/cromefire/fritzbox-cloudflare-dyndns/pkg/util"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"golang.org/x/net/publicsuffix"
	"log/slog"
	"net"
	"strings"
	"time"
)

type Action struct {
	DnsRecord string
	CfZoneId  string
	IpVersion uint8

	updates prometheus.Summary
	status  *util.UpdateStatus
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

	subsystem string
}

func NewUpdater(log *slog.Logger, subsystem string) *Updater {
	return &Updater{
		isInit:    false,
		In:        make(chan *net.IP, 10),
		log:       log.With(slog.String("module", "cloudflare")),
		ipv4Zones: make([]string, 0),
		ipv6Zones: make([]string, 0),
		subsystem: subsystem,
	}
}

func (u Updater) makeSummary(labels prometheus.Labels) prometheus.Summary {
	return promauto.NewSummary(prometheus.SummaryOpts{
		Subsystem:   util.MakePromSubsystem(u.subsystem),
		Name:        "update_seconds",
		Help:        "A summary of the push server executions",
		Objectives:  map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
		ConstLabels: labels,
	})
}

func (u *Updater) SetIPv4Zones(zones string) {
	u.ipv4Zones = strings.Split(zones, ",")
}

func (u *Updater) SetIPv6Zones(zones string) {
	u.ipv6Zones = strings.Split(zones, ",")
}

func (u *Updater) InitWithToken(token string) (error, []*util.UpdateStatus) {
	api, err := cf.NewWithAPIToken(token)

	if err != nil {
		return err, nil
	}

	return u.init(api)
}

func (u *Updater) InitWithKey(email string, key string) (error, []*util.UpdateStatus) {
	api, err := cf.New(key, email)

	if err != nil {
		return err, nil
	}

	return u.init(api)
}

func (u *Updater) init(api *cf.API) (error, []*util.UpdateStatus) {
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
			return err, nil
		}

		id, err := api.ZoneIDByName(zone)

		if err != nil {
			return err, nil
		}

		zoneIdMap[val] = id
	}

	statusVec := []*util.UpdateStatus{}

	// Now create an updater action list
	for _, val := range u.ipv4Zones {
		zoneId := zoneIdMap[val]
		labels := prometheus.Labels{"record": val, "ip_version": "4"}
		updates := u.makeSummary(labels)
		status := util.UpdateStatus{Domain: val, IpVersion: 4, Succeeded: true}
		statusVec = append(statusVec, &status)

		a := &Action{
			DnsRecord: val,
			CfZoneId:  zoneId,
			IpVersion: 4,
			updates:   updates,
			status:    &status,
		}

		u.actions = append(u.actions, a)
	}

	for _, val := range u.ipv6Zones {
		zoneId := zoneIdMap[val]
		labels := prometheus.Labels{"record": val, "ip_version": "6"}
		updates := u.makeSummary(labels)
		status := util.UpdateStatus{Domain: val, IpVersion: 4, Succeeded: true}
		statusVec = append(statusVec, &status)

		a := &Action{
			DnsRecord: val,
			CfZoneId:  zoneId,
			IpVersion: 6,
			updates:   updates,
			status:    &status,
		}

		u.actions = append(u.actions, a)
	}

	u.api = api
	u.isInit = true

	return nil, statusVec
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
				timer := prometheus.NewTimer(action.updates)

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
					alog.Error("Action failed, could not research DNS records", util.ErrorAttr(err))
					action.status.Last = time.Now()
					action.status.Succeeded = false
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
					})

					if err != nil {
						alog.Error("Action failed, could not create DNS record", util.ErrorAttr(err))
						action.status.Last = time.Now()
						action.status.Succeeded = false
						continue
					}
				}

				// Update existing records
				for _, record := range records {
					alog.Info("Updating DNS record", slog.Any("record-id", record.ID))

					if record.Content == ip.String() {
						action.status.Last = time.Now()
						action.status.Succeeded = true
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
						alog.Error("Action failed, could not update DNS record", util.ErrorAttr(err))
						action.status.Last = time.Now()
						action.status.Succeeded = false
						continue
					}
				}

				cancel()

				action.status.Last = time.Now()
				action.status.Succeeded = true

				timer.ObserveDuration()
			}

			if ip.To4() == nil {
				u.lastIpv6 = ip
			} else {
				u.lastIpv4 = ip
			}
		}
	}
}
