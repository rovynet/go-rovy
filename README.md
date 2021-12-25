
# The Rovy Network

Rovy is a global encrypted computer network that anyone can join and extend.

Rovy aims to tear down artifical barriers in internetworking, make routing and transit secure against certain attacks, and smoothly integrate with existing Internet infrastructure.

**Anyone can join** -- TODO: openness and accessiblity through 1. zero config 2. permissionless and flat keyspace 3. no costly registry fees 4. good performance on cheap hardware

**Encrypted & authenticated** -- Rovy guarantees the confidentiality, integrity, and authenticity of every single packet. This applies to connections between two nodes with other nodes in between (end-to-end) as well as the underlying direct connections between the involved nodes (hop-to-hop). Rovy connections use a handshake protocol called Noise_IKpsk2_25519_ChaChaPoly_BLAKE2s originally introduced by Wireguard. In addition, all records shared with the network in order to facilitate routing are authenticated with signatures to avoid forgery.

**Compatible with all IPv6-capable applications** -- TODO: 1. need for upgrade path 2. fc00::/8 on tun 3. per-port dual-stack

**End-to-end principle restored, middleboxes simplified** -- TODO

**Secure Internet routing** -- 1. can carry any typical ipv4/ipv6 traffic 2. bgp and reusing keys from rpki for our own stuff 3. control plane is encrypted

**An upgrade to the Internet's protocols** -- TODO

The protocols making up Rovy are pretty closely related to the ideas and concepts of [cjdns](https://github.com/cjdelisle/cjdns) and [libp2p](https://libp2p.io).

Right now (late 2021) Rovy is an early work-in-progress with some foundational parts in place and working, and many of the more interesting parts still missing.

## Usage

There is no CLI yet, just a bunch of smoketests :-)

```sh
> go test -v ./examples
> go run ./examples/benchmark
> go test -bench=. ./forwarder
```

To test IPv6 networking over Rovy's TUN interface, run traceroute against the IPv6 address of `nodeD` from the following command's output:
```
> go build -o rovy-fc00 ./examples/fc00 && sudo ./rovy-fc00
...
[nodeD] 02:28:45 main.go:32: /ip6/fcc7:6ea0:d8bb:448a:4d82:fcc0:982c:ceed
...
> mtr -n fcc7:6ea0:d8bb:448a:4d82:fcc0:982c:ceed
```

---

## *From here on just random notes*

