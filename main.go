package main

import (
	"fmt"
	"github.com/adrianrudnik/fritzbox-cloudflare-dyndns/pkg/avm"
	"github.com/adrianrudnik/fritzbox-cloudflare-dyndns/pkg/cloudflare"
	"github.com/joho/godotenv"
	log "github.com/sirupsen/logrus"
	"net/url"
	"os"
	"strings"
	"time"
)

func main() {
	// Load any env variables defined in .env.dev files
	_ = godotenv.Load(".env", ".env.dev")

	fb := newFritzBox()

	ipv4, err := fb.GetWanIpv4()
	if err != nil {
		panic(err)
	}

	ipv6, err := fb.GetwanIpv6()
	if err != nil {
		panic(err)
	}

	fmt.Printf("%s und %s", ipv4, ipv6)
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
	token := os.Getenv("CLOUDFLARE_API_TOKEN")

	u := newUpdater()

	if token == "" {
		log.Info("Env CLOUDFLARE_API_TOKEN not found, disabling CloudFlare updates")
		return nil
	}

	ipv4Zone := os.Getenv("CLOUDFLARE_ZONE_IPV4")
	ipv6Zone := os.Getenv("CLOUDFLARE_ZONE_IPV6")

	if ipv4Zone == "" && ipv6Zone == "" {
		log.Warn("Env CLOUDFLARE_ZONE_IPV4 and CLOUDFLARE_ZONE_IPV6 not found, disabling CloudFlare updates")
		return nil
	}

	if ipv4Zone != "" {
		u.IPv4Zone = ipv4Zone
	}

	if ipv6Zone != "" {
		u.IPv6Zone = ipv6Zone
	}

	err := u.Init(token)

	if err != nil {
		log.WithError(err).Error("Failed to init Cloudflare updater, disabling CloudFlare updates")
		return nil
	}

	return u
}
