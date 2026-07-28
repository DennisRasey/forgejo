// Copyright 2016 The Gogs Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package avatar

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"forgejo.org/modules/log"
	"forgejo.org/modules/proxy"
	"forgejo.org/modules/setting"
	"forgejo.org/modules/test"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_RandomImageSize(t *testing.T) {
	_, err := RandomImageSize(0, []byte("gitea@local"))
	require.Error(t, err)

	_, err = RandomImageSize(64, []byte("gitea@local"))
	require.NoError(t, err)
}

func Test_RandomImage(t *testing.T) {
	_, err := RandomImage([]byte("gitea@local"))
	require.NoError(t, err)
}

func Test_ProcessAvatarPNG(t *testing.T) {
	defer test.MockVariableValue(&setting.Avatar.MaxWidth, 4096)()
	defer test.MockVariableValue(&setting.Avatar.MaxHeight, 4096)()

	data, err := os.ReadFile("testdata/avatar.png")
	require.NoError(t, err)

	_, img, err := processAvatarImage(data, 262144)
	require.NoError(t, err)
	require.Nil(t, img)
}

func Test_ProcessAvatarJPEG(t *testing.T) {
	defer test.MockVariableValue(&setting.Avatar.MaxWidth, 4096)()
	defer test.MockVariableValue(&setting.Avatar.MaxHeight, 4096)()

	data, err := os.ReadFile("testdata/avatar.jpeg")
	require.NoError(t, err)

	_, img, err := processAvatarImage(data, 262144)
	require.NoError(t, err)
	require.Nil(t, img)
}

func Test_ProcessAvatarGIF(t *testing.T) {
	defer test.MockVariableValue(&setting.Avatar.MaxWidth, 4096)()
	defer test.MockVariableValue(&setting.Avatar.MaxHeight, 4096)()

	data, err := os.ReadFile("testdata/avatar.gif")
	require.NoError(t, err)

	_, img, err := processAvatarImage(data, 262144)
	require.NoError(t, err)
	require.Nil(t, img)
}

func Test_ProcessAvatarInvalidData(t *testing.T) {
	defer test.MockVariableValue(&setting.Avatar.MaxWidth, 5)()
	defer test.MockVariableValue(&setting.Avatar.MaxHeight, 5)()

	_, _, err := processAvatarImage([]byte{}, 12800)
	assert.EqualError(t, err, "image.DecodeConfig: image: unknown format")
}

func Test_ProcessAvatarInvalidImageSize(t *testing.T) {
	defer test.MockVariableValue(&setting.Avatar.MaxWidth, 5)()
	defer test.MockVariableValue(&setting.Avatar.MaxHeight, 5)()

	data, err := os.ReadFile("testdata/avatar.png")
	require.NoError(t, err)

	_, _, err = processAvatarImage(data, 12800)
	assert.EqualError(t, err, "image width is too large: 10 > 5")
}

func Test_ProcessAvatarImage(t *testing.T) {
	defer test.MockVariableValue(&setting.Avatar.MaxWidth, 4096)()
	defer test.MockVariableValue(&setting.Avatar.MaxHeight, 4096)()
	scaledSize := DefaultAvatarSize * setting.Avatar.RenderedSizeFactor

	newImgData := func(size int, optHeight ...int) []byte {
		width := size
		height := size
		if len(optHeight) == 1 {
			height = optHeight[0]
		}
		img := image.NewRGBA(image.Rect(0, 0, width, height))
		bs := bytes.Buffer{}
		err := png.Encode(&bs, img)
		require.NoError(t, err)
		return bs.Bytes()
	}

	// if origin image canvas is too large, crop and resize it
	origin := newImgData(500, 600)
	result, img, err := processAvatarImage(origin, 0)
	require.NoError(t, err)
	assert.NotEqual(t, origin, result)
	decoded, err := png.Decode(bytes.NewReader(result))
	require.NoError(t, err)
	assert.Equal(t, scaledSize, decoded.Bounds().Max.X)
	assert.Equal(t, scaledSize, decoded.Bounds().Max.Y)
	assert.Equal(t, scaledSize, img.Bounds().Max.X)
	assert.Equal(t, scaledSize, img.Bounds().Max.Y)

	// if origin image is smaller than the default size, use the origin image
	origin = newImgData(1)
	result, _, err = processAvatarImage(origin, 0)
	require.NoError(t, err)
	assert.Equal(t, origin, result)

	// use the origin image if the origin is smaller
	origin = newImgData(scaledSize + 100)
	result, _, err = processAvatarImage(origin, 0)
	require.NoError(t, err)
	assert.Less(t, len(result), len(origin))

	// still use the origin image if the origin doesn't exceed the max-origin-size
	origin = newImgData(scaledSize + 100)
	result, img, err = processAvatarImage(origin, 262144)
	require.NoError(t, err)
	require.Nil(t, img)
	assert.Equal(t, origin, result)

	// allow to use known image format (eg: webp) if it is small enough
	origin, err = os.ReadFile("testdata/animated.webp")
	require.NoError(t, err)
	result, img, err = processAvatarImage(origin, 262144)
	require.NoError(t, err)
	require.Nil(t, img)
	assert.Equal(t, origin, result)

	// do not support unknown image formats, eg: SVG may contain embedded JS
	origin = []byte("<svg></svg>")
	_, _, err = processAvatarImage(origin, 262144)
	require.ErrorContains(t, err, "image: unknown format")

	// make sure the canvas size limit works
	setting.Avatar.MaxWidth = 5
	setting.Avatar.MaxHeight = 5
	origin = newImgData(10)
	_, _, err = processAvatarImage(origin, 262144)
	require.ErrorContains(t, err, "image width is too large: 10 > 5")
}

