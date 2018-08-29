package main

import (
	"errors"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"path"

	"github.com/TheCacophonyProject/lepton3"
)

var (
	previousSnapshotID = 0
	snapshotDir        string
)

func newSnapshot() error {
	f := frameLoop.Previous()
	if f == nil {
		return errors.New("no frames yet")
	}
	g16 := image.NewGray16(image.Rect(0, 0, lepton3.FrameCols, lepton3.FrameRows))
	// Max and min are needed for normalization of the frame
	var valMax uint16
	var valMin uint16 = math.MaxUint16
	var id int
	for _, row := range f {
		for _, val := range row {
			id += int(val)
			valMax = uint16(math.Max(float64(valMax), float64(val)))
			valMin = uint16(math.Min(float64(valMin), float64(val)))
		}
	}

	// Check if frame had already been processed
	if id == previousSnapshotID {
		return nil
	}
	previousSnapshotID = id

	var norm = math.MaxUint16 / (valMax - valMin)
	for y, row := range f {
		for x, val := range row {
			g16.SetGray16(x, y, color.Gray16{Y: (val - valMin) * norm})
		}
	}

	out, err := os.Create(path.Join(snapshotDir, "still.png"))
	if err != nil {
		return err
	}
	defer out.Close()
	return png.Encode(out, g16)
}
