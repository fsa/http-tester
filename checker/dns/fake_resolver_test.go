package dns

import (
	"net"

	mdns "github.com/miekg/dns"
)

type fakeZone struct {
	A     []string
	AAAA  []string
	HTTPS []fakeHTTPSRecord
}

type fakeHTTPSRecord struct {
	Priority uint16
	Target   string
	ALPN     []string
	IPv4Hint []string
	IPv6Hint []string
}

type fakeDNSHandler struct {
	zones map[string]fakeZone
}

func (h *fakeDNSHandler) ServeDNS(w mdns.ResponseWriter, r *mdns.Msg) {
	m := new(mdns.Msg)
	m.SetRcode(r, mdns.RcodeSuccess)

	domain := r.Question[0].Name
	domain = domain[:len(domain)-1]

	z, ok := h.zones[domain]
	if !ok {
		m.Rcode = mdns.RcodeNameError
		w.WriteMsg(m)
		return
	}

	switch r.Question[0].Qtype {
	case mdns.TypeA:
		for _, ip := range z.A {
			rr := new(mdns.A)
			rr.Hdr.Name = domain + "."
			rr.Hdr.Rrtype = mdns.TypeA
			rr.Hdr.Class = mdns.ClassINET
			rr.Hdr.Ttl = 300
			rr.A = net.ParseIP(ip)
			m.Answer = append(m.Answer, rr)
		}
	case mdns.TypeAAAA:
		for _, ip := range z.AAAA {
			rr := new(mdns.AAAA)
			rr.Hdr.Name = domain + "."
			rr.Hdr.Rrtype = mdns.TypeAAAA
			rr.Hdr.Class = mdns.ClassINET
			rr.Hdr.Ttl = 300
			rr.AAAA = net.ParseIP(ip)
			m.Answer = append(m.Answer, rr)
		}
	case mdns.TypeHTTPS:
		if len(z.HTTPS) == 0 {
			m.Rcode = mdns.RcodeNameError
		}
		for _, h := range z.HTTPS {
			rr := new(mdns.HTTPS)
			rr.Hdr.Name = domain + "."
			rr.Hdr.Rrtype = mdns.TypeHTTPS
			rr.Hdr.Class = mdns.ClassINET
			rr.Hdr.Ttl = 300
			rr.Priority = h.Priority
			rr.Target = h.Target

			if len(h.ALPN) > 0 {
				rr.Value = append(rr.Value, &mdns.SVCBAlpn{Alpn: h.ALPN})
			}
			if len(h.IPv4Hint) > 0 {
				var ips []net.IP
				for _, ip := range h.IPv4Hint {
					ips = append(ips, net.ParseIP(ip))
				}
				rr.Value = append(rr.Value, &mdns.SVCBIPv4Hint{Hint: ips})
			}
			if len(h.IPv6Hint) > 0 {
				var ips []net.IP
				for _, ip := range h.IPv6Hint {
					ips = append(ips, net.ParseIP(ip))
				}
				rr.Value = append(rr.Value, &mdns.SVCBIPv6Hint{Hint: ips})
			}
			m.Answer = append(m.Answer, rr)
		}
	}

	w.WriteMsg(m)
}
