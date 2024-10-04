package main

import (
	"context"
	"errors"
	"github.com/cromefire/fritzbox-cloudflare-dyndns/pkg/avm"
	"github.com/cromefire/fritzbox-cloudflare-dyndns/pkg/cloudflare"
	"github.com/cromefire/fritzbox-cloudflare-dyndns/pkg/dyndns"
	"github.com/cromefire/fritzbox-cloudflare-dyndns/pkg/logging"
	"github.com/joho/godotenv"
	"log/slog"
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

	ctx, cancel := context.WithCancelCause(context.Background())

	ipv6LocalAddress := os.Getenv("DEVICE_LOCAL_ADDRESS_IPV6")

	var localIp net.IP
	if ipv6LocalAddress != "" {
		localIp = net.ParseIP(ipv6LocalAddress)
		if localIp == nil {
			slog.Error("Failed to parse IP from DEVICE_LOCAL_ADDRESS_IPV6, exiting")
			return
		}
		slog.Info("Using the IPv6 Prefix to construct the IPv6 Address")
	}

	startPollServer(updater.In, &localIp)
	startPushServer(updater.In, &localIp, cancel)

	// Create a OS signal shutdown channel
	shutdown := make(chan os.Signal)

	signal.Notify(shutdown, syscall.SIGTERM)
	signal.Notify(shutdown, syscall.SIGINT)

	// Wait for either the context to finish or the shutdown signal
	select {
	case <-ctx.Done():
		slog.Error("Context closed", logging.ErrorAttr(context.Cause(ctx)))
		os.Exit(1)
	case <-shutdown:
		break
	}

	slog.Info("Shutdown detected")
}

func newFritzBox() *avm.FritzBox {
	fb := avm.NewFritzBox()

	// Import FritzBox endpoint url
	endpointUrl := os.Getenv("FRITZBOX_ENDPOINT_URL")

	if endpointUrl != "" {
		v, err := url.ParseRequestURI(endpointUrl)

		if err != nil {
			slog.Error("Failed to parse env FRITZBOX_ENDPOINT_URL", logging.ErrorAttr(err))
			panic(err)
		}

		fb.Url = strings.TrimRight(v.String(), "/")
	} else {
		slog.Info("Env FRITZBOX_ENDPOINT_URL not found, disabling FritzBox polling")
		return nil
	}

	// Import FritzBox endpoint timeout setting
	endpointTimeout := os.Getenv("FRITZBOX_ENDPOINT_TIMEOUT")

	if endpointTimeout != "" {
		v, err := time.ParseDuration(endpointTimeout)

		if err != nil {
			slog.Warn("Failed to parse FRITZBOX_ENDPOINT_TIMEOUT, using defaults", logging.ErrorAttr(err))
		} else {
			fb.Timeout = v
		}
	}

	return fb
}

func newUpdater() *cloudflare.Updater {
	u := cloudflare.NewUpdater(slog.Default())

	token := os.Getenv("CLOUDFLARE_API_TOKEN")
	email := os.Getenv("CLOUDFLARE_API_EMAIL")
	key := os.Getenv("CLOUDFLARE_API_KEY")

	if token == "" {
		if email == "" || key == "" {
			slog.Info("Env CLOUDFLARE_API_TOKEN not found, disabling Cloudflare updates")
			return u
		} else {
			slog.Warn("Using deprecated credentials via the API key")
		}
	}

	ipv4Zone := os.Getenv("CLOUDFLARE_ZONES_IPV4")
	ipv6Zone := os.Getenv("CLOUDFLARE_ZONES_IPV6")

	if ipv4Zone == "" && ipv6Zone == "" {
		slog.Warn("Env CLOUDFLARE_ZONES_IPV4 and CLOUDFLARE_ZONES_IPV6 not found, disabling Cloudflare updates")
		return u
	}

	if ipv4Zone != "" {
		u.SetIPv4Zones(ipv4Zone)
	}

	if ipv6Zone != "" {
		u.SetIPv6Zones(ipv6Zone)
	}

	var err error

	if token != "" {
		err = u.InitWithToken(token)
	} else {
		err = u.InitWithKey(email, key)
	}

	if err != nil {
		slog.Error("Failed to init Cloudflare updater, disabling Cloudflare updates")
		return u
	}

	return u
}

