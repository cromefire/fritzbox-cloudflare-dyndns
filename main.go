package main

import (
	"fmt"
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

	fritzbox := newFritzBox()

	ipv4, err := fritzbox.GetWanIpv4()
	if err != nil {
		panic(err)
	}

	ipv6, err := fritzbox.GetwanIpv6()
	if err != nil {
		panic(err)
	}

	updater := newUpdater()
	updater.StartWorker()

	startDynDnsServer(updater.In)

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
	email := os.Getenv("CLOUDFLARE_API_EMAIL")

	if email == "" {
		log.Info("Env CLOUDFLARE_API_TOKEN not found, disabling CloudFlare updates")
		return nil
	}

	key := os.Getenv("CLOUDFLARE_API_KEY")

	if key == "" {
		log.Info("Env CLOUDFLARE_API_KEY not found, disabling CloudFlare updates")
		return nil
	}

	ipv4Zone := os.Getenv("CLOUDFLARE_ZONES_IPV4")
	ipv6Zone := os.Getenv("CLOUDFLARE_ZONES_IPV6")

	if ipv4Zone == "" && ipv6Zone == "" {
		log.Warn("Env CLOUDFLARE_ZONES_IPV4 and CLOUDFLARE_ZONES_IPV6 not found, disabling CloudFlare updates")
		return nil
	}

	u := cloudflare.NewUpdater()

	if ipv4Zone != "" {
		u.SetIPv4Zones(ipv4Zone)
	}

	if ipv6Zone != "" {
		u.SetIPv6Zones(ipv6Zone)
	}

	err := u.Init(email, key)

	if err != nil {
		log.WithError(err).Error("Failed to init Cloudflare updater, disabling CloudFlare updates")
		return nil
	}

	return u
}

func startDynDnsServer(out chan *net.IP) {
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
