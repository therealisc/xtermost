package main

import (
	"encoding/hex"
	"bufio"
	"fmt"
	"log"
	"net"
	"os"
	"time"

	"go.bug.st/serial"
)

// Configuration constants
const (
	SerialPortName = "/dev/ttyACM0" // Change this to your serial port (e.g., "COM3" on Windows)
	BaudRate       = 9600

	// LoRaWAN Network Server (LNS) address and port
	LNSAddress = "xtermost.eu2.cloud.thethings.industries:1700"
	GatewayEUIStr = "B94F8F10A7D164C2"
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
	// 1. Open Serial Port
	mode := &serial.Mode{
		BaudRate: BaudRate,
	}

	port, err := serial.Open(SerialPortName, mode)
	if err != nil {
		log.Fatalf("Error opening serial port %s: %v", SerialPortName, err)
	}
	defer port.Close()

	// 2. Setup UDP Connection
	udpAddr, err := net.ResolveUDPAddr("udp", LNSAddress)
	if err != nil {
		log.Fatalf("Error resolving UDP address %s: %v", LNSAddress, err)
	}

	conn, err := net.DialUDP("udp", nil, udpAddr)
	if err != nil {
		log.Fatalf("Error connecting to UDP address %s: %v", LNSAddress, err)
	}
	defer conn.Close()

	log.Printf("Serial Port: %s at %d Baud", SerialPortName, BaudRate)
	log.Printf("UDP Target: %s", LNSAddress)


	// 3. Validate and Decode the Gateway EUI
	eui, err := hex.DecodeString(GatewayEUIStr)
	if err != nil || len(eui) != 8 {
		log.Fatalf("Invalid Gateway EUI: %s. Must be exactly 16 hex characters.", GatewayEUIStr)
	}

	log.Printf("Starting LoRaWAN bridge with EUI: %s", GatewayEUIStr)

	// 4. Send the packet over UDP
	// MOCK DATA: Simulate reading a raw JSON LoRaWAN packet from the serial port.
	// In a real application, you would read this from the 'go.bug.st/serial' port.
	rawSerialData := []byte(`{"rxpk":[{"tmst":123456789,"chan":0,"rfch":0,"freq":868.100000,"stat":1,"modu":"LORA","datr":"SF7BW125","codr":"4/5","lsnr":9.8,"rssi":-46,"size":23,"data":"QKjAIAEAAAAAAgAAaXG8A/Q="}]}`)

	packetToSend := getGatewayPacket(rawSerialData, eui)
	n, err := conn.Write(packetToSend)
	if err != nil {
		log.Fatalf("Failed to send UDP packet: %v", err)
	}
	
	log.Printf("Successfully sent %d bytes to %s", n, LNSAddress)

	// 5. Concurrently Read Serial and prepend EUI and Header
	scanner := bufio.NewScanner(port)
	scanner.Split(bufio.ScanLines) // Read line by line

	for scanner.Scan() {
		line := scanner.Text()
		log.Printf("output: %s", line)
		//go sendUDP(conn, line) // Use a goroutine to send UDP packet asynchronously
	}

	if err := scanner.Err(); err != nil {
		log.Fatalf("Error reading from serial port: %v", err)
	}
}

// sendUDP sends the string data as a UDP packet.
func sendUDP(conn *net.UDPConn, data string) {
	// Add a timestamp to the data before sending, just for extra context
	message := fmt.Sprintf("[%s] %s\n", time.Now().Format("15:04:05.000"), data)
	
	n, err := conn.Write([]byte(message))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error sending UDP message: %v\n", err)
		return
	}
	log.Printf("Sent %d bytes: %s", n, message)
}
