package mercury

import "testing"

/*
func setupTestController(stream connection.PacketStream) *spirc.Controller {
	s := &core.Session{
		stream:   stream,
		deviceId: "testDevice",
	}
	s.mercury = CreateMercury(s)
	return setupController(s, "fakeUser", []byte{})
}

func TestMultiPart(t *testing.T) {
	stream := &fakeStream{
		recvPackets: make(chan shanPacket, 5),
		sendPackets: make(chan shanPacket, 5),
	}

	controller := setupTestController(stream)

	subHeader := &Spotify.Header{
		Uri: proto.String("hm://searchview/km/v2/search/Future"),
	}
	subHeaderData, _ := proto.Marshal(subHeader)

	header := &Spotify.Header{
		Uri:         proto.String("hm://searchview/km/v2/search/Future"),
		ContentType: proto.String("application/json; charset=UTF-8"),
		StatusCode:  proto.Int32(200),
	}
	body := []byte("{searchResults: {tracks: [], albums: [], tracks: []}}")

	headerData, _ := proto.Marshal(header)
	seq := []byte{0, 0, 0, 1}

	p0, _ := encodeMercuryHead([]byte{0, 0, 0, 0}, 1, 1)
	binary.Write(p0, binary.BigEndian, uint16(len(subHeaderData)))
	p0.Write(subHeaderData)

	p1, _ := encodeMercuryHead(seq, 1, 0)
	binary.Write(p1, binary.BigEndian, uint16(len(headerData)))
	p1.Write(headerData)

	p2, _ := encodeMercuryHead(seq, 1, 1)
	binary.Write(p2, binary.BigEndian, uint16(len(body)))
	p2.Write(body)

	didRecieveCallback := false
	controller.session.mercurySendRequest(Request{
		Method:  "SEND",
		Uri:     "hm://searchview/km/v2/search/Future",
		Payload: [][]byte{},
	}, func(res Response) {
		didRecieveCallback = true
		if string(res.Payload[0]) != string(body) {
			t.Errorf("bad body received")
		}
	})

	stream.recvPackets <- shanPacket{cmd: 0xb2, buf: p0.Bytes()}
	stream.recvPackets <- shanPacket{cmd: 0xb2, buf: p1.Bytes()}
	stream.recvPackets <- shanPacket{cmd: 0xb2, buf: p2.Bytes()}

	controller.session.poll()
	controller.session.poll()
	controller.session.poll()

	if !didRecieveCallback {
		t.Errorf("never received callback")
	}

}

*/


