
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

Right now (mid 2022) Rovy is an early work-in-progress with some foundational parts in place and working, while many of the more interesting parts are still missing.

## Usage

```sh
> go install go.rovy.net/cmd/rovy@latest
> rovy start -K @
00:45:36 start.go:132: starting with ephemeral private key
00:45:36 start.go:94: we are /rovy/bafzqaidqzzy5ykgv6hovz6u6lbpmzqzddcmwegzgu3pmc7tlrjff2m4age
00:45:36 start.go:106: api socket ready at http:/home/user/.rovy/api.sock
00:45:36 start.go:199: started fcnet endpoint fc78:4ece:63c9:903c:5a54:cb0c:fda3:39cf using NetworkManager
...
> rovy info
PeerID: bafzqaidqzzy5ykgv6hovz6u6lbpmzqzddcmwegzgu3pmc7tlrjff2m4age

> ping fc00::1
> ip addr show rovy0
> nmcli
> resolvectl
```

To test IPv6 networking over Rovy's TUN interface, run traceroute against the IPv6 address of `nodeD` from the following command's output:
```
> go run ./examples/fcnet
...
[nodeD] 02:28:45 main.go:32: /rovy/bafzqaih2xv4tvuihz3vfwpxqr73qnfdtvggze6z53pzfbtywdoucznzwbm
[nodeD] 02:28:45 main.go:32: /ip6/fc75:d625:ca71:1e82:7636:37ea:3e8a:aa63
...
> curl http://bafzqaih2xv4tvuihz3vfwpxqr73qnfdtvggze6z53pzfbtywdoucznzwbm.rovy
Hello world!
> mtr bafzqaih2xv4tvuihz3vfwpxqr73qnfdtvggze6z53pzfbtywdoucznzwbm.rovy
```
