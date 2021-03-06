package quic

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"hash"
	"net"
	"sync"
	"time"

	"github.com/stormlin/mitm/quic/internal/protocol"
	"github.com/stormlin/mitm/quic/internal/utils"
	"github.com/stormlin/mitm/quic/internal/wire"
)

type statelessResetErr struct {
	token *[16]byte
}

func (e statelessResetErr) StatelessResetToken() *[16]byte { return e.token }

func (e statelessResetErr) Error() string {
	return fmt.Sprintf("received a stateless reset with token %x", *e.token)
}

// The packetHandlerMap stores packetHandlers, identified by connection ID.
// It is used:
// * by the server to store sessions
// * when multiplexing outgoing connections to store clients
type packetHandlerMap struct {
	mutex sync.RWMutex

	conn      net.PacketConn
	connIDLen int

	handlers    map[string] /* string(ConnectionID)*/ packetHandler
	resetTokens map[[16]byte] /* stateless reset token */ packetHandler
	server      unknownPacketHandler

	listening chan struct{} // is closed when listen returns
	closed    bool

	deleteRetiredSessionsAfter time.Duration

	statelessResetEnabled bool
	statelessResetMutex   sync.Mutex
	statelessResetHasher  hash.Hash

	logger utils.Logger
}

var _ packetHandlerManager = &packetHandlerMap{}

func newPacketHandlerMap(
	conn net.PacketConn,
	connIDLen int,
	statelessResetKey []byte,
	logger utils.Logger,
) packetHandlerManager {
	m := &packetHandlerMap{
		conn:                       conn,
		connIDLen:                  connIDLen,
		listening:                  make(chan struct{}),
		handlers:                   make(map[string]packetHandler),
		resetTokens:                make(map[[16]byte]packetHandler),
		deleteRetiredSessionsAfter: protocol.RetiredConnectionIDDeleteTimeout,
		statelessResetEnabled:      len(statelessResetKey) > 0,
		statelessResetHasher:       hmac.New(sha256.New, statelessResetKey),
		logger:                     logger,
	}
	go m.listen()

	if logger.Debug() {
		go m.logUsage()
	}

	return m
}

func (h *packetHandlerMap) logUsage() {
	ticker := time.NewTicker(2 * time.Second)
	var printedZero bool
	for {
		select {
		case <-h.listening:
			return
		case <-ticker.C:
		}

		h.mutex.Lock()
		numHandlers := len(h.handlers)
		numTokens := len(h.resetTokens)
		h.mutex.Unlock()
		// If the number tracked handlers and tokens is zero, only print it a single time.
		hasZero := numHandlers == 0 && numTokens == 0
		if !hasZero || (hasZero && !printedZero) {
			h.logger.Debugf("Tracking %d connection IDs and %d reset tokens.\n", numHandlers, numTokens)
			printedZero = false
			if hasZero {
				printedZero = true
			}
		}
	}
}

func (h *packetHandlerMap) Add(id protocol.ConnectionID, handler packetHandler) bool /* was added */ {
	sid := string(id)

	h.mutex.Lock()
	defer h.mutex.Unlock()

	if _, ok := h.handlers[sid]; ok {
		return false
	}
	h.handlers[sid] = handler
	return true
}

func (h *packetHandlerMap) Remove(id protocol.ConnectionID) {
	h.mutex.Lock()
	delete(h.handlers, string(id))
	h.mutex.Unlock()
}

func (h *packetHandlerMap) Retire(id protocol.ConnectionID) {
	time.AfterFunc(h.deleteRetiredSessionsAfter, func() {
		h.mutex.Lock()
		delete(h.handlers, string(id))
		h.mutex.Unlock()
	})
}

func (h *packetHandlerMap) ReplaceWithClosed(id protocol.ConnectionID, handler packetHandler) {
	h.mutex.Lock()
	h.handlers[string(id)] = handler
	h.mutex.Unlock()

	time.AfterFunc(h.deleteRetiredSessionsAfter, func() {
		h.mutex.Lock()
		handler.shutdown()
		delete(h.handlers, string(id))
		h.mutex.Unlock()
	})
}

func (h *packetHandlerMap) AddResetToken(token [16]byte, handler packetHandler) {
	h.mutex.Lock()
	h.resetTokens[token] = handler
	h.mutex.Unlock()
}

func (h *packetHandlerMap) RemoveResetToken(token [16]byte) {
	h.mutex.Lock()
	delete(h.resetTokens, token)
	h.mutex.Unlock()
}

func (h *packetHandlerMap) RetireResetToken(token [16]byte) {
	time.AfterFunc(h.deleteRetiredSessionsAfter, func() {
		h.mutex.Lock()
		delete(h.resetTokens, token)
		h.mutex.Unlock()
	})
}

func (h *packetHandlerMap) SetServer(s unknownPacketHandler) {
	h.mutex.Lock()
	h.server = s
	h.mutex.Unlock()
}

