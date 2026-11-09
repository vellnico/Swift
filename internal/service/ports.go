package service

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"net"
	"strconv"
	"strings"

	"github.com/vellnico/swift/internal/store"
	"github.com/vellnico/swift/internal/xray"
)

// PortChecker reports whether the OS lets us bind a port; mockable in tests.
type PortChecker interface {
	Free(network string, port int) bool
}

// OSPorts binds the port briefly. Only "address in use" counts as busy:
// a permission error (port < 1024 without root) says nothing about conflicts.
type OSPorts struct{}

func (OSPorts) Free(network string, port int) bool {
	addr := ":" + strconv.Itoa(port)
	var err error
	if network == "udp" {
		var pc net.PacketConn
		pc, err = net.ListenPacket("udp", addr)
		if err == nil {
			pc.Close()
		}
	} else {
		var ln net.Listener
		ln, err = net.Listen("tcp", addr)
		if err == nil {
			ln.Close()
		}
	}
	return err == nil || !isAddrInUse(err)
}

func isAddrInUse(err error) bool {
	s := strings.ToLower(err.Error())
	return strings.Contains(s, "address already in use") || strings.Contains(s, "only one usage of each socket address")
}

// ErrPortConflict names what already uses a port.
type ErrPortConflict struct {
	Port    int
	Network string
	By      string // profile name, "panel", "subscriptions" or "system"
}

func (e *ErrPortConflict) Error() string {
	return fmt.Sprintf("%s port %d is used by %s", e.Network, e.Port, e.By)
}

var RecommendedPorts = []int{443, 8443, 2053}

// Ports resolves port usage across profiles, the panel and the subscription server.
type Ports struct {
	Store   *store.Store
	Checker PortChecker
	// Rand returns a random int in [0, n); replaced in tests.
	Rand func(n int) int
}

func (p *Ports) rand(n int) int {
	if p.Rand != nil {
		return p.Rand(n)
	}
	v, err := rand.Int(rand.Reader, big.NewInt(int64(n)))
	if err != nil {
		panic(err)
	}
	return int(v.Int64())
}

// Check returns an *ErrPortConflict if port on network is taken, ignoring profile exceptID.
func (p *Ports) Check(ctx context.Context, network string, port int, exceptID int64) error {
	if port < 1 || port > 65535 {
		return fmt.Errorf("invalid port %d", port)
	}
	profiles, err := p.Store.Profiles(ctx)
	if err != nil {
		return err
	}
	for _, pr := range profiles {
		if pr.ID == exceptID {
			continue
		}
		if pr.Port == port && pr.Kind.Network() == network {
			return &ErrPortConflict{Port: port, Network: network, By: pr.Name}
		}
	}
	if network == "tcp" {
		for _, key := range []string{"panel_port", "sub_port"} {
			if v, err := p.Store.Setting(ctx, key); err == nil && v == strconv.Itoa(port) {
				by := "panel"
				if key == "sub_port" {
					by = "subscriptions"
				}
				return &ErrPortConflict{Port: port, Network: network, By: by}
			}
		}
	}
	// a port already used by one of our own enabled inbounds is busy in the OS too
	ownedByUs := false
	for _, pr := range profiles {
		if pr.ID == exceptID && pr.Port == port && pr.Kind.Network() == network {
			ownedByUs = true
		}
	}
	if !ownedByUs && p.Checker != nil && !p.Checker.Free(network, port) {
		return &ErrPortConflict{Port: port, Network: network, By: "system"}
	}
	return nil
}

// Pick chooses a free port for a new profile of kind k.
func (p *Ports) Pick(ctx context.Context, k xray.Kind) (int, error) {
	network := k.Network()
	order := []int{xray.DefaultPort(k)}
	for _, r := range RecommendedPorts {
		if r != order[0] {
			order = append(order, r)
		}
	}
	for _, port := range order {
		err := p.Check(ctx, network, port, 0)
		if err == nil {
			return port, nil
		}
		var c *ErrPortConflict
		if !errors.As(err, &c) {
			return 0, err
		}
	}
	for i := 0; i < 200; i++ {
		port := 20000 + p.rand(40001)
		if err := p.Check(ctx, network, port, 0); err == nil {
			return port, nil
		}
	}
	return 0, errors.New("no free port found")
}
