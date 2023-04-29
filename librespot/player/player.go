package player

import (
	"bytes"
	"crypto/aes"
	"encoding/binary"
	"fmt"
	"log"
	"sync"

	"github.com/librespot-org/librespot-golang/Spotify"
	"github.com/librespot-org/librespot-golang/librespot/connection"
	"github.com/librespot-org/librespot-golang/librespot/mercury"
)

// DownloadMgr
type Player struct {
	stream   connection.PacketStream
	mercury  *mercury.Client
	//seq      uint32
	//audioKey []byte

	chMu    sync.Mutex
	chMap    map[uint16]*Channel
	seqChans    sync.Map
	nextChan    uint16
}

func CreatePlayer(conn connection.PacketStream, client *mercury.Client) *Player {
	return &Player{
		stream:   conn,
		mercury:  client,
		chMap: map[uint16]*Channel{},
		seqChans: sync.Map{},
		chMu: sync.Mutex{},
		nextChan: 0,
	}
}

func (p *Player) LoadTrack(file *Spotify.AudioFile, trackId []byte) (*spotifyAsset, error) {
	return p.LoadTrackWithIdAndFormat(file.FileId, file.GetFormat(), trackId)
}

func (p *Player) LoadTrackWithIdAndFormat(fileId []byte, format Spotify.AudioFile_Format, trackId []byte) (*spotifyAsset, error) {
	dl := newAudioFileWithIdAndFormat(fileId, format, p)
	
	key, err := p.loadTrackKey(trackId, fileId)
	if err != nil {
		return nil, fmt.Errorf("failed to load key: %+v", err)
	}
	dl.cipher, err = aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt aes cipher: %+v", err)
	}
	
	// Then start loading the audio itself
	dl.loadChunks()
	return dl, err
}

func (p *Player) loadTrackKey(trackId []byte, fileId []byte) ([]byte, error) {
	seqInt, seq := p.mercury.NextSeqWithInt()

	channel := make(chan []byte)
	p.seqChans.Store(seqInt, channel)

	req := buildKeyRequest(seq, trackId, fileId)
	err := p.stream.SendPacket(connection.PacketRequestKey, req)
	if err != nil {
		log.Println("error sending packet", err)
		return nil, err
	}

	key := <-channel
	p.seqChans.Delete(seqInt)

	return key, nil
}



// opens a new data channel to recv the requested chunk
func (p *Player) RequestChunk(chunkIdx uint32, fileID []byte, a *spotifyAsset) (*Channel, error) {
	
	p.chMu.Lock()
	cc := &Channel{
		num:       p.nextChan,
	}
	p.nextChan++
	p.chMap[cc.num] = cc
	p.chMu.Unlock()


	if cap(cc.chunkData) < kChunkByteSize {
		cc.chunkData = make([]byte, 0, kChunkByteSize) 
	} else {
		cc.chunkData = cc.chunkData[:0]
	}
	if cc.onComplete == nil {
		cc.onComplete = make(chan bool)
	}

	//cc.onHeader = a.onChannelHeader
	//cc.onData = a.onChannelData

	wordOfs := uint32(chunkIdx * kChunkWordSize)
	cc.chunkIdx = chunkIdx
	cc.a = a
	if err := p.stream.SendPacket(
		connection.PacketStreamChunk,
		buildAudioChunkRequest(
			cc.num,
			fileID,
			wordOfs,
			wordOfs + kChunkWordSize,
		),
	); err != nil {
		return nil, fmt.Errorf("could not send stream chunk: %+v", err)
	}

	return cc, nil
}




