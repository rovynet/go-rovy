module github.com/rovynet/go-rovy

go 1.15

require (
	github.com/ipfs/go-cid v0.1.0
	github.com/klauspost/cpuid/v2 v2.0.9 // indirect
	github.com/kr/pretty v0.3.0
	github.com/multiformats/go-base32 v0.0.4 // indirect
	github.com/multiformats/go-multiaddr v0.4.1
	github.com/multiformats/go-multihash v0.0.16
	github.com/multiformats/go-varint v0.0.6
	github.com/rogpeppe/go-internal v1.8.0 // indirect
	github.com/vishvananda/netlink v1.1.0
	github.com/vishvananda/netns v0.0.0-20211101163701-50045581ed74
	golang.org/x/crypto v0.0.0-20210921155107-089bfa567519
	golang.org/x/net v0.0.0-20211101193420-4a448f8816b3
	golang.org/x/sys v0.0.0-20211101204403-39c9dd37992c // indirect
	golang.zx2c4.com/wireguard v0.0.0-20211030003956-52704c4b9288
)

//replace golang.zx2c4.com/wireguard => ../wireguard-go
