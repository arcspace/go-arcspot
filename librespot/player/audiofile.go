package player

import (
	"crypto/cipher"
	"io"
	"sync"
	"sync/atomic"

	"github.com/librespot-org/librespot-golang/Spotify"
)

const kChunkWordSize = 32768 // In number of words (so actual byte size is kChunkSize*4, aka. kChunkByteSize)
const kChunkByteSize = kChunkWordSize * 4
const kOggSkipBytes = 167 // Number of bytes to skip at the beginning of the file

// min helper function for integers
func min(a, b uint32) uint32 {
	if a < b {
		return a
	}
	return b
}

type AssetProvider interface {


}


type AssetStreamer interface {

	// Pin() is called first and signals the asset is about to be accessed.
	//  and Close() should be called last.
	// If the context is cancelled, Close() is internally called.
    // 	Pin(ctx context.Context) error
	
	io.ReadSeekCloser
}





// type SpotifyAsset struct {
// 	AssetStreamer
// 	ReadNextRun(ctx context.Context, ) (error
// }




// AssetDownloader?
// TrackDownloader
// spotifyAsset represents a downloadable/cached audio file fetched by Spotify, in an encoded format (OGG, etc)
type spotifyAsset struct {
	totalSize       atomic.Uint32
	//lock           sync.RWMutex
	format         Spotify.AudioFile_Format
	fileId         []byte
	player         *Player
	cipher         cipher.Block
	decrypter      BlockDecrypter
	chunkLock      sync.RWMutex
	chunkLoadOrder []uint32
	data           []byte
	cursor         uint32
	chunks         map[uint32]bool
	chunksLoading  bool
	cancelled      bool
}



























func newAudioFile(file *Spotify.AudioFile, player *Player) *spotifyAsset {
	return newAudioFileWithIdAndFormat(file.GetFileId(), file.GetFormat(), player)
}

func newAudioFileWithIdAndFormat(fileId []byte, format Spotify.AudioFile_Format, player *Player) *spotifyAsset {
	a := &spotifyAsset{
		player:        player,
		fileId:        fileId,
		format:        format,
		chunks:        map[uint32]bool{},
		chunkLock:     sync.RWMutex{},
		chunksLoading: false,
	}
	a.totalSize.Store(kChunkByteSize) // Set an initial size to fetch the first chunk regardless of the actual size
	return a
}

// Size returns the size, in bytes, of the final audio file
func (a *spotifyAsset) Size() uint32 {
	return a.totalSize.Load() - uint32(a.headerOffset())
}

// Read is an implementation of the io.Reader interface. Note that due to the nature of the streaming, we may return
// zero bytes when we are waiting for audio data from the Spotify servers, so make sure to wait for the io.EOF error
// before stopping playback.
func (a *spotifyAsset) Read(buf []byte) (int, error) {
	length := len(buf)
	outBufCursor := uint32(0)
	totalWritten := 0
	eof := false

	size := a.totalSize.Load()
	// Offset the data start by the header, if needed
	if a.cursor == 0 {
		a.cursor += a.headerOffset()
	} else if uint32(a.cursor) >= size {
		// We're at the end
		return 0, io.EOF
	}

	// Ensure at least the next required chunk is fully available, otherwise request and wait for it. Even if we
	// don't have the entire data required for len(buf) (because it overlaps two or more chunks, and only the first
	// one is available), we can still return the data already available, we don't need to wait to fill the entire
	// buffer.
	chunkIdx := chunkIndexAtByte(a.cursor)

	for totalWritten < length {
		if chunkIdx >= a.totalChunks() {
			// We've reached the last chunk, so we can signal EOF
			eof = true
			break
		} else if !a.hasChunk(chunkIdx) {
			// A chunk we are looking to read is unavailable, request it so that we can return it on the next Read call
			a.requestChunk(chunkIdx)
			break
		} else {
			// cursorEnd is the ending position in the output buffer. It is either the current outBufCursor + the size
			// of a chunk, in bytes, or the length of the buffer, whichever is smallest.
			cursorEnd := min(outBufCursor+kChunkByteSize, uint32(length))
			writtenLen := cursorEnd - outBufCursor

			// Calculate where our data cursor will end: either at the boundary of the current chunk, or the end
			// of the song itself
			dataCursorEnd := min(a.cursor+writtenLen, (chunkIdx+1)*kChunkByteSize)
			dataCursorEnd = min(dataCursorEnd, a.totalSize.Load())

			writtenLen = dataCursorEnd - a.cursor
			if writtenLen <= 0 {
				// No more space in the output buffer, bail out
				break
			}

			// Copy into the output buffer, from the current outBufCursor, up to the cursorEnd. The source is the
			// current cursor inside the audio file, up to the dataCursorEnd.
			copy(buf[outBufCursor:cursorEnd], a.data[a.cursor:dataCursorEnd])
			outBufCursor += writtenLen
			a.cursor += writtenLen
			totalWritten += int(writtenLen)

			// Update our current chunk, if we need to
			chunkIdx = chunkIndexAtByte(a.cursor)
		}
	}

	// The only error we can return here, is if we reach the end of the stream
	var err error
	if eof {
		err = io.EOF
	}

	return totalWritten, err
}

