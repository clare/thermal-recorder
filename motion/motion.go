package motion

import (
	"log"

	"github.com/TheCacophonyProject/lepton3"
)

const NO_DATA = -1
const TOO_MANY_POINTS_CHANGED = -2

func NewMotionDetector(args MotionConfig) *motionDetector {

	d := new(motionDetector)
	d.flooredFrames = *NewFrameLoop(args.FrameCompareGap + 1)
	d.diffFrames = *NewFrameLoop(2)
	d.useOneDiff = args.UseOneDiffOnly
	d.framesGap = uint64(args.FrameCompareGap)
	d.deltaThresh = args.DeltaThresh
	d.countThresh = args.CountThresh
	d.tempThresh = args.TempThresh + args.DeltaThresh
	totalPixels := lepton3.FrameRows * lepton3.FrameCols
	d.nonzeroLimit = totalPixels * args.NonzeroMaxPercent / 100
	d.verbose = args.Verbose
	d.warmerOnly = args.WarmerOnly
	d.animalTempThresh = 4000
	d.vicinitySize = 10
	d.negativeDeltaThresh = 30

	return d
}

type motionDetector struct {
	flooredFrames       FrameLoop
	diffFrames          FrameLoop
	firstDiff           bool
	useOneDiff          bool
	tempThresh          uint16
	animalTempThresh    uint16
	deltaThresh         uint16
	countThresh         int
	nonzeroLimit        int
	framesGap           uint64
	verbose             bool
	warmerOnly          bool
	negatives           lepton3.Frame
	negativeDeltaThresh uint16
	vicinitySize        int
}

func (d *motionDetector) Detect(frame *lepton3.Frame) bool {
	movement, _ := d.pixelsChanged(frame)
	return movement
}

func (d *motionDetector) pixelsChanged(frame *lepton3.Frame) (bool, int) {

	processedFrame := d.flooredFrames.Current()
	d.setFloor(frame, processedFrame)

	// we will compare with the oldest saved frame.
	compareFrame := d.flooredFrames.Oldest()
	defer d.flooredFrames.Move()

	diffFrame := d.diffFrames.Current()
	if d.warmerOnly {
		d.warmerDiffFrames(processedFrame, compareFrame, diffFrame, &d.negatives)
	}
	prevDiffFrame := d.diffFrames.Move()

	if !d.firstDiff {
		d.firstDiff = true
		return false, NO_DATA
	}

	if d.useOneDiff {
		return d.hasMotion(frame, compareFrame, diffFrame, nil)
	} else {
		return d.hasMotion(frame, compareFrame, diffFrame, prevDiffFrame)
	}
}

func (d *motionDetector) setFloor(f, out *lepton3.Frame) *lepton3.Frame {
	for y := 0; y < lepton3.FrameRows; y++ {
		for x := 0; x < lepton3.FrameCols; x++ {
			v := f[y][x]
			if v < 1000 {
				out[y][x] = d.tempThresh
			} else {
				out[y][x] = v
			}
		}
	}
	return out
}

func (d *motionDetector) CountPixelsTwoCompare(f1 *lepton3.Frame, f2 *lepton3.Frame) (nonZeros, deltas int) {
	var nonzeroCount int
	var deltaCount int
	for y := 0; y < lepton3.FrameRows; y++ {
		for x := 0; x < lepton3.FrameCols; x++ {
			v1 := f1[y][x]
			v2 := f2[y][x]
			if (v1 > 0) || (v2 > 0) {
				nonzeroCount++
				if (v1 > d.deltaThresh) && (v2 > d.deltaThresh) {
					deltaCount++
				}
			}
		}
	}
	return nonzeroCount, deltaCount
}

