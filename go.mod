module go.rovy.net

go 1.19

require (
	github.com/cucumber/godog v0.12.5
	github.com/godbus/dbus/v5 v5.1.0
	github.com/gorilla/mux v1.8.0
	github.com/ipfs/go-cid v0.3.0
	github.com/miekg/dns v1.1.50
	github.com/mitchellh/go-homedir v1.1.0
	github.com/multiformats/go-multiaddr v0.6.0
	github.com/multiformats/go-multibase v0.1.1
	github.com/multiformats/go-multihash v0.2.1
	github.com/multiformats/go-varint v0.0.6
	github.com/pelletier/go-toml/v2 v2.0.5
	github.com/urfave/cli/v2 v2.11.2
	github.com/vishvananda/netlink v1.2.1-beta.2
	github.com/vishvananda/netns v0.0.0-20211101163701-50045581ed74
	golang.org/x/crypto v0.0.0-20220829220503-c86fa9a7ed90
	golang.org/x/net v0.0.0-20220826154423-83b083e8dc8b
	golang.org/x/sys v0.0.0-20220829200755-d48e67d00261
	golang.zx2c4.com/wireguard v0.0.0-20220829161405-d1d08426b27b
)

require (
	github.com/cpuguy83/go-md2man/v2 v2.0.2 // indirect
	github.com/cucumber/gherkin-go/v19 v19.0.3 // indirect
	github.com/cucumber/messages-go/v16 v16.0.1 // indirect
	github.com/gofrs/uuid v4.0.0+incompatible // indirect
	github.com/google/btree v1.1.2 // indirect
	github.com/hashicorp/go-immutable-radix v1.3.0 // indirect
	github.com/hashicorp/go-memdb v1.3.0 // indirect
	github.com/hashicorp/golang-lru v0.5.4 // indirect
	github.com/klauspost/cpuid/v2 v2.1.1 // indirect
	github.com/minio/sha256-simd v1.0.0 // indirect
	github.com/mr-tron/base58 v1.2.0 // indirect
	github.com/multiformats/go-base32 v0.1.0 // indirect
	github.com/multiformats/go-base36 v0.1.0 // indirect
	github.com/russross/blackfriday/v2 v2.1.0 // indirect
	github.com/spaolacci/murmur3 v1.1.0 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
	github.com/xrash/smetrics v0.0.0-20201216005158-039620a65673 // indirect
	golang.org/x/mod v0.6.0-dev.0.20220419223038-86c51ed26bb4 // indirect
	golang.org/x/time v0.0.0-20220722155302-e5dcc9cfc0b9 // indirect
	golang.org/x/tools v0.1.12 // indirect
	golang.zx2c4.com/wintun v0.0.0-20211104114900-415007cec224 // indirect
	gvisor.dev/gvisor v0.0.0-20220817001344-846276b3dbc5 // indirect
	lukechampine.com/blake3 v1.1.7 // indirect
)

//replace golang.zx2c4.com/wireguard => ../wireguard-go
