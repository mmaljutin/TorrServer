package torr

import (
	"io"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/anacrolix/torrent"
	"github.com/anacrolix/torrent/metainfo"

	"server/log"
	sets "server/settings"
)

var bts *BTServer

func InitApiHelper(bt *BTServer) {
	bts = bt
}

func LoadTorrent(tor *Torrent) *Torrent {
	if tor.TorrentSpec == nil {
		return nil
	}
	tr, err := NewTorrent(tor.TorrentSpec, bts)
	if err != nil {
		return nil
	}
	if !tr.WaitInfo() {
		return nil
	}
	tr.Title = tor.Title
	tr.Poster = tor.Poster
	tr.Data = tor.Data
	return tr
}

func AddTorrent(spec *torrent.TorrentSpec, title, poster string, data string, category string) (*Torrent, error) {
	torr, err := NewTorrent(spec, bts)
	if err != nil {
		log.TLogln("error add torrent:", err)
		return nil, err
	}

	torDB := GetTorrentDB(spec.InfoHash)

	if torr.Title == "" {
		torr.Title = title
		if title == "" && torDB != nil {
			torr.Title = torDB.Title
		}
		if torr.Title == "" && torr.Torrent != nil && torr.Torrent.Info() != nil {
			torr.Title = torr.Info().Name
		}
	}

	if torr.Category == "" {
		torr.Category = category
		if torr.Category == "" && torDB != nil {
			torr.Category = torDB.Category
		}
	}

	if torr.Poster == "" {
		torr.Poster = poster
		if torr.Poster == "" && torDB != nil {
			torr.Poster = torDB.Poster
		}
	}

	if torr.Data == "" {
		torr.Data = data
		if torr.Data == "" && torDB != nil {
			torr.Data = torDB.Data
		}
	}

	return torr, nil
}

func SaveTorrentToDB(torr *Torrent) {
	log.TLogln("save to db:", torr.Hash())
	AddTorrentDB(torr)
}

func GetTorrent(hashHex string) *Torrent {
	hash := metainfo.NewHashFromHex(hashHex)
	timeout := time.Second * time.Duration(sets.BTsets.TorrentDisconnectTimeout)
	if timeout > time.Minute {
		timeout = time.Minute
	}
	tor := bts.GetTorrent(hash)
	if tor != nil {
		tor.AddExpiredTime(timeout)
		return tor
	}

	tr := GetTorrentDB(hash)
	if tr != nil {
		tor = tr
		go func() {
			log.TLogln("New torrent", tor.Hash())
			tr, _ := NewTorrent(tor.TorrentSpec, bts)
			if tr != nil {
				tr.Title = tor.Title
				tr.Poster = tor.Poster
				tr.Data = tor.Data
				tr.Size = tor.Size
				tr.Timestamp = tor.Timestamp
				tr.Category = tor.Category
				tr.GotInfo()
			}
		}()
	}
	return tor
}

func SetTorrent(hashHex, title, poster, category string, data string, keepFiles *bool) *Torrent {
	hash := metainfo.NewHashFromHex(hashHex)
	torr := bts.GetTorrent(hash)
	torrDb := GetTorrentDB(hash)

	if title == "" && torr == nil && torrDb != nil {
		torr = GetTorrent(hashHex)
		torr.GotInfo()
		if torr.Torrent != nil && torr.Torrent.Info() != nil {
			title = torr.Info().Name
		}
	}

	if torr != nil {
		if title == "" && torr.Torrent != nil && torr.Torrent.Info() != nil {
			title = torr.Info().Name
		}
		torr.Title = title
		torr.Poster = poster
		torr.Category = category
		if data != "" {
			torr.Data = data
		}
		if keepFiles != nil {
			torr.KeepFiles = *keepFiles
			if torr.cache != nil {
				torr.cache.SetKeepFiles(*keepFiles)
			}
		}
	}
	// update torrent data in DB
	if torrDb != nil {
		torrDb.Title = title
		torrDb.Poster = poster
		torrDb.Category = category
		if data != "" {
			torrDb.Data = data
		}
		if keepFiles != nil {
			torrDb.KeepFiles = *keepFiles
		}
		AddTorrentDB(torrDb)
	}
	if torr != nil {
		return torr
	} else {
		return torrDb
	}
}

