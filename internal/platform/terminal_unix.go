//go:build !windows

package platform

import (
	"os"
	"syscall"
	"unsafe"
)

// Key represents a parsed terminal keypress.
type Key int

const (
	KeyNone  Key = iota // no key / timeout
	KeyUp               // arrow up
	KeyDown             // arrow down
	KeyEnter            // enter / return
	KeyEsc              // bare escape
	KeyQ                // q or Q
	KeyR                // r or R
	KeyS                // s or S
	KeyRune             // any other printable rune
)

// termios mirrors the C struct termios used by TCGETS/TCSETS ioctls.
type termios struct {
	Iflag  uint32
	Oflag  uint32
	Cflag  uint32
	Lflag  uint32
	Line   uint8
	Cc     [32]uint8
	Ispeed uint32
	Ospeed uint32
}

const (
	tcgets = 0x5401 // TCGETS ioctl number
	tcsets = 0x5402 // TCSETS ioctl number

	icanon = 0x0002 // canonical mode
	echo   = 0x0008 // echo input
	isig   = 0x0001 // signal generation (Ctrl-C, etc.)
	iexten = 0x8000 // extended input processing
)

// EnableRawMode switches stdin to raw mode by disabling echo, canonical mode,
// and signal generation. It returns a function that restores the original
// terminal state.
func EnableRawMode() (restore func(), err error) {
	fd := int(os.Stdin.Fd())

	var orig termios
	if _, _, errno := syscall.Syscall(
		syscall.SYS_IOCTL,
		uintptr(fd),
		uintptr(tcgets),
		uintptr(unsafe.Pointer(&orig)),
	); errno != 0 {
		return nil, errno
	}

	raw := orig
	raw.Lflag &^= icanon | echo | isig | iexten

	if _, _, errno := syscall.Syscall(
		syscall.SYS_IOCTL,
		uintptr(fd),
		uintptr(tcsets),
		uintptr(unsafe.Pointer(&raw)),
	); errno != 0 {
		return nil, errno
	}

	restore = func() {
		syscall.Syscall(
			syscall.SYS_IOCTL,
			uintptr(fd),
			uintptr(tcsets),
			uintptr(unsafe.Pointer(&orig)),
		)
	}
	return restore, nil
}

// ReadKey blocks until a keypress is available on stdin and returns the
// parsed Key. Escape sequences for arrow keys (\x1b[A, \x1b[B) are decoded.
func ReadKey() (Key, error) {
	buf := make([]byte, 3)
	n, err := os.Stdin.Read(buf)
	if err != nil {
		return KeyNone, err
	}
	if n == 0 {
		return KeyNone, nil
	}

	return parseKeyBytes(buf[:n]), nil
}

// parseKeyBytes converts raw bytes from stdin into a Key constant.
func parseKeyBytes(b []byte) Key {
	if len(b) == 3 && b[0] == 0x1b && b[1] == '[' {
		switch b[2] {
		case 'A':
			return KeyUp
		case 'B':
			return KeyDown
		}
		return KeyNone
	}

	if len(b) == 1 {
		switch b[0] {
		case 0x1b:
			return KeyEsc
		case '\r', '\n':
			return KeyEnter
		case 'q', 'Q':
			return KeyQ
		case 'r', 'R':
			return KeyR
		case 's', 'S':
			return KeyS
		default:
			return KeyRune
		}
	}

	return KeyNone
}
