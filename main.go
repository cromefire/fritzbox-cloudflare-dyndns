package main

import (
	"github.com/adrianrudnik/fritzbox-cloudflare-dyndns/pkg/avm"
	"github.com/adrianrudnik/fritzbox-cloudflare-dyndns/pkg/cloudflare"
	"github.com/adrianrudnik/fritzbox-cloudflare-dyndns/pkg/dyndns"
	"github.com/joho/godotenv"
	log "github.com/sirupsen/logrus"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

func main() {
	// Load any env variables defined in .env.dev files
	_ = godotenv.Load(".env", ".env.dev")

	updater := newUpdater()
	updater.StartWorker()

	startPollServer(updater.In)
	startPushServer(updater.In)

	shutdown := make(chan os.Signal)

	signal.Notify(shutdown, syscall.SIGTERM)
	signal.Notify(shutdown, syscall.SIGINT)

	<-shutdown

	log.Info("Shutdown detected")
}

func newFritzBox() *avm.FritzBox {
	fb := avm.NewFritzBox()

	// Import FritzBox endpoint url
	endpointUrl := os.Getenv("FRITZBOX_ENDPOINT_URL")

	if endpointUrl != "" {
		v, err := url.ParseRequestURI(endpointUrl)

		if err != nil {
			log.WithError(err).Panic("Failed to parse env FRITZBOX_ENDPOINT_URL")
		}

		fb.Url = strings.TrimRight(v.String(), "/")
	} else {
		log.Info("Env FRITZBOX_ENDPOINT_URL not found, disabling FritzBox polling")
		return nil
	}

	// Import FritzBox endpoint timeout setting
	endpointTimeout := os.Getenv("FRITZBOX_ENDPOINT_TIMEOUT")

	if endpointTimeout != "" {
		v, err := time.ParseDuration(endpointTimeout)

		if err != nil {
			log.WithError(err).Warn("Failed to parse FRITZBOX_ENDPOINT_TIMEOUT, using defaults")
		} else {
			fb.Timeout = v
		}
	}

	return fb
}

func newUpdater() *cloudflare.Updater {
	u := cloudflare.NewUpdater()

	email := os.Getenv("CLOUDFLARE_API_EMAIL")

	if email == "" {
		log.Info("Env CLOUDFLARE_API_EMAIL not found, disabling CloudFlare updates")
		return u
	}

	key := os.Getenv("CLOUDFLARE_API_KEY")

	if key == "" {
		log.Info("Env CLOUDFLARE_API_KEY not found, disabling CloudFlare updates")
		return u
	}

	ipv4Zone := os.Getenv("CLOUDFLARE_ZONES_IPV4")
	ipv6Zone := os.Getenv("CLOUDFLARE_ZONES_IPV6")
	ipv6LocalAddress := os.Getenv("CLOUDFLARE_LOCAL_ADDRESS_IPV6")

	if ipv4Zone == "" && ipv6Zone == "" {
		log.Warn("Env CLOUDFLARE_ZONES_IPV4 and CLOUDFLARE_ZONES_IPV6 not found, disabling CloudFlare updates")
		return u
	}

	if ipv4Zone != "" {
		u.SetIPv4Zones(ipv4Zone)
	}

	if ipv6Zone != "" {
		u.SetIPv6Zones(ipv6Zone)
	}

	if ipv6LocalAddress != "" {
		localIp := net.ParseIP(ipv6LocalAddress)
		if localIp == nil {
			log.Error("Failed to parse IP from CLOUDFLARE_LOCAL_ADDRESS_IPV6, disabling CloudFlare updates")
			return u
		}
		u.SetIPv6LocalAddress(&localIp)
	}

	err := u.Init(email, key)

	if err != nil {
		log.WithError(err).Error("Failed to init Cloudflare updater, disabling CloudFlare updates")
		return u
	}

	return u
}

func startPushServer(out chan<- *net.IP) {
	bind := os.Getenv("DYNDNS_SERVER_BIND")

	if bind == "" {
		log.Info("Env DYNDNS_SERVER_BIND not found, disabling DynDns server")
		return
	}

	server := dyndns.NewServer(out)
	server.Username = os.Getenv("DYNDNS_SERVER_USERNAME")
	server.Password = os.Getenv("DYNDNS_SERVER_PASSWORD")

	s := &http.Server{
		Addr: bind,
	}

	http.HandleFunc("/ip", server.Handler)

	go func() {
		log.Fatal(s.ListenAndServe())
	}()
}

func startPollServer(out chan<- *net.IP) {
	fritzbox := newFritzBox()

	// Import endpoint polling interval duration
	interval := os.Getenv("FRITZBOX_ENDPOINT_INTERVAL")

	var ticker *time.Ticker

	if interval != "" {
		v, err := time.ParseDuration(interval)

		if err != nil {
			log.WithError(err).Warn("Failed to parse FRITZBOX_ENDPOINT_INTERVAL, using defaults")
			ticker = time.NewTicker(300 * time.Second)
		} else {
			ticker = time.NewTicker(v)
		}
	} else {
		log.Info("Env FRITZBOX_ENDPOINT_INTERVAL not found, disabling polling")
		return
	}

	go func() {
		lastV4 := net.IP{}
		lastV6 := net.IP{}

		poll := func() {
			log.Debug("Polling WAN IPs from router")

			ipv4, err := fritzbox.GetWanIpv4()

			if err != nil {
				log.WithError(err).Warn("Failed to poll WAN IPv4 from router")
			} else {
				if !lastV4.Equal(ipv4) {
					log.WithField("ipv4", ipv4).Info("New WAN IPv4 found")
					out <- &ipv4
					lastV4 = ipv4
				}

			}

			ipv6, err := fritzbox.GetwanIpv6()

			if err != nil {
				log.WithError(err).Warn("Failed to poll WAN IPv6 from router")
			} else {
				if !lastV6.Equal(ipv6) {
					log.WithField("ipv6", ipv6).Info("New WAN IPv6 found")
					out <- &ipv6
					lastV6 = ipv6
				}
			}
		}

		poll()

		for {
			select {
			case <-ticker.C:
				poll()
			}
		}
	}()
}
