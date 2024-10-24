package dyndns

import (
	"github.com/cromefire/fritzbox-cloudflare-dyndns/pkg/util"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"log/slog"
	"net"
	"net/http"
	"time"
)

type Server struct {
	log            *slog.Logger
	out            chan<- *net.IP
	localIp        *net.IP
	pushExecutions prometheus.Summary
	status         *util.PushStatus

	Username string
	Password string
}

func NewServer(out chan<- *net.IP, localIp *net.IP, log *slog.Logger, subsystem string, status *util.PushStatus) *Server {
	pushExecutions := promauto.NewSummary(prometheus.SummaryOpts{
		Subsystem:  util.MakePromSubsystem(subsystem),
		Name:       "execution_seconds",
		Help:       "A summary of the push server executions",
		Objectives: map[float64]float64{0: 0, 0.5: 0.05, 0.9: 0.01, 0.99: 0.001, 1: 1},
	})
	return &Server{
		log:            log.With(slog.String("module", "dyndns")),
		out:            out,
		localIp:        localIp,
		pushExecutions: pushExecutions,
		status:         status,
	}
}

// Handler offers a simple HTTP handler func for an HTTP server.
// It expects the IP address parameters and will relay them towards the Cloudflare updater
// worker once they get submitted.
//
// Expected parameters can be
//
//	"v4" IPv4 address
//	"v6" IPv6 address
//	"prefix" IPv6 prefix
//
// see https://service.avm.de/help/de/FRITZ-Box-Fon-WLAN-7490/016/hilfe_dyndns
func (s *Server) Handler(w http.ResponseWriter, r *http.Request) {
	s.status.Last = time.Now()
	timer := prometheus.NewTimer(s.pushExecutions)
	defer func() {
		timer.ObserveDuration()
	}()
	success := true
	params := r.URL.Query()

	defer func() {
		s.status.Succeeded = success
	}()

	s.log.Info("Received incoming DynDNS update")

	if params.Get("username") != s.Username {
		s.log.Warn("Rejected due to username mismatch")
		success = false
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	if params.Get("password") != s.Password {
		s.log.Warn("Rejected due to password mismatch")
		success = false
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	// Parse IPv4
	v4Str := params.Get("v4")
	if v4Str == "" {
		s.log.Warn("No IPv4 can be set as the `v4` parameter was not supplied")
	} else {
		ipv4 := net.ParseIP(v4Str)
		if ipv4 != nil && ipv4.To4() != nil {
			s.log.Info("Forwarding update request for IPv4", slog.Any("ipv4", ipv4))
			s.out <- &ipv4
		} else {
			s.log.Warn("Failed to parse IPv4 address", slog.String("input", v4Str))
			success = false
		}
	}

	if *s.localIp == nil {
		// Parse IPv6
		v6Str := params.Get("v6")
		if v6Str == "" {
			s.log.Warn("No IPv6 can be set as the `v6` parameter was not supplied")
		} else {
			ipv6 := net.ParseIP(v6Str)
			if ipv6 != nil && ipv6.To4() == nil {
				s.log.Info("Forwarding update request for IPv6", slog.Any("ipv6", ipv6))
				s.out <- &ipv6
			} else {
				s.log.Warn("Failed to parse IPv6 address", slog.String("input", v6Str))
				success = false
			}
		}
	} else {
		// Parse Prefix
		prefixStr := params.Get("prefix")
		if prefixStr == "" {
			s.log.Warn("No IPv6 can be calculated and set as the `prefix` parameter was not supplied")
		} else {
			_, prefix, err := net.ParseCIDR(prefixStr)
			if err != nil {
				s.log.Warn("Failed to parse prefix", slog.String("input", prefixStr), util.ErrorAttr(err))
				success = false
			} else {

				constructedIp := make(net.IP, net.IPv6len)
				copy(constructedIp, prefix.IP)

				maskLen, _ := prefix.Mask.Size()

				for i := 0; i < net.IPv6len; i++ {
					b := constructedIp[i]
					lb := (*s.localIp)[i]
					var mask byte = 0b00000000
					for j := 0; j < 8; j++ {
						if (i*8 + j) >= maskLen {
							mask += 0b00000001 << (7 - j)
						}
					}
					b += lb & mask
					constructedIp[i] = b
				}

				s.log.Info("Forwarding update request for IPv6", slog.Any("prefix", prefix), slog.Any("ipv6", constructedIp))
				s.out <- &constructedIp
			}
		}
	}

	if success {
		w.WriteHeader(http.StatusAccepted)
	} else {
		w.WriteHeader(http.StatusBadRequest)
	}
}
