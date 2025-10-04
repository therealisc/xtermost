package main

import (
	"encoding/hex"
	//"fmt"
	"log"
	"net"
	//"os"
)

// Configuration Constants
const (
	// Replace this with your actual 8-byte Gateway EUI (16 hex characters)
	// Example:     "B94F8F10A7D164C2"
	GatewayEUIStr = "B94F8F10A7D164C2"
	
	// LoRaWAN Network Server (LNS) address and port
	LNSAddress = "xtermost.eu2.cloud.thethings.industries:1700"
)

// getGatewayPacket takes a raw LoRa packet payload and prepends the 8-byte EUI
// and a 4-byte header for the Semtech UDP protocol.
func getGatewayPacket(rawPayload []byte, eui []byte) []byte {
	// The Semtech UDP Packet Forwarder protocol defines the format as:
	// 1. Protocol Version (1 byte, usually 0x02)
	// 2. Random Token (2 bytes)
	// 3. Packet Type (1 byte, e.g., 0x00 for PUSH_DATA)
	// 4. Gateway EUI (8 bytes)
	// 5. JSON Payload (N bytes)
    
    // NOTE: This is a highly simplified header, actual implementations use a random token.
    // For simplicity, we use a static header (Version: 0x02, Token: 0x0000, Type: 0x00 PUSH_DATA)
	
	// Header components: Version (1) + Token (2) + Type (1)
	header := []byte{0x02, 0x00, 0x00, 0x00} 
	
	// Combined packet: Header (4) + EUI (8) + Raw Payload (N)
	packet := make([]byte, 0, len(header) + len(eui) + len(rawPayload))
	packet = append(packet, header...)
	packet = append(packet, eui...)
	packet = append(packet, rawPayload...)

	return packet
}

func main() {
	// 1. Validate and Decode the Gateway EUI
	eui, err := hex.DecodeString(GatewayEUIStr)
	if err != nil || len(eui) != 8 {
		log.Fatalf("Invalid Gateway EUI: %s. Must be exactly 16 hex characters.", GatewayEUIStr)
	}
	
	log.Printf("Starting LoRaWAN bridge with EUI: %s", GatewayEUIStr)
	
	// 2. Set up UDP Connection to the Network Server
	serverAddr, err := net.ResolveUDPAddr("udp", LNSAddress)
	if err != nil {
		log.Fatalf("Failed to resolve LNS address: %v", err)
	}
	
	conn, err := net.DialUDP("udp", nil, serverAddr)
	if err != nil {
		log.Fatalf("Failed to dial UDP server: %v", err)
	}
	defer conn.Close()
	
	// --- Replace this section with your actual serial reading logic ---
	
	// MOCK DATA: Simulate reading a raw JSON LoRaWAN packet from the serial port.
	// In a real application, you would read this from the 'go.bug.st/serial' port.
	rawSerialData := []byte(`{"rxpk":[{"tmst":123456789,"chan":0,"rfch":0,"freq":868.100000,"stat":1,"modu":"LORA","datr":"SF7BW125","codr":"4/5","lsnr":9.8,"rssi":-46,"size":23,"data":"QKjAIAEAAAAAAgAAaXG8A/Q="}]}`)
	
	// -----------------------------------------------------------------

	// 3. Prepend EUI and Header
	packetToSend := getGatewayPacket(rawSerialData, eui)

	// 4. Send the packet over UDP
	n, err := conn.Write(packetToSend)
	if err != nil {
		log.Fatalf("Failed to send UDP packet: %v", err)
	}
	
	log.Printf("Successfully sent %d bytes to %s", n, LNSAddress)
	// The LNS will now recognize the packet came from the GatewayEUIStr.
}
