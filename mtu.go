package rovy

const (
	PreliminaryMTU = 1500 - 48 - 36 // UDPv6 is 40+8 bytes, Rovy direct is 20+16 bytes
	// TptMTU         = 1500
	// LowerMTU       = TptMTU - 48 - 36 // UDPv6 is 40+8 bytes, Rovy direct is 20+16 bytes
	// UpperMTU       = LowerMTU - 36 - 8
)
