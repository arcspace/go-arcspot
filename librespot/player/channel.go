package player

// ChunkChannel?
type Channel struct {
	num       uint16
	gotHeader bool
	
	a *spotifyAsset // REMOVE ME
	
	chunkData  []byte // chunk data
	chunkIdx   uint32
	onComplete chan bool // signals completion of the chunk
}

