package asset

import (
	"crypto/cipher"
	"errors"
	"io"
	"sync"
	"sync/atomic"

	"github.com/arcspace/go-arcspace/arc/assets"
	"github.com/arcspace/go-cedar/process"
	"github.com/librespot-org/librespot-golang/Spotify"
	"github.com/librespot-org/librespot-golang/librespot/core/crypto"
)

type ChunkIdx int32 // 0-based index of the chunk in the asset

const (
	kChunkWordSize = 1 << 15             // Number of 4-byte words per chunk
	kChunkByteSize = kChunkWordSize << 2 // 4 bytes per word

	// Spotify inserts a custom Ogg packet at the start with custom metadata values, that you would
	// otherwise expect in Vorbis comments. This packet isn't well-formed and players may balk at it.
	// Search for "parse_from_ogg" in librespot -- https://github.com/librespot-org/librespot
	SPOTIFY_OGG_HEADER_SIZE = 0xa7
)

type ChunkStatus int32

const (
	chunkHalted ChunkStatus = iota
	chunkInProgress
	chunkReadyToDecrypt // chunk obtained but is still encrypted
	chunkReady          // chunk obtained and decrypted successfully (chunkData is ready and the correct size)

	ChunkIdx_Nil          = ChunkIdx(-1)
	kMaxConcurrentFetches = 1
)

type assetChunk struct {
	chID         uint16
	totalAssetSz int64
	gotHeader    bool
	asset        *mediaAsset
	Data         []byte
	Idx          ChunkIdx
	status       ChunkStatus
	accessStamp  int64        // allows stale chunks to be identified and evicted
	numReaders   atomic.Int32 // tracks what it in use

}

/*

// inclusive range of chunks
type chunkRange struct {
	Start  ChunkIdx
	Length ChunkIdx
}

func (r *chunkRange) Contains(idx ChunkIdx) bool {
	return idx >= r.Start && idx < r.Start+r.Length
}

func (r *chunkRange) AssignRange(lo, hi, hi_max ChunkIdx) {
	if lo < 0 {
		lo = 0
	}
	r.Start = lo
	if hi > hi_max {
		hi = hi_max
	}
	r.Length = hi - lo + 1
}
*/

// mediaAsset represents a downloadable/cached audio file fetched by Spotify, in an encoded format (OGG, etc)
type mediaAsset struct {
	process.Context

	label        string
	mediaType    string
	totalBytes   int64 // 0 denotes not known
	finalChunk   ChunkIdx
	track        *Spotify.Track
	trackFile    *Spotify.AudioFile
	downloader   *downloader
	cipher       cipher.Block
	decrypter    crypto.BlockDecrypter
	dataStartOfs int64
	chunksMu     sync.Mutex
	chunks       map[ChunkIdx]*assetChunk
	//chunks        redblacktree.Tree

	residentChunkLimit int
	latestRead         ChunkIdx         // most recently requested chunk
	accessCount        int64            // incremented each time a different chunk is read
	fetchAhead         ChunkIdx         // number of chunks to fetch ahead of the current read position
	fetching           int32            // number of chunks currently being fetched
	onChunkComplete    chan *assetChunk // consumed by this assets runLoop to process completed chunks
	chunkChange        sync.Cond        // signaled when a chunk is available
	fatalErr           error
	//onOpen         chan struct{} // sent from the internal runLoop to signal the asset is ready (or failed to open)
	//onChunkFetch   chan ChunkIdx // sent from outside thread to the internal runLoop to initiate a chunk fetch
	//onChunkReady   chan ChunkIdx // consumed from internal runLoop to signal a chunk is now ready
}

func newMediaAsset(dl *downloader, track *Spotify.Track) *mediaAsset {

	ma := &mediaAsset{
		downloader: dl,
		track:      track,
		// chunks: redblacktree.Tree{
		// 	Comparator: func(A, B interface{}) int {
		// 		idx_a := A.(ChunkIdx)
		// 		idx_b := B.(ChunkIdx)
		// 		return int(idx_b - idx_a)
		// 	},
		// },
		onChunkComplete: make(chan *assetChunk, 1),
		fetchAhead:      5,
	}
	ma.chunkChange = sync.Cond{
		L: &ma.chunksMu,
	}

	// TEST ME
	ma.setResidentByteLimit(10 * 1024 * 1024)

	return ma
}

