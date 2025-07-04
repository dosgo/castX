// Package main contains an example.
package castxClient

import (
	"encoding/binary"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/bluenviron/gortsplib/v4"
	"github.com/bluenviron/gortsplib/v4/pkg/base"
	"github.com/bluenviron/gortsplib/v4/pkg/description"
	"github.com/bluenviron/gortsplib/v4/pkg/format"
	"github.com/pion/webrtc/v3"
)

// This example shows how to
// 1. create a RTSP server which accepts plain connections.
// 2. read from disk a MPEG-TS file which contains a H264 track.
// 3. serve the content of the file to all connected readers.

type serverHandler struct {
	server      *gortsplib.Server
	stream      *gortsplib.ServerStream
	mutex       sync.RWMutex
	tracksReady chan struct{}
}

// called when a connection is opened.
func (sh *serverHandler) OnConnOpen(_ *gortsplib.ServerHandlerOnConnOpenCtx) {
	log.Printf("conn opened")
}

// called when a connection is closed.
func (sh *serverHandler) OnConnClose(ctx *gortsplib.ServerHandlerOnConnCloseCtx) {
	log.Printf("conn closed (%v)", ctx.Error)
}

// called when a session is opened.
func (sh *serverHandler) OnSessionOpen(_ *gortsplib.ServerHandlerOnSessionOpenCtx) {
	log.Printf("session opened")
}

// called when a session is closed.
func (sh *serverHandler) OnSessionClose(_ *gortsplib.ServerHandlerOnSessionCloseCtx) {
	log.Printf("session closed")
}

// called when receiving a DESCRIBE request.
func (sh *serverHandler) OnDescribe(
	_ *gortsplib.ServerHandlerOnDescribeCtx,
) (*base.Response, *gortsplib.ServerStream, error) {
	log.Printf("DESCRIBE request")

	sh.mutex.RLock()
	defer sh.mutex.RUnlock()

	return &base.Response{
		StatusCode: base.StatusOK,
	}, sh.stream, nil
}

// called when receiving a SETUP request.
func (sh *serverHandler) OnSetup(
	_ *gortsplib.ServerHandlerOnSetupCtx,
) (*base.Response, *gortsplib.ServerStream, error) {
	log.Printf("SETUP request")

	sh.mutex.RLock()
	defer sh.mutex.RUnlock()

	return &base.Response{
		StatusCode: base.StatusOK,
	}, sh.stream, nil
}

// called when receiving a PLAY request.
func (sh *serverHandler) OnPlay(_ *gortsplib.ServerHandlerOnPlayCtx) (*base.Response, error) {
	log.Printf("PLAY request")

	return &base.Response{
		StatusCode: base.StatusOK,
	}, nil
}

func (sh *serverHandler) createRTSPStream(videoPayloadTyp uint8, audioPayloadTyp uint8, sps []byte, pps []byte) {
	if sh.stream == nil && videoPayloadTyp > 0 {
		desc := &description.Session{
			Medias: []*description.Media{{
				Type: description.MediaTypeVideo,
				Formats: []format.Format{&format.H264{
					PayloadTyp:        videoPayloadTyp,
					SPS:               sps, // 您的SPS
					PPS:               pps, // 您的PPS
					PacketizationMode: 1,
				}},
			}, {
				Type: description.MediaTypeAudio,
				Formats: []format.Format{&format.Opus{
					PayloadTyp: audioPayloadTyp,

					ChannelCount: 2,
				}},
			}},
		}
		// create a server stream
		sh.stream = gortsplib.NewServerStream(
			sh.server,
			desc,
		)
	}
}
func (h *serverHandler) Start(peerConnection *webrtc.PeerConnection) {
	h.tracksReady = make(chan struct{})
	// prevent clients from connecting to the server until the stream is properly set up
	h.mutex.Lock()

	// create the server
	h.server = &gortsplib.Server{
		Handler:           h,
		RTSPAddress:       ":8554",
		UDPRTPAddress:     ":8000",
		UDPRTCPAddress:    ":8001",
		MulticastIPRange:  "224.1.0.0/16",
		MulticastRTPPort:  8002,
		MulticastRTCPPort: 8003,
	}

	// start the server
	err := h.server.Start()
	if err != nil {
		panic(err)
	}
	defer h.server.Close()

	var videoPayloadTyp = uint8(0)
	var audioPayloadTyp = uint8(0)
	var sps = []byte{}
	var pps = []byte{}
	peerConnection.OnTrack(func(track *webrtc.TrackRemote, receiver *webrtc.RTPReceiver) {

		fmt.Printf("接收到 %s 轨道\n", track.Kind())
		// 创建内存缓冲区
		fmt.Printf("开始接收轨道: %s\n", track.Codec().MimeType)
		if track.Codec().MimeType == "video/H264" {
			videoPayloadTyp = uint8(track.Codec().PayloadType)
			go func() {
				for {
					rtpPacket, _, err := track.ReadRTP()

					if err != nil {
						break
					}

					naluType := rtpPacket.Payload[0] & 0x1F
					if naluType == 7 || naluType == 8 {
						fmt.Printf("naluType:%d\r\n", naluType)
					}
					if naluType == 24 {
						nalSize := binary.BigEndian.Uint16(rtpPacket.Payload[1:])
						subNalType := rtpPacket.Payload[3] & 0x1F
						if subNalType == 7 {
							sps = append([]byte{}, rtpPacket.Payload[3:nalSize+3]...)
							pps = append([]byte{}, rtpPacket.Payload[3+nalSize+2:]...)

							fmt.Printf("sps:%+v\r\n", sps)
							fmt.Printf("pps:%+v\r\n", pps)
							if h.tracksReady != nil {
								close(h.tracksReady)
								h.tracksReady = nil
							}
							//continue

						}
					}

					if h.stream != nil {
						//fmt.Printf("rtpPacket.Timestamp:%d\r\n", rtpPacket.Timestamp)
						//fmt.Printf("rtpPacket.SequenceNumber:%d\r\n", rtpPacket.Header.SequenceNumber)
						h.stream.WritePacketRTP(h.stream.Description().Medias[0], rtpPacket)
					}
				}
			}()
		}
		if track.Codec().MimeType == "audio/opus" {
			fmt.Printf("audio\r\n")
			audioPayloadTyp = uint8(track.Codec().PayloadType)
			go func() {
				for {
					rtpPacket, _, err := track.ReadRTP()
					if err != nil {
						break
					}
					if h.stream != nil {
						h.stream.WritePacketRTP(h.stream.Description().Medias[1], rtpPacket)
					}
				}
			}()
		}
	})

	select {
	case <-h.tracksReady:
		h.createRTSPStream(videoPayloadTyp, audioPayloadTyp, sps, pps)
	case <-time.After(20 * time.Second):
		h.createRTSPStream(videoPayloadTyp, audioPayloadTyp, sps, pps)
	}
	h.tracksReady = nil
	// allow clients to connect
	h.mutex.Unlock()

	// wait until a fatal error
	log.Printf("server is ready on %s", h.server.RTSPAddress)
	panic(h.server.Wait())
}
