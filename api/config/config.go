package rconfig

import (
	"bytes"
	"fmt"
	"io"
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

// TODO: simplify all this by impl'ing Keyfile.Marshal/Unmarshal

func LoadKeyfile(path string) (*Keyfile, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	return NewKeyfile(bytes.NewReader(b))
}

func NewKeyfile(r io.Reader) (*Keyfile, error) {
	var kfraw struct {
		PrivateKey string
		PeerID     string
		IPAddr     string
	}
	if err := toml.NewDecoder(r).Decode(&kfraw); err != nil {
		return nil, err
	}

	var kf Keyfile
	var err error
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

func SaveKeyfile(path string, kf *Keyfile) error {
	str := fmt.Sprintf(`#
# Rovy Keyfile
#
# Privatekey is base64-encoded with a Multibase header.
# PeerID is the base32-encoded public key, prefixed with the "bafzqai" CID header.
# IPAddr is bytes 16 to 31 of the public key's double Blake2s-256 hash.
#
# For a PrivateKey and PeerID to be valid, their IPAddr must be within fc00::/8.
#
PrivateKey = '%s'
PeerID = '%s'
IPAddr = '%s'
`, kf.PrivateKey, kf.PeerID, kf.IPAddr)

	if err := os.WriteFile(path, []byte(str), 0600); err != nil {
		return err
	}
	return nil
}

type Config struct {
	Peer      Peer
	Fcnet     Fcnet
	Discovery Discovery
}

type Peer struct {
	Listen  []rovy.Multiaddr
	Connect []rovy.Multiaddr
	Policy  []string
}

type Fcnet struct {
	Enabled bool
	Ifname  string
}

type Discovery struct {
	LinkLocal LinkLocal
}

type LinkLocal struct {
	Enabled  bool
	Interval string
}

func DefaultConfig() *Config {
	cfg := &Config{
		Peer: Peer{
			Listen: []rovy.Multiaddr{
				rovy.MustParseMultiaddr("/ip6/::/udp/1312"),
				rovy.MustParseMultiaddr("/ip4/0.0.0.0/udp/1312"),
				// rovy.MustParseMultiaddr("/ethif/wlp3s0"),
			},
			Connect: []rovy.Multiaddr{},
			Policy:  []string{"local", "open"},
		},
		Fcnet: Fcnet{
			Enabled: true,
			Ifname:  "rovy0",
		},
		Discovery: Discovery{
			LinkLocal: LinkLocal{
				Enabled:  true,
				Interval: "5s",
			},
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
	b, err := toml.Marshal(cfg)
	if err != nil {
		return err
	}

	if err := os.WriteFile(path, b, 0600); err != nil {
		return err
	}
	return nil
}
