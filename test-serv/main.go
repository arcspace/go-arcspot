package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
)

func main() {
	// Define the pathname of the MP3 file
	mp3FilePath := "/Users/aomeara/Desktop/inflowmotion_apr2023_DepGlobe.mp3"

	// Get the file size
	fileInfo, err := os.Stat(mp3FilePath)
	if err != nil {
		panic(err)
	}
	//fileSize := fileInfo.Size()

	// Create a handler function that serves the MP3 file
	http.HandleFunc("/test", func(w http.ResponseWriter, r *http.Request) {
		// Set the Content-Type header to audio/mpeg
		w.Header().Set("Content-Type", "audio/mpeg")
		/*
		// Set the Accept-Ranges header to bytes
		w.Header().Set("Accept-Ranges", "bytes")

		w.Header().Set("Cache-Control", "no-store")

		rangeStart := int64(0)
		rangeEnd := fileSize - 1

		// Check if the client has requested a byte range
		rangeHeader := r.Header.Get("Range")
		if rangeHeader != "" {
			// Parse the byte range requested by the client
			rangeParts := strings.Split(rangeHeader, "=")[1]
			rangeBytes := strings.Split(rangeParts, "-")
			rangeStart, _ = strconv.ParseInt(rangeBytes[0], 10, 64)

			// Set the Content-Range header to the byte range being served
			if len(rangeBytes) > 1 {
				if rangeBytes[1] != "" { // "<startOfs>-" means "to the end"
					rangeEnd, _ = strconv.ParseInt(rangeBytes[1], 10, 64)
				}
			}
		}

		// Set the Content-Length header to the size of the byte range being served
		w.Header().Set("Content-Length", fmt.Sprintf("%d", rangeEnd-rangeStart+1))

		w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", rangeStart, rangeEnd, fileSize))
*/
		// Serve the byte range using http.ServeContent and the custom fileRange type
		src := &assetReader{assetPath: mp3FilePath}
		http.ServeContent(w, r, fileInfo.Name(), fileInfo.ModTime(), src)
		src.Close()

	})

	// Start the HTTP server on port 8080
	fmt.Println("Starting server on port 8080...")
	http.ListenAndServe(":8080", nil)
}


type assetReader struct {
	assetPath string
	file      *os.File
	totalSz   int64
}

func (a *assetReader) isReady() error {
    if a.file != nil {
        return nil
    }
    var err error
    a.file, err = os.Open(a.assetPath)
    if err != nil {
        return err
    }
    a.totalSz, err = a.file.Seek(-1, io.SeekEnd)
    if err != nil {
        return err
    }
    // a.file.Seek(0, io.SeekStart)
    return nil
}

// func (a *assetReader) Size() int64 {
//     if a.isReady() != nil {
//         return 0
//     }
//     return a.totalSz
// }

func (a *assetReader) Seek(offset int64, whence int) (int64, error) {
    if err := a.isReady(); err != nil {
        return 0, err
    }

	pos, err := a.file.Seek(offset, whence); 
	if err != nil {
	    return 0, err
    }
    
    fmt.Printf("=======> Seeking to %d\n", pos)
	
	
	// if newPos < 0 || newPos >= a.totalSz {
	//     return 0, fmt.Errorf("invalid seek position %d", newPos)
    // }
    // a.pos = newPos
	return pos, nil
}

func (a *assetReader) Read(p []byte) (int, error) {
    if err := a.isReady(); err != nil {
        return 0, err
    }
    return a.file.Read(p)
}

func (a *assetReader) Close() error {
    var err error
    if a.file != nil {
        err = a.file.Close()
        a.file = nil
    }
    return err
}

