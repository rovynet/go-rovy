package rovy

const (
	PreliminaryMTU = 1500 - 48 - 36 // UDPv6 is 40+8 bytes, Rovy direct is 20+16 bytes
)