func (ma *mediaAsset) setResidentByteLimit(byteLimit int) {
	chunkLimit := 6 + byteLimit/kChunkByteSize
	ma.residentChunkLimit = chunkLimit
	if ma.chunks == nil {
		initialSz := min(chunkLimit, 70)
		ma.chunks = make(map[ChunkIdx]*assetChunk, initialSz)
	}
}

func (ma *mediaAsset) Label() string {
	return ma.label
}

func (ma *mediaAsset) MediaType() string {
	return ma.mediaType
}

// func (ma *mediaAsset) WaitUntilReady() error {
// 	ma.chunksMu.Lock()
// 	defer ma.chunksMu.Unlock()

// 	if ma.totalBytes > 0 && ma.fatalErr == nil {
// 		return nil
// 	}

// }

// func (ma *mediaAsset) GetFilename() string {
// 	return fmt.Sprintf("%s - %s.ogg", ma.track.GetArtist()[0].GetName(), ma.track.GetName()),
// }

// pre: ma.chunksMu is locked
func (ma *mediaAsset) OnStart(ctx process.Context) error {
	ma.Context = ctx
	go ma.runLoop()
	return nil
}

// // pre: ma.chunksMu is locked
// func (ma *mediaAsset) getChunk(idx ChunkIdx) *assetChunk {
// 	if chunk := ma.chunks[idx]; chunk != nil {
// 		return chunk
// 	}
// 	return nil
// }

// pre: ma.chunksMu is locked
func (ma *mediaAsset) getReadyChunk(idx ChunkIdx) *assetChunk {
	if chunk := ma.chunks[idx]; chunk != nil {
		if chunk.status == chunkReady {
			return chunk
		}
	}

	// val, exists := ma.chunks.Get(idx)
	// if exists {
	// 	chunk := val.(*assetChunk)
	// 	if chunk.status == chunkReady {
	// 		return chunk
	// 	}
	// }
	return nil
}

/*

// Steps rightward from an index until a hole is found (or the maxDelta is reached)
// pre: ma.chunksMu is locked
func (ma *mediaAsset) findNextChunkHole(startIdx, maxDelta ChunkIdx) ChunkIdx {
	holeIdx := ChunkIdx_Nil

	startNode := ma.chunks.GetNode(startIdx)
	if startNode == nil {
		holeIdx = startIdx
	} else {
		itr := ma.chunks.IteratorAt(startNode)
		for i := ChunkIdx(1); itr.Next() && i < maxDelta; i++ {
			idx := itr.Key().(ChunkIdx)
			idx_expected := startIdx + i
			if idx != idx_expected {
				holeIdx = idx_expected
				break
			}
		}
	}

	if holeIdx > ma.finalChunk {
		holeIdx = ChunkIdx_Nil
	}

	return holeIdx
}
*/

// func (ma *mediaAsset) checkErr(err error) {
// 	if err == nil {
// 		return
// 	}
// 	ma.throwErr(err)
// }

func (ma *mediaAsset) throwErr(err error) {
	if ma.fatalErr == nil {
		if err == nil {
			err = errors.New("unspecified fatal error")
		}
		ma.fatalErr = err
		ma.Context.Close()
	}
}

// Primary run loop for this media asset
// runs in its own goroutine and pushes events as they come
func (ma *mediaAsset) runLoop() {

	for running := true; running; {
		if ma.fatalErr != nil {
			return
		}

		select {
		case chunk := <-ma.onChunkComplete:
			if chunk.status == chunkReadyToDecrypt {
				ma.decrypter.DecryptSegment(chunk.Idx.StartByteOffset(), ma.cipher, chunk.Data, chunk.Data)
				chunk.status = chunkReady

				ma.chunksMu.Lock()
				ma.fetching -= 1
				//ma.Context.Infof(2, "RECV  %03d   fetching: %003d\n", chunk.Idx, ma.fetching)
				{
					// The size should only change when the first chunk arrives
					if sz := chunk.totalAssetSz; sz != ma.totalBytes {
						ma.totalBytes = sz
						ma.finalChunk = ChunkIdxAtOffset(sz)
					}
					ma.requestChunkIfNeeded(ma.latestRead)
				}
				ma.chunkChange.Signal()
				ma.chunksMu.Unlock()

			} else {
				panic("chunk failed")
			}

		case <-ma.Context.Closing():
			ma.throwErr(ma.Context.Err())
			running = false

			ma.chunksMu.Lock()
			ma.chunkChange.Signal()
			ma.chunksMu.Unlock()
		}
	}
}