Rovy addresses are [multiaddrs](https://multiformats.io) and look like this:

- `/rovy/bafzqaicfbypxj4o5vk2d5k2jxueq62zhfhmhdhihsspndgjswdft74eehe` is the permanent address of a Rovy node which also contains its long-term public key. It is case-insensitive Base32 so it can be used in DNS domain names. It can also be encoded in other bases because it's simply a [CID](https://github.com/multiformats/cid).
- `/rovyrt/7c.37.e5.3a.0f.b2.28` is an address describing a route from one Rovy node to another, through 6 hops of Rovy nodes in between. These routes are relative to the respective nodes.
- `/ip6/fc6b:1f34:574e:837a:937f:317c:b280:0fb5` is the address in the optional fc00::/8 network for backward-compatibility with applications supporting IPv6 networking. This address is derived from the `/rovy` address above, and its public key is used to encrypt and sign every packet.

This repository will eventually contain:

- `rovyd`, the Rovy networking daemon (TODO)
- `rovyctl`, a CLI tool that configures Rovy daemons (TODO)
- `librovy`, a Go library for speaking to Rovy daemons or embedding them in Go programs (WIP)
- `rovybgp`, a BGP server that announces routes for regular Internet traffic that's allowed to be carried over Rovy (TODO)
- `rovydns`, a DNS server that helps facilitate global routing (TODO)

For the time being, check out the `examples/` directory.

## Roadmap

### Epics

- [x] [IKpsk2](https://noiseprotocol.org/noise.html) handshake
- [x] Forwarding/Switching using route labels
- [x] fc00::/8 network via TUN interface
- [x] ICMP traceroutes for fc00::/8
- [ ] Minimum-viable routing
- [ ] Daemon and CLI
- [ ] Local peer discovery
- [ ] DNS server for global routing lookups (this is technically cheating)
- [ ] 1 Gbps routed throughput on fc00::/8
- [ ] 1 Gbps routed throughput on fc00::/8 on a cheap ARM board
- [ ] 10 Gbps routed throughput on fc00::/8
- [ ] Transit of Internet traffic using TUN, BGP, and RPKI
- [ ] DHT for decentralized global and local routing lookups
- [ ] Roaming and multi-homing
- [ ] Creative ways of facilitating peering
- [ ] Jumbo frames, path-specific MTU
- [ ] Node management, modifying the node, shutting down

### Next

- [x] remove multigram table negotiation, it doesn't make sense much sense at the moment
- [ ] make multigram varints exactly 4 bytes long, padding + length restriction
- [ ] unify lowerpacket and upperpacket in one packet type, which tracks state (lower, upper, tpt) and gives access to respective payload slices
- [ ] transmitter object that knows route (and thus mtu) before we construct packet
- [x] node: add lower codec for direct-upper hack
- [ ] node: payload overflow checks
- [ ] node: enforce max route length of 14 bytes
- [ ] constants for sizes and offsets
- [ ] fix endianness once and for all, do what wireguard and cjdns do
- [ ] fc00: signatures on ping/pong
- [ ] fc00: multicast ping
- [ ] perf: lock-free maps for SessionManager and Forwarder
- [ ] perf: buffer pool
- [x] perf: ring buffers
- [x] session: handshake waiters are strange - not anymore :-)
- [ ] session: get the stages in order
- [ ] session: replay protection, flood protection, cookie
- [ ] session: timeouts, handshake retransmission
- [ ] session: research whether hello/response payload construction is okay

### Project TODOs

- [x] ASN + IP addrs
- [x] Basic website
- [ ] Git repo and issue tracker
- [ ] CI for tests
- [ ] CI for performance
- [ ] Squat twitter account
- [ ] Multicodec registrations
  - 0x73    /rovy/v0/peerid (same codec for cid and multiaddr)
  -         /rovy/v0/route (same codec for route and multiaddr)
  - 0x1     /rovy/v0/hello
  - 0x2     /rovy/v0/response
  - 0x3     /rovy/v0/cookie
  - 0x4     /rovy/v0/message
  - 0x12345 /rovy/v0/fwd
  - 0x12346 /rovy/v0/fwdctl
  - 0x12347 /rovy/v0/directupper
  - 0x42004 /rovy/v0/fc00
  - 0x42005 /rovy/v0/fc00trace


## Notes

- [ ] check out wireguard-go/conn, why does it exist? sticky sockets, perf?
- [ ] check out x/sys
- [ ] benchmark: goroutine throughput, large routing table, udp read pps, udp write pps
- [ ] security: check noise-protocol application responsibilities and security considerations

Measures to take for higher throughput:

- Higher throughput is achieved by spending less time per packet
- Spend less time allocating memory by reusing buffers
- Split up and parallelize work, e.g. with per-peer queues
- Avoid parsing multiaddrs, make custom Multiaddr types backed by net.Addr and PeerID
- Be more reasonable about pointers
- Do some profiling to find more hotspots
- Use net.af/netaddr for IP addresses
- Profiling: https://github.com/pyroscope-io/pyroscope

Data structures:

- https://github.com/cornelk/hashmap
- https://tanzu.vmware.com/content/blog/a-channel-based-ring-buffer-in-go
- https://github.com/textnode/gringo

Per-packet overhead:

- UDPv4 plain                   -- 32 bytes       -- 1468/1500 = 97.9%
- UDPv6 plain                   -- 48 bytes       -- 1452/1500 = 96.8%
- UDPv4 over Rovy over UDPv6    -- 32+88+48 bytes -- 1332/1500 = 88.8%
- UDPv6 over Rovy over UDPv6    -- 48+88+48 bytes -- 1316/1500 = 87.7%
- UDPv4 over Rovy over Ethernet -- 32+88 bytes    -- 1380/1500 = 92.0%
- UDPv6 over Rovy over Ethernet -- 48+88 bytes    -- 1364/1500 = 90.9%
- Rovy over UDPv6               -- 88+48 bytes    -- 1364/1500 = 90.9%
- Rovy over Ethernet            -- 88 bytes       -- 1412/1500 = 94.1%

MTU reading list:

- https://news.ycombinator.com/item?id=22364830
- https://news.ycombinator.com/item?id=27673945

UDP performance tuning:

- https://www.slideshare.net/lfevents/boost-udp-transaction-performance

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
- https://ftp.ripe.net/rpki/ripencc.tal/2021/02/15/
- https://www.ripe.net/manage-ips-and-asns/resource-management/rpki
- https://www.arin.net/resources/manage/rpki/roa_request/#submitting-a-manually-signed-roa
- RTA Resource Tagged Attestation https://blog.apnic.net/2020/11/20/moving-rpki-beyond-routing-security/
- Discussing the Future of RPKI https://blog.apnic.net/2021/01/29/discussing-the-future-of-rpki/
- NRTM v4 Discussion https://github.com/mxsasha/nrtmv4/blob/main/nrtmv4.md
- Routinator has RTA support https://github.com/NLnetLabs/routinator
- Krill has RTA support https://github.com/NLnetLabs/krill
- IRR Explorer https://github.com/dashcare/irrexplorer
- AS Path Prepending https://catalog.caida.org/details/paper/2020_aspath_prepending/

How to handle double encryption when sending to peers:

- When upper layer gets a packet to send, if the route is single-hop, we'll skip the upper layer and directly send on the lower layer
- When lower layer receives a packet and it's not a forwarder packet, we'll skip the lower layer and directly receive on the upper layer
- This also means we want a combined multigram table for upper and lower layer
- Different packet overhead should be taken into account when calculating MTU
- We'll use a single active session per peer at a time, no matter if in upper or lower layer. If a packet from a peer can suddenly come from a different internet address because of roaming, then a packet can also suddenly come from a different peer by means of forwarding.

Other relevant networking software:

- Yggdrasil
- Pinecone https://matrix.org/blog/2021/05/06/introducing-the-pinecone-overlay-network
- cjdns
- Tailscale
- Innernet
- Nebula
- Tinc
- ZeroTier
- gVisor
- SCION https://www.scion-architecture.net/ https://labs.ripe.net/author/hausheer/scion-a-novel-internet-architecture/
- frp https://github.com/fatedier/frp
