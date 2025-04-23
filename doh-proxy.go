package main

import (
	"encoding/base64" // for encoding DNS queries to base64
	"fmt"             // for formatting strings
	"io"              // for reading response bodies
	"log"             // for logging errors and info
	"net"             // for handling UDP connections
	"net/http"        // for sending DNS queries over HTTPS (DoH)
)

const (
	listenAddr  = ":53530"                               // the local UDP port this proxy will listen on
	dohEndpoint = "https://cloudflare-dns.com/dns-query" // the DoH endpoint to use (Cloudflare)
)

func main() {
	// Resolve the UDP address to listen on
	udpAddr, err := net.ResolveUDPAddr("udp", listenAddr)
	if err != nil {
		log.Fatalf("Failed to resolve UDP address: %v", err)
	}

	// Start listening for UDP packets on the specified address
	conn, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		log.Fatalf("Failed to listen on UDP: %v", err)
	}
	defer conn.Close() // Ensure the connection is closed when program ends

	log.Printf("Listening on %s for DNS queries...\n", listenAddr)

	// Allocate a buffer to read incoming DNS queries
	buf := make([]byte, 512) // 512 bytes is the max size for most DNS packets

	for {
		// Wait to receive a DNS query from a client
		n, addr, err := conn.ReadFromUDP(buf)
		if err != nil {
			log.Printf("Error reading from UDP: %v", err)
			continue
		}

		// Handle the DNS query in a separate goroutine for concurrency
		go func(query []byte, clientAddr *net.UDPAddr) {
			// Send the query to the DoH server and get the response
			resp, err := sendToDoH(query[:n])
			if err != nil {
				log.Println("DoH error:", err)
				return
			}

			// Send the DoH response back to the original client
			_, _ = conn.WriteToUDP(resp, clientAddr)
		}(append([]byte(nil), buf[:n]...), addr) // copy the buffer for the goroutine
	}
}

// sendToDoH sends a raw DNS query over HTTPS using the DoH protocol
func sendToDoH(dnsQuery []byte) ([]byte, error) {
	// Encode the DNS query using base64 (without padding) for GET request
	encoded := base64.RawURLEncoding.EncodeToString(dnsQuery)

	// Construct the full DoH URL with the encoded query
	url := fmt.Sprintf("%s?dns=%s", dohEndpoint, encoded)

	// Create a new HTTP GET request
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	// Set the Accept header to indicate we expect a DNS binary message in response
	req.Header.Set("Accept", "application/dns-message")

	// Send the HTTP request using the default HTTP client
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close() // Close the response body when done

	// Read the full DNS response from the HTTP response body
	return io.ReadAll(resp.Body)
}