// Pre: ma.chunksMu is locked
func (ma *mediaAsset) requestChunkIfNeeded(needIdx ChunkIdx) {

	if ma.fetching >= kMaxConcurrentFetches {
		return
	}

	fetchIdx := ChunkIdx_Nil

	{
		idx := needIdx + ma.fetchAhead
		if idx > ma.finalChunk {
			idx = ma.finalChunk
		}
		// Most of the time, we'll have fetched ahead, so start from the right
		for ; idx >= needIdx; idx-- {
			chunk := ma.chunks[idx]
			if chunk == nil {
				fetchIdx = idx
			}
		}
	}

	if fetchIdx >= 0 {
		ma.fetching += 1
		chunk, err := ma.downloader.RequestChunk(fetchIdx, ma)
		if err != nil {
			ma.throwErr(err)
			return
		} else {
			ma.chunks[fetchIdx] = chunk
		}
	}

	/*
		{
			L := readingAt
			R := readingAt + ma.fetchAhead
			if R > ma.finalChunk {
				R = ma.finalChunk
			}

			// find the next chunk we don't have withing the fetch ahead range
			for R - L > 0 {
				idx := (L + R) >> 1
				chunk := ma.chunks[idx]
				if chunk == nil {
					fetchIdx = idx
					R = idx
				} else {
					L = idx
				}
			}
			for ahead := ma.fetchAhead; ahead >= 0; ahead-- {
				idx := readingAt + ahead
				if idx > ma.finalChunk {
					break
				}
				chunk := ma.chunks[idx]
				if chunk == nil {
					fetchIdx = idx
				}
			}
		}

		// Update readyRange (bookkeeping)
		{
			idx := ma.readyRange.Start + ma.readyRange.Length
			for ; idx <= ma.finalChunk; idx++ {
				chunk := ma.getReadyChunk(idx)
				if chunk == nil {
					break
				}
				ma.readyRange.Length++  // readyRange bookkeeping
			}
		}


		// Reset ready range if the current read position is outside of it.
		// Include an additional rightmost element since
		if readingAt < ma.readyRange.Start || readingAt >= ma.readyRange.Start+ma.readyRange.Length {
			fetchIdx = readingAt
			ma.readyRange.Start = readingAt
			ma.readyRange.Length = 0
		}

		if ma.readyRange.Contains(readingAt) {
			readUpto := readingAt + ma.fetchAhead
			idx := ma.readyRange.Start + ma.readyRange.Length
			if idx <= ma.finalChunk && idx <= readUpto {
				chunk := ma.chunks[idx]
				if chunk == nil {
					fetchIdx = idx
				}
			}
		} else {
			fetchIdx = readingAt
			ma.readyRange.Start = readingAt
			ma.readyRange.Length = 0
		}

		// // Update readyRange (bookkeeping) and figure out what to fetch next
		// idx := ma.readyRange.Start + ma.readyRange.Length
		// for ; idx <= ma.finalChunk; idx++ {
		// 	chunk := ma.chunks[idx]
		// 	if chunk != nil {
		// 		if chunk.status == chunkReady {
		// 			ma.readyRange.Length++  // readyRange bookkeeping
		// 		}
		// 	} else {
		// 		if idx >= readingAt && idx <= readUpto {
		// 			fetchIdx = idx
		// 		}
		// 		break
		// 	}
		// }
	*/

}

