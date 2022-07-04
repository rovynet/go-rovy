package main

import (
	"context"
	"errors"
	"log"
	"net"
	"net/netip"
	"time"
)

func main() {
	ctx, _ := context.WithTimeout(context.Background(), 3*time.Second)

	addr1 := netip.MustParseAddrPort("[ff02::1%wlp3s0]:12345")
	addr2 := addr1 // netip.MustParseAddrPort("[::]:12345")

	conn, err := net.ListenUDP("udp6", net.UDPAddrFromAddrPort(addr2))
	if err != nil {
		log.Fatalf("ListenUDP: %s", err)
	}

	ticker := time.NewTicker(1 * time.Second)

	recv := make(chan []byte, 16)

	go func() {
		for {
			pkt := make([]byte, 16)
			n, raddr, err := conn.ReadFromUDPAddrPort(pkt)
			if errors.Is(err, net.ErrClosed) {
				close(recv)
				return
			}
			if err != nil {
				log.Printf("receive: %s", err)
				continue
			}
			log.Printf("received: %#v from %s", pkt[:n], raddr)
			if raddr.Addr().IsLinkLocalUnicast() {
				recv <- pkt[:n]
			}
		}
	}()

loop:
	for {
		select {
		case <-ctx.Done():
			log.Printf("closing...")
			conn.Close()
			ticker.Stop()
			break loop

		case <-ticker.C:
			log.Printf("announcing...")
			pkt := []byte{0x1, 0x2, 0x3}
			_, err := conn.WriteToUDPAddrPort(pkt, addr1)
			if err != nil {
				log.Printf("announce: %s", err)
			}

		case pkt := <-recv:
			log.Printf("received: %#v", pkt)
		}
	}
}