func Test_FetchExternalImageData(t *testing.T) {
	blackPng, err := base64.URLEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABAQAAAAA3bvkkAAAACklEQVR4AWNgAAAAAgABc3UBGAAAAABJRU5ErkJggg==")
	if err != nil {
		t.Error(err)
		return
	}

	var tooWideBuf bytes.Buffer
	imgTooWide := image.NewGray(image.Rect(0, 0, 16001, 10))
	err = png.Encode(&tooWideBuf, imgTooWide)
	if err != nil {
		t.Error(err)
		return
	}
	imgTooWidePng := tooWideBuf.Bytes()

	var tooTallBuf bytes.Buffer
	imgTooTall := image.NewGray(image.Rect(0, 0, 10, 16002))
	err = png.Encode(&tooTallBuf, imgTooTall)
	if err != nil {
		t.Error(err)
		return
	}
	imgTooTallPng := tooTallBuf.Bytes()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/timeout":
			// Simulate a timeout by taking a long time to respond
			time.Sleep(8 * time.Second)
			w.Header().Set("Content-Type", "image/png")
			w.Write(blackPng)
		case "/notfound":
			http.NotFound(w, r)
		case "/image.png":
			w.Header().Set("Content-Type", "image/png")
			w.Write(blackPng)
		case "/weird-content":
			w.Header().Set("Content-Type", "text/html")
			w.Write([]byte("<html></html>"))
		case "/giant-response":
			w.Header().Set("Content-Type", "image/png")
			w.Write(make([]byte, 10485760))
		case "/invalid.png":
			w.Header().Set("Content-Type", "image/png")
			w.Write(make([]byte, 100))
		case "/mismatched.jpg":
			w.Header().Set("Content-Type", "image/jpeg")
			w.Write(blackPng) // valid png, but wrong content-type
		case "/too-wide.png":
			w.Header().Set("Content-Type", "image/png")
			w.Write(imgTooWidePng)
		case "/too-tall.png":
			w.Header().Set("Content-Type", "image/png")
			w.Write(imgTooTallPng)
		default:
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer server.Close()

	tests := []struct {
		name            string
		url             string
		expectedSuccess bool
		expectedLog     string
	}{
		{
			name:            "timeout error",
			url:             "/timeout",
			expectedSuccess: false,
			expectedLog:     "error when fetching external image from",
		},
		{
			name:            "external fetch success",
			url:             "/image.png",
			expectedSuccess: true,
			expectedLog:     "",
		},
		{
			name:            "404 fallback",
			url:             "/notfound",
			expectedSuccess: false,
			expectedLog:     "non-OK error code when fetching external image",
		},
		{
			name:            "unsupported content type",
			url:             "/weird-content",
			expectedSuccess: false,
			expectedLog:     "fetching external image returned unsupported Content-Type",
		},
		{
			name:            "response too large",
			url:             "/giant-response",
			expectedSuccess: false,
			expectedLog:     "while fetching external image response size hit MaxFileSize",
		},
		{
			name:            "invalid png",
			url:             "/invalid.png",
			expectedSuccess: false,
			expectedLog:     "error when decoding external image",
		},
		{
			name:            "mismatched content type",
			url:             "/mismatched.jpg",
			expectedSuccess: false,
			expectedLog:     "while fetching external image, mismatched image body",
		},
		{
			name:            "too wide",
			url:             "/too-wide.png",
			expectedSuccess: false,
			expectedLog:     "while fetching external image, width 16001 exceeds Avatar.MaxWidth",
		},
		{
			name:            "too tall",
			url:             "/too-tall.png",
			expectedSuccess: false,
			expectedLog:     "while fetching external image, height 16002 exceeds Avatar.MaxHeight",
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			// stopMark is used as a logging boundary to verify that the expected message (testCase.expectedLog) is
			// logged during the `fetchExternalImage` operation.  This is verified by a combination of checking that the
			// stopMark message was received, and that the filtered log (logFiltered[0]) was received.
			stopMark := fmt.Sprintf(">>>>>>>>>>>>>STOP: %s<<<<<<<<<<<<<<<", testCase.name)

			logChecker, cleanup := test.NewLogChecker(log.DEFAULT, log.TRACE)
			logChecker.Filter(testCase.expectedLog).StopMark(stopMark)
			defer cleanup()

			client := &http.Client{
				Timeout: 1 * time.Second,
				Transport: &http.Transport{
					Proxy: proxy.Proxy(),
				},
			}
			img, err := FetchExternalImageData(server.URL+testCase.url, client)

			if testCase.expectedSuccess {
				require.NoError(t, err, "expected success from fetchExternalImage")
				assert.NotNil(t, img)
			} else {
				require.Error(t, err, "expected failure from fetchExternalImage")
				assert.Nil(t, img)
			}

			log.Info(stopMark)

			logFiltered, logStopped := logChecker.Check(5 * time.Second)
			assert.True(t, logStopped, "failed to find log stop mark")
			assert.True(t, logFiltered[0], "failed to find in log: '%s'", testCase.expectedLog)
		})
	}
}
