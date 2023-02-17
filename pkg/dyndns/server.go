package dyndns

import (
	log "github.com/sirupsen/logrus"
	"net"
	"net/http"
)

type Server struct {
	log     *log.Entry
	out     chan<- *net.IP
	localIp *net.IP

	Username string
	Password string
}

func NewServer(out chan<- *net.IP, localIp *net.IP) *Server {
	return &Server{
		log:     log.WithField("module", "dyndns"),
		out:     out,
		localIp: localIp,
	}
}

// Handler offers a simple HTTP handler func for an HTTP server.
// It expects the IP address parameters and will relay them towards the CloudFlare updater
// worker once they get submitted.
//
// Expected parameters can be
//
//	"ipaddr" IPv4 address
//	"ip6addr" IPv6 address
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
		s.log.WithField("ipv4", ipv4).Info("Forwarding update request for IPv4")
		s.out <- &ipv4
	}

	if *s.localIp == nil {
		// Parse IPv6
		ipv6 := net.ParseIP(params.Get("v6"))
		if ipv6 != nil && ipv6.To4() == nil {
			s.log.WithField("ipv6", ipv6).Info("Forwarding update request for IPv6")
			s.out <- &ipv6
		}
	} else {
		// Parse Prefix
		_, prefix, err := net.ParseCIDR(params.Get("prefix"))
		if err != nil {
			s.log.WithError(err).Warn("Failed to parse prefix")
		} else {

			constructedIp := make(net.IP, net.IPv6len)
			copy(constructedIp, prefix.IP)

			maskLen, _ := prefix.Mask.Size()

			for i := 0; i < net.IPv6len; i++ {
				for j := 0; j < 8; j++ {
					b := constructedIp[i]
					if (i*8 + j) > maskLen {
						mask := 0b00000001 << j
						b += j & mask
					}
					constructedIp[i] = b
				}
			}

			s.log.WithField("prefix", prefix).WithField("ipv6", constructedIp).Info("Forwarding update request for IPv6")
			s.out <- &constructedIp
		}
	}

	w.WriteHeader(200)
}