func (h *packetHandlerMap) CloseServer() {
	h.mutex.Lock()
	h.server = nil
	var wg sync.WaitGroup
	for _, handler := range h.handlers {
		if handler.getPerspective() == protocol.PerspectiveServer {
			wg.Add(1)
			go func(handler packetHandler) {
				// blocks until the CONNECTION_CLOSE has been sent and the run-loop has stopped
				handler.shutdown()
				wg.Done()
			}(handler)
		}
	}
	h.mutex.Unlock()
	wg.Wait()
}

// Destroy the underlying connection and wait until listen() has returned.
// It does not close active sessions.
func (h *packetHandlerMap) Destroy() error {
	if err := h.conn.Close(); err != nil {
		return err
	}
	<-h.listening // wait until listening returns
	return nil
}

func (h *packetHandlerMap) close(e error) error {
	h.mutex.Lock()
	if h.closed {
		h.mutex.Unlock()
		return nil
	}

	var wg sync.WaitGroup
	for _, handler := range h.handlers {
		wg.Add(1)
		go func(handler packetHandler) {
			handler.destroy(e)
			wg.Done()
		}(handler)
	}

	if h.server != nil {
		h.server.setCloseError(e)
	}
	h.closed = true
	h.mutex.Unlock()
	wg.Wait()
	return getMultiplexer().RemoveConn(h.conn)
}

func (h *packetHandlerMap) listen() {
	defer close(h.listening)
	for {
		buffer := getPacketBuffer()
		data := buffer.Data[:protocol.MaxReceivePacketSize]
		// The packet size should not exceed protocol.MaxReceivePacketSize bytes
		// If it does, we only read a truncated packet, which will then end up undecryptable
		n, addr, err := h.conn.ReadFrom(data)
		if err != nil {
			h.close(err)
			return
		}
		h.handlePacket(addr, buffer, data[:n])
	}
}

func (h *packetHandlerMap) handlePacket(
	addr net.Addr,
	buffer *packetBuffer,
	data []byte,
) {
	connID, err := wire.ParseConnectionID(data, h.connIDLen)
	if err != nil {
		h.logger.Debugf("error parsing connection ID on packet from %s: %s", addr, err)
		return
	}
	rcvTime := time.Now()

	h.mutex.RLock()
	defer h.mutex.RUnlock()

	if isStatelessReset := h.maybeHandleStatelessReset(data); isStatelessReset {
		return
	}

	handler, handlerFound := h.handlers[string(connID)]

	p := &receivedPacket{
		remoteAddr: addr,
		rcvTime:    rcvTime,
		buffer:     buffer,
		data:       data,
	}
	if handlerFound { // existing session
		handler.handlePacket(p)
		return
	}
	if data[0]&0x80 == 0 {
		go h.maybeSendStatelessReset(p, connID)
		return
	}
	if h.server == nil { // no server set
		h.logger.Debugf("received a packet with an unexpected connection ID %s", connID)
		return
	}
	h.server.handlePacket(p)
}

func (h *packetHandlerMap) maybeHandleStatelessReset(data []byte) bool {
	// stateless resets are always short header packets
	if data[0]&0x80 != 0 {
		return false
	}
	if len(data) < 17 /* type byte + 16 bytes for the reset token */ {
		return false
	}

	var token [16]byte
	copy(token[:], data[len(data)-16:])
	if sess, ok := h.resetTokens[token]; ok {
		h.logger.Debugf("Received a stateless reset with token %#x. Closing session.", token)
		go sess.destroy(&statelessResetErr{token: &token})
		return true
	}
	return false
}

func (h *packetHandlerMap) GetStatelessResetToken(connID protocol.ConnectionID) [16]byte {
	var token [16]byte
	if !h.statelessResetEnabled {
		// Return a random stateless reset token.
		// This token will be sent in the server's transport parameters.
		// By using a random token, an off-path attacker won't be able to disrupt the connection.
		rand.Read(token[:])
		return token
	}
	h.statelessResetMutex.Lock()
	h.statelessResetHasher.Write(connID.Bytes())
	copy(token[:], h.statelessResetHasher.Sum(nil))
	h.statelessResetHasher.Reset()
	h.statelessResetMutex.Unlock()
	return token
}

func (h *packetHandlerMap) maybeSendStatelessReset(p *receivedPacket, connID protocol.ConnectionID) {
	defer p.buffer.Release()
	if !h.statelessResetEnabled {
		return
	}
	// Don't send a stateless reset in response to very small packets.
	// This includes packets that could be stateless resets.
	if len(p.data) <= protocol.MinStatelessResetSize {
		return
	}
	token := h.GetStatelessResetToken(connID)
	h.logger.Debugf("Sending stateless reset to %s (connection ID: %s). Token: %#x", p.remoteAddr, connID, token)
	data := make([]byte, protocol.MinStatelessResetSize-16, protocol.MinStatelessResetSize)
	rand.Read(data)
	data[0] = (data[0] & 0x7f) | 0x40
	data = append(data, token[:]...)
	if _, err := h.conn.WriteTo(data, p.remoteAddr); err != nil {
		h.logger.Debugf("Error sending Stateless Reset: %s", err)
	}
}
