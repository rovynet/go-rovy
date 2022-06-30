
# Epics

- [x] Noise IKpsk2 handshake
- [x] Forwarding/Switching using route labels
- [x] fc00::/8 network via TUN interface
- [x] ICMP traceroutes for fc00::/8
- [x] Daemon and CLI
- [x] Configuration
- [ ] Local peer discovery
- [ ] TLS termination and re-encryption
- [ ] Systemd unit file for servers
- [ ] Minimum-viable routing
- [ ] 1 Gbps routed throughput on fc00::/8
- [ ] Gnome extension via DBus API
- [ ] Petnames in .rovy TLD
- [ ] External routing protocols, e.g. Babel and OLSR
- [ ] DHT for decentral global and local routing lookups
- [ ] Support for onion-like sessions
- [ ] Transit of Internet traffic using TUN interface, BGP, and RPKI RTAs

# Next

- [ ] node: proper transport/listener/dialer interfaces
- [ ] node: clean shutdown (close tun, delete nmconn, notify peers)
- [ ] all: Context wiring

- [ ] test: basic shared cucumber suite for CLI, HTTP API, Go API
- [ ] fcnet: signatures on ping/pong
- [ ] session: replay protection, flood protection, cookie
- [ ] session: timeouts, handshake retransmission
- [ ] session: get the stages in order
- [ ] session: research whether hello/response payload construction is okay
- [ ] session: rework the complicated way we handle session remoteAddr
- [x] perf: GC-friendly PeerID and Addr types
- [x] perf: Break work up into queues

# Backlog

- [x] session: handshake waiters are strange - not anymore :-)
- [x] remove multigram table negotiation, it doesn't make sense much sense at the moment
- [x] node: add lower codec for direct-upper hack
- [x] fcnet: less verbose error handling
- [ ] fcnet: ping ff02::1%rovy
- [ ] fcnet: reverse dns
- [ ] fcnet: clarify the fcnet api interface
- [ ] fcnet: rename to fcnet
- [x] fcnet: embedded virtual tun device
- [ ] fcnet: node keeps track of fcnet service
- [ ] cli: rovy fcnet start command with --nm and other options
- [ ] fcnet: fcnet stop and status commands
- [ ] fcnet: default-deny and fcnet ports command
- [ ] fcnet: define fc00::/64 as unroutable
- [ ] fcnet: learn routes from traceroute replies
- [ ] cli: rovy reload command
- [x] perf: faked ring buffer queues
- [ ] perf: Buffer pool for fewer allocations
- [ ] perf: Better data structures for sessionmanager, forwarder, routing
- [ ] perf: transmitter object which moves work off the hot path (route lookup, transport lookup, pubkey and session lookup)
- [ ] perf: Lockless goroutine-equivalent for our ringbuf
- [ ] perf: io_uring to avoid syscalls and copying
- [ ] multicodec registrations
- [ ] motd on startup

# Messy internals

- [ ] make multigram varints exactly 4 bytes long, padding + length restriction
- [x] netip.Addr and rovy.Multiaddr everywhere
- [x] remove multigram table negotiation, it doesn't make sense much sense at the moment
- [ ] make multigram varints exactly 4 bytes long, padding + length restriction
- [ ] unify lowerpacket and upperpacket in one packet type, which tracks state (lower, upper, tpt) and gives access to respective payload slices
- [ ] node: payload overflow checks
- [ ] node: enforce max route length of 14 bytes
- [ ] constants for sizes and offsets
- [ ] fix endianness once and for all, do what wireguard and cjdns do

---

# Random Notes

Steps to take for higher throughput:
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
- NRTM v4 Discussion https://github.com/mxsasha/nrtmv4/blob/main/nrtmv4.md
- IRR Explorer https://github.com/dashcare/irrexplorer

RPKI / ROA / RTA:
- https://rpki.readthedocs.io/en/latest/
- RTA Resource Tagged Attestation https://blog.apnic.net/2020/11/20/moving-rpki-beyond-routing-security/
- Discussing the Future of RPKI https://blog.apnic.net/2021/01/29/discussing-the-future-of-rpki/
- ROA syntax and signing https://www.arin.net/resources/manage/rpki/roa_request/
- Routinator has RTA support https://github.com/NLnetLabs/routinator/blob/main/doc/manual/source/advanced-features.rst
- Krill has RTA support https://github.com/NLnetLabs/krill
- AS Path Prepending https://catalog.caida.org/details/paper/2020_aspath_prepending/

IPv6:
- Unique Local IPv6 Unicast Addresses https://www.rfc-editor.org/rfc/rfc4193.html

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
- Weron https://news.ycombinator.com/item?id=31297917
