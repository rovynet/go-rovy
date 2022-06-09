package rovy

import (
	"bytes"
	"fmt"
	"net/netip"
	"strconv"
	"strings"

	cid "github.com/ipfs/go-cid"
	multiaddr "github.com/multiformats/go-multiaddr"
)

const (
	// TODO: officially register the multicodec numbers
	RovyMultiaddrCodec = 0x1a6
	MultiaddrFmtCodec  = 0x34
)

var (
	udp4protocols = []multiaddr.Protocol{
		multiaddr.ProtocolWithCode(multiaddr.P_IP4),
		multiaddr.ProtocolWithCode(multiaddr.P_UDP),
	}
	udp6protocols = []multiaddr.Protocol{
		multiaddr.ProtocolWithCode(multiaddr.P_IP6),
		multiaddr.ProtocolWithCode(multiaddr.P_UDP),
	}
	rovyProtocol = multiaddr.Protocol{
		Name:       "rovy",
		Code:       RovyMultiaddrCodec,
		VCode:      multiaddr.CodeToVarint(RovyMultiaddrCodec),
		Size:       multiaddr.LengthPrefixedVarSize,
		Transcoder: multiaddr.NewTranscoderFromFunctions(maddrStr2b, maddrB2Str, maddrValid),
	}
)

func init() {
	multiaddr.AddProtocol(rovyProtocol)
	multiaddr.AddProtocol(multiaddr.Protocol{
		Name:       "maddrfmt",
		Code:       MultiaddrFmtCodec,
		VCode:      multiaddr.CodeToVarint(MultiaddrFmtCodec),
		Size:       multiaddr.LengthPrefixedVarSize,
		Path:       true,
		Transcoder: multiaddr.NewTranscoderFromFunctions(maddrfmtStr2b, maddrfmtB2Str, nil),
	})
}

func maddrStr2b(s string) ([]byte, error) {
	c, err := cid.Decode(s)
	if err != nil {
		return nil, fmt.Errorf("failed to parse rovy addr: '%s' %s", s, err)
	}

	if ty := c.Type(); ty == RovyKeyMulticodec {
		return c.Bytes(), nil
	} else {
		return nil, fmt.Errorf("failed to parse rovy addr: '%s' has invalid codec %d", s, ty)
	}
}

func maddrB2Str(b []byte) (string, error) {
	c, err := cid.Cast(b)
	if err != nil {
		return "", err
	}
	pid, err := PeerIDFromCid(c)
	if err != nil {
		return "", err
	}
	return pid.String(), nil
}

func maddrValid(b []byte) error {
	_, err := cid.Cast(b)
	return err
}

func maddrfmtStr2b(s string) ([]byte, error) {
	return []byte(s), nil
}

func maddrfmtB2Str(b []byte) (string, error) {
	return string(b), nil
}

type Multiaddr struct {
	IP     netip.AddrPort
	PeerID PeerID
	More   multiaddr.Multiaddr
}

func MustParseMultiaddr(addr string) Multiaddr {
	ma, err := ParseMultiaddr(addr)
	if err != nil {
		panic(err)
	}
	return ma
}

func ParseMultiaddr(addr string) (ma Multiaddr, err error) {
	a := strings.Split(addr, "/")
	a = a[1:]
	if len(a) >= 4 && a[0] == "ip6" && a[2] == "udp" {
		if ma.IP, err = netip.ParseAddrPort("[" + a[1] + "]:" + a[3]); err != nil {
			return ma, err
		}
		a = a[4:]
	}
	if len(a) >= 4 && a[0] == "ip4" && a[2] == "udp" {
		if ma.IP, err = netip.ParseAddrPort(a[1] + ":" + a[3]); err != nil {
			return ma, err
		}
		a = a[4:]
	}
	if len(a) >= 2 && a[0] == "rovy" {
		c, err := cid.Parse(a[1])
		if err != nil {
			return ma, err
		}
		ma.PeerID, err = PeerIDFromCid(c)
		if err != nil {
			return ma, err
		}
		a = a[2:]
	}
	if len(a) > 0 {
		ma.More, err = multiaddr.NewMultiaddr("/" + strings.Join(a, "/"))
		if err != nil {
			return ma, err
		}
	}
	return ma, nil
}

func (ma Multiaddr) Empty() bool {
	return ma == Multiaddr{}
}

