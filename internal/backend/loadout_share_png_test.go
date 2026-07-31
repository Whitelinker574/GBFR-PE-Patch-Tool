package backend

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/png"
	"strings"
	"testing"
)

func sharePNGDataURL(t *testing.T, width, height int) string {
	t.Helper()
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, image.NewNRGBA(image.Rect(0, 0, width, height))); err != nil {
		t.Fatal(err)
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(encoded.Bytes())
}

func TestDecodeLoadoutSharePNGAcceptsEveryExportContract(t *testing.T) {
	for _, size := range [][2]int{{1920, 1080}, {1440, 1920}, {1600, 1600}} {
		payload, err := decodeLoadoutSharePNG(sharePNGDataURL(t, size[0], size[1]))
		if err != nil {
			t.Fatalf("%dx%d: %v", size[0], size[1], err)
		}
		config, err := png.DecodeConfig(bytes.NewReader(payload))
		if err != nil || config.Width != size[0] || config.Height != size[1] {
			t.Fatalf("%dx%d decoded as %+v, %v", size[0], size[1], config, err)
		}
	}
}

func TestDecodeLoadoutSharePNGRejectsWrongTypeDamageAndDimensions(t *testing.T) {
	var tiny bytes.Buffer
	if err := png.Encode(&tiny, image.NewNRGBA(image.Rect(0, 0, 1, 1))); err != nil {
		t.Fatal(err)
	}
	cases := []string{
		"data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(tiny.Bytes()),
		"data:image/png;base64,not-base64",
		"data:image/png;base64," + base64.StdEncoding.EncodeToString([]byte("not png")),
		sharePNGDataURL(t, 960, 540),
		"data:image/png;base64," + strings.Repeat("A", base64.StdEncoding.EncodedLen(loadoutSharePNGMaxSize)+1),
	}
	for index, dataURL := range cases {
		if _, err := decodeLoadoutSharePNG(dataURL); err == nil {
			t.Fatalf("case %d unexpectedly accepted", index)
		}
	}
}
