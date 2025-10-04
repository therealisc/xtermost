// LA66 Raw Sender (Go XModem Flasher)
// This script implements the necessary XModem protocol logic internally
// to stream the firmware file directly to the LA66 via the Arduino passthrough sketch.

package main

import (
	"fmt"
	"io"
	"os"
	"time"

	"go.bug.st/serial"
)

// --- XModem Protocol Constants ---
const (
	SOH       byte = 0x01 // Start Of Header (128-byte block)
	EOT       byte = 0x04 // End Of Transmission
	ACK       byte = 0x06 // Acknowledge
	NAK       byte = 0x15 // Negative Acknowledge (Checksum request/Error)
	CAN       byte = 0x18 // Cancel
	CRC_START byte = 0x43 // 'C' character, used to initiate XModem-CRC
	DATA_SIZE = 128      // XModem block size
	MAX_RETRIES = 10     // Max retries for block send
)

const (
	SERIAL_PORT  = "/dev/ttyACM0"
	// CRITICAL FIX: The handshake worked at 9600. The higher 115200 failed to transmit
	// data reliably. Reverting to 9600 to stabilize data integrity while retaining CRC.
	BAUD_RATE    = 9600
	FIRMWARE_FILE = "/home/therealisc/Downloads/LA66_P2P_v1.2.4_application.bin"
	HANDSHAKE_TIMEOUT = 10 * time.Second
)

// calculateChecksum computes the 8-bit checksum (simple sum) for the data block.
// We are aggressively switching to Checksum mode because all CRC variants failed,
// suggesting the bootloader may only use the basic 8-bit checksum validation, despite
// requesting XModem-CRC with the 'C' handshake character.
func calculateChecksum(data []byte) byte {
	var checksum byte = 0
	for _, b := range data {
		checksum += b
	}
	return checksum
}

// readWithTimeout wraps a port read operation with a timeout using a channel and goroutine.
// Returns the number of bytes read, the buffer, and an error (which will be io.EOF on timeout).
func readWithTimeout(port serial.Port, buf []byte, timeout time.Duration) (int, []byte, error) {
	result := make(chan struct {
		n   int
		err error
	})

	go func() {
		n, err := port.Read(buf)
		result <- struct {
			n   int
			err error
		}{n, err}
	}()

	select {
	case res := <-result:
		return res.n, buf, res.err
	case <-time.After(timeout):
		// Note: The goroutine is still running and will eventually finish its blocking read.
		// We return io.EOF (End of File) to signify a timeout, which is handled as a retry.
		return 0, nil, io.EOF
	}
}

// flushSerialBuffer attempts to read and discard all data currently in the serial buffer.
func flushSerialBuffer(port serial.Port) {
    // Read 100 bytes or until timeout
    buffer := make([]byte, 100)
    for {
        n, _, err := readWithTimeout(port, buffer, 100*time.Millisecond)
        if err != nil || n == 0 {
            break // Break on error (timeout) or if no bytes were read
        }
    }
}

