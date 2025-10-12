package utils

import (
	"context"
	"fmt"
	"net"

	"fernandoglatz/openai-compatible-proxy/internal/core/common/utils/log"

	"github.com/sabhiram/go-wol/wol"
)

// WakeOnLAN sends a magic packet to wake up a host with the given MAC address
func WakeOnLAN(ctx context.Context, macAddress string, broadcastAddr string) error {
	log.Info(ctx).Msg(fmt.Sprintf("Sending WOL magic packet to MAC: %s", macAddress))

	// Parse and validate the MAC address
	mac, err := net.ParseMAC(macAddress)
	if err != nil {
		log.Error(ctx).Msg(fmt.Sprintf("Invalid MAC address: %v", err))
		return fmt.Errorf("invalid MAC address: %w", err)
	}

	// If no broadcast address is specified, use the default local broadcast
	if broadcastAddr == "" {
		broadcastAddr = "255.255.255.255:9"
	}

	// Create the magic packet
	mp, err := wol.New(mac.String())
	if err != nil {
		log.Error(ctx).Msg(fmt.Sprintf("Failed to create magic packet: %v", err))
		return fmt.Errorf("failed to create magic packet: %w", err)
	}

	// Marshal the magic packet to bytes
	packet, err := mp.Marshal()
	if err != nil {
		log.Error(ctx).Msg(fmt.Sprintf("Failed to marshal magic packet: %v", err))
		return fmt.Errorf("failed to marshal magic packet: %w", err)
	}

	// Create UDP connection for broadcasting
	udpAddr, err := net.ResolveUDPAddr("udp", broadcastAddr)
	if err != nil {
		log.Error(ctx).Msg(fmt.Sprintf("Failed to resolve broadcast address: %v", err))
		return fmt.Errorf("failed to resolve broadcast address: %w", err)
	}

	conn, err := net.DialUDP("udp", nil, udpAddr)
	if err != nil {
		log.Error(ctx).Msg(fmt.Sprintf("Failed to create UDP connection: %v", err))
		return fmt.Errorf("failed to create UDP connection: %w", err)
	}
	defer conn.Close()

	// Send the magic packet
	_, err = conn.Write(packet)
	if err != nil {
		log.Error(ctx).Msg(fmt.Sprintf("Failed to send magic packet: %v", err))
		return fmt.Errorf("failed to send magic packet: %w", err)
	}

	log.Info(ctx).Msg(fmt.Sprintf("WOL magic packet sent successfully to %s", macAddress))
	return nil
}
