package torr

import (
	// "context"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/anacrolix/dms/dlna"
	"github.com/anacrolix/missinggo/v2/httptoo"
	"github.com/anacrolix/torrent"

	mt "server/mimetype"
	sets "server/settings"
	"server/torr/state"
)

// Add atomic counter for concurrent streams
var activeStreams int32

// globalServedBytes accumulates all bytes served to players over HTTP (LAN) across
// every torrent. Session-scoped: resets on server restart, survives per-torrent drop.
var globalServedBytes int64

// GlobalServedBytes returns total bytes served to players since server start.
func GlobalServedBytes() int64 {
	return atomic.LoadInt64(&globalServedBytes)
}

// viewedThresholdPercent is the share of a file that must be served to a player
// before the file is marked as Viewed.
const viewedThresholdPercent = 85

// viewedTracker accumulates bytes served per (torrent file) across the many separate
// HTTP range requests that make up a single playback (the stream uses Connection: close,
// so each request is its own handler call). When the cumulative served bytes for a file
// reach viewedThresholdPercent of its length, the file is marked Viewed once.
type viewedTracker struct {
	mu     sync.Mutex
	served map[string]int64
}

var viewTracker = &viewedTracker{served: make(map[string]int64)}

func (vt *viewedTracker) add(hash string, index int, n, fileLen int64) {
	if fileLen <= 0 || n <= 0 {
		return
	}
	key := hash + "/" + strconv.Itoa(index)
	vt.mu.Lock()
	cur := vt.served[key] + n
	vt.served[key] = cur
	reached := cur*100 >= fileLen*viewedThresholdPercent
	if reached {
		delete(vt.served, key) // stop tracking; already counted toward Viewed
	}
	vt.mu.Unlock()
	if reached {
		go sets.SetViewed(&sets.Viewed{Hash: hash, FileIndex: index})
	}
}

// countingWriter wraps the HTTP ResponseWriter to count bytes served to the player.
// It delegates every call to the underlying writer (so gin still sees the data),
// increments the per-torrent and global served counters, and feeds the viewed tracker.
type countingWriter struct {
	http.ResponseWriter
	t       *Torrent
	hash    string
	index   int
	fileLen int64
}

func (cw *countingWriter) Write(b []byte) (int, error) {
	n, err := cw.ResponseWriter.Write(b)
	if n > 0 {
		atomic.AddInt64(&cw.t.ServedBytes, int64(n))
		atomic.AddInt64(&globalServedBytes, int64(n))
		viewTracker.add(cw.hash, cw.index, int64(n), cw.fileLen)
	}
	return n, err
}

// type contextResponseWriter struct {
// 	http.ResponseWriter
// 	ctx context.Context
// }

// func (w *contextResponseWriter) Write(p []byte) (n int, err error) {
// 	// Check context before each write
// 	select {
// 	case <-w.ctx.Done():
// 		return 0, w.ctx.Err()
// 	default:
// 		return w.ResponseWriter.Write(p)
// 	}
// }

