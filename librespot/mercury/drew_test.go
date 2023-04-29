package mercury_test

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"testing"

	"github.com/librespot-org/librespot-golang/Spotify"
	"github.com/librespot-org/librespot-golang/librespot"
	"github.com/librespot-org/librespot-golang/librespot/core"
	"github.com/librespot-org/librespot-golang/librespot/utils"
)


const (
	// The device name that is registered to Spotify servers
	defaultDeviceName = "librespot"
)

func TestSave(t *testing.T) {

	// Read flags from commandline
	username := "" //"1228340827"
	password := "" //"ellipse55"
	blob := flag.String("blob", "blob.bin", "spotify auth blob")
	devicename := flag.String("devicename", defaultDeviceName, "name of device")
	flag.Parse()

	// Authenticate
	var session *core.Session
	var err error
	
	client_secret := "f7e632155cf445248a2e16e068a78d97"
	client_id := "8de730d205474e1490e696adfc10d61c"
	
	accessToken := "BQArSiOxiyudLnNSPPl7sImU-TjB3M-Ww6GRzaS4SPqSW6BEN43yxnF4SYvN-NS8eavG6Jk43o4WMFfOarH4sEGwbntS95vf1iTbNbaex_tR3U81QhtrHEPLGSfuS-CyCQfx-GjdNEuajX_Emal5OWkWHDwNoPE62jN50nTP7W7Wq4gcXs8n25xwTA"
	
	if username != "" && password != "" {
		// Authenticate using a regular login and password, and store it in the blob file.
		session, err = librespot.Login(username, password, *devicename)
	} else if *blob != "" && username != "" {
		// Authenticate reusing an existing blob
		blobBytes, err := ioutil.ReadFile(*blob)

		if err != nil {
			fmt.Printf("Unable to read auth blob from %s: %s\n", *blob, err)
			os.Exit(1)
			return
		}

		session, err = librespot.LoginSaved(username, blobBytes, *devicename)
	} else if client_secret != "" {
	    if accessToken == "" {
		    // Authenticate using OAuth (untested)
		    session, err = librespot.LoginOAuth(*devicename, client_id, client_secret, "http://localhost:5000/callback")
        } else {
            session, err = librespot.LoginOAuthToken(*devicename, accessToken)
        }
	} else {
		// No valid options, show the helo
		fmt.Println("need to supply a username and password or a blob file path")
		fmt.Println("./microclient --username SPOTIFY_USERNAME [--blob ./path/to/blob]")
		fmt.Println("or")
		fmt.Println("./microclient --username SPOTIFY_USERNAME --password SPOTIFY_PASSWORD [--blob ./path/to/blob]")
		return
	}

	if err != nil {
		fmt.Println("Error logging in: ", err)
		os.Exit(1)
		return
	}

    funcSearch(session, "rebecca st james")
    
    funcTrack(session, "1iNYxr6BgevbX4PA19yt1q") // 11dFghVXANMlKmJXsNCbNl
    
    funcPlay(session, "1iNYxr6BgevbX4PA19yt1q")
    
}


func funcTrack(session *core.Session, trackID string) {
	fmt.Println("Loading track: ", trackID)

	track, err := session.Mercury().GetTrack(utils.Base62ToHex(trackID))
	if err != nil {
		fmt.Println("Error loading track: ", err)
		return
	}

	fmt.Println("Track title: ", track.GetName())
}

func funcSearch(session *core.Session, keyword string) {
	resp, err := session.Mercury().Search(keyword, 12, session.Country, session.Username)

	if err != nil {
		fmt.Println("Failed to search:", err)
		return
	}

	res := resp.Results

	fmt.Println("Search results for ", keyword)
	fmt.Println("=============================")

	if res.Error != nil {
		fmt.Println("Search result error:", res.Error)
	}

	fmt.Printf("Albums: %d (total %d)\n", len(res.Albums.Hits), res.Albums.Total)

	for _, album := range res.Albums.Hits {
		fmt.Printf(" => %s (%s)\n", album.Name, album.Uri)
	}

	fmt.Printf("\nArtists: %d (total %d)\n", len(res.Artists.Hits), res.Artists.Total)

	for _, artist := range res.Artists.Hits {
		fmt.Printf(" => %s (%s)\n", artist.Name, artist.Uri)
	}

	fmt.Printf("\nTracks: %d (total %d)\n", len(res.Tracks.Hits), res.Tracks.Total)

	for _, track := range res.Tracks.Hits {
		fmt.Printf(" => %s (%s)\n", track.Name, track.Uri)
	}
}

func funcPlay(session *core.Session, trackID string) {
	fmt.Println("Loading track for play: ", trackID)

	// Get the track metadata: it holds information about which files and encodings are available
	track, err := session.Mercury().GetTrack(utils.Base62ToHex(trackID))
	if err != nil {
		fmt.Println("Error loading track: ", err)
		return
	}

	fmt.Println("Track:", track.GetName())

	// As a demo, select the OGG 160kbps variant of the track. The "high quality" setting in the official Spotify
	// app is the OGG 320kbps variant.
	var selectedFile *Spotify.AudioFile
	for _, file := range track.GetFile() {
		if file.GetFormat() == Spotify.AudioFile_OGG_VORBIS_160 {
			selectedFile = file
		}
	}

	// Synchronously load the track
	audioFile, err := session.Player().LoadTrack(selectedFile, track.GetGid())
	if err != nil {
		fmt.Printf("Error while loading track: %s\n", err)
		return
	}
	fmt.Printf("Received audio file: %d bytes\n", audioFile.Size())
	buffer, err := ioutil.ReadAll(audioFile)
	if err != nil {
		fmt.Printf("Error while streaming file: %s\n", err)
		return
	}
	err = ioutil.WriteFile(fmt.Sprintf("%s - %s.ogg", track.GetArtist()[0].GetName(), track.GetName()), buffer, os.ModePerm)
	if err != nil {
		fmt.Printf("Error while writing file: %s\n", err)
		return
	}
}
