package dyndns

import (
	log "github.com/sirupsen/logrus"
	"net"
	"net/http"
)

type Server struct {
	log *log.Entry
	out chan<- *net.IP

	Username string
	Password string
}

func NewServer(out chan<- *net.IP) *Server {
	return &Server{
		log: log.WithField("module", "dyndns"),
		out: out,
	}
}

// Handler offers a simple HTTP handler func for an HTTP server.
// It expects the IP address parameters and will relay them towards the CloudFlare updater
// worker once they get submitted.
//
// Expected parameters can be
//   "ipaddr" IPv4 address
//   "ip6addr" IPv6 address
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

	// Parse IPv6
	ipv6 := net.ParseIP(params.Get("v6"))
	if ipv6 != nil && ipv6.To4() == nil {
		s.log.WithField("ipv6", ipv6).Info("Forwarding update request for IPv6")
		s.out <- &ipv6
	}

	w.WriteHeader(200)
}
