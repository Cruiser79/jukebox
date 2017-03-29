package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/stianeikeland/go-rpio"
	"github.com/tarm/serial"
	"io/ioutil"
	"jukebox/lockmpdclient"
	"jukebox/lockhd44780"
	"log"
	"math"
	"os"
	"strconv"
	"strings"
	"time"

	"sync"
	"os/signal"
	"syscall"
	_ "expvar"
	"os/exec"
)

type (
	AlbumType struct {
		RFID string
		Name string
	}

	AlbumListType []AlbumType
)

const (
	LOW = iota
	HIGH
)

const (
	NEXT = 1 + iota
	PREV
	PLAYPAUSE
)

const SECONDSTOWAIT = 5

var (
	updateCycle bool
	mu = &sync.Mutex{}
)

func GetUpdateCyle()  bool{
	mu.Lock()
	defer mu.Unlock()
	return updateCycle
}

func SetUpdateCycle(us bool, hd *lockhd44780.LockHD44780) {
	mu.Lock()
	defer mu.Unlock()
	hd.SendingData(us)
	updateCycle = us
}

func (al AlbumListType) findRFIDInFileList(rfidInteger int) string {
	rfid := strconv.Itoa(rfidInteger)
	for _, entry := range al {
		if entry.RFID == rfid {
			return entry.Name
		}
	}
	return ""
}

func hex2int(hexStr string) int {
	// base 16 for hexadecimal
	result, _ := strconv.ParseInt(hexStr, 16, 64)
	return int(result)
}

func checkRFIDTagAndSendCommand(rfidTag int, lockMpdClient *lockmpdclient.LockMPDClient) bool {
	fileAlbumList := AlbumListType{}
	// Fill struct fileAlbum>List with album and rfids from json file
	file, err := ioutil.ReadFile("./albums.json")
	if err != nil {
		log.Printf("File error: %v\r\n", err)
	} else {
		json.Unmarshal(file, &fileAlbumList)
	}

	playlist := fileAlbumList.findRFIDInFileList(rfidTag)
	if playlist == "" {
		log.Printf("ERROR: Couldn't find playlist for rfid <%d>\r\n", rfidTag)
		return false
	} else {
		log.Printf("Found playlist <%s> for rfid <%d>\r\n", playlist, rfidTag)
		lockMpdClient.LoadPlaylistAndPlay(playlist)
		return true
	}
}

func readRFID(lockMpdClient *lockmpdclient.LockMPDClient) {
	c := &serial.Config{Name: "/dev/ttyAMA0", Baud: 9600}
	s, err := serial.OpenPort(c)
	if err != nil {
		log.Fatal(err)
	}

	currentRFIDTag := 0
	startSequence := false
	rfidSequence := bytes.NewBufferString("")
	buf := make([]byte, 128)
	for {
		n, err := s.Read(buf)
		if err != nil {
			log.Fatal(err)
		}
		for i := 0; i < n; i++ {
			//log.Printf("%v\n", buf[i])

			if buf[i] == 2 {
				startSequence = true
				//log.Printf("Sequence start\n")
			} else if buf[i] == 3 {
				startSequence = false
				//log.Printf("Sequence end\n")
				rfidSequenceString := rfidSequence.String()
				log.Printf("rfidSequenceString: %s\n", rfidSequenceString)
				if strings.HasPrefix(rfidSequenceString, "0300") ||
				 strings.HasPrefix(rfidSequenceString, "0200") {
					rfidChecksum := rfidSequenceString[len(rfidSequenceString)-2:]
					rfidTag := rfidSequenceString[4:10]
					log.Printf("Checksum <%v> tag <%v>", rfidChecksum, rfidTag)
					rfidTagInteger := hex2int(rfidTag)
					log.Printf("Hexadecimal to Integer : %d", rfidTagInteger)
					if rfidTagInteger != currentRFIDTag {

						if checkRFIDTagAndSendCommand(rfidTagInteger, lockMpdClient) {
							currentRFIDTag = rfidTagInteger
						}
					}
				} else {
					log.Print("This is no rfid sequence. Card prefix missing.")
				}
				rfidSequence.Reset()
			} else if startSequence {
				rfidSequence.WriteByte(buf[i])
			}
		}
		//log.Printf("Read data from serial: %q", buf[:n])
	}
	return

}