func (p *Player) HandleCmd(cmd byte, data []byte) error {
	switch {
	case cmd == connection.PacketAesKey:
		// Audio key response
		dataReader := bytes.NewReader(data)
		var seqNum uint32
		err := binary.Read(dataReader, binary.BigEndian, &seqNum)
		if err != nil {
			return fmt.Errorf("could not read binary seqNum: %+v", err)
		}
		if channel, ok := p.seqChans.Load(seqNum); ok {
			channel.(chan []byte) <- data[4:20]
		} else {
			return fmt.Errorf("unknown channel for audio key %d", seqNum)
		}
	case cmd == connection.PacketAesKeyError:
		return fmt.Errorf("audio key error")
	case cmd == connection.PacketStreamChunkRes:
		// Audio data response
		var channel uint16
		dataReader := bytes.NewReader(data)
		err := binary.Read(dataReader, binary.BigEndian, &channel)
		if err != nil {
			return fmt.Errorf("could not read binary channel: %+v", err)
		}
		p.chMu.Lock()
		dstCh, ok := p.chMap[channel]
		p.chMu.Unlock()
		if ok {
			p.handlePacket(dstCh, data[2:])
		} else {
			return fmt.Errorf("unknown channel")
		}
	}
	return nil
}

func (p *Player) releaseChannel(channel *Channel) {
	p.chMu.Lock()
	delete(p.chMap, channel.num)
	p.chMu.Unlock()
}



func (p *Player) handlePacket(cc *Channel, data []byte) {
	dataReader := bytes.NewReader(data)

	if !cc.gotHeader {
		// Read the header
		length := uint16(0)
		var err error = nil
		for err == nil {
			err = binary.Read(dataReader, binary.BigEndian, &length)

			if err != nil {
				break
			}

			if length > 0 {
				var headerId uint8
				binary.Read(dataReader, binary.BigEndian, &headerId)

				read := uint16(0)
				p.onChannelHeader(cc, headerId, dataReader)

				// Consume the remaining un-read data
				dataReader.Read(make([]byte, length-read))
			}
		}
		cc.gotHeader = true

	} else {

		// is there a more robust way to signal completion?
		if len(data) == 0 {
			cc.onComplete <- true 
			//p.a.putEncryptedChunk(chunkIndex, chunkData[0:chunkSz])
			p.releaseChannel(cc)
		} else {
			cc.chunkData = append(cc.chunkData, data...)
		}

	}

}






func (p *Player) onChannelHeader(cc *Channel, id byte, data *bytes.Reader) uint16 {
	read := uint16(0)

	if id == 0x3 {
		var size uint32
		binary.Read(data, binary.BigEndian, &size)
		size *= 4

		a := cc.a
		
		if a.totalSize.Load() != size {
			a.totalSize.Store(size)
			if a.data == nil {
				a.data = make([]byte, size)
			}

			// Recalculate the number of chunks pending for load
			a.chunkLock.Lock()
			for i := uint32(0); i < a.totalChunks(); i++ {
				a.chunkLoadOrder = append(a.chunkLoadOrder, i)
			}
			a.chunkLock.Unlock()

			// Re-launch the chunk loading system. It will check itself if another goroutine is already loading chunks.
			go a.loadNextChunk()
		}

		// Return 4 bytes read
		read = 4
	}

	return read
}


/*

func (a *spotifyAsset) onChannelHeader(channel *Channel, id byte, data *bytes.Reader) uint16 {
	read := uint16(0)

	if id == 0x3 {
		var size uint32
		binary.Read(data, binary.BigEndian, &size)
		size *= 4

		if a.totalSize.Load() != size {
			a.totalSize.Store(size)
			if a.data == nil {
				a.data = make([]byte, size)
			}

			// Recalculate the number of chunks pending for load
			a.chunkLock.Lock()
			for i := 0; i < a.totalChunks(); i++ {
				a.chunkLoadOrder = append(a.chunkLoadOrder, i)
			}
			a.chunkLock.Unlock()

			// Re-launch the chunk loading system. It will check itself if another goroutine is already loading chunks.
			go a.loadNextChunk()
		}

		// Return 4 bytes read
		read = 4
	}

	return read
}

func (a *spotifyAsset) onChannelData(channel *Channel, data []byte) uint16 {
	if data != nil {
		a.responseChan <- data
		return 0 // uint16(len(data))
	} else {
		a.responseChan <- []byte{}
		return 0
	}

}

*/