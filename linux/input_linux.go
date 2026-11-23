package main

// Kernel input plumbing, without cgo:
//
//   - the knob is read from its evdev nodes (/dev/input/eventN) and grabbed
//     (EVIOCGRAB), so its own volume / mute / brightness keys reach nobody else;
//   - everything the app sends (keys, wheel, clicks, and the knob's keys passed
//     through) goes out through one virtual device made with /dev/uinput.
//
// Both need the user to be allowed to open those files; see install.sh.

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

const (
	evSyn = 0x00
	evKey = 0x01
	evRel = 0x02
	evMsc = 0x04

	relX          = 0x00
	relY          = 0x01
	relHWheel     = 0x06
	relWheel      = 0x08
	relWheelHiRes = 0x0b
	relHWheelHi   = 0x0c

	btnLeft   = 0x110
	btnRight  = 0x111
	btnMiddle = 0x112

	inputEventSize = 24 // struct input_event on 64-bit: timeval(16) type(2) code(2) value(4)

	// ioctl numbers (asm-generic, x86_64 and arm64 alike)
	eviocgrab    = 0x40044590 // _IOW('E', 0x90, int)
	uiSetEvBit   = 0x40045564 // _IOW('U', 100, int)
	uiSetKeyBit  = 0x40045565
	uiSetRelBit  = 0x40045566
	uiDevSetup   = 0x405c5503 // _IOW('U', 3, struct uinput_setup), 92 bytes
	uiDevCreate  = 0x5501
	uiDevDestroy = 0x5502
	busVirtual   = 0x06
)

func ioctl(fd uintptr, req, arg uintptr) error {
	if _, _, e := syscall.Syscall(syscall.SYS_IOCTL, fd, req, arg); e != 0 {
		return e
	}
	return nil
}

// ioctlPtr passes a pointer argument. The conversion to uintptr is written in
// the Syscall call itself, the only place where the unsafe.Pointer rules keep
// the pointed-to memory alive and in place for the kernel.
func ioctlPtr(fd uintptr, req uintptr, p unsafe.Pointer) error {
	if _, _, e := syscall.Syscall(syscall.SYS_IOCTL, fd, req, uintptr(p)); e != 0 {
		return e
	}
	return nil
}

// ---- finding the knob -------------------------------------------------------

type evdevNode struct {
	Path    string // /dev/input/eventN
	Name    string
	Vendor  uint16
	Product uint16
}

func readHex16(path string) uint16 {
	b, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	v, _ := strconv.ParseUint(strings.TrimSpace(string(b)), 16, 16)
	return uint16(v)
}

// listEvdev lists every input event node with its USB / Bluetooth ids.
func listEvdev() []evdevNode {
	dirs, _ := filepath.Glob("/sys/class/input/event*")
	var out []evdevNode
	for _, d := range dirs {
		name, _ := os.ReadFile(filepath.Join(d, "device", "name"))
		out = append(out, evdevNode{
			Path:    "/dev/input/" + filepath.Base(d),
			Name:    strings.TrimSpace(string(name)),
			Vendor:  readHex16(filepath.Join(d, "device", "id", "vendor")),
			Product: readHex16(filepath.Join(d, "device", "id", "product")),
		})
	}
	return out
}

// ---- reading the knob ---------------------------------------------------------

type inputEvent struct {
	Time  time.Time
	Type  uint16
	Code  uint16
	Value int32
}

// knobReader owns one grabbed evdev node of the knob.
type knobReader struct {
	node evdevNode
	f    *os.File
}

func openKnob(n evdevNode) (*knobReader, error) {
	f, err := os.OpenFile(n.Path, os.O_RDONLY, 0)
	if err != nil {
		return nil, err
	}
	if err := ioctl(f.Fd(), eviocgrab, 1); err != nil {
		f.Close()
		return nil, fmt.Errorf("grab: %v", err)
	}
	return &knobReader{node: n, f: f}, nil
}

// run reads events until the node goes away (knob unplugged / disconnected).
func (k *knobReader) run(onEvent func(evdevNode, inputEvent)) error {
	buf := make([]byte, inputEventSize*64)
	for {
		n, err := k.f.Read(buf)
		if err != nil {
			return err
		}
		for off := 0; off+inputEventSize <= n; off += inputEventSize {
			b := buf[off : off+inputEventSize]
			sec := int64(binary.LittleEndian.Uint64(b[0:8]))
			usec := int64(binary.LittleEndian.Uint64(b[8:16]))
			onEvent(k.node, inputEvent{
				Time:  time.Unix(sec, usec*1000),
				Type:  binary.LittleEndian.Uint16(b[16:18]),
				Code:  binary.LittleEndian.Uint16(b[18:20]),
				Value: int32(binary.LittleEndian.Uint32(b[20:24])),
			})
		}
	}
}

