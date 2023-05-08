package main

import (
	"bufio"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strings"

	"github.com/arcspace/go-arcspot/pkg/core"
	"github.com/arcspace/go-arcspot/pkg/utils"
	"github.com/arcspace/go-cedar/errors"
)

const (
	// The device name that is registered to Spotify servers
	defaultDeviceName = "librespot"
	// The number of samples per channel in the decoded audio
	samplesPerChannel = 2048
	// The samples bit depth
	bitDepth = 16
)

func main() {

	err := main_loop()
	if err != nil {
		log.Fatal(err)
	}

}

func main_loop() error {

	// Read flags from commandline
	accessToken := ""
	username := flag.String("username", "", "spotify username")
	password := flag.String("password", "", "spotify password")
	blobPath := flag.String("blob", "blob.bin", "spotify auth blob")
	devicename := flag.String("devicename", defaultDeviceName, "name of device")
	flag.Parse()

	opts := core.SessionOpts{
		DeviceName: *devicename,
		//Context: host,
	}

	sess, err := opts.StartSession()
	if err != nil {
		log.Fatalln("StartSession: ", err)
	}

	if *username != "" && *password != "" {
		// Authenticate using a regular login and password, and store it in the blob file.
		err = sess.Login(*username, *password)
	} else if *blobPath != "" && *username != "" {
		// Authenticate reusing an existing blob
		blobBytes, err := ioutil.ReadFile(*blobPath)

		if err != nil {
			return errors.Wrapf(err, "Unable to read auth blob from %s", *blobPath)
		}

		err = sess.LoginSaved(*username, blobBytes)
	} else if os.Getenv("client_secret") != "" {
		if accessToken == "" {
			accessToken, err = core.LoginOAuth(os.Getenv("client_id"), os.Getenv("client_secret"), os.Getenv("redirect_uri"))
			if err != nil {
				return err
			}
		}
		err = sess.LoginOAuthToken(accessToken)
	} else {
		// No valid options, show the helo
		fmt.Println("need to supply a username and password or a blob file path")
		fmt.Println("./microclient --username SPOTIFY_USERNAME [--blob ./path/to/blob]")
		fmt.Println("or")
		fmt.Println("./microclient --username SPOTIFY_USERNAME --password SPOTIFY_PASSWORD [--blob ./path/to/blob]")
		return nil
	}

	if err != nil {
		return err
	}

	// Command loop
	reader := bufio.NewReader(os.Stdin)

	printHelp()

	for {
		fmt.Print("> ")
		text, _ := reader.ReadString('\n')
		cmds := strings.Split(strings.TrimSpace(text), " ")

		switch cmds[0] {
		case "help":
			printHelp()

		case "track":
			if len(cmds) < 2 {
				fmt.Println("You must specify the Base62 Spotify ID of the track")
			} else {
				funcTrack(sess, cmds[1])
			}

		case "artist":
			if len(cmds) < 2 {
				fmt.Println("You must specify the Base62 Spotify ID of the artist")
			} else {
				funcArtist(sess, cmds[1])
			}

		case "album":
			if len(cmds) < 2 {
				fmt.Println("You must specify the Base62 Spotify ID of the album")
			} else {
				funcAlbum(sess, cmds[1])
			}

		case "playlists":
			funcPlaylists(sess)

		case "search":
			funcSearch(sess, cmds[1])

		case "play":
			if len(cmds) < 2 {
				fmt.Println("You must specify the Base62 Spotify ID of the track")
			} else {
				funcPlay(sess, cmds[1])
			}

		default:
			fmt.Println("Unknown command")
		}
	}
}