func startPushServer(out chan<- *net.IP, localIp *net.IP, cancel context.CancelCauseFunc) {
	bind := os.Getenv("DYNDNS_SERVER_BIND")

	if bind == "" {
		slog.Info("Env DYNDNS_SERVER_BIND not found, disabling DynDns server")
		return
	}

	server := dyndns.NewServer(out, localIp, slog.Default())
	server.Username = os.Getenv("DYNDNS_SERVER_USERNAME")
	server.Password = os.Getenv("DYNDNS_SERVER_PASSWORD")

	s := &http.Server{
		Addr:     bind,
		ErrorLog: slog.NewLogLogger(slog.Default().Handler(), slog.LevelError),
	}

	http.HandleFunc("/ip", server.Handler)

	go func() {
		err := s.ListenAndServe()
		cancel(errors.Join(errors.New("http server error"), err))
	}()
}

func startPollServer(out chan<- *net.IP, localIp *net.IP) {
	fritzbox := newFritzBox()

	// Import endpoint polling interval duration
	interval := os.Getenv("FRITZBOX_ENDPOINT_INTERVAL")
	useIpv4 := os.Getenv("CLOUDFLARE_ZONES_IPV4") != ""
	useIpv6 := os.Getenv("CLOUDFLARE_ZONES_IPV6") != ""

	var ticker *time.Ticker

	if interval != "" {
		v, err := time.ParseDuration(interval)

		if err != nil {
			slog.Warn("Failed to parse FRITZBOX_ENDPOINT_INTERVAL, using defaults", logging.ErrorAttr(err))
			ticker = time.NewTicker(300 * time.Second)
		} else {
			ticker = time.NewTicker(v)
		}
	} else {
		slog.Info("Env FRITZBOX_ENDPOINT_INTERVAL not found, disabling polling")
		return
	}

	go func() {
		lastV4 := net.IP{}
		lastV6 := net.IP{}

		poll := func() {
			slog.Debug("Polling WAN IPs from router")

			if useIpv4 {
				ipv4, err := fritzbox.GetWanIpv4()

				if err != nil {
					slog.Warn("Failed to poll WAN IPv4 from router", logging.ErrorAttr(err))
				} else {
					out <- &ipv4
					if !lastV4.Equal(ipv4) {
						slog.Info("New WAN IPv4 found", slog.Any("ipv4", ipv4))
						lastV4 = ipv4
					}
				}
			}

			if *localIp == nil && useIpv6 {
				ipv6, err := fritzbox.GetwanIpv6()

				if err != nil {
					slog.Warn("Failed to poll WAN IPv6 from router", logging.ErrorAttr(err))
				} else {
					if !lastV6.Equal(ipv6) {
						slog.Info("New WAN IPv6 found", slog.Any("ipv6", ipv6))
						out <- &ipv6
						lastV6 = ipv6
					}
				}
			} else if useIpv6 {
				prefix, err := fritzbox.GetIpv6Prefix()

				if err != nil {
					slog.Warn("Failed to poll IPv6 Prefix from router", logging.ErrorAttr(err))
				} else {
					constructedIp := make(net.IP, net.IPv6len)
					copy(constructedIp, prefix.IP)

					maskLen, _ := prefix.Mask.Size()

					for i := 0; i < net.IPv6len; i++ {
						b := constructedIp[i]
						lb := (*localIp)[i]
						var mask byte = 0b00000000
						for j := 0; j < 8; j++ {
							if (i*8 + j) >= maskLen {
								mask += 0b00000001 << (7 - j)
							}
						}
						b += lb & mask
						constructedIp[i] = b
					}

					slog.Info("New IPv6 Prefix found", slog.Any("prefix", prefix), slog.Any("ipv6", constructedIp))

					out <- &constructedIp

					if !lastV6.Equal(prefix.IP) {
						lastV6 = prefix.IP
					}
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
