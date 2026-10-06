package ppmviewer

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"runtime"
	"strconv"
	"unsafe"

	"github.com/veandco/go-sdl2/sdl"
)

func Run(path string) error {
	// SDL/OpenGL-related operations should stay on the main OS thread.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	// ------------------------------------------------------------
	// Read PPM
	// ------------------------------------------------------------

	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open PPM: %w", err)
	}
	defer file.Close()

	reader := bufio.NewReaderSize(file, 64*1024)

	magic, err := readToken(reader)
	if err != nil {
		return fmt.Errorf("read PPM magic: %w", err)
	}

	if magic != "P6" {
		return fmt.Errorf("unsupported PPM format %q: expected P6", magic)
	}

	widthToken, err := readToken(reader)
	if err != nil {
		return fmt.Errorf("read width: %w", err)
	}

	heightToken, err := readToken(reader)
	if err != nil {
		return fmt.Errorf("read height: %w", err)
	}

	maxColorToken, err := readToken(reader)
	if err != nil {
		return fmt.Errorf("read max color value: %w", err)
	}

	width, err := strconv.Atoi(widthToken)
	if err != nil || width <= 0 {
		return fmt.Errorf("invalid width: %q", widthToken)
	}

	height, err := strconv.Atoi(heightToken)
	if err != nil || height <= 0 {
		return fmt.Errorf("invalid height: %q", heightToken)
	}

	maxColor, err := strconv.Atoi(maxColorToken)
	if err != nil || maxColor != 255 {
		return fmt.Errorf("unsupported max color value: %q", maxColorToken)
	}

	// P6 = 3 bytes per pixel: RGB.
	pixelBytes := width * height * 3
	pixels := make([]byte, pixelBytes)

	if _, err := io.ReadFull(reader, pixels); err != nil {
		return fmt.Errorf("read pixel data: %w", err)
	}

	// ------------------------------------------------------------
	// SDL
	// ------------------------------------------------------------

	if err := sdl.Init(sdl.INIT_VIDEO); err != nil {
		return fmt.Errorf("initialize SDL: %w", err)
	}
	defer sdl.Quit()

	windowWidth := int32(width)
	windowHeight := int32(height)

	// Don't create a ridiculous initial window for huge images.
	const maxWindowDimension = 1200

	if windowWidth > maxWindowDimension || windowHeight > maxWindowDimension {
		scaleX := float64(maxWindowDimension) / float64(windowWidth)
		scaleY := float64(maxWindowDimension) / float64(windowHeight)

		scale := scaleX
		if scaleY < scale {
			scale = scaleY
		}

		windowWidth = int32(float64(windowWidth) * scale)
		windowHeight = int32(float64(windowHeight) * scale)
	}

	window, err := sdl.CreateWindow(
		"PPM Viewer",
		sdl.WINDOWPOS_CENTERED,
		sdl.WINDOWPOS_CENTERED,
		windowWidth,
		windowHeight,
		sdl.WINDOW_SHOWN|sdl.WINDOW_RESIZABLE,
	)
	if err != nil {
		return fmt.Errorf("create window: %w", err)
	}
	defer window.Destroy()

	renderer, err := sdl.CreateRenderer(
		window,
		-1,
		sdl.RENDERER_ACCELERATED|sdl.RENDERER_PRESENTVSYNC,
	)
	if err != nil {
		return fmt.Errorf("create renderer: %w", err)
	}
	defer renderer.Destroy()

	// ------------------------------------------------------------
	// Upload RGB data directly into an SDL texture.
	// ------------------------------------------------------------

	texture, err := renderer.CreateTexture(
		sdl.PIXELFORMAT_RGB24,
		sdl.TEXTUREACCESS_STATIC,
		int32(width),
		int32(height),
	)
	if err != nil {
		return fmt.Errorf("create texture: %w", err)
	}
	defer texture.Destroy()

	pitch := width * 3

	if len(pixels) == 0 {
		return fmt.Errorf("empty pixel buffer")
	}

	if err := texture.Update(
		nil,
		unsafe.Pointer(&pixels[0]),
		pitch,
	); err != nil {
		return fmt.Errorf("upload texture: %w", err)
	}

	// We no longer need the PPM file buffer after uploading it.
	// Keep this assignment here so the lifetime is explicit.
	pixels = nil

	// ------------------------------------------------------------
	// Render loop
	// ------------------------------------------------------------

	running := true

	for running {
		for event := sdl.PollEvent(); event != nil; event = sdl.PollEvent() {
			switch event := event.(type) {

			case *sdl.QuitEvent:
				running = false

			case *sdl.KeyboardEvent:
				if event.Type == sdl.KEYDOWN &&
					event.Repeat == 0 &&
					event.Keysym.Sym == sdl.K_ESCAPE {
					running = false
				}
			}
		}

		windowW, windowH := window.GetSize()

		dst := fitImage(
			width,
			height,
			int(windowW),
			int(windowH),
		)

		// Dark background.
		renderer.SetDrawColor(20, 20, 20, 255)
		renderer.Clear()

		// GPU-side texture → window.
		if err := renderer.Copy(texture, nil, &dst); err != nil {
			return fmt.Errorf("render texture: %w", err)
		}

		renderer.Present()
	}

	return nil
}

// fitImage keeps the original aspect ratio and centers the image.
func fitImage(
	imageWidth int,
	imageHeight int,
	windowWidth int,
	windowHeight int,
) sdl.Rect {
	imageAspect := float64(imageWidth) / float64(imageHeight)
	windowAspect := float64(windowWidth) / float64(windowHeight)

	var width int
	var height int

	if imageAspect > windowAspect {
		width = windowWidth
		height = int(float64(width) / imageAspect)
	} else {
		height = windowHeight
		width = int(float64(height) * imageAspect)
	}

	return sdl.Rect{
		X: int32((windowWidth - width) / 2),
		Y: int32((windowHeight - height) / 2),
		W: int32(width),
		H: int32(height),
	}
}

// readToken reads one whitespace-delimited PPM header token.
// It also handles PPM comments beginning with '#'.
func readToken(reader *bufio.Reader) (string, error) {
	var token []byte

	for {
		b, err := reader.ReadByte()
		if err != nil {
			return "", err
		}

		// Skip comments.
		if b == '#' {
			for {
				b, err = reader.ReadByte()
				if err != nil {
					return "", err
				}

				if b == '\n' {
					break
				}
			}

			continue
		}

		// Token separator.
		if b == ' ' || b == '\n' || b == '\r' || b == '\t' {
			if len(token) > 0 {
				return string(token), nil
			}

			continue
		}

		token = append(token, b)
	}
}
