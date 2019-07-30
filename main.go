package main

import (
	"fmt"
	"github.com/adrianrudnik/fritzbox-cloudflare-dyndns/pkg/avm"
	"github.com/joho/godotenv"
	"net/url"
	"os"
	"strings"
	"time"
)

func main() {
	// Load any env variables defined in .env files
	_ = godotenv.Load()

	fb := avm.NewFritzBox()

	parseEnv(fb)

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

func parseEnv(fb *avm.FritzBox) {
	// Import FritzBox endpoint url
	endpointUrl := os.Getenv("FRITZBOX_ENDPOINT_URL")

	if endpointUrl != "" {
		v, err := url.ParseRequestURI(endpointUrl)

		if err != nil {
			panic(err)
		}

		fb.Url = strings.TrimRight(v.String(), "/")
	}

	// Import FritzBox endpoint timeout setting
	endpointTimeout := os.Getenv("FRITZBOX_ENDPOINT_TIMEOUT")

	if endpointTimeout != "" {
		v, err := time.ParseDuration(endpointTimeout)

		if err != nil {
			panic(err)
		}

		fb.Timeout = v
	}
}
