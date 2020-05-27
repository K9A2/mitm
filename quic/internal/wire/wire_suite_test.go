package wire

import (
	"bytes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/stormlin/mitm/quic/internal/protocol"
	"github.com/stormlin/mitm/quic/internal/utils"

	"testing"
)

func TestWire(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Wire Suite")
}

const (
	// a QUIC version that uses the IETF frame types
	versionIETFFrames = protocol.VersionTLS
)

func encodeVarInt(i uint64) []byte {
	b := &bytes.Buffer{}
	utils.WriteVarInt(b, i)
	return b.Bytes()
}
