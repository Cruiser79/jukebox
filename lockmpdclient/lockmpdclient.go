package lockmpdclient

import (
	"github.com/fhs/gompd/mpd"
	"log"
	"strconv"
	"sync"
)

type LockMPDClient struct {
	mux       sync.Mutex
	conn      *mpd.Client
	connected bool
	playing   bool
	startFromBeginning  bool
}

func New() *LockMPDClient {
	return &LockMPDClient{connected: false}
}

func (lockMPD *LockMPDClient) initMPDConnection() {
	if !lockMPD.connected {
		// Connect to MPD server
		//d, err := mpd.Dial("tcp", "jukebox.fritz.box:6600")
		log.Println("Initialize MPDClient")
		d, err := mpd.Dial("tcp", "localhost:6600")
		lockMPD.conn = d
		if err != nil {
			log.Fatalln(err)
		}
		lockMPD.connected = true
		lockMPD.playing = false
		lockMPD.startFromBeginning = false

	}
}

func (lockMPD *LockMPDClient) Close() {
	log.Println("Close MPDClient")
	lockMPD.conn.Close()
	lockMPD.connected = false
}

func (lockMPD *LockMPDClient) LoadPlaylistAndPlay(playlist string) {
	lockMPD.mux.Lock()
	defer lockMPD.mux.Unlock()
	lockMPD.initMPDConnection()
	lockMPD.conn.Clear()
	lockMPD.conn.PlaylistLoad(playlist, -1, -1)
	lockMPD.conn.Play(0)
	lockMPD.playing = true
}

func (lockMPD *LockMPDClient) SetStartFromBeginning() {
	lockMPD.mux.Lock()
	defer lockMPD.mux.Unlock()
	lockMPD.startFromBeginning = true
}

func (lockMPD *LockMPDClient) restartPlaylist() {
	lockMPD.mux.Lock()
	defer lockMPD.mux.Unlock()
	lockMPD.initMPDConnection()
	lockMPD.conn.Play(0)
	lockMPD.playing = true
	lockMPD.startFromBeginning = false
}

func (lockMPD *LockMPDClient) Play() {
	lockMPD.mux.Lock()
	defer lockMPD.mux.Unlock()
	lockMPD.initMPDConnection()
	lockMPD.conn.Pause(false)
	lockMPD.playing = true
}

func (lockMPD *LockMPDClient) Pause() {
	lockMPD.mux.Lock()
	defer lockMPD.mux.Unlock()
	lockMPD.initMPDConnection()
	lockMPD.conn.Pause(true)
	lockMPD.playing = false
}

func (lockMPD *LockMPDClient) TooglePlay() {
	if lockMPD.startFromBeginning {
		lockMPD.restartPlaylist()
		return
	}
	if lockMPD.playing {
		lockMPD.Pause()
	} else {
		lockMPD.Play()
	}
}

func (lockMPD *LockMPDClient) Next() {
	lockMPD.mux.Lock()
	defer lockMPD.mux.Unlock()
	lockMPD.initMPDConnection()
	mpdStatus, _ := lockMPD.conn.Status()
	iSongIndex, _ := strconv.Atoi(mpdStatus["song"])
	iPlaylistLength, _ := strconv.Atoi(mpdStatus["playlistlength"])
	if iSongIndex+1 < iPlaylistLength {
		lockMPD.conn.Next()
	}
}

func (lockMPD *LockMPDClient) Previous() {
	lockMPD.mux.Lock()
	defer lockMPD.mux.Unlock()
	lockMPD.initMPDConnection()
	lockMPD.conn.Previous()
}

func (lockMPD *LockMPDClient) Status() (mpd.Attrs, error) {
	lockMPD.mux.Lock()
	defer lockMPD.mux.Unlock()
	lockMPD.initMPDConnection()
	return lockMPD.conn.Status()
}

func (lockMPD *LockMPDClient) ListAllInfos() ([]mpd.Attrs, error) {
	lockMPD.mux.Lock()
	defer lockMPD.mux.Unlock()
	lockMPD.initMPDConnection()
	return lockMPD.conn.ListAllInfo("/")
}

func (lockMPD *LockMPDClient) GetCurrentPlaylistInfo() ([]mpd.Attrs, error) {
	lockMPD.mux.Lock()
	defer lockMPD.mux.Unlock()
	lockMPD.initMPDConnection()
	return lockMPD.conn.PlaylistInfo(-1, -1)
}
