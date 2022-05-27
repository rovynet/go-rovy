package rovyfc00

import (
	"log"
	"net"
	"strings"

	cid "github.com/ipfs/go-cid"
	dns "github.com/miekg/dns"

	rovy "go.rovy.net"
)

func (fc *Fc00) handleDnsRequest(buf []byte) error {
	_, err := fc.fc001tun.Write(buf, 0)
	return err
}

func dnsHandlerFunc(localPid rovy.PeerID, w dns.ResponseWriter, r *dns.Msg) {
	qtype := r.Question[0].Qtype
	qname := r.Question[0].Name

	log.Printf("dns request: %s %s", dns.Type(qtype), qname)

	m := new(dns.Msg)
	m.SetReply(r)

	if qname == "local.rovy." {
		rr := &dns.AAAA{
			Hdr:  dns.RR_Header{Name: qname, Rrtype: dns.TypeAAAA, Class: dns.ClassINET, Ttl: 0},
			AAAA: localPid.PublicKey().Addr(),
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
		AAAA: pid.PublicKey().Addr(),
	}
	m.Answer = append(m.Answer, rr)
	w.WriteMsg(m)
}

func (fc *Fc00) initDns(localPid rovy.PeerID, mtu int) error {
	pktconn, err := fc.fc001net.ListenUDP(&net.UDPAddr{Port: 53})
	if err != nil {
		return err
	}
	dns.HandleFunc("rovy.", func(w dns.ResponseWriter, r *dns.Msg) {
		dnsHandlerFunc(localPid, w, r)
	})
	serv := &dns.Server{Net: "udp6", PacketConn: pktconn}
	go func() {
		if err = serv.ActivateAndServe(); err != nil {
			fc.log.Printf("dns: %s", err)
		}
	}()

	fc.fc001dns = serv
	return nil
}