func printHelp() {
	fmt.Println("\nAvailable commands:")
	fmt.Println("play <track>:                   play specified track by spotify base62 id")
	fmt.Println("track <track>:                  show details on specified track by spotify base62 id")
	fmt.Println("album <album>:                  show details on specified album by spotify base62 id")
	fmt.Println("artist <artist>:                show details on specified artist by spotify base62 id")
	fmt.Println("search <keyword>:               start a search on the specified keyword")
	fmt.Println("playlists:                      show your playlists")
	fmt.Println("help:                           show this help")
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

func funcArtist(session *core.Session, artistID string) {
	artist, err := session.Mercury().GetArtist(utils.Base62ToHex(artistID))
	if err != nil {
		fmt.Println("Error loading artist:", err)
		return
	}

	fmt.Printf("Artist: %s\n", artist.GetName())
	fmt.Printf("Popularity: %.0f\n", artist.GetPopularity())
	fmt.Printf("Genre: %s\n", artist.GetGenre())

	if artist.GetTopTrack() != nil && len(artist.GetTopTrack()) > 0 {
		// Spotify returns top tracks in multiple countries. We take the first
		// one as example, but we should use the country data returned by the
		// Spotify server (session.Country())
		tt := artist.GetTopTrack()[0]
		fmt.Printf("\nTop tracks (country %s):\n", tt.GetCountry())

		for _, t := range tt.GetTrack() {
			// To save bandwidth, only track IDs are returned. If you want
			// the track name, you need to fetch it.
			fmt.Printf(" => %s\n", utils.ConvertTo62(t.GetGid()))
		}
	}

	fmt.Printf("\nAlbums:\n")
	for _, ag := range artist.GetAlbumGroup() {
		for _, a := range ag.GetAlbum() {
			fmt.Printf(" => %s\n", utils.ConvertTo62(a.GetGid()))
		}
	}

}

func funcAlbum(session *core.Session, albumID string) {
	album, err := session.Mercury().GetAlbum(utils.Base62ToHex(albumID))
	if err != nil {
		fmt.Println("Error loading album:", err)
		return
	}

	fmt.Printf("Album: %s\n", album.GetName())
	fmt.Printf("Popularity: %.0f\n", album.GetPopularity())
	fmt.Printf("Genre: %s\n", album.GetGenre())
	fmt.Printf("Date: %d-%d-%d\n", album.GetDate().GetYear(), album.GetDate().GetMonth(), album.GetDate().GetDay())
	fmt.Printf("Label: %s\n", album.GetLabel())
	fmt.Printf("Type: %s\n", album.GetTyp())

	fmt.Printf("Artists: ")
	for _, artist := range album.GetArtist() {
		fmt.Printf("%s ", utils.ConvertTo62(artist.GetGid()))
	}
	fmt.Printf("\n")

	for _, disc := range album.GetDisc() {
		fmt.Printf("\nDisc %d (%s): \n", disc.GetNumber(), disc.GetName())

		for _, track := range disc.GetTrack() {
			fmt.Printf(" => %s\n", utils.ConvertTo62(track.GetGid()))
		}
	}

}

func funcPlaylists(session *core.Session) {
	fmt.Println("Listing playlists")

	playlist, err := session.Mercury().GetRootPlaylist(session.Username)

	if err != nil || playlist.Contents == nil {
		fmt.Println("Error getting root list: ", err)
		return
	}

	items := playlist.Contents.Items
	for i := 0; i < len(items); i++ {
		id := strings.TrimPrefix(items[i].GetUri(), "spotify:")
		id = strings.Replace(id, ":", "/", -1)
		list, _ := session.Mercury().GetPlaylist(id)
		fmt.Println(list.Attributes.GetName(), id)

		if list.Contents != nil {
			for j := 0; j < len(list.Contents.Items); j++ {
				item := list.Contents.Items[j]
				fmt.Println(" ==> ", *item.Uri)
			}
		}
	}
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

	asset, err := session.Downloader().PinAsset(trackID)
	if err != nil {
		fmt.Printf("Error while loading track: %s\n", err)
		return
	}
	r, err := asset.NewAssetReader()
	if err != nil {
		fmt.Printf("NewAssetReader: %s\n", err)
		return
	}
	defer r.Close()

	buffer, err := ioutil.ReadAll(r)
	if err != nil {
		fmt.Printf("Error while reading file: %s\n", err)
		return
	}

	err = ioutil.WriteFile(asset.Label(), buffer, os.ModePerm)
	if err != nil {
		fmt.Printf("Error while writing file: %s\n", err)
		return
	}
}
