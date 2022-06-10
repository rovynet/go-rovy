package rovycfg

import (
	"bytes"
	"fmt"
	"net/netip"
	"os"

	cid "github.com/ipfs/go-cid"
	toml "github.com/pelletier/go-toml/v2"
	rovy "go.rovy.net"
)

type Keyfile struct {
	PrivateKey rovy.PrivateKey
	PeerID     rovy.PeerID
	IPAddr     netip.Addr
}

// TODO: simplify this by impl'ing Keyfile.Marshal/Unmarshal
func LoadKeyfile(path string) (*Keyfile, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var kfraw struct {
		PrivateKey string
		PeerID     string
		IPAddr     string
	}
	if err := toml.NewDecoder(bytes.NewReader(b)).Decode(&kfraw); err != nil {
		return nil, err
	}

	var kf Keyfile
	kf.PrivateKey, err = rovy.ParsePrivateKey(kfraw.PrivateKey)
	if err != nil {
		return nil, err
	}
	c, err := cid.Decode(kfraw.PeerID)
	if err != nil {
		return nil, err
	}
	kf.PeerID, err = rovy.PeerIDFromCid(c)
	if err != nil {
		return nil, err
	}
	kf.IPAddr, err = netip.ParseAddr(kfraw.IPAddr)
	if err != nil {
		return nil, err
	}

	if kf.PeerID != rovy.NewPeerID(kf.PrivateKey.PublicKey()) {
		return nil, fmt.Errorf("wrong PeerID in keyfile: %s", kf.PeerID)
	}
	if kf.IPAddr != kf.PrivateKey.PublicKey().IPAddr() {
		return nil, fmt.Errorf("wrong IPAddr in keyfile: %s", kf.IPAddr)
	}

	return &kf, nil
}

func SaveKeyfile(path string, cfg *Keyfile) error {
	return nil
}

type Config struct {
	Peer Peer
	Fc00 Fc00
}

type Peer struct {
	Dialers   []rovy.Multiaddr
	Listeners []rovy.Multiaddr
}

type Fc00 struct {
	Enabled bool
	Ifname  string
}

func DefaultConfig() *Config {
	cfg := &Config{
		Peer: Peer{
			Dialers: []rovy.Multiaddr{
				rovy.MustParseMultiaddr("/maddrfmt/ip6/udp"),
				rovy.MustParseMultiaddr("/maddrfmt/ip4/udp"),
				// rovy.MustParseMultiaddr("/maddrfmt/ethif"),
			},
			Listeners: []rovy.Multiaddr{
				rovy.MustParseMultiaddr("/ip6/::/udp/1312"),
				rovy.MustParseMultiaddr("/ip4/0.0.0.0/udp/1312"),
				// rovy.MustParseMultiaddr("/ethif/wlp3s0"),
			},
		},
		Fc00: Fc00{
			Enabled: true,
			Ifname:  "rovy0",
		},
	}

	return cfg
}

func LoadConfig(path string) (*Config, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := toml.NewDecoder(bytes.NewReader(b)).Decode(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func SaveConfig(path string, cfg *Config) error {
	return nil
}