func (k *knobReader) close() { k.f.Close() }

// ---- the virtual output device -------------------------------------------------

type uinputDev struct {
	mu sync.Mutex
	f  *os.File
}

var out *uinputDev

func openUinput() (*uinputDev, error) {
	f, err := os.OpenFile("/dev/uinput", os.O_WRONLY|syscall.O_NONBLOCK, 0)
	if err != nil {
		return nil, err
	}
	fd := f.Fd()
	set := func(req uintptr, v int) error { return ioctl(fd, req, uintptr(v)) }
	for _, ev := range []int{evKey, evRel, evSyn} {
		if err := set(uiSetEvBit, ev); err != nil {
			f.Close()
			return nil, err
		}
	}
	// Every ordinary key code, so keys of the knob that this app does not know
	// can still be passed through, plus the mouse buttons.
	for code := 1; code < 0x2c0; code++ { // 0x2c0 on: BTN_TRIGGER_HAPPY (joystick)
		if code >= btnLeft-0x10 && code < 0x160 && code != btnLeft && code != btnRight && code != btnMiddle {
			continue // joystick / gamepad / touch buttons would make it look like a game pad
		}
		set(uiSetKeyBit, code)
	}
	for _, r := range []int{relX, relY, relHWheel, relWheel, relWheelHiRes, relHWheelHi} {
		set(uiSetRelBit, r)
	}
	var setup [92]byte
	binary.LittleEndian.PutUint16(setup[0:], busVirtual)
	binary.LittleEndian.PutUint16(setup[2:], 0x1209) // pid.codes vendor id for open-source projects
	binary.LittleEndian.PutUint16(setup[4:], 0xa71c)
	binary.LittleEndian.PutUint16(setup[6:], 1)
	copy(setup[8:88], "Anticater Knob (virtual)")
	if err := ioctlPtr(fd, uiDevSetup, unsafe.Pointer(&setup[0])); err != nil {
		f.Close()
		return nil, fmt.Errorf("UI_DEV_SETUP: %v", err)
	}
	if err := ioctl(fd, uiDevCreate, 0); err != nil {
		f.Close()
		return nil, fmt.Errorf("UI_DEV_CREATE: %v", err)
	}
	// Give the compositor a moment to pick up the new device before the first event.
	time.Sleep(200 * time.Millisecond)
	return &uinputDev{f: f}, nil
}

func (u *uinputDev) close() {
	if u == nil {
		return
	}
	u.mu.Lock()
	defer u.mu.Unlock()
	ioctl(u.f.Fd(), uiDevDestroy, 0)
	u.f.Close()
}

// write sends events followed by a SYN_REPORT.
// testSink, set by tests, receives the events instead of the device.
var testSink *[][3]int32

func (u *uinputDev) write(evs ...[3]int32) {
	if testSink != nil {
		*testSink = append(*testSink, evs...)
		return
	}
	if u == nil {
		return
	}
	u.mu.Lock()
	defer u.mu.Unlock()
	buf := make([]byte, 0, inputEventSize*(len(evs)+1))
	put := func(typ, code uint16, val int32) {
		var b [inputEventSize]byte // the kernel fills in the time
		binary.LittleEndian.PutUint16(b[16:], typ)
		binary.LittleEndian.PutUint16(b[18:], code)
		binary.LittleEndian.PutUint32(b[20:], uint32(val))
		buf = append(buf, b[:]...)
	}
	for _, e := range evs {
		put(uint16(e[0]), uint16(e[1]), e[2])
	}
	put(evSyn, 0, 0)
	if _, err := u.f.Write(buf); err != nil {
		logf("uinput write: %v", err)
	}
}

func (u *uinputDev) key(code uint16, down bool) {
	v := int32(0)
	if down {
		v = 1
	}
	u.write([3]int32{evKey, int32(code), v})
}

func (u *uinputDev) tap(code uint16) {
	u.key(code, true)
	u.key(code, false)
}

// wheel scrolls by notches; vertical > 0 is up, horizontal > 0 is right.
// The high-resolution event (120 per notch) goes with the classic one, as a
// real mouse wheel sends them.
func (u *uinputDev) wheel(horizontal bool, notches int) {
	lo, hi := int32(relWheel), int32(relWheelHiRes)
	if horizontal {
		lo, hi = relHWheel, relHWheelHi
	}
	u.write([3]int32{evRel, hi, int32(notches * 120)}, [3]int32{evRel, lo, int32(notches)})
}

func (u *uinputDev) click(btn uint16) {
	u.write([3]int32{evKey, int32(btn), 1})
	u.write([3]int32{evKey, int32(btn), 0})
}
