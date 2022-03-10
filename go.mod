module go.rovy.net

go 1.15

require (
	github.com/godbus/dbus/v5 v5.0.6
	github.com/gorilla/mux v1.7.2
	github.com/ipfs/go-cid v0.1.0
	github.com/klauspost/cpuid/v2 v2.0.9 // indirect
	github.com/miekg/dns v1.1.46
	github.com/multiformats/go-base32 v0.0.4 // indirect
	github.com/multiformats/go-multiaddr v0.4.1
	github.com/multiformats/go-multibase v0.0.3
	github.com/multiformats/go-multihash v0.0.16
	github.com/multiformats/go-varint v0.0.6
	github.com/urfave/cli/v2 v2.3.0
	github.com/vishvananda/netlink v1.1.1-0.20211101163509-b10eb8fe5cf6
	github.com/vishvananda/netns v0.0.0-20211101163701-50045581ed74
	golang.org/x/crypto v0.0.0-20211202192323-5770296d904e
	golang.org/x/net v0.0.0-20211205041911-012df41ee64c
	golang.org/x/sys v0.0.0-20211205182925-97ca703d548d
	golang.zx2c4.com/go118/netip v0.0.0-20211105124833-002a02cb0e53
	golang.zx2c4.com/wireguard v0.0.0-20211116201604-de7c702ace45
	golang.zx2c4.com/wireguard/tun/netstack v0.0.0-20220202223031-3b95c81cc178
)

//replace golang.zx2c4.com/wireguard => ../wireguard-go