// xmodemSend performs the XModem-Checksum transfer, using 'C' handshake.
func xmodemSend(port serial.Port, file *os.File) (int64, error) {
	fmt.Println("Attempting XModem-CRC handshake (sending 'C')...")

	// --- Handshake ---
	// The sender must repeatedly send 'C' until the receiver is ready (sends NAK or C back).
	startTime := time.Now()
	handshakeDone := false
	for time.Since(startTime) < HANDSHAKE_TIMEOUT {
		// FIX: Send CRC_START to request XModem-CRC protocol
		port.Write([]byte{CRC_START, CRC_START, CRC_START, CRC_START})

		// Wait for response using the new timeout wrapper (1.5s for handshake)
		response := make([]byte, 1)
		_, response, err := readWithTimeout(port, response, 1500 * time.Millisecond)

		if err == nil && len(response) > 0 {
			// Accept NAK or CRC_START to proceed
			if response[0] == CRC_START || response[0] == NAK {
				fmt.Printf("Handshake successful! Receiver ready. Response: 0x%X\n", response[0])
				handshakeDone = true
				break
			}
		}
		if time.Since(startTime) >= HANDSHAKE_TIMEOUT {
			return 0, fmt.Errorf("handshake timeout: did not receive 'C' or NAK from bootloader")
		}
	}

	if !handshakeDone {
		return 0, fmt.Errorf("handshake failed before data transfer attempt")
	}

	// --- Data Transfer ---
	totalBytesSent := int64(0)
	blockNum := byte(1)
	dataBuffer := make([]byte, DATA_SIZE)

	for {
		// Read a block of data from the file
		n, err := io.ReadFull(file, dataBuffer)
		if err == io.EOF {
			// End of file, no data left. Break to send EOT.
			break
		}
		if err != nil && err != io.ErrUnexpectedEOF {
			return totalBytesSent, fmt.Errorf("file read error: %v", err)
		}

		// Pad the last block if necessary
		if n < DATA_SIZE {
			for i := n; i < DATA_SIZE; i++ {
				dataBuffer[i] = 0x1A // CP/M EOF marker or 0x00 padding
			}
		}

		// Send Block Loop
		for retry := 0; retry < MAX_RETRIES; retry++ {

			// 1. Build the Packet
			// XModem Checksum uses 1 Checksum byte
			packet := make([]byte, 0, DATA_SIZE + 4)

			// Header: SOH + Block Num + 1's Complement
			packet = append(packet, SOH, blockNum, ^blockNum)

			// Data
			packet = append(packet, dataBuffer...)

			// Checksum (1 byte only)
			checksum := calculateChecksum(dataBuffer)
			packet = append(packet, checksum)

			// 2. Send the Packet
			port.Write(packet)

			// 3. Wait for ACK using 5 second timeout (Increased from 3s)
			response := make([]byte, 1)
			_, response, err = readWithTimeout(port, response, 5 * time.Second)

			if err == nil && len(response) > 0 {
				switch response[0] {
				case ACK:
					// Success! Move to next block.
					totalBytesSent += int64(n)
					blockNum++
					fmt.Printf("Block %d sent, total bytes: %d\r", blockNum-1, totalBytesSent)
					goto NextBlock
				case NAK:
					// Receiver error, retry block
					fmt.Printf("NAK received for block %d, retrying...\n", blockNum)
					// Flush buffer before retry to discard junk bytes
					flushSerialBuffer(port)
					continue
				case CAN:
					// Transfer cancelled by receiver
					return totalBytesSent, fmt.Errorf("transfer cancelled by receiver")
				default:
					// Unexpected response, retry block
					fmt.Printf("Unexpected response (0x%X) for block %d, retrying...\n", response[0], blockNum)
					// Flush buffer before retry to discard junk bytes
					flushSerialBuffer(port)
					continue
				}
			} else {
				// Timeout or Read error, retry block
				fmt.Printf("Timeout waiting for ACK for block %d, retrying...\n", blockNum)
				continue
			}
		}

		// If MAX_RETRIES reached
		return totalBytesSent, fmt.Errorf("transfer failed: max retries reached for block %d", blockNum)

	NextBlock:
	}

	// --- End of Transfer ---
	// Send EOT until ACK
	response := make([]byte, 1)
	for retry := 0; retry < 10; retry++ {
		port.Write([]byte{EOT})
		// Increased timeout for EOT ACK as well
		_, response, err := readWithTimeout(port, response, 5 * time.Second)
		if err == nil && len(response) > 0 && response[0] == ACK {
			return totalBytesSent, nil // Final success
		}
		time.Sleep(500 * time.Millisecond)
	}

	return totalBytesSent, fmt.Errorf("transfer successful but failed to receive final ACK for EOT")
}

func main() {
	fmt.Println("--- LA66 XModem Flasher (Go Sender) ---")
	fmt.Printf("1. Opening serial port %s at %d baud...\n", SERIAL_PORT, BAUD_RATE)

	// Configure and open the serial port
	mode := &serial.Mode{
		BaudRate: BAUD_RATE,
		Parity:   serial.NoParity,
		DataBits: 8,
		StopBits: serial.OneStopBit,
	}

	// serial.Open returns a serial.Port
	port, err := serial.Open(SERIAL_PORT, mode)
	if err != nil {
		fmt.Printf("❌ Failed to open serial port: %v. Check permissions (sudo) and device name.\n", err)
		return
	}
	defer port.Close()

	// Wait briefly for the Arduino to stabilize after opening the port
	time.Sleep(2 * time.Second)

    // Flush any leftover junk from the application firmware
    flushSerialBuffer(port)

	fmt.Printf("2. Loading firmware file: %s\n", FIRMWARE_FILE)
	file, err := os.Open(FIRMWARE_FILE)
	if err != nil {
		fmt.Printf("❌ Failed to open firmware file: %v\n", err)
		return
	}
	defer file.Close()

	fmt.Println("\n=========================================================")
	fmt.Println("*** ACTION REQUIRED: PRESS LA66 RESET BUTTON NOW ***")
	fmt.Println("=========================================================\n")

	// Start the transfer using the custom implementation
	n, err := xmodemSend(port, file)

	// Check the results
	if err != nil {
		fmt.Printf("\n❌ XModem Transfer FAILED after sending %d bytes: %v\n", n, err)
		fmt.Println("Troubleshooting: This is likely a timing error (press RESET sooner) or a baud rate mismatch.")
// Line 204 is the closing brace for the 'if err != nil' block above.
		return
	}

	fmt.Printf("\n✅ XModem Transfer SUCCESSFUL! Sent %d bytes.\n", n)
	fmt.Println("LA66 should now reboot with the new firmware.")
}