func readButtons(pinNo int, lockMpdClient *lockmpdclient.LockMPDClient, command int) {
	log.Printf("Start readButton with pin <%d> command<%d>\r\n", pinNo, command)
	// 17 27 22
	pin := rpio.Pin(pinNo)
	pin.Input()
	pin.PullUp()

	lastButtonStat := rpio.Low

	for {
		if !GetUpdateCyle() {
			time.Sleep(500 * time.Millisecond)
			continue
		}

		button := pin.Read()

		if button == rpio.Low && button != lastButtonStat {
			time.Sleep(10 * time.Millisecond)
			button = pin.Read()

			if button == rpio.Low {
				log.Printf("Pressed key %v\r\n", command)
				if command == NEXT {
					lockMpdClient.Next()
				}
				if command == PREV {
					lockMpdClient.Previous()
				}
				if command == PLAYPAUSE {
					lockMpdClient.TooglePlay()
				}
				lastButtonStat = rpio.Low
			}
		}
		if button == rpio.High && button != lastButtonStat {
			time.Sleep(10 * time.Millisecond)
			button = pin.Read()

			if button == rpio.High {
				lastButtonStat = rpio.High
			}
		}

		time.Sleep(10 * time.Millisecond)

	}
}

func readPowerOff(pinNo int, relayPin rpio.Pin, lockMpdClient *lockmpdclient.LockMPDClient, hd *lockhd44780.LockHD44780) {
	var startTime time.Time
	log.Printf("Start readPowerOff with pin <%d>\r\n", pinNo)

	pin := rpio.Pin(pinNo)
	pin.Input()
	pin.PullUp()

	// Display anschalten
	log.Println("Switch display on")
	relayPin.Low()

	powerOn := true

	SetUpdateCycle(true, hd)

	lastButtonStat := rpio.Low

	for {
		button := pin.Read()

		if button == rpio.Low && button != lastButtonStat {
			time.Sleep(10 * time.Millisecond)
			button = pin.Read()

			if button == rpio.Low {
				var elapsed time.Duration
				startTime = time.Now()
				for button == rpio.Low && elapsed.Seconds() < SECONDSTOWAIT {
					time.Sleep(10 * time.Millisecond)
					button = pin.Read()
					elapsed = time.Since(startTime)
				}

				log.Printf("Pressed power on/off\r\n")
				log.Printf("Pressing took %s\r\n", elapsed)

				// Display ausschalten
				if powerOn {
					SetUpdateCycle(false, hd)
					hd.Clear()
					hd.SetCursor(4, 0)
					hd.WriteMessage("Goodbye")
					time.Sleep(2 * time.Second)
					if elapsed.Seconds() > SECONDSTOWAIT {
						hd.Clear()
						hd.SetCursor(4, 0)
						hd.WriteMessage("Shutdown")
						time.Sleep(3 * time.Second)
					}
					lockMpdClient.Pause()
					lockMpdClient.Close()
					hd.Close()
					relayPin.High()
					powerOn = false
					if elapsed.Seconds() > SECONDSTOWAIT {
						log.Printf("Shutdown pi\r\n")
						if err := exec.Command("/bin/bash", "-c", "shutdown -h now").Run(); err != nil {
							fmt.Println("Failed to initiate shutdown:", err)
						}
					}
				} else {
					// DIsplay anschalten
					relayPin.Low()
					time.Sleep(20 * time.Millisecond)
					SetUpdateCycle(true, hd)
					powerOn = true
				}

				lastButtonStat = rpio.Low
			}
		}
		if button == rpio.High && button != lastButtonStat {
			time.Sleep(10 * time.Millisecond)
			button = pin.Read()

			if button == rpio.High {
				lastButtonStat = rpio.High
			}
		}

		time.Sleep(10 * time.Millisecond)

	}
}

