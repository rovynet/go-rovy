package rovyfc00

import (
	"log"
	"net"
	"strings"

	cid "github.com/ipfs/go-cid"
	dns "github.com/miekg/dns"

	rovy "go.rovy.net"
)

type DNSHandler struct {
	LocalPeerID rovy.PeerID
}

func (h DNSHandler) ServeDNS(w dns.ResponseWriter, r *dns.Msg) {
	qtype := r.Question[0].Qtype
	qname := r.Question[0].Name

	log.Printf("dns request: %s %s", dns.Type(qtype), qname)

	m := new(dns.Msg)
	m.SetReply(r)

	if qname == "localhost.rovy." {
		rr := &dns.AAAA{
			Hdr:  dns.RR_Header{Name: qname, Rrtype: dns.TypeAAAA, Class: dns.ClassINET, Ttl: 0},
			AAAA: net.IP(h.LocalPeerID.PublicKey().IPAddr().AsSlice()),
		}
		m.Answer = append(m.Answer, rr)
		w.WriteMsg(m)
		return
	}

	if qtype != dns.TypeAAAA || !strings.HasPrefix(qname, "bafzqai") || !strings.HasSuffix(qname, ".rovy.") {
		m.SetRcode(r, dns.RcodeNameError)
		w.WriteMsg(m)
		return
	}

	cid, err := cid.Decode(strings.TrimSuffix(qname, ".rovy."))
	if err != nil {
		log.Printf("cid: %s", err)
		m.SetRcode(r, dns.RcodeBadName)
		w.WriteMsg(m)
		return
	}
	pid, err := rovy.PeerIDFromCid(cid)
	if err != nil {
		log.Printf("cid: %s", err)
		m.SetRcode(r, dns.RcodeBadName)
		w.WriteMsg(m)
		return
	}

	rr := &dns.AAAA{
		Hdr:  dns.RR_Header{Name: qname, Rrtype: dns.TypeAAAA, Class: dns.ClassINET, Ttl: 0},
		AAAA: net.IP(pid.PublicKey().IPAddr().AsSlice()),
	}
	m.Answer = append(m.Answer, rr)
	w.WriteMsg(m)
}

func (fc *Fc00) initDns() error {
	pktconn, err := fc.fc001net.ListenUDP(&net.UDPAddr{Port: 53})
	if err != nil {
		return err
	}
	serv := &dns.Server{
		Net:        "udp6",
		PacketConn: pktconn,
		Handler:    DNSHandler{fc.node.PeerID()},
	}
	go func() {
		if err = serv.ActivateAndServe(); err != nil {
			fc.log.Printf("dns: %s", err)
		}
	}()

	fc.fc001dns = serv
	return nil
}
