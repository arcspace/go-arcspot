package player

import (
	"bytes"
)

type headerFunc func(channel *Channel, id byte, data *bytes.Reader) uint16
type dataFunc func(channel *Channel, data []byte) uint16

// ChunkChannel?
type Channel struct {
	num       uint16
	gotHeader bool
	onHeader  headerFunc
	onData    dataFunc
	
	chunkDat   []byte // chunk data
	chunkOfs   uint32 // offset to where this chunk appears in the parent asset
	onComplete chan bool // signals completion of the chunk
}