// readChunk returns the chunk at the given index, blocking until it is available or a fatal error.
func (ma *mediaAsset) readChunk(idx ChunkIdx) (*assetChunk, error) {

	// Note: this is wired where chunk 0 is always assumed to exist (and is always the first accessed)
	if idx < 0 || idx > ma.finalChunk {
		return nil, io.EOF
	}

	ma.chunkChange.L.Lock()
	defer ma.chunkChange.L.Unlock()

	ma.latestRead = idx

	for ma.fatalErr == nil {
		ma.requestChunkIfNeeded(idx)

		// Is the chunk ready -- most of the time it should be ready since we read ahead
		chunk := ma.getReadyChunk(idx)
		if chunk != nil {
			ma.accessCount++
			chunk.accessStamp = ma.accessCount
			chunk.numReaders.Add(1)
			return chunk, nil
		}

		//ma.Context.Infof(2, "WAIT  %03d   fetching: %003d\n", idx, ma.fetching)

		// Wait for signal; unlocks ma.chunkChange.L
		ma.chunkChange.Wait()
	}

	return nil, ma.fatalErr
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func (asset *mediaAsset) NewAssetReader() (assets.AssetReader, error) {
	reader := &assetReader{
		asset:   asset,
		readPos: 0,
	}
	return reader, nil
}

// func (ma *mediaAsset) headerOffset() int64 {
// 	// If the file format is an OGG, we skip the first kOggSkipBytes (167) bytes. We could implement despotify's
// 	// SpotifyOggHeader (https://sourceforge.net/p/despotify/code/HEAD/tree/java/trunk/src/main/java/se/despotify/client/player/SpotifyOggHeader.java)
// 	// to read Spotify's metadata (samples, length, gain, ...). For now, we simply skip the custom header to the actual
// 	// OGG/Vorbis data.
// 	return int64(ma.headerOfs)
// }

func (ci ChunkIdx) StartByteOffset() int64 {
	return kChunkByteSize * int64(ci)
}

func ChunkIdxAtOffset(byteOfs int64) ChunkIdx {
	return ChunkIdx(byteOfs >> 17) // int(math.Floor(float64(byteIndex) / float64(kChunkSize) / 4.0))
}

type assetReader struct {
	asset    *mediaAsset
	hotChunk *assetChunk
	readPos  int64
}

// Read is an implementation of the io.Reader interface.
// This function will block until a non-zero amount of data is available (or io.EOF or a fatal error occurs).
func (r *assetReader) Read(buf []byte) (int, error) {
	bytesRemain := len(buf)
	bytesRead := 0

	if r.readPos < r.asset.dataStartOfs {
		r.readPos = r.asset.dataStartOfs
	}

	//r.asset.Infof(2, "      REQ  ==>  %d bytes @%v", bytesRemain, r.readPos-r.asset.headerOffset())

	for bytesRemain > 0 {
		hotIdx := ChunkIdxAtOffset(r.readPos)
		hotChunk, err := r.seekChunk(hotIdx)
		if err != nil {
			return 0, err
		}

		relPos := int(r.readPos - hotIdx.StartByteOffset())
		runSz := len(hotChunk.Data) - relPos
		runSz = min(runSz, bytesRemain)
		if runSz > 0 {
			copy(buf[bytesRead:bytesRead+runSz], hotChunk.Data[relPos:relPos+runSz])
			r.readPos += int64(runSz)
			bytesRead += runSz
			bytesRemain -= runSz
		} else if runSz == 0 && hotIdx >= r.asset.finalChunk {
			if bytesRead == 0 {
				return 0, io.EOF
			}
			break
		} else {
			panic("bad runSz")
		}
	}

	//r.asset.Infof(2, "      READ ==>  %d bytes @%v", bytesRead, r.readPos-r.asset.headerOffset())

	return bytesRead, nil
}

func (r *assetReader) Seek(offset int64, whence int) (int64, error) {

	// If we already have a chunk then we don't have to bootstrap
	if r.hotChunk == nil {
		_, err := r.seekChunk(0)
		if err != nil {
			return 0, err
		}
	}

	switch whence {
	case io.SeekStart:
		r.readPos = offset + r.asset.dataStartOfs
	case io.SeekEnd:
		r.readPos = offset + r.asset.totalBytes
	case io.SeekCurrent:
		r.readPos += offset
	}

	//r.asset.Context.Infof(2, "      SEEK      %d", r.readPos-r.asset.headerOffset())

	return r.readPos - r.asset.dataStartOfs, nil
}

func (r *assetReader) seekChunk(idx ChunkIdx) (*assetChunk, error) {

	// If we already have the chunk, no need to get it from the asset
	{
		hotChunk := r.hotChunk
		if hotChunk != nil {
			if hotChunk.Idx == idx {
				return hotChunk, nil
			} else {
				hotChunk.numReaders.Add(-1)
			}
		}
	}

	asset := r.asset
	if asset == nil {
		return nil, io.ErrClosedPipe
	}

	var err error
	r.hotChunk, err = asset.readChunk(idx)
	return r.hotChunk, err
}

func (r *assetReader) Close() error {
	r.seekChunk(ChunkIdx_Nil) // released cached chunk
	r.asset = nil
	return nil
}
