package rovy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/netip"
	"strconv"
	"strings"

	cid "github.com/ipfs/go-cid"
	multiaddr "github.com/multiformats/go-multiaddr"
)

const (
	// TODO: officially register the multicodec numbers
	RovyMultiaddrCodec  = 0x1a6
	ProtoMultiaddrCodec = 0x34
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
	protoProtocol = multiaddr.Protocol{
		Name:       "proto",
		Code:       ProtoMultiaddrCodec,
		VCode:      multiaddr.CodeToVarint(ProtoMultiaddrCodec),
		Size:       multiaddr.LengthPrefixedVarSize,
		Path:       true,
		Transcoder: multiaddr.NewTranscoderFromFunctions(protoMaddrStr2b, protoMaddrB2Str, nil),
	}
)

func init() {
	multiaddr.AddProtocol(rovyProtocol)
	multiaddr.AddProtocol(protoProtocol)
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

func protoMaddrStr2b(s string) ([]byte, error) {
	return []byte(s), nil
}

func protoMaddrB2Str(b []byte) (string, error) {
	return string(b), nil
}

type Multiaddr struct {
	IP     netip.Addr
	Port   uint16
	PeerID PeerID
	More   multiaddr.Multiaddr
}

// TODO: implement ParseMultiaddrBytes
// TODO: implement encoding.BinaryMarshaler & Unmarshaler
// TODO: implement Multiaddr.Encapsulate & Decapsulate
//var _ multiaddr.Multiaddr = Multiaddr{}

func MustParseMultiaddr(addr string) Multiaddr {
	ma, err := ParseMultiaddr(addr)
	if err != nil {
		panic(err)
	}
	return ma
}

// TODO: support parsing IP addresses without a port (currently end up in ma.More)
func ParseMultiaddr(addr string) (ma Multiaddr, err error) {
	a := strings.Split(addr, "/")
	a = a[1:]
	if len(a) >= 4 && a[0] == "ip6" && a[2] == "udp" {
		ip, err := netip.ParseAddrPort("[" + a[1] + "]:" + a[3])
		if err != nil {
			return ma, err
		}
		ma.IP, ma.Port = ip.Addr(), ip.Port()
		a = a[4:]
	}
	if len(a) >= 4 && a[0] == "ip4" && a[2] == "udp" {
		ip, err := netip.ParseAddrPort(a[1] + ":" + a[3])
		if err != nil {
			return ma, err
		}
		ma.IP, ma.Port = ip.Addr(), ip.Port()
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

func FromAddrPort(ip netip.AddrPort) Multiaddr {
	return Multiaddr{IP: ip.Addr(), Port: ip.Port()}
}

func (ma Multiaddr) AddrPort() netip.AddrPort {
	return netip.AddrPortFrom(ma.IP, ma.Port)
}

func (ma Multiaddr) Empty() bool {
	return ma == Multiaddr{}
}

func (ma Multiaddr) Equal(other multiaddr.Multiaddr) bool {
	return bytes.Equal(ma.Bytes(), other.Bytes())
}

// TODO: /rovy part doesn't have a multiaddr codec prefix
func (ma Multiaddr) Bytes() []byte {
	l := 0
	if ma.IP.IsValid() {
		if ma.IP.Is4() {
			l += 9
		} else {
			l += 21
		}
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

	if ma.IP.IsValid() {
		if ma.IP.Is4() {
			buf[0] = 0x4 // varint multicodec for /ip4, code=4
			copy(buf[1:5], ma.IP.AsSlice())
			n += 5
		} else {
			buf[0] = 0x29 // varint multicodec for /ip6, code=41
			copy(buf[1:17], ma.IP.AsSlice())
			n += 17
		}
		buf[n+0] = 0x91 // varint multicodec for /udp, code=273
		buf[n+1] = 0x02
		buf[n+2] = byte(ma.Port >> 8)
		buf[n+3] = byte(ma.Port)
		n += 4
	}

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
	if ma.IP.IsValid() {
		if ma.IP.Is4() {
			out += "/ip4/" + ma.IP.String()
		} else {
			out += "/ip6/" + ma.IP.String()
		}
		if ma.Port > 0 {
			out += "/udp/" + strconv.FormatUint(uint64(ma.Port), 10)
		}
	}
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
	if ma.IP.IsValid() {
		if ma.IP.Is4() {
			protos = udp4protocols
		} else {
			protos = udp6protocols
		}
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
	if code == multiaddr.P_IP4 && ma.IP.Is4() {
		return ma.IP.String(), nil
	} else if code == multiaddr.P_IP6 && ma.IP.Is6() {
		return ma.IP.String(), nil
	} else if code == multiaddr.P_UDP && ma.IP.IsValid() {
		return strconv.FormatUint(uint64(ma.Port), 10), nil
	}
	if code == RovyMultiaddrCodec && ma.PeerID != emptyPeerID {
		return ma.PeerID.String(), nil
	}

	if ma.More != nil {
		return ma.More.ValueForProtocol(code)
	}

	return "", fmt.Errorf("can't get address value for protocol 0x%x", code)
}

func (ma *Multiaddr) MarshalText() ([]byte, error) {
	return []byte(ma.String()), nil
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
	return json.Marshal(ma.String())
}

func (ma *Multiaddr) UnmarshalJSON(data []byte) error {
	if len(data) == 0 {
		*ma = Multiaddr{}
		return nil
	}

	var v string
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	var err error
	*ma, err = ParseMultiaddr(v)
	return err
}
