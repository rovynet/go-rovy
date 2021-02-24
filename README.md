# Rovy

Rovy is a (work-in-progress) permissionless routed peer-to-peer network. Rovy aims to tear down artifical barriers in internetworking, make routing and transit secure against attacks, and be backward-compatible by smoothly integrating with existing Internet infrastructure.

Rovy is pretty closely related to [libp2p](https://libp2p.io) and [cjdns](https://github.com/cjdelisle/cjdns), so you'll recognize many of their ideas and techniques here.

Rovy addresses are [multiaddrs](https://multiformats.io) and look like this:

- `/rovy/bafzqaicfbypxj4o5vk2d5k2jxueq62zhfhmhdhihsspndgjswdft74eehe` is the permanent address of a Rovy node which also contains its long-term public key. It is case-insensitive Base32 so it can be used in DNS domain names. It can also be encoded in other bases because it's simply a [CID](https://github.com/multiformats/cid).
- `/rovyfwd/7c37.e53a.0fb2.2800` is a label describing a route from one Rovy node to another, through 6 hops of Rovy nodes in between. These routes are relative to the respective nodes.
- `/ip6/fc6b:1f34:574e:837a:937f:317c:b280:0fb5` is the address in the optional fc00::/8 network for backward-compatibility with applications supporting IPv6 networking. This address is derived from the `/rovy` address above, and its public key is used to encrypt and sign every packet.

This repository will eventually contain:

- `rovyd`, the Rovy networking daemon (TODO)
- `rovyctl`, a CLI tool that configures Rovy daemons (TODO)
- `librovy`, a Go library for speaking to Rovy daemons or embedding them in Go programs (WIP)
- `rovybgp`, a BGP server that announces routes for regular Internet traffic that's allowed to be carried over Rovy (TODO)
- `rovydns`, a DNS server that helps facilitate global routing (TODO)

For the time being, check out the `examples/` directory.

## Roadmap

- [x] [IKpsk2](https://noiseprotocol.org/noise.html) handshake
- [x] 1 Gbps between two direct peers
- [ ] Forwarding/Switching using route labels
- [ ] Minimum-viable routing
- [ ] fc00::/8 network via TUN interface
- [ ] ICMP traceroutes for fc00::/8
- [ ] DNS server for global routing lookups (this is technically cheating)
- [ ] Local peer discovery
- [ ] 1 Gbps on fc00::/8
- [ ] 10 Gbps on fc00::/8
- [ ] 40 Gbps switching throughput on commodity hardware
- [ ] 100 Gbps switching throughput on commodity hardware
- [ ] Transit of Internet traffic using TUN, BGP, and RPKI
- [ ] DHT for decentralized global and local routing lookups
- [ ] Roaming and multi-homing
- [ ] Creative ways of facilitating peering
- [ ] Jumbo frames, path-specific MTU


## Immediate TODOs

- [ ] double-check SessionManager state transitions
- [ ] decouple Session.state and MsgType
- [ ] combine MsgType and Nonce into one header field to save bytes
- [ ] forwarder v0
- [ ] multigram
- [ ] forwarder error replies


## Project TODOs

- [ ] Basic website
- [ ] Git repo and issue tracker
- [ ] CI for tests
- [ ] CI for performance
- [ ] Squat twitter account
- [ ] ASN + IP addrs


## Notes

Measures to take for higher throughput

- Higher throughput is achieved by spending less time per packet
- Spend less time allocating memory by reusing buffers
- Split up and parallelize work, e.g. with per-peer queues
- Avoid parsing multiaddrs, make custom Multiaddr types backed by net.Addr and PeerID
- Be more reasonable about pointers
- Do some profiling to find more hotspots

Lock-free ring buffers:

- https://tanzu.vmware.com/content/blog/a-channel-based-ring-buffer-in-go
- https://github.com/textnode/gringo

Rovy packet headers:

- IPv4 header = 24 bytes
- IPv6 header = 40 bytes
- UDP header = 8 bytes
- At MTU=1500, an IPv4 UDP packet is 1468 bytes
- At MTU=1500, an IPv6 UDP packet is 1452 bytes
- Rovy header = 28 bytes
- Rovy footer = 16 bytes
- Rovy MTU = 1452 - 44 - 44 = 1364 bytes

Per-packet efficiency:

- UDP IPv6: 96.8%
- UDP IPv4: 97.9%
- Rovy IPv6: 90.9%
- Rovy IPv4: 92.0%
- Rovy Ethernet: 97.1%

Building blocks benchmarks (Ryzen 9 3900X):

- BenchmarkChacha20Poly1305/Open-1350-24  2315.59 MB/s
- BenchmarkChacha20Poly1305/Seal-1350-24  2148.09 MB/s
- BenchmarkChacha20Poly1305/Open-8192-24  2892.55 MB/s
- BenchmarkChacha20Poly1305/Seal-8192-24  2873.19 MB/s
- BenchmarkChacha20Poly1305/Open-65536-24 3123.44 MB/s
- BenchmarkChacha20Poly1305/Seal-65536-24 3087.66 MB/s
- BenchmarkForwarder/HandlePacket-1500-24 40788.59 MB/s 0 allocs/op

DNS-facilitated routing:

- TXT bafzqaieveriforqgnk65hpm7sqxqovgzjldb2jv4jybfxy7tiza2otdhk4.00af.fedb.12bc.1312.acab.cafe.bafzqaiaeapxje5ifb2a6mhpdb3epdj6rydwaustcilbztiydymchitopiy.route.rovy.net "routes=00af.fedb.12bc.1234.1234.cafe,00af.fedb.12bc.fcfc.afaf.cafe"

IRR / RPKI:

- http://www.irr.net/
- http://www.irr.net/docs/list.html
- https://www.ripe.net/manage-ips-and-asns/db/nrtm-mirroring
- https://github.com/job/irrexplorer
- https://ftp.ripe.net/rpki/ripencc.tal/2021/02/15/
- https://www.ripe.net/manage-ips-and-asns/resource-management/rpki