func (d *motionDetector) CountPixels(frame, compareFrame, f1 *lepton3.Frame) (nonZeros, deltas int) {
	var nonzeroCount int
	var deltaCount int
	for y := 0; y < lepton3.FrameRows; y++ {
		for x := 0; x < lepton3.FrameCols; x++ {
			v1 := f1[y][x]
			if v1 > 0 {
				nonzeroCount++
				if v1 > d.deltaThresh && frame[y][x] > d.tempThresh {
					if frame[y][x] > d.animalTempThresh || !d.reverseChangeInVicinity(frame[y][x], compareFrame, x, y) {
						if d.verbose {
							log.Printf("Motion (%d, %d) = %d = %d", x, y, v1, frame[y][x])
						}
						deltaCount++
						if frame[y][x] <= d.animalTempThresh {
							log.Printf("Real change (%d, %d) = %d = %d", x, y, v1, frame[y][x])
						}
					}
				}
				// }
				// else if frame[y][x] <= d.animalTempThresh {
				// 	log.Printf("ignore change (%d, %d)= %d = %d", x, y, v1, frame[y][x])
				// }
			}
		}
	}
	return nonzeroCount, deltaCount
}

func (d *motionDetector) reverseChangeInVicinity(val uint16, compareFrame *lepton3.Frame, x, y int) bool {
	maxVal := int32(-30)
	value := int32(val)
	for deltaY := -1 * d.vicinitySize; deltaY < d.vicinitySize+1; deltaY++ {
		for deltaX := -1 * d.vicinitySize; deltaX < d.vicinitySize+1; deltaX++ {
			xx := x + deltaX
			yy := y + deltaY

			if xx >= 0 && xx < lepton3.FrameCols && yy >= 0 && yy < lepton3.FrameRows {
				if int32(compareFrame[yy][xx])-value > maxVal {
					maxVal = int32(compareFrame[yy][xx]) - value
				}
			}
		}
	}
	log.Printf("Max negative is %d", maxVal)
	return maxVal > 10
}

func (d *motionDetector) hasMotion(frame, compareFrame, f1, f2 *lepton3.Frame) (bool, int) {
	var nonzeroCount int
	var deltaCount int
	if d.useOneDiff {
		nonzeroCount, deltaCount = d.CountPixels(frame, compareFrame, f1)
	} else {
		nonzeroCount, deltaCount = d.CountPixelsTwoCompare(f1, f2)
	}

	// Motion detection is suppressed when over nonzeroLimit motion
	// pixels are nonzero. This is to deal with sudden jumps in the
	// readings as the camera recalibrates due to rapid temperature
	// change.

	if nonzeroCount > d.nonzeroLimit {
		log.Printf("Motion detector - too many points changed, probably a recalculation")
		d.flooredFrames.SetAsOldest()
		d.firstDiff = false
		return false, TOO_MANY_POINTS_CHANGED
	}

	if deltaCount > 0 && d.verbose {
		log.Printf("deltaCount %d", deltaCount)
	}
	return deltaCount >= d.countThresh, deltaCount
}

func absDiffFrames(a, b, out *lepton3.Frame) *lepton3.Frame {
	for y := 0; y < lepton3.FrameRows; y++ {
		for x := 0; x < lepton3.FrameCols; x++ {
			out[y][x] = absDiff(a[y][x], b[y][x])
		}
	}
	return out
}

func (d *motionDetector) warmerDiffFrames(a, b, out, negatives *lepton3.Frame) *lepton3.Frame {
	for y := 0; y < lepton3.FrameRows; y++ {
		for x := 0; x < lepton3.FrameCols; x++ {
			diff := int32(a[y][x]) - int32(b[y][x])
			if diff < 0 {
				out[y][x] = 0
				d.negatives[y][x] = uint16(-diff)
			} else {
				d.negatives[y][x] = 0
				out[y][x] = uint16(diff)
			}
		}
	}
	return out
}

func absDiff(a, b uint16) uint16 {
	d := int32(a) - int32(b)

	if d < 0 {
		return uint16(-d)
	}
	return uint16(d)
}