// Seek implements the io.Seeker interface
func (a *spotifyAsset) Seek(offset int64, whence int) (int64, error) {
	switch whence {
	case io.SeekStart:
		a.cursor = uint32(offset) + a.headerOffset()

	case io.SeekEnd:
		a.cursor = uint32(int64(a.totalSize.Load()) + offset)

	case io.SeekCurrent:
		a.cursor += uint32(offset)
	}

	return int64(a.cursor - a.headerOffset()), nil
}

// Cancels the current audio file - no further data will be downloaded
func (a *spotifyAsset) Cancel() {
	a.cancelled = true
}

func (a *spotifyAsset) headerOffset() uint32 {
	// If the file format is an OGG, we skip the first kOggSkipBytes (167) bytes. We could implement despotify's
	// SpotifyOggHeader (https://sourceforge.net/p/despotify/code/HEAD/tree/java/trunk/src/main/java/se/despotify/client/player/SpotifyOggHeader.java)
	// to read Spotify's metadata (samples, length, gain, ...). For now, we simply skip the custom header to the actual
	// OGG/Vorbis data.
	switch {
	case a.format == Spotify.AudioFile_OGG_VORBIS_96 || a.format == Spotify.AudioFile_OGG_VORBIS_160 ||
		a.format == Spotify.AudioFile_OGG_VORBIS_320:
		return kOggSkipBytes

	default:
		return 0
	}
}

func chunkIndexAtByte(byteIndex uint32) uint32 {
	return byteIndex >> 17 // int(math.Floor(float64(byteIndex) / float64(kChunkSize) / 4.0))
}

func (a *spotifyAsset) totalChunks() uint32 {
	return (a.totalSize.Load() + 131071) >> 17 // int(math.Ceil(float64(a.size.Load()) / float64(kChunkSize) / 4.0))
}

func (a *spotifyAsset) hasChunk(index uint32) bool {
	a.chunkLock.RLock()
	has, ok := a.chunks[index]
	a.chunkLock.RUnlock()

	return has && ok
}


func (a *spotifyAsset) loadChunks() {
	// By default, we will load the track in the normal order. If we need to skip to a specific piece of audio,
	// we will prepend the chunks needed so that we load them as soon as possible. Since loadNextChunk will check
	// if a chunk is already loaded (using hasChunk), we won't be downloading the same chunk multiple times.

	// We can however only download the first chunk for now, as we have no idea how many chunks this track has. The
	// remaining chunks will be added once we get the headers with the file size.
	a.chunkLoadOrder = append(a.chunkLoadOrder, 0)

	go a.loadNextChunk()
}

func (a *spotifyAsset) requestChunk(chunkIdx uint32) {
	a.chunkLock.RLock()

	// Check if we don't already have this chunk in the 2 next chunks requested
	if len(a.chunkLoadOrder) >= 1 && a.chunkLoadOrder[0] == chunkIdx ||
		len(a.chunkLoadOrder) >= 2 && a.chunkLoadOrder[1] == chunkIdx {
		a.chunkLock.RUnlock()
		return
	}

	a.chunkLock.RUnlock()

	// Set an artificial limit of 500 chunks to prevent overflows and buggy readers/seekers
	a.chunkLock.Lock()

	if len(a.chunkLoadOrder) < 500 {
		a.chunkLoadOrder = append([]uint32{chunkIdx}, a.chunkLoadOrder...)
	}

	a.chunkLock.Unlock()
}

// opens a new data channel to recv the requested chunk
func (a *spotifyAsset) loadChunk(chunkIdx uint32) error {
	cc, err := a.player.RequestChunk(chunkIdx, a.fileId, a)
	if err != nil {
		return err
	}
	
	<-cc.onComplete
	
	ofs := cc.chunkIdx * kChunkByteSize
	a.decrypter.DecryptAudioWithBlock(cc.chunkIdx, a.cipher, cc.chunkData, a.data[ofs:])

	a.chunkLock.Lock()
	a.chunks[cc.chunkIdx] = true
	a.chunkLock.Unlock()
	
	return nil
}

func (a *spotifyAsset) loadNextChunk() {
	if a.cancelled {
		return
	}
	
	a.chunkLock.Lock()

	if a.chunksLoading {
		// We are already loading a chunk, don't need to start another goroutine
		a.chunkLock.Unlock()
		return
	}

	a.chunksLoading = true
	chunkIndex := a.chunkLoadOrder[0]
	a.chunkLoadOrder = a.chunkLoadOrder[1:]

	a.chunkLock.Unlock()

	if !a.hasChunk(chunkIndex) {
		a.loadChunk(chunkIndex)
	}

	a.chunkLock.Lock()
	a.chunksLoading = false

	if len(a.chunkLoadOrder) > 0 {
		a.chunkLock.Unlock()
		a.loadNextChunk()
	} else {
		a.chunkLock.Unlock()
	}
}

