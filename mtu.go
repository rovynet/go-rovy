package rovy

const (
	UpperMTU = 1500 - 48 - 20 - 16 - 20 - 16 - 16 // 1364

	// ethernet + udp6
	TptMTU = 1500 - 48
)
