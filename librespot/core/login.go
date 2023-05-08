package core

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"log"

	"github.com/golang/protobuf/proto"

	"github.com/librespot-org/librespot-golang/Spotify"
	"github.com/librespot-org/librespot-golang/librespot/core/connection"
	"github.com/librespot-org/librespot-golang/librespot/utils"
)

var Version = "master"
var BuildID = "dev"

// Login to Spotify using username and password
func (sess *Session) Login(username string, password string) error {
	loginPacket := sess.makeLoginBlobPacket(
		username,
		[]byte(password),
		Spotify.AuthenticationType_AUTHENTICATION_USER_PASS.Enum(),
	)
	return sess.doLogin(loginPacket, username)
}

// Login to Spotify using an existing authData blob
func (sess *Session) LoginSaved(username string, authData []byte) error {
	packet := sess.makeLoginBlobPacket(
		username,
		authData,
		Spotify.AuthenticationType_AUTHENTICATION_STORED_SPOTIFY_CREDENTIALS.Enum(),
	)
	return sess.doLogin(packet, username)
}

/*
// Registers librespot as a Spotify Connect device via mdns. When user connects, logs on to Spotify and saves
// credentials in file at cacheBlobPath. Once saved, the blob credentials allow the program to connect to other
// Spotify Connect devices and control them.
func (sess *Session) LoginDiscovery(cacheBlobPath string) (*Session, error) {
	disc := discovery.LoginFromConnect(cacheBlobPath, sess.opts.DeviceID, sess.opts.DeviceName)
	return sessionFromDiscovery(disc)
}

// Login using an authentication blob through Spotify Connect discovery system, reading an existing blob data. To read
// from a file, see LoginDiscoveryBlobFile.
func (sess *Session) LoginDiscoveryBlob(username string, blob string, deviceName string) (*Session, error) {
	deviceId := utils.GenerateDeviceId(deviceName)
	disc := discovery.CreateFromBlob(utils.BlobInfo{
		Username:    username,
		DecodedBlob: blob,
	}, "", deviceId, deviceName)
	return sessionFromDiscovery(disc)
}

// Login from credentials at cacheBlobPath previously saved by LoginDiscovery. Similar to LoginDiscoveryBlob, except
// it reads it directly from a file.
func (sess *Session) LoginDiscoveryBlobFile(cacheBlobPath, deviceName string) (*Session, error) {
	deviceId := utils.GenerateDeviceId(deviceName)
	disc := discovery.CreateFromFile(cacheBlobPath, deviceId, deviceName)
	return sessionFromDiscovery(disc)
}
*/

// Login to Spotify using the OAuth method
func LoginOAuth(clientId, clientSecret, callbackURL string) (string, error) {
	if callbackURL == "" {
		callbackURL = "http://localhost:8888/callback"
	}
	token, err := getOAuthToken(clientId, clientSecret, callbackURL)
	if err != nil {
		return "", err
	}
	tok := token.AccessToken
	fmt.Println("Got oauth token:\n", tok)
	return tok, nil
}

func (sess *Session) LoginOAuthToken(accessToken string) error {
	packet := sess.makeLoginBlobPacket(
		"",
		[]byte(accessToken),
		Spotify.AuthenticationType_AUTHENTICATION_SPOTIFY_TOKEN.Enum(),
	)
	return sess.doLogin(packet, "")
}

func (s *Session) doLogin(packet []byte, username string) error {
	err := s.stream.SendPacket(connection.PacketLogin, packet)
	if err != nil {
		log.Fatal("bad shannon write", err)
	}

	// Pll once for authentication response
	welcome, err := s.handleLogin()
	if err != nil {
		return err
	}

	// Store the few interesting values
	s.Username = welcome.GetCanonicalUsername()
	if s.Username == "" {
		// Spotify might not return a canonical username, so reuse the blob's one instead
		s.Username = s.discovery.LoginBlob().Username
	}
	s.ReusableAuthBlob = welcome.GetReusableAuthCredentials()

	// Poll for acknowledge before loading - needed for gopherjs
	// s.poll()
	go s.runPollLoop() // TODO: add context.Context exit!

	return nil
}

func (s *Session) handleLogin() (*Spotify.APWelcome, error) {
	cmd, data, err := s.stream.RecvPacket()
	if err != nil {
		return nil, fmt.Errorf("authentication failed: %v", err)
	}

	if cmd == connection.PacketAuthFailure {
		errCode := Spotify.ErrorCode(data[1])
		return nil, fmt.Errorf("authentication failed: %v", errCode)
	} else if cmd == connection.PacketAPWelcome {
		welcome := &Spotify.APWelcome{}
		err := proto.Unmarshal(data, welcome)
		if err != nil {
			return nil, fmt.Errorf("authentication failed: %v", err)
		}
		return welcome, nil
	} else {
		return nil, fmt.Errorf("authentication failed: unexpected cmd %v", cmd)
	}
}

func (s *Session) getLoginBlobPacket(blob utils.BlobInfo) ([]byte, error) {
	data, _ := base64.StdEncoding.DecodeString(blob.DecodedBlob)
	buffer := bytes.NewBuffer(data)
	if _, err := buffer.ReadByte(); err != nil {
		return nil, fmt.Errorf("could not read byte: %+v", err)
	}
	_, err := readBytes(buffer)
	if err != nil {
		return nil, fmt.Errorf("could not read bytes: %+v", err)
	}
	if _, err := buffer.ReadByte(); err != nil {
		return nil, fmt.Errorf("could not read byte: %+v", err)
	}
	authNum := readInt(buffer)
	authType := Spotify.AuthenticationType(authNum)
	if _, err := buffer.ReadByte(); err != nil {
		return nil, fmt.Errorf("could not read byte: %+v", err)
	}
	authData, err := readBytes(buffer)
	if err != nil {
		return nil, fmt.Errorf("could not read bytes: %+v", err)
	}
	return s.makeLoginBlobPacket(blob.Username, authData, &authType), nil
}



func (s *Session) makeLoginBlobPacket(
	username string,
	authData []byte,
	authType *Spotify.AuthenticationType,
) []byte{
	versionString := "librespot-golang_" + Version + "_" + BuildID
	packet := &Spotify.ClientResponseEncrypted{
		LoginCredentials: &Spotify.LoginCredentials{
			Username: proto.String(username),
			Typ:      authType,
			AuthData: authData,
		},
		SystemInfo: &Spotify.SystemInfo{
			CpuFamily:               Spotify.CpuFamily_CPU_UNKNOWN.Enum(),
			Os:                      Spotify.Os_OS_UNKNOWN.Enum(),
			SystemInformationString: proto.String("librespot-golang"),
			DeviceId:                proto.String(s.Opts.DeviceID),
		},
		VersionString: proto.String(versionString),
	}
	buf, _ := proto.Marshal(packet)
	return buf
}
