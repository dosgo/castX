package main

import (
	"context"
	"fmt"
	"os"
	"path"

	"codeberg.org/gruf/go-ffmpreg/ffmpreg"
	"codeberg.org/gruf/go-ffmpreg/wasm"
	"github.com/tetratelabs/wazero"
)

var dir string

func init() {
	var err error

	// Get current test dir.
	dir, err = os.Getwd()
	if err != nil {
		panic(err)
	}

	// Determine base repo dir.
	dir = path.Join(dir, "../")

	fmt.Println("initializing ...")
	ffmpreg.Initialize()
	fmt.Println("success.")
}

func main() {
	output := "pipe:1"
	test([]string{
		"-f", "h264",
		"-i", "pipe:0",
		"-filter_complex", "[0]scale=864:1920[s0]",
		"-map", "[s0]",
		"-f", "rawvideo",
		"-avioflags", "direct",
		"-flags", "low_delay",
		"-pix_fmt", "yuv420p",
		output,
	}, output, "")
}

func test(args []string, output, check string) {
	defer os.Remove(output)
	// Create new test context.
	ctx := context.Background()
	ctx, cncl := context.WithCancel(ctx)
	defer cncl()

	// Run ffmpeg with given arguments.
	rc, err := ffmpreg.Ffmpeg(ctx, wasm.Args{
		Stdout: os.Stdout,
		Stderr: os.Stderr,
		Args:   args,
		Config: func(cfg wazero.ModuleConfig) wazero.ModuleConfig {
			fscfg := wazero.NewFSConfig()
			fscfg = fscfg.WithDirMount(dir, dir)
			return cfg.WithFSConfig(fscfg)
		},
	})
	if err != nil {
		fmt.Printf("err:%+v\r\n", err)
	} else if rc != 0 {
		fmt.Printf("non-zero exit code:%+v\r\n", rc)
	}

}
