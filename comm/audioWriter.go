package comm

import (
	"errors"
	"io"
	"sync"
	"time"

	"github.com/pion/webrtc/v4/pkg/media/oggreader"
)

type AudioWriter struct {
	buffer       []byte
	mu           sync.Mutex
	webrtcServer *WebrtcServer
}

func NewAudioWriter(webrtcServer *WebrtcServer) *AudioWriter {
	a := &AudioWriter{}
	a.webrtcServer = webrtcServer
	return a
}

func (a *AudioWriter) Strem(readStrem io.Reader) {
	// Open on oggfile in non-checksum mode.
	ogg, _, oggErr := oggreader.NewWith(readStrem)
	if oggErr != nil {
		panic(oggErr)
	}
	var lastGranule uint64
	for {
		pageData, pageHeader, oggErr := ogg.ParseNextPage()
		if errors.Is(oggErr, io.EOF) {
			break
		}

		sampleCount := float64(pageHeader.GranulePosition - lastGranule)
		lastGranule = pageHeader.GranulePosition
		sampleDuration := time.Duration((sampleCount/48000)*1000) * time.Millisecond
		a.webrtcServer.SendAudioNew(pageData, sampleDuration)
	}
}
