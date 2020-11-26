module pkt.dev/go-rovy

go 1.15

require (
	github.com/ipfs/go-cid v0.0.7
	github.com/minio/sha256-simd v0.1.1 // indirect
	github.com/mr-tron/base58 v1.2.0 // indirect
	github.com/multiformats/go-multiaddr v0.3.1
	github.com/multiformats/go-multihash v0.0.14
	github.com/multiformats/go-varint v0.0.6
	golang.org/x/crypto v0.0.0-20201124201722-c8d3bf9c5392
	golang.org/x/sys v0.0.0-20201119102817-f84b799fce68 // indirect
)

//replace golang.zx2c4.com/wireguard => ../wireguard-go
