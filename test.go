package main

import (
	"fmt"
	"math/rand/v2"
	"time"

	"github.com/gopxl/beep/v2"
	"github.com/gopxl/beep/v2/speaker"
)

func main() {
	sr := beep.SampleRate(44100)
	speaker.Init(sr, sr.N(time.Second/10))
	speaker.Play(getNoise())
	select {}
}

var start = time.Now()

func getNoise() beep.Streamer {
	return beep.StreamerFunc(func(samples [][2]float64) (n int, ok bool) {

		// 计算执行间隔
		elapsed := time.Since(start)
		fmt.Printf("samples:%d 执行耗时: %v\n", len(samples), elapsed)
		start = time.Now()
		for i := range samples {
			samples[i][0] = rand.Float64()*2 - 1
			samples[i][1] = rand.Float64()*2 - 1
		}
		return len(samples), true
	})
}

type Noise struct {
	now *time.Time
}

func (no Noise) Stream(samples [][2]float64) (n int, ok bool) {
	if no.now != nil {
		elapsed := time.Since(*no.now)
		fmt.Printf("samples:%d 执行耗时: %v\n", len(samples), elapsed)
	}
	ddd := time.Now()
	no.now = &ddd
	for i := range samples {
		samples[i][0] = rand.Float64()*2 - 1
		samples[i][1] = rand.Float64()*2 - 1
	}
	return len(samples), true
}

func (no Noise) Err() error {
	return nil
}