func round(x float64) float64 {
	v, frac := math.Modf(x)
	if x > 0.0 {
		if frac > 0.5 || (frac == 0.5 && uint64(v)%2 != 0) {
			v += 1.0
		}
	} else {
		if frac < -0.5 || (frac == -0.5 && uint64(v)%2 != 0) {
			v -= 1.0
		}
	}

	return v
}

func secToTime(seconds string) string {
	iTotalSeconds, _ := strconv.Atoi(seconds)
	iMinutes := round(float64(iTotalSeconds) / 60)
	iSeconds := math.Mod(float64(iTotalSeconds), 60)
	return fmt.Sprintf("%02.0f:%02.0f", iMinutes, iSeconds)

}

func getCurrentAndTotalTimes(seconds string) (string, string) {
	currentAndTotal := strings.Split(seconds, ":")
	return secToTime(currentAndTotal[0]), secToTime(currentAndTotal[1])
}


func updateDisplay(lockMpdClient *lockmpdclient.LockMPDClient, hd *lockhd44780.LockHD44780) {
	log.Println("Start updateDisplay")
	for !GetUpdateCyle() {
		time.Sleep(500 * time.Millisecond)
	}
	log.Println("Print welcome message to display")
	hd.Clear()
	hd.SetCursor(1, 0)
	hd.WriteMessage("Starte Jukebox")
	time.Sleep(1 * time.Second)
	hd.Clear()

	log.Println("Start display cycling")
	for {
		if !GetUpdateCyle() {
			time.Sleep(500 * time.Millisecond)
			continue
		}
		hd.SetCursor(0, 0)

		mpdStatus, _ := lockMpdClient.Status()
		mpdStatusTime, mpdStatusTimeFound := mpdStatus["time"]
		mpStatusSong, mpdStatusSongFound := mpdStatus["song"]
		if !mpdStatusTimeFound {
			mpdStatusTime = "0:0"
		}
		currentTimeString, totalTimeString := getCurrentAndTotalTimes(mpdStatusTime)
		iSongIndex, _ := strconv.Atoi(mpStatusSong)
		iPlaylistLength, _ := strconv.Atoi(mpdStatus["playlistlength"])
		if mpdStatus["state"] == "play" {
			log.Printf("%s <%02d/%02d>: %s - %s\r\n", mpdStatus["state"], iSongIndex+1, iPlaylistLength, currentTimeString, totalTimeString)
			hd.WritePlay()
		} else {
			hd.WritePause()
		}
		if mpdStatus["state"] == "stop" && !mpdStatusTimeFound && !mpdStatusSongFound {
			lockMpdClient.SetStartFromBeginning()
		}

		songPositionString := fmt.Sprintf("%02d/%02d", iSongIndex+1, iPlaylistLength)
		hd.WriteAlbumLengthInfo(songPositionString)

		hd.SetCursor(0, 1)

		songLengthString := currentTimeString + "/" + totalTimeString
		hd.WriteSongLengthInfo(songLengthString)

		time.Sleep(300 * time.Millisecond)
	}
	return
}

func cleanup(hd* lockhd44780.LockHD44780, mpd * lockmpdclient.LockMPDClient,  relayPin rpio.Pin) {
	log.Println("cleanup")
	mpd.Pause()
	mpd.Close()
	hd.Close()
	relayPin.High()
	rpio.Close()
	os.Exit(1)
}

func main() {
	hd := lockhd44780.New()

	mpd := lockmpdclient.New()

	if err := rpio.Open(); err != nil {
		log.Printf("%v", err)
		os.Exit(1)
	}

	relayPin := rpio.Pin(9)
	relayPin.Output()

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigs
		cleanup(hd, mpd, relayPin)
		hd.Close()
		os.Exit(1)
	}()

	go readButtons(17, mpd, NEXT)
	go readButtons(27, mpd, PREV)
	go readButtons(22, mpd, PLAYPAUSE)
	go readPowerOff(10, relayPin, mpd, hd)
	go updateDisplay(mpd, hd)
	readRFID(mpd)
}