func (t *Torrent) Stream(fileID int, req *http.Request, resp http.ResponseWriter) error {
	// Increment active streams counter
	streamID := atomic.AddInt32(&activeStreams, 1)
	defer atomic.AddInt32(&activeStreams, -1)
	// Stream disconnect timeout (same as torrent)
	streamTimeout := sets.BTsets.TorrentDisconnectTimeout

	if !t.GotInfo() {
		http.NotFound(resp, req)
		return errors.New("torrent doesn't have info yet")
	}
	// Get file information
	st := t.Status()
	var stFile *state.TorrentFileStat
	for _, fileStat := range st.FileStats {
		if fileStat.Id == fileID {
			stFile = fileStat
			break
		}
	}
	if stFile == nil {
		return fmt.Errorf("file with id %v not found", fileID)
	}
	// Find the actual torrent file
	files := t.Files()
	var file *torrent.File
	for _, tfile := range files {
		if tfile.Path() == stFile.Path {
			file = tfile
			break
		}
	}
	if file == nil {
		return fmt.Errorf("file with id %v not found", fileID)
	}
	// Check file size limit
	if int64(sets.MaxSize) > 0 && file.Length() > int64(sets.MaxSize) {
		err := fmt.Errorf("file size exceeded max allowed %d bytes", sets.MaxSize)
		log.Printf("File %s size (%d) exceeded max allowed %d bytes", file.DisplayPath(), file.Length(), sets.MaxSize)
		http.Error(resp, err.Error(), http.StatusForbidden)
		return err
	}
	// Create reader with context for timeout
	reader := t.NewReader(file)
	if reader == nil {
		return errors.New("cannot create torrent reader")
	}
	// Ensure reader is always closed
	defer t.CloseReader(reader)

	if sets.BTsets.ResponsiveMode {
		reader.SetResponsive()
	}
	// Log connection
	host, port, clerr := net.SplitHostPort(req.RemoteAddr)

	if sets.BTsets.EnableDebug {
		if clerr != nil {
			log.Printf("[Stream:%d] Connect client (Active streams: %d)", streamID, atomic.LoadInt32(&activeStreams))
		} else {
			log.Printf("[Stream:%d] Connect client %s:%s (Active streams: %d)",
				streamID, host, port, atomic.LoadInt32(&activeStreams))
		}
	}

	// Viewed is marked later, once >viewedThresholdPercent of the file has actually
	// been served to the player (see viewedTracker), not merely on opening the stream.

	// Set response headers
	resp.Header().Set("Connection", "close")
	// Add timeout header if configured
	if streamTimeout > 0 {
		resp.Header().Set("X-Stream-Timeout", fmt.Sprintf("%d", streamTimeout))
	}
	// Add ETag
	etag := hex.EncodeToString([]byte(fmt.Sprintf("%s/%s", t.Hash().HexString(), file.Path())))
	resp.Header().Set("ETag", httptoo.EncodeQuotedString(etag))
	// DLNA headers
	resp.Header().Set("transferMode.dlna.org", "Streaming")
	// add MimeType
	mime, err := mt.MimeTypeByPath(file.Path())
	if err == nil && mime.IsMedia() {
		resp.Header().Set("content-type", mime.String())
	}
	// DLNA Seek
	if req.Header.Get("getContentFeatures.dlna.org") != "" {
		resp.Header().Set("contentFeatures.dlna.org", dlna.ContentFeatures{
			SupportRange:    true,
			SupportTimeSeek: true,
		}.String())
	}
	// Add support for range requests
	if req.Header.Get("Range") != "" {
		resp.Header().Set("Accept-Ranges", "bytes")
	}
	// // Create a context with timeout if configured
	// ctx := req.Context()
	// if streamTimeout > 0 {
	// 	var cancel context.CancelFunc
	// 	ctx, cancel = context.WithTimeout(ctx, time.Duration(streamTimeout)*time.Second)
	// 	defer cancel()
	// }
	// // Update request with new context
	// req = req.WithContext(ctx)
	// // Handle client disconnections better
	// wrappedResp := &contextResponseWriter{
	// 	ResponseWriter: resp,
	// 	ctx:            ctx,
	// }
	// http.ServeContent(wrappedResp, req, file.Path(), time.Unix(t.Timestamp, 0), reader)

	cw := &countingWriter{ResponseWriter: resp, t: t, hash: t.Hash().HexString(), index: fileID, fileLen: file.Length()}
	http.ServeContent(cw, req, file.Path(), time.Unix(t.Timestamp, 0), reader)

	if sets.BTsets.EnableDebug {
		if clerr != nil {
			log.Printf("[Stream:%d] Disconnect client", streamID)
		} else {
			log.Printf("[Stream:%d] Disconnect client %s:%s", streamID, host, port)
		}
	}
	return nil
}

// GetActiveStreams returns number of currently active streams
func GetActiveStreams() int32 {
	return atomic.LoadInt32(&activeStreams)
}
