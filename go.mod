module pkt.dev/go-rovy

go 1.15

require (
	github.com/ipfs/go-cid v0.1.0
	github.com/klauspost/cpuid/v2 v2.0.9 // indirect
	github.com/kr/pretty v0.2.1
	github.com/multiformats/go-multiaddr v0.4.0
	github.com/multiformats/go-multihash v0.0.16
	github.com/multiformats/go-varint v0.0.6
	github.com/vishvananda/netlink v1.1.0
	github.com/vishvananda/netns v0.0.0-20191106174202-0a2b9b5464df
	golang.org/x/crypto v0.0.0-20210817164053-32db794688a5
	golang.org/x/net v0.0.0-20210226172049-e18ecbb05110
	golang.org/x/sys v0.0.0-20210903071746-97244b99971b // indirect
	golang.zx2c4.com/wireguard v0.0.20201118
)

//replace golang.zx2c4.com/wireguard => ../wireguard-go
