package lockhd44780

import (
	"github.com/kidoman/embd"
	"github.com/kidoman/embd/controller/hd44780"

	_ "github.com/kidoman/embd/host/all"
	"sync"
	"fmt"
)

type LockHD44780 struct {
	mux         sync.Mutex
	conn        *hd44780.HD44780
	initialized bool
	sendingData bool
}

func New() *LockHD44780 {
	return &LockHD44780{initialized: false}
}

func (lockHD *LockHD44780) initHD44780() {
	if !lockHD.sendingData {
		return
	}
	if !lockHD.initialized {
		if err := embd.InitI2C(); err != nil {
			panic(err)
		}

		bus := embd.NewI2CBus(1)

		conn, err := hd44780.NewI2C(
			bus,
			0x3f,
			hd44780.PCF8574PinMap,
			hd44780.RowAddress16Col,
			hd44780.TwoLine,
			hd44780.BlinkOff,
		)
		lockHD.conn = conn
		if err != nil {
			fmt.Printf("%#v\r\n", bus)
			fmt.Printf("%#v\r\n", conn)
			panic(err)
		}

		lockHD.conn.Clear()
		lockHD.conn.BacklightOn()

		// Create Play Button
		bufPlay := []byte{0x0, 0x8, 0xc, 0xe, 0xc, 0x8, 0x0, 0 }
		lockHD.conn.CreateChar(0, bufPlay)

		bufPause := []byte{0x0, 0x0, 0xa, 0xa, 0xa, 0xa, 0x0, 0 } // Pause
		lockHD.conn.CreateChar(1, bufPause)

		lockHD.initialized = true
	}
}

func (lockHD *LockHD44780) Close() {
	embd.CloseI2C()
	lockHD.conn.Close()
	lockHD.sendingData = false
	lockHD.initialized = false
}

func (lockHD *LockHD44780) SetCursor(col int, row int) {
	lockHD.mux.Lock()
	defer lockHD.mux.Unlock()
	lockHD.initHD44780()
	lockHD.conn.SetCursor(col, row)
}

func (lockHD *LockHD44780) WritePlay() {
	lockHD.mux.Lock()
	defer lockHD.mux.Unlock()
	lockHD.initHD44780()
	lockHD.conn.SetCursor(0, 0)
	lockHD.conn.WriteChar(0)
}
func (lockHD *LockHD44780) WritePause() {
	lockHD.mux.Lock()
	defer lockHD.mux.Unlock()
	lockHD.initHD44780()
	lockHD.conn.SetCursor(0, 0)
	lockHD.conn.WriteChar(1)
}

func (lockHD *LockHD44780) writeMessage(message string) {
	bytes := []byte(message)
	for _, b := range bytes {
		lockHD.conn.WriteChar(b)
	}
}

func (lockHD *LockHD44780) WriteMessage(message string) {
	lockHD.mux.Lock()
	defer lockHD.mux.Unlock()
	lockHD.initHD44780()
	lockHD.writeMessage(message)
}

func (lockHD *LockHD44780) WriteSongLengthInfo(message string) {
	lockHD.mux.Lock()
	defer lockHD.mux.Unlock()
	lockHD.initHD44780()
	lockHD.conn.SetCursor(0, 1)
	lockHD.writeMessage(message)
}

func (lockHD *LockHD44780) WriteAlbumLengthInfo(message string) {
	lockHD.mux.Lock()
	defer lockHD.mux.Unlock()
	lockHD.initHD44780()
	lockHD.conn.SetCursor(11, 0)
	lockHD.writeMessage(message)
}

func (lockHD *LockHD44780) BacklightOff() {
	lockHD.mux.Lock()
	defer lockHD.mux.Unlock()
	lockHD.initHD44780()
	lockHD.conn.Clear()
	lockHD.conn.BacklightOff()
}

func (lockHD *LockHD44780) BacklightOn() {
	lockHD.mux.Lock()
	defer lockHD.mux.Unlock()
	lockHD.initHD44780()
	lockHD.conn.BacklightOn()
}

func (lockHD *LockHD44780) SendingData(b bool) {
	lockHD.mux.Lock()
	defer lockHD.mux.Unlock()
	lockHD.sendingData = b
}

func (lockHD *LockHD44780) Clear() {
	lockHD.mux.Lock()
	defer lockHD.mux.Unlock()
	lockHD.initHD44780()
	lockHD.conn.Clear()
}
