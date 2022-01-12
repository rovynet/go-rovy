package rovy

import (
	"bytes"
	"fmt"
	"net/netip"
	"strconv"

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
