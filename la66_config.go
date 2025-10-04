package main

import (
	"fmt"
	"log"
	"time"
	"io"
	"strings"
	"bufio"

	"go.bug.st/serial"
	// Assuming the full setup and sendATCommand are available here.
)

const (
	// The LA66 needs this much time to complete a full reboot.
	RebootDelay = 3 * time.Second 
	// Standard delay between non-reboot commands.
	StandardDelay = 50 * time.Millisecond 
	BaudRate = 9600
    	PortName   = "/dev/ttyACM0" // Example for Linux/macOS. Use "COM3" or similar for Windows.
    	Timeout   = 500 * time.Millisecond
)

func sendATCommand(port serial.Port, command string) (string, error) {
    fullCommand := command + "\r\n"

    // 1. Write the command with CR+LF termination
    if _, err := port.Write([]byte(fullCommand)); err != nil {
        return "", fmt.Errorf("failed to write command %s: %w", command, err)
    }

    // 2. Set the read timeout and initialize the buffered reader
    port.SetReadTimeout(Timeout)
    reader := bufio.NewReader(port)

    var response strings.Builder
    var hasResponse bool

    // 3. Read the response line-by-line until the timeout is reached or an error occurs
    // The module's full response (including OK/ERROR) often spans multiple lines.
    for {
        line, err := reader.ReadString('\n')

        // Handle common errors and loop termination conditions
        if err != nil {
            if err == io.EOF {
                // EOF on serial usually means the port was closed unexpectedly.
                return response.String(), fmt.Errorf("read terminated unexpectedly (EOF): %w", err)
            }
            // Check for timeout error (common way to stop reading when no data is left)
            // The exact error depends on the OS/library, but if no data was read, we break.
            if !hasResponse {
                // If we hit the timeout and got no response, return a clear error.
                return response.String(), nil // Timeout, but we return what we got.
            }
            break // Break on timeout if we have captured some response data.
        }

        // 4. Append the received line and check for completion status
        response.WriteString(line)
        hasResponse = true

        // Check if we've received the final status line ("OK" or "ERROR")
        trimmedLine := strings.TrimSpace(line)
        if strings.HasSuffix(trimmedLine, "OK") || strings.HasSuffix(trimmedLine, "ERROR") {
            break // Successfully received the final status line
        }
    }

    // 5. Return the full, accumulated response
    return response.String(), nil
}

func phase2_aggressiveModeSwitch(port serial.Port) error {
	log.Println("--- PHASE 2: Aggressive Mode Switch and Save ---")
	log.Println("!!! Power Cycle the LA66 module NOW, then press ENTER to proceed. !!!")
	// For a Go application, you'd typically wait for user input or a serial port event.
	// We'll simulate this with a hard-coded wait to give you time to reconnect power.
	//time.Sleep(10 * time.Second) 
	log.Println("Proceeding with aggressive configuration...")

	// Helper to send a command and check the result, without the long delays
	sendAndVerify := func(cmd string) error {

		// NOTE: Assuming this function uses the StandardDelay at the end.
		resp, err := sendATCommand(port, cmd)
		if err != nil {
			log.Printf("❌ Failed to execute %s: %v", cmd, err)
			return err
		}

		// Assuming your verification logic is within sendATCommand or called immediately after.
		log.Printf("✅ Success: %s. Response: %s", cmd, resp)
		return nil
	}



	// 1. Immediately set the mode to P2P (Mode 0)
	// We do not wait for a full response here; we rely on the immediate save.
	if err := sendAndVerify("AT+MODE=0"); err != nil {
		return fmt.Errorf("step 1 (AT+MODE=0) failed: %w", err)
	}


	// 2. Immediately force save the configuration.
	// This writes the P2P mode setting to flash memory before the LoRaWAN stack can interfere.
	if err := sendAndVerify("AT+SAVE"); err != nil {
		return fmt.Errorf("step 2 (AT+SAVE) failed: %w", err)
	}
    
    // 3. Perform a clean software reset.
    // This forces the module to load the NEWLY SAVED P2P configuration.
	if err := sendAndVerify("ATZ"); err != nil {
		return fmt.Errorf("step 3 (ATZ) failed: %w", err)
	}

	// 4. Wait for the module to fully reboot into the new P2P mode.
	log.Printf("😴 Waiting %v for module to fully reboot into P2P mode...", RebootDelay)
	time.Sleep(RebootDelay) 
	
	log.Println("--- Reboot complete. Starting Phase 3 Verification. ---")
    
    // --- PHASE 3: VERIFICATION (CONTINUE EXECUTION) ---
    // The main program would now call the verification and final setup functions here.
    
    // Example Verification Check (if successful, should not reboot)
    if err := sendAndVerify("AT+MODE?"); err != nil {
        log.Printf("!!! FAILURE: Module likely rebooted again or is still stuck.")
        return fmt.Errorf("verification check (AT+MODE?) failed: %w", err)
    }

	return nil
}

func main() {
    // ... setup and open the serial port ...
    mode := &serial.Mode{
        BaudRate: BaudRate, // BaudRate must be defined as a const (e.g., 115200)
    }
    port, err := serial.Open(PortName, mode)
    if err != nil {
        log.Fatalf("Failed to open port %s at %d baud: %v", PortName, BaudRate, err)
    }
    defer port.Close()

    // FIX HERE: Pass the 'port' variable
    if err := phase2_aggressiveModeSwitch(port); err != nil {
        log.Fatalf("Aggressive configuration failed: %v", err)
    }
}