func (ma Multiaddr) Equal(other multiaddr.Multiaddr) bool {
	return bytes.Equal(ma.Bytes(), other.Bytes())
}

func (ma Multiaddr) Bytes() []byte {
	l := 0
	if ma.IP.Addr().Is4() {
		l += 9
	} else {
		l += 21
	}
	if ma.More != nil {
		mb := ma.More.Bytes()
		l += len(mb)
	}
	if ma.PeerID != emptyPeerID {
		l += 36 // TODO: correct length?
	}

	buf := make([]byte, l)
	n := 0

	if ma.IP.Addr().Is4() {
		buf[0] = 0x4 // varint multicodec for /ip4, code=4
		copy(buf[1:5], ma.IP.Addr().AsSlice())
		n += 5
	} else {
		buf[0] = 0x29 // varint multicodec for /ip6, code=41
		copy(buf[1:17], ma.IP.Addr().AsSlice())
		n += 17
	}
	buf[n+0] = 0x91 // varint multicodec for /udp, code=273
	buf[n+1] = 0x02
	buf[n+2] = byte(ma.IP.Port() >> 8)
	buf[n+3] = byte(ma.IP.Port())
	n += 4

	if ma.More != nil {
		mb := ma.More.Bytes()
		copy(buf[n+0:n+len(mb)], mb)
		n += len(mb)
	}
	if ma.PeerID != emptyPeerID {
		copy(buf[n+0:n+36], ma.More.Bytes())
		n += 36
	}

	if l != n {
		panic(fmt.Errorf("l != n: %d != %d", l, n))
	}
	return buf[:n]
}

func (ma Multiaddr) String() string {
	var out string
	if ma.IP.Addr().Is4() {
		out += "/ip4/"
	} else {
		out += "/ip6/"
	}
	port := strconv.FormatUint(uint64(ma.IP.Port()), 10)
	out += ma.IP.Addr().String() + "/udp/" + port
	if ma.More != nil {
		out += ma.More.String()
	}
	if ma.PeerID != emptyPeerID {
		out += "/rovy/" + ma.PeerID.String()
	}
	return out
}

func (ma Multiaddr) Protocols() []multiaddr.Protocol {
	var protos []multiaddr.Protocol
	if ma.IP.Addr().Is4() {
		protos = udp4protocols
	} else {
		protos = udp6protocols
	}
	if ma.More != nil {
		protos = append(protos, ma.More.Protocols()...)
	}
	if ma.PeerID != emptyPeerID {
		protos = append(protos, rovyProtocol)
	}
	return protos
}

func (ma Multiaddr) ValueForProtocol(code int) (string, error) {
	if code == multiaddr.P_IP4 && ma.IP.Addr().Is4() {
		return ma.IP.Addr().String(), nil
	} else if code == multiaddr.P_IP6 && ma.IP.Addr().Is6() {
		return ma.IP.Addr().String(), nil
	} else if code == multiaddr.P_UDP {
		return strconv.FormatUint(uint64(ma.IP.Port()), 10), nil
	}
	if code == RovyMultiaddrCodec && ma.PeerID != emptyPeerID {
		return ma.PeerID.String(), nil
	}

	if ma.More != nil {
		return ma.More.ValueForProtocol(code)
	}

	return "", fmt.Errorf("can't get address value for protocol 0x%x", code)
}

func (ma Multiaddr) Decapsulate(outer multiaddr.Multiaddr) multiaddr.Multiaddr {
	panic("not yet implemented")
	return nil
}

func (ma *Multiaddr) MarshalBinary() ([]byte, error) {
	panic("not yet implemented")
	return nil, nil
}

func (ma *Multiaddr) UnmarshalBinary(data []byte) error {
	panic("not yet implemented")
	return nil
}

func (ma *Multiaddr) MarshalText() ([]byte, error) {
	panic("not yet implemented")
	return nil, nil
}

func (ma *Multiaddr) UnmarshalText(data []byte) error {
	new, err := ParseMultiaddr(string(data))
	if err != nil {
		return err
	}
	*ma = new
	return nil
}

func (ma *Multiaddr) MarshalJSON() ([]byte, error) {
	panic("not yet implemented")
	return nil, nil
}

func (ma *Multiaddr) UnmarshalJSON(data []byte) error {
	panic("not yet implemented")
	return nil
}
