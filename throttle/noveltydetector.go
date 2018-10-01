package throttle

import (
	"github.com/TheCacophonyProject/lepton3"
	"github.com/TheCacophonyProject/thermal-recorder/motion"
)

type NoveltyDetector struct {
	step         uint
	rows         int
	cols         int
	memory       lepton3.Frame
	ignore       [lepton3.FrameRows][lepton3.FrameCols]bool
	threshold    uint16
	triggerThres int
	count        uint
	removeThres  uint16
	maxThres     uint16
}

func NewNoveltyDetector() *NoveltyDetector {
	var stp uint = 4
	return &NoveltyDetector{
		step:        stp,
		threshold:   10,
		rows:        lepton3.FrameRows<<stp + 1,
		cols:        lepton3.FrameCols<<stp + 1,
		removeThres: 5,
	}
}

func (nd *NoveltyDetector) ProcessRecorded(state motion.MotionState) {
	nd.update()
	for y := 0; y < lepton3.FrameRows; y++ {
		for x := 0; x < lepton3.FrameCols; x++ {
			if state.Mask[y][x] {
				nd.memory[y>>nd.step][x>>nd.step]++
			}
		}
	}
}

func (nd *NoveltyDetector) HasThrottledNewMovement(state motion.MotionState) bool {
	nd.update()
	new := 0
	for y := 0; y < lepton3.FrameRows; y++ {
		for x := 0; x < lepton3.FrameCols; x++ {
			if state.Mask[y][x] {
				if nd.ignore[y>>nd.step][x>>nd.step] {
					// assume part of existing - so add to 'background'
					nd.memory[y>>nd.step][x>>nd.step]++
				} else if new > nd.triggerThres {
					return true
				} else {
					new++
				}
			}
		}
	}
	return false
}

func (nd *NoveltyDetector) update() {
	nd.count++

	if nd.count >= nd.step {
		nd.resetIgnoreMask()
		nd.diluteMemory()
		nd.makeIgnoreMask()
	}
}

func (nd *NoveltyDetector) resetIgnoreMask() {
	for y := 0; y < nd.rows; y++ {
		for x := 0; x < nd.cols; x++ {
			nd.ignore[y][x] = false
		}
	}
}

func (nd *NoveltyDetector) diluteMemory() {
	for y := 0; y < nd.rows; y++ {
		for x := 0; x < nd.cols; x++ {
			if nd.memory[y][x] < nd.removeThres {
				nd.memory[y][x] = 0
			} else if nd.memory[y][x] > nd.maxThres {
				nd.memory[y][x] = nd.maxThres
			} else {
				nd.memory[y][x] = nd.memory[y][x] - nd.removeThres
			}
		}
	}
}

func (nd *NoveltyDetector) makeIgnoreMask() {
	for y := 0; y < nd.rows; y++ {
		for x := 0; x < nd.cols; x++ {
			if nd.memory[y][x] > nd.threshold {
				nd.ignore[y-1][x-1] = true
				nd.ignore[y][x-1] = true
				nd.ignore[y+1][x-1] = true
				nd.ignore[y-1][x] = true
				nd.ignore[y][x] = true
				nd.ignore[y+1][x] = true
				nd.ignore[y-1][x+1] = true
				nd.ignore[y][x+1] = true
				nd.ignore[y+1][x+1] = true
			}
		}
	}
}
