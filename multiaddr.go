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

type UDPMultiaddr struct {
	ip netip.AddrPort
}

var emptyUDPMultiaddr = UDPMultiaddr{}

func NewUDPMultiaddr(ip netip.AddrPort) UDPMultiaddr {
	return UDPMultiaddr{ip}
}

func (ma UDPMultiaddr) AddrPort() netip.AddrPort {
	return ma.ip
}

func (ma UDPMultiaddr) Empty() bool {
	return ma == emptyUDPMultiaddr
}

func (ma UDPMultiaddr) Equal(other multiaddr.Multiaddr) bool {
	return bytes.Equal(ma.Bytes(), other.Bytes())
}

func (ma UDPMultiaddr) Bytes() []byte {
	out := make([]byte, 21)
	i := 16
	if ma.ip.Addr().Is4() {
		i = 4
		out[0] = 0x4 // varint multicodec for /ip4, code=4
		copy(out[1:5], ma.ip.Addr().AsSlice())
	} else {
		out[0] = 0x29 // varint multicodec for /ip6, code=41
		copy(out[1:17], ma.ip.Addr().AsSlice())
	}
	out[i+1] = 0x91 // varint multicodec for /udp, code=273
	out[i+2] = 0x02
	out[i+3] = byte(ma.ip.Port() >> 8)
	out[i+4] = byte(ma.ip.Port())
	return out
}

func (ma UDPMultiaddr) String() string {
	port := strconv.FormatUint(uint64(ma.ip.Port()), 10)
	if ma.ip.Addr().Is4() {
		return "/ip4/" + ma.ip.Addr().String() + "/udp/" + port
	} else {
		return "/ip6/" + ma.ip.Addr().String() + "/udp/" + port
	}
}

var ip4protocols = []multiaddr.Protocol{
	multiaddr.ProtocolWithCode(multiaddr.P_IP4),
	multiaddr.ProtocolWithCode(multiaddr.P_UDP),
}
var ip6protocols = []multiaddr.Protocol{
	multiaddr.ProtocolWithCode(multiaddr.P_IP6),
	multiaddr.ProtocolWithCode(multiaddr.P_UDP),
}
var rovyProtocol = multiaddr.ProtocolWithCode(RovyMultiaddrCodec)

func (ma UDPMultiaddr) Protocols() []multiaddr.Protocol {
	if ma.ip.Addr().Is4() {
		return ip4protocols
	} else {
		return ip6protocols
	}
}

func (ma UDPMultiaddr) ValueForProtocol(code int) (string, error) {
	if code == multiaddr.P_IP4 && ma.ip.Addr().Is4() {
		return ma.ip.Addr().String(), nil
	} else if code == multiaddr.P_IP6 && ma.ip.Addr().Is6() {
		return ma.ip.Addr().String(), nil
	} else if code == multiaddr.P_UDP {
		return strconv.FormatUint(uint64(ma.ip.Port()), 10), nil
	}
	return "", fmt.Errorf("can't get address value for protocol 0x%x", code)
}

func (ma UDPMultiaddr) Encapsulate(inner multiaddr.Multiaddr) multiaddr.Multiaddr {
	panic("not yet implemented")
	return nil
}

func (ma UDPMultiaddr) Decapsulate(outer multiaddr.Multiaddr) multiaddr.Multiaddr {
	panic("not yet implemented")
	return nil
}

func (ma UDPMultiaddr) MarshalBinary() ([]byte, error) {
	panic("not yet implemented")
	return nil, nil
}

func (ma UDPMultiaddr) UnmarshalBinary(data []byte) error {
	panic("not yet implemented")
	return nil
}

func (ma UDPMultiaddr) MarshalText() ([]byte, error) {
	panic("not yet implemented")
	return nil, nil
}

func (ma UDPMultiaddr) UnmarshalText(data []byte) error {
	panic("not yet implemented")
	return nil
}

func (ma UDPMultiaddr) MarshalJSON() ([]byte, error) {
	panic("not yet implemented")
	return nil, nil
}

func (ma UDPMultiaddr) UnmarshalJSON(data []byte) error {
	panic("not yet implemented")
	return nil
}

func init() {
	multiaddr.AddProtocol(multiaddr.Protocol{
		Name:       "maddrfmt",
		Code:       0x34,
		VCode:      multiaddr.CodeToVarint(0x34),
		Size:       multiaddr.LengthPrefixedVarSize,
		Path:       true,
		Transcoder: multiaddr.NewTranscoderFromFunctions(maddrfmtStr2b, maddrfmtB2Str, nil),
	})
}

func maddrfmtStr2b(s string) ([]byte, error) {
	return []byte(s), nil
}

func maddrfmtB2Str(b []byte) (string, error) {
	return string(b), nil
}

type Multiaddr struct {
	IP     netip.AddrPort
	More   multiaddr.Multiaddr
	PeerID PeerID
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
		protos = ip4protocols
	} else {
		protos = ip6protocols
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
