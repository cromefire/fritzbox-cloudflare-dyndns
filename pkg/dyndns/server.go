package dyndns

import (
	"github.com/cromefire/fritzbox-cloudflare-dyndns/pkg/logging"
	"log/slog"
	"net"
	"net/http"
)

type Server struct {
	log     *slog.Logger
	out     chan<- *net.IP
	localIp *net.IP

	Username string
	Password string
}

func NewServer(out chan<- *net.IP, localIp *net.IP, log *slog.Logger) *Server {
	return &Server{
		log:     log.With(slog.String("module", "dyndns")),
		out:     out,
		localIp: localIp,
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
	params := r.URL.Query()

	s.log.Info("Received incoming DynDNS update")

	if params.Get("username") != s.Username {
		s.log.Warn("Rejected due to username mismatch")
		return
	}

	if params.Get("password") != s.Password {
		s.log.Warn("Rejected due to password mismatch")
		return
	}

	// Parse IPv4
	ipv4 := net.ParseIP(params.Get("v4"))
	if ipv4 != nil && ipv4.To4() != nil {
		s.log.Info("Forwarding update request for IPv4", slog.Any("ipv4", ipv4))
		s.out <- &ipv4
	}

	if *s.localIp == nil {
		// Parse IPv6
		ipv6 := net.ParseIP(params.Get("v6"))
		if ipv6 != nil && ipv6.To4() == nil {
			s.log.Info("Forwarding update request for IPv6", slog.Any("ipv6", ipv6))
			s.out <- &ipv6
		}
	} else {
		// Parse Prefix
		_, prefix, err := net.ParseCIDR(params.Get("prefix"))
		if err != nil {
			s.log.Warn("Failed to parse prefix", slog.Any("prefix", prefix), logging.ErrorAttr(err))
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

	w.WriteHeader(200)
}
