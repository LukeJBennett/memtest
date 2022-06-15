package main

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"log"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/exec"
	"runtime"
	"runtime/debug"
	"strings"
	"time"
	"unsafe"

	"github.com/veandco/go-sdl2/img"
	"github.com/veandco/go-sdl2/sdl"
)

func main() {
	//start web server for pprof
	go func() {
		log.Println(http.ListenAndServe("localhost:8000", nil))
	}()

	//initialize a window
	sdl.Init(sdl.INIT_VIDEO)
	img.Init(img.INIT_PNG)
	Window, _ := sdl.CreateWindow("ICO Memory Test", 75, 10, 400, 300, sdl.WINDOW_SHOWN)
	Renderer, _ := sdl.CreateRenderer(Window, -1, sdl.RENDERER_SOFTWARE)
	go func() {
		for event := sdl.PollEvent(); true; event = sdl.PollEvent() {
			if event != nil && event.GetType() == sdl.QUIT {
				os.Exit(0)
			}
			time.Sleep(time.Second / 10)
		}
	}()

	Renderer.Clear()

	//create byte slice containing a png image
	dst := image.NewRGBA(image.Rect(0, 0, 300, 200))
	dst.Set(10, 10, color.RGBA{255, 255, 255, 255})
	pngbuff := new(bytes.Buffer)
	(&png.Encoder{CompressionLevel: png.BestSpeed}).Encode(pngbuff, dst)
	rowImg := pngbuff.Bytes()

	//this is only used for the less leaky method
	//storedSurface := MakeStorableSurface(rowImg)

	lastTime := time.Now()
	for i := 0; i < 1000000; i++ {
		if time.Since(lastTime) > time.Second/2 {
			lastTime = time.Now()
			fmt.Println(checkResourceUsage())
		}

		//put png into RWops buffer, convert to surface, then to a texture, then put the texture on the screen

		//this is the very leaky method
		buf, _ := sdl.RWFromMem(rowImg)
		defer buf.Close()
		graphic, _ := img.LoadTypedRW(buf, false, "PNG")
		//end very leaky method

		//less leaky method
		//graphic := GetStoredSurface(storedSurface)
		//end less leaky method

		defer graphic.Free()

		texture, _ := Renderer.CreateTextureFromSurface(graphic)
		defer texture.Destroy()

		Renderer.Copy(texture, nil, &sdl.Rect{X: 0, Y: 0, W: 400, H: 300})
		Renderer.Present()
		debug.FreeOSMemory()

	}

}

/////start of functions for workaround with static global array

type storableSurface struct {
	Pixels []byte
	Width  int32
	Height int32
	Depth  int
	Pitch  int
	Rmask  uint32
	Gmask  uint32
	Bmask  uint32
	Amask  uint32
}

var imgBuf [2500000]byte ///3 bytes per pixel, 1024x768 max size

func GetStoredSurface(in storableSurface) (out *sdl.Surface) {

	copy(imgBuf[:], in.Pixels)
	out, _ = sdl.CreateRGBSurfaceFrom(
		unsafe.Pointer(&imgBuf),
		in.Width,
		in.Height,
		in.Depth,
		in.Pitch,
		in.Rmask,
		in.Gmask,
		in.Bmask,
		in.Amask,
	)
	return
}

func MakeStorableSurface(rowImg []byte) (out storableSurface) {
	buf, _ := sdl.RWFromMem(rowImg)
	in, _ := img.LoadTypedRW(buf, false, "PNG")

	pixels := in.Pixels()
	out = storableSurface{
		Pixels: pixels,
		Width:  in.W,
		Height: in.H,
		Depth:  int(in.Format.BitsPerPixel),
		Pitch:  int(in.Pitch),
		Rmask:  in.Format.Rmask,
		Gmask:  in.Format.Gmask,
		Bmask:  in.Format.Bmask,
		Amask:  in.Format.Amask,
	}
	return
}

func checkResourceUsage() string {
	executableName := os.Args[0]
	var m runtime.MemStats
	const meg = 1024 * 1024
	runtime.ReadMemStats(&m)
	returnString := fmt.Sprintf("sys: %4d M, alloc: %4d M, idle: %4d M, rel: %4d M, inuse: %4d M     ",
		m.HeapSys/meg, m.HeapAlloc/meg, m.HeapIdle/meg, m.HeapReleased/meg, m.HeapInuse/meg)

	//linux ps based
	cmd := exec.Command("ps", "ux")
	result, err := cmd.Output()
	if err != nil {
		returnString += "ps ux fail "
	} else {
		lines := strings.Split(string(result), "\n")
		for _, line := range lines {
			if strings.Contains(line, executableName) {
				columns := strings.Fields(line)
				if len(columns) > 5 {
					psmem := columns[5]
					if len(psmem) > 3 {
						psmem = psmem[:len(psmem)-3] + " M"
					} else {
						psmem += " K"
					}
					returnString += "ps: " + psmem
					break
				} else {
					returnString += "ps ux bad"
				}
			}
		}
	}

	cmd = exec.Command("ps", "-eLf")
	result, err = cmd.Output()
	if err != nil {
		returnString += "ps -eLF fail"
	} else {
		threadCount := strings.Count(string(result), executableName)
		returnString += fmt.Sprintf("   th: %d", threadCount)
	}

	return returnString
}
