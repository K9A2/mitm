package ackhandler

import (
	"github.com/stormlin/mitm/quic/internal/congestion"
	"github.com/stormlin/mitm/quic/internal/protocol"
	"github.com/stormlin/mitm/quic/internal/utils"
	"github.com/stormlin/mitm/quic/qlog"
	"github.com/stormlin/mitm/quic/quictrace"
)

func NewAckHandler(
	initialPacketNumber protocol.PacketNumber,
	rttStats *congestion.RTTStats,
	pers protocol.Perspective,
	traceCallback func(quictrace.Event),
	qlogger qlog.Tracer,
	logger utils.Logger,
	version protocol.VersionNumber,
) (SentPacketHandler, ReceivedPacketHandler) {
	sph := newSentPacketHandler(initialPacketNumber, rttStats, pers, traceCallback, qlogger, logger)
	return sph, newReceivedPacketHandler(sph, rttStats, logger, version)
}
