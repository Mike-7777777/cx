//go:build windows

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

var (
	kernel32           = syscall.NewLazyDLL("kernel32.dll")
	procGetConsoleMode = kernel32.NewProc("GetConsoleMode")
	procSetConsoleMode = kernel32.NewProc("SetConsoleMode")
)

const (
	enableLineInput      = 0x0002
	enableEchoInput      = 0x0004
	enableProcessedInput = 0x0001
	enableVTProcessing   = 0x0004 // ENABLE_VIRTUAL_TERMINAL_PROCESSING (stdout)
	enableVTInput        = 0x0200 // ENABLE_VIRTUAL_TERMINAL_INPUT (stdin)
)

// EnableRawMode switches the console to raw mode by disabling line input, echo,
// and processed input on stdin, while enabling virtual terminal input so that
// arrow keys generate VT escape sequences. It returns a function that restores
// the original console mode.
func EnableRawMode() (restore func(), err error) {
	fd := os.Stdin.Fd()

	var origMode uint32
	r, _, e := procGetConsoleMode.Call(fd, uintptr(unsafe.Pointer(&origMode)))
	if r == 0 {
		return nil, e
	}

	rawMode := origMode
	rawMode &^= enableLineInput | enableEchoInput | enableProcessedInput
	rawMode |= enableVTInput

	r, _, e = procSetConsoleMode.Call(fd, uintptr(rawMode))
	if r == 0 {
		return nil, e
	}

	// Also enable VT processing on stdout so escape codes render correctly.
	outFd := os.Stdout.Fd()
	var outMode uint32
	r, _, _ = procGetConsoleMode.Call(outFd, uintptr(unsafe.Pointer(&outMode)))
	if r != 0 {
		procSetConsoleMode.Call(outFd, uintptr(outMode|enableVTProcessing))
	}

	restore = func() {
		procSetConsoleMode.Call(fd, uintptr(origMode))
		if r != 0 {
			procSetConsoleMode.Call(outFd, uintptr(outMode))
		}
	}
	return restore, nil
}

// ReadKey blocks until a keypress is available on stdin and returns the
// parsed Key. With virtual terminal input enabled, Windows sends the same
// escape sequences as Unix terminals for arrow keys.
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