func RemTorrent(hashHex string) {
	if sets.ReadOnly {
		log.TLogln("API RemTorrent: Read-only DB mode!", hashHex)
		return
	}
	hash := metainfo.NewHashFromHex(hashHex)
	if bts.RemoveTorrent(hash) {
		if sets.BTsets.UseDisk && hashHex != "" && hashHex != "/" {
			name := filepath.Join(sets.BTsets.TorrentsSavePath, hashHex)
			ff, _ := os.ReadDir(name)
			for _, f := range ff {
				os.Remove(filepath.Join(name, f.Name()))
			}
			err := os.Remove(name)
			if err != nil {
				log.TLogln("Error remove cache:", err)
			}
		}
	}
	RemTorrentDB(hash)
}

func ListTorrent() []*Torrent {
	btlist := bts.ListTorrents()
	dblist := ListTorrentsDB()

	for hash, t := range dblist {
		if _, ok := btlist[hash]; !ok {
			btlist[hash] = t
		}
	}
	var ret []*Torrent

	for _, t := range btlist {
		ret = append(ret, t)
	}

	sort.Slice(ret, func(i, j int) bool {
		if ret[i].Timestamp != ret[j].Timestamp {
			return ret[i].Timestamp > ret[j].Timestamp
		} else {
			return ret[i].Title > ret[j].Title
		}
	})

	return ret
}

// GlobalTraffic returns server-wide, session-scoped traffic totals in bytes:
// download/upload are summed over currently active torrents (anacrolix session stats),
// served is the cumulative bytes sent to players over HTTP since server start.
func GlobalTraffic() (download, upload, served int64) {
	for _, torr := range bts.ListTorrents() {
		if torr.Torrent == nil {
			continue
		}
		st := torr.Torrent.Stats()
		download += st.BytesRead.Int64()
		upload += st.BytesWritten.Int64()
	}
	served = GlobalServedBytes()
	return
}

func DropTorrent(hashHex string) {
	hash := metainfo.NewHashFromHex(hashHex)
	bts.RemoveTorrent(hash)
}

func SetSettings(set *sets.BTSets) {
	if sets.ReadOnly {
		log.TLogln("API SetSettings: Read-only DB mode!")
		return
	}
	sets.SetBTSets(set)
	log.TLogln("drop all torrents")
	dropAllTorrent()
	time.Sleep(time.Second * 1)
	log.TLogln("disconect")
	bts.Disconnect()
	log.TLogln("connect")
	bts.Connect()
	time.Sleep(time.Second * 1)
	log.TLogln("end set settings")
}

func SetDefSettings() {
	if sets.ReadOnly {
		log.TLogln("API SetDefSettings: Read-only DB mode!")
		return
	}
	sets.SetDefaultConfig()
	log.TLogln("drop all torrents")
	dropAllTorrent()
	time.Sleep(time.Second * 1)
	log.TLogln("disconect")
	bts.Disconnect()
	log.TLogln("connect")
	bts.Connect()
	time.Sleep(time.Second * 1)
	log.TLogln("end set default settings")
}

func dropAllTorrent() {
	for _, torr := range bts.torrents {
		torr.drop()
		<-torr.closed
	}
}

// ClearAllCache drops every active torrent and removes its on-disk cache files,
// but keeps the torrent entries in the database so they can be re-played later.
func ClearAllCache() int {
	if sets.ReadOnly {
		log.TLogln("API ClearAllCache: Read-only DB mode!")
		return 0
	}
	cleared := 0
	torrs := bts.ListTorrents()
	hashes := make([]metainfo.Hash, 0, len(torrs))
	for h := range torrs {
		hashes = append(hashes, h)
	}
	for _, h := range hashes {
		bts.RemoveTorrent(h)
	}
	if sets.BTsets.UseDisk && sets.BTsets.TorrentsSavePath != "" {
		entries, _ := os.ReadDir(sets.BTsets.TorrentsSavePath)
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			name := entry.Name()
			if name == "" || name == "/" || len(name) != 40 {
				continue
			}
			dir := filepath.Join(sets.BTsets.TorrentsSavePath, name)
			ff, _ := os.ReadDir(dir)
			for _, f := range ff {
				os.Remove(filepath.Join(dir, f.Name()))
			}
			if err := os.Remove(dir); err == nil {
				cleared++
			}
		}
	}
	log.TLogln("ClearAllCache: cleared cache for", cleared, "torrents")
	return cleared
}

func Shutdown() {
	bts.Disconnect()
	sets.CloseDB()
	log.TLogln("Received shutdown. Quit")
	os.Exit(0)
}

func WriteStatus(w io.Writer) {
	bts.client.WriteStatus(w)
}

func Preload(torr *Torrent, index int) {
	size := int64(sets.BTsets.PreloadCache) * 1024 * 1024
	if size <= 0 {
		return
	}
	if size > sets.BTsets.CacheSize {
		size = sets.BTsets.CacheSize
	}
	torr.Preload(index, size)
}
