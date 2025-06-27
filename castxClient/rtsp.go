package castxClient

import "github.com/bluenviron/gortsplib/v4"

func startRtsp() {
	server := &gortsplib.Server{
		Handler: &myHandler{},
	}
	server.Start()
}

type myHandler struct {
	streams map[string]*gortsplib.ServerStream
}

/*
func (h *myHandler) OnSetup(ctx *gortsplib.ServerHandlerOnSetupCtx) {

	&ServerStream{Server: server}

	ServerStream.Initialize().
	stream := gortsplib.NewServerStream(rtsp.Tracks{
		{Codec: &gortsplib.CodecH264},
		{Codec: &gortsplib.CodecOpus}, // 音频
	})
	h.streams[ctx.Path] = stream
}

// 接收 WebRTC 数据并转发
func relayWebRTCtoRTSP() {
	for pkt := range videoChan {
		if stream, ok := streams[streamID]; ok {
			stream.WriteRTP(0, pkt) // 0=视频轨道
		}
	}
}*/