func TestSuggest(t *testing.T) {
	body := `{"sections":[{"type":"top-results","items":[{"name":"Heartbeats","uri":"spotify:album:19WDf08G2WEC79RE94n5Ze","artists":[{"name":"Various Artists","uri":"spotify:artist:0LyfQWJT6nXafLPZqxe9Of"}],"image":"https://d3rt1990lpmkn.cloudfront.net/120/e73927144181509d38d1e933fa5a339659fcd394","log":{"top_hit":"albums","origin":"suggest"}}]},{"type":"track-results","items":[{"name":"Heartbeats","uri":"spotify:track:2YacpExEbX9tF8IbFlFOo4","album":{"name":"Deep Cuts","uri":"spotify:album:1iqMDM4Io1tnDDl58NGeVJ"},"artists":[{"name":"The Knife","uri":"spotify:artist:7eQZTqEMozBcuSubfu52i4"}],"image":"https://d3rt1990lpmkn.cloudfront.net/120/3d06fa074f91e222d2eb6a68c27d374c1845f753","log":{"top_hit":"albums","origin":"suggest"}},{"name":"Heartbeats","uri":"spotify:track:5YqpHuXpFjDVZ7tY1ClFll","album":{"name":"Veneer","uri":"spotify:album:2e0BYdQ7VJlzSNHafdmfrl"},"artists":[{"name":"José González","uri":"spotify:artist:6xrCU6zdcSTsG2hLrojpmI"}],"image":"https://d3rt1990lpmkn.cloudfront.net/120/8ee5e7276f8aec109c37434b1e0e36e0d10479e5","log":{"top_hit":"albums","origin":"suggest"}},{"name":"Heartbeats","uri":"spotify:track:0yfWSUKUNA13Xy1zuLE3f4","album":{"name":"Heartbeats","uri":"spotify:album:7i1iWK8e4opfXm4OOV3O9I"},"artists":[{"name":"Daniela Andrade","uri":"spotify:artist:0WfaItAbs4vlgIA1cuqGtJ"},{"name":"Dabin","uri":"spotify:artist:7lZauDnRoAC3kmaYae2opv"}],"image":"https://d3rt1990lpmkn.cloudfront.net/120/b0129fc373bbeebb6c3200870ee55293496dc092","log":{"top_hit":"albums","origin":"suggest"}}]},{"type":"artist-results","items":[{"name":"The Heartbeats","uri":"spotify:artist:12InvBNZTKboiU2xT663oK","image":"https://d3rt1990lpmkn.cloudfront.net/120/686252a8a11f18c39cd10c55a25bc18ffe24d3b2","log":{"top_hit":"albums","origin":"suggest"}},{"name":"The 5 Heartbeats","uri":"spotify:artist:08XJ8En6r470i5QJV4vzrG","log":{"top_hit":"albums","origin":"suggest"}},{"name":"HeartBeats Pro","uri":"spotify:artist:4gILz9pWk2kOHM3vgn8tZi","image":"https://d3rt1990lpmkn.cloudfront.net/120/0e2fdcaf3bdab06b7a9724303a6a61b44c341f67","log":{"top_hit":"albums","origin":"suggest"}}]},{"type":"album-results","items":[{"name":"Heartbeats","uri":"spotify:album:19WDf08G2WEC79RE94n5Ze","artists":[{"name":"Various Artists","uri":"spotify:artist:0LyfQWJT6nXafLPZqxe9Of"}],"image":"https://d3rt1990lpmkn.cloudfront.net/120/e73927144181509d38d1e933fa5a339659fcd394","log":{"top_hit":"albums","origin":"suggest"}},{"name":"Heartbeats - EP","uri":"spotify:album:3cM7bhwxxzbhhTfrCOxRbH","artists":[{"name":"Avec","uri":"spotify:artist:6N8vbhxZ0CYJHd8WGJ9Snf"}],"image":"https://d3rt1990lpmkn.cloudfront.net/120/3564ebeb04d5a920610d317cba29161d536b0402","log":{"top_hit":"albums","origin":"suggest"}},{"name":"Heartbeats","uri":"spotify:album:2sDfdp7RQQZnMoM4hWbrsh","artists":[{"name":"Mirror Kisses","uri":"spotify:artist:3QsA8x5kNe6XkKT6uwaaio"}],"image":"https://d3rt1990lpmkn.cloudfront.net/120/f9ddabe80ab560f9a911a3185bf1c98831c4dbdf","log":{"top_hit":"albums","origin":"suggest"}}]},{"type":"playlist-results","items":[{"name":"The Knife - Heartbeats","uri":"spotify:user:1228858172:playlist:4vEyU9bTcuALukJMs8MAG3","followers":1003,"image":"https://d3rt1990lpmkn.cloudfront.net/120/3d06fa074f91e222d2eb6a68c27d374c1845f753b938b0685042d686315a949ee153593709e495e52dd032b0e78dd3722df270a797ab18ad533a83dab80655dfb5e10a67486f0189f9bc2d1dd3b0cd5e","log":{"top_hit":"albums","origin":"suggest"},"owner":{"name":"Al Gordon","uri":"spotify:user:1228858172"}},{"name":"José González — Heartbeats","uri":"spotify:user:12185260184:playlist:1LAvLvk08XvB0OZeFABp8d","followers":676,"image":"https://d3rt1990lpmkn.cloudfront.net/120/8ee5e7276f8aec109c37434b1e0e36e0d10479e572d78924e506cb6fd12ac77c5f4a0e3fa1de6880422a60d8628dd6b47cb16206be41599c55443796e491217b123cfbf84d169352d89cd9ba15f08d6b","log":{"top_hit":"albums","origin":"suggest"},"owner":{"name":"Brice Parker","uri":"spotify:user:12185260184"}}]},{"type":"profile-results","items":[{"name":"heartbeatsss","uri":"spotify:user:heartbeatsss","followers":48,"log":{"top_hit":"albums","origin":"suggest"}},{"name":"#heartbeat","uri":"spotify:user:%23heartbeat","followers":99,"log":{"misspelling":true,"top_hit":"albums","origin":"suggest"}},{"name":"Alfredo Simon Romeo Caceres","uri":"spotify:user:heartbeat1997","followers":47,"image":"https://scontent.xx.fbcdn.net/v/t1.0-1/p200x200/12065742_10209381269912728_7979961412840376089_n.jpg?oh=54007c8311bcf40f8978886f785609e4&oe=5808EC03","log":{"misspelling":true,"top_hit":"albums","origin":"suggest"}}]}]}`
	result, _ := ParseSuggest([]byte(body))
	if result.TopHits[0].Uri != "spotify:album:19WDf08G2WEC79RE94n5Ze" {
		t.Error("bad uri for top hit")
	}
}
