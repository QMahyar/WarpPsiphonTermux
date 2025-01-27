package wiresocks

import (
	"context"
	"crypto/rand"
	"fmt"
	"net"
	"net/netip"
	"strings"
	"time"

	"github.com/bepass-org/ipscanner"
	"github.com/bepass-org/wireguard-go/warp"
	"github.com/go-ini/ini"
)

func canConnectIPv6(remoteAddr netip.AddrPort) bool {
	dialer := net.Dialer{
		Timeout: 5 * time.Second,
	}

	conn, err := dialer.Dial("tcp6", remoteAddr.String())
	if err != nil {
		return false
	}
	defer conn.Close()
	return true
}

func RunScan(ctx context.Context, rtt time.Duration) (result []string, err error) {
	cfg, err := ini.Load("./primary/wgcf-profile.ini")
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	// Reading the private key from the 'Interface' section
	privateKey := cfg.Section("Interface").Key("PrivateKey").String()

	// Reading the public key from the 'Peer' section
	publicKey := cfg.Section("Peer").Key("PublicKey").String()

	// TODO: ipscanner doesn't support netip.Prefix yet
	prefixes := warp.WarpPrefixes()
	stringedPrefixes := make([]string, len(prefixes))
	for i, p := range prefixes {
		stringedPrefixes[i] = p.String()
	}

	// new scanner
	scanner := ipscanner.NewScanner(
		ipscanner.WithWarpPing(),
		ipscanner.WithWarpPrivateKey(privateKey),
		ipscanner.WithWarpPeerPublicKey(publicKey),
		ipscanner.WithUseIPv6(canConnectIPv6(netip.MustParseAddrPort("[2001:4860:4860::8888]:80"))),
		ipscanner.WithUseIPv4(true),
		ipscanner.WithMaxDesirableRTT(int(rtt.Milliseconds())),
		ipscanner.WithCidrList(stringedPrefixes),
	)

	scanner.Run()
	timeoutTimer := time.NewTimer(2 * time.Minute)
	defer timeoutTimer.Stop()

	for {
		select {
		case <-ctx.Done():
			// Context is done - canceled externally
			scanner.Stop()
			return nil, fmt.Errorf("user canceled the operation")
		case <-timeoutTimer.C:
			// Handle the internal timeout
			scanner.Stop()
			return nil, fmt.Errorf("scanner maximum time exceeded")
		default:
			ipList := scanner.GetAvailableIPS()
			if len(ipList) > 1 {
				scanner.Stop()
				for i := 0; i < 2; i++ {
					result = append(result, ipToAddress(ipList[i]))
				}
				return result, nil
			}
			time.Sleep(1 * time.Second) // Prevent the loop from spinning too fast
		}
	}
}

func ipToAddress(ip net.IP) string {
	ports := []int{500, 854, 859, 864, 878, 880, 890, 891, 894, 903, 908, 928, 934, 939, 942,
		943, 945, 946, 955, 968, 987, 988, 1002, 1010, 1014, 1018, 1070, 1074, 1180, 1387, 1701,
		1843, 2371, 2408, 2506, 3138, 3476, 3581, 3854, 4177, 4198, 4233, 4500, 5279,
		5956, 7103, 7152, 7156, 7281, 7559, 8319, 8742, 8854, 8886}

	// Pick a random port number
	b := make([]byte, 8)
	n, err := rand.Read(b)
	if n != 8 {
		panic(n)
	} else if err != nil {
		panic(err)
	}
	serverAddress := fmt.Sprintf("%s:%d", ip.String(), ports[int(b[0])%len(ports)])
	if strings.Contains(ip.String(), ":") {
		//ip6
		serverAddress = fmt.Sprintf("[%s]:%d", ip.String(), ports[int(b[0])%len(ports)])
	}
	return serverAddress
}
