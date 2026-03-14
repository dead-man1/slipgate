package dnsrouter

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"log"
	"net"
	"strings"
)

// verifyPrefix is the first DNS label that triggers HMAC verification.
// Designed to look like a CDN cache key lookup.
const verifyPrefix = "_ck"

// verifyRoute holds the pubkey for a domain's HMAC verification.
type verifyRoute struct {
	domainLabels []string // tunnel domain split into lowercase labels
	pubkey       []byte   // server public key used as HMAC key
}

// handleVerify checks if packet is a _ck.* verification query and responds
// with HMAC-SHA256(pubkey, nonce) if so. Returns true if the packet was handled.
func (r *Router) handleVerify(packet []byte, clientAddr *net.UDPAddr) bool {
	if len(packet) < 12 {
		return false
	}
	// Must be a query (QR=0)
	if packet[2]&0x80 != 0 {
		return false
	}
	// QDCOUNT must be 1
	if binary.BigEndian.Uint16(packet[4:6]) != 1 {
		return false
	}

	// Parse the question name into labels
	offset := 12
	var labels []string
	for offset < len(packet) {
		length := int(packet[offset])
		if length == 0 {
			offset++
			break
		}
		if length >= 0xC0 {
			return false // pointer in query — unexpected
		}
		offset++
		if offset+length > len(packet) {
			return false
		}
		labels = append(labels, strings.ToLower(string(packet[offset:offset+length])))
		offset += length
	}

	// Need QTYPE + QCLASS after name
	if offset+4 > len(packet) {
		return false
	}
	qtype := binary.BigEndian.Uint16(packet[offset : offset+2])
	if qtype != 16 { // must be TXT
		return false
	}
	qEnd := offset + 4

	// First label must be "_ck"
	if len(labels) < 3 || labels[0] != verifyPrefix {
		return false
	}

	// Find matching verify route by domain suffix
	vr := r.findVerifyRoute(labels)
	if vr == nil {
		return false
	}

	// Extract nonce hex (labels between "_ck" and domain)
	dl := len(vr.domainLabels)
	off := len(labels) - dl
	nonceHex := strings.Join(labels[1:off], "")
	nonceBytes, err := hex.DecodeString(nonceHex)
	if err != nil || len(nonceBytes) == 0 {
		return false
	}

	// Compute HMAC-SHA256(pubkey, nonce)
	mac := hmac.New(sha256.New, vr.pubkey)
	mac.Write(nonceBytes)
	sig := mac.Sum(nil)
	sigHex := hex.EncodeToString(sig)

	// Format as CDN cache validation token
	txt := "v=ck1; h=" + sigHex

	// Build and send TXT response
	resp := buildTXTResponse(packet, qEnd, txt)
	if _, err := r.conn.WriteToUDP(resp, clientAddr); err != nil {
		log.Printf("verify: write: %v", err)
	}
	return true
}

// findVerifyRoute finds a verify route matching the domain suffix of the labels.
func (r *Router) findVerifyRoute(labels []string) *verifyRoute {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for i := range r.verifyRoutes {
		vr := &r.verifyRoutes[i]
		dl := len(vr.domainLabels)
		if len(labels) < 2+dl {
			continue
		}
		off := len(labels) - dl
		match := true
		for j, want := range vr.domainLabels {
			if labels[off+j] != want {
				match = false
				break
			}
		}
		if match {
			return vr
		}
	}
	return nil
}

// buildTXTResponse constructs a minimal DNS TXT response.
func buildTXTResponse(query []byte, qEnd int, txt string) []byte {
	var resp []byte

	// Header
	resp = append(resp, query[0], query[1])              // Transaction ID
	resp = append(resp, 0x84|(query[2]&0x01), 0x00)      // QR=1, AA=1, RD=copy
	resp = append(resp, 0x00, 0x01)                      // QDCOUNT = 1
	resp = append(resp, 0x00, 0x01)                      // ANCOUNT = 1
	resp = append(resp, 0x00, 0x00)                      // NSCOUNT = 0
	resp = append(resp, 0x00, 0x00)                      // ARCOUNT = 0

	// Question section (copy from query)
	resp = append(resp, query[12:qEnd]...)

	// Answer: name pointer + TXT RR
	resp = append(resp, 0xC0, 0x0C)             // name pointer to offset 12
	resp = append(resp, 0x00, 0x10)             // TYPE = TXT
	resp = append(resp, 0x00, 0x01)             // CLASS = IN
	resp = append(resp, 0x00, 0x01, 0x51, 0x80) // TTL = 86400 (24h, typical CDN)

	// RDATA: one character-string
	txtBytes := []byte(txt)
	rdlen := 1 + len(txtBytes)
	resp = append(resp, byte(rdlen>>8), byte(rdlen)) // RDLENGTH
	resp = append(resp, byte(len(txtBytes)))          // string length
	resp = append(resp, txtBytes...)                  // string data

	return resp
}
