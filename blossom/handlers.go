package blossom

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"mime"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/liamg/magic"
	"github.com/nbd-wtf/go-nostr"
)

func (bs BlossomServer) handleUploadCheck(w http.ResponseWriter, r *http.Request) {
	auth, err := readAuthorization(r)
	if err != nil {
		blossomError(w, err.Error(), 400)
		return
	}
	if auth == nil {
		blossomError(w, "missing \"Authorization\" header", 401)
		return
	}
	if auth.Tags.FindWithValue("t", "upload") == nil {
		blossomError(w, "invalid \"Authorization\" event \"t\" tag", 403)
		return
	}

	mimetype := r.Header.Get("X-Content-Type")
	exts, _ := mime.ExtensionsByType(mimetype)
	var ext string
	if len(exts) > 0 {
		ext = exts[0]
	}

	// get the file size from the incoming header
	size, _ := strconv.Atoi(r.Header.Get("X-Content-Length"))

	for _, rb := range bs.RejectUpload {
		reject, reason, code := rb(r.Context(), auth, size, ext)
		if reject {
			blossomError(w, reason, code)
			return
		}
	}
}

func (bs BlossomServer) handleUpload(w http.ResponseWriter, r *http.Request) {
	auth, err := readAuthorization(r)
	if err != nil {
		blossomError(w, "invalid \"Authorization\": "+err.Error(), 404)
		return
	}
	if auth == nil {
		blossomError(w, "missing \"Authorization\" header", 401)
		return
	}
	if auth.Tags.FindWithValue("t", "upload") == nil {
		blossomError(w, "invalid \"Authorization\" event \"t\" tag", 403)
		return
	}

	// get the file size from the incoming header
	size, _ := strconv.Atoi(r.Header.Get("Content-Length"))
	if size == 0 {
		blossomError(w, "missing \"Content-Length\" header", 400)
		return
	}

	// read first bytes of upload so we can find out the filetype
	b := make([]byte, min(50, size), size)
	if n, err := r.Body.Read(b); err != nil && n != size {
		blossomError(w, "failed to read initial bytes of upload body: "+err.Error(), 400)
		return
	}
	var ext string
	if ft, _ := magic.Lookup(b); ft != nil {
		ext = "." + ft.Extension
	} else {
		// if we can't find, use the filetype given by the upload header
		mimetype := r.Header.Get("Content-Type")
		ext = getExtension(mimetype)
	}

	// special case of android apk -- if we see a .zip but they say it's .apk we trust them
	if ext == ".zip" && getExtension(r.Header.Get("Content-Type")) == ".apk" {
		ext = ".apk"
	}

	// run the reject hooks
	for _, ru := range bs.RejectUpload {
		reject, reason, code := ru(r.Context(), auth, size, ext)
		if reject {
			blossomError(w, reason, code)
			return
		}
	}

	// if it passes then we have to read the entire thing into memory so we can compute the sha256
	for {
		var n int
		n, err = r.Body.Read(b[len(b):cap(b)])
		b = b[:len(b)+n]
		if err != nil {
			if err == io.EOF {
				err = nil
			}
			break
		}
		if len(b) == cap(b) {
			// add more capacity (let append pick how much)
			// if Content-Length was correct we shouldn't reach this
			b = append(b, 0)[:len(b)]
		}
	}
	if err != nil {
		blossomError(w, "failed to read upload body: "+err.Error(), 400)
		return
	}

	hash := sha256.Sum256(b)
	hhash := hex.EncodeToString(hash[:])

	// keep track of the blob descriptor
	bd := BlobDescriptor{
		URL:      bs.ServiceURL + "/" + hhash + ext,
		SHA256:   hhash,
		Size:     len(b),
		Type:     mime.TypeByExtension(ext),
		Uploaded: nostr.Now(),
	}
	if err := bs.Store.Keep(r.Context(), bd, auth.PubKey); err != nil {
		blossomError(w, "failed to save event: "+err.Error(), 400)
		return
	}

	// save actual blob
	for _, sb := range bs.StoreBlob {
		if err := sb(r.Context(), hhash, b); err != nil {
			blossomError(w, "failed to save: "+err.Error(), 500)
			return
		}
	}

	// return response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(bd)
}

func (bs BlossomServer) handleGetBlob(w http.ResponseWriter, r *http.Request) {
	spl := strings.SplitN(r.URL.Path, ".", 2)
	hhash := spl[0]
	if len(hhash) != 65 {
		blossomError(w, "invalid /<sha256>[.ext] path", 400)
		return
	}
	hhash = hhash[1:]

	// check for an authorization tag, if any
	auth, err := readAuthorization(r)
	if err != nil {
		blossomError(w, err.Error(), 400)
		return
	}

	// if there is one, we check if it has the extra requirements
	if auth != nil {
		if auth.Tags.FindWithValue("t", "get") == nil {
			blossomError(w, "invalid \"Authorization\" event \"t\" tag", 403)
			return
		}

		if auth.Tags.FindWithValue("x", hhash) == nil &&
			auth.Tags.FindWithValue("server", bs.ServiceURL) == nil {
			blossomError(w, "invalid \"Authorization\" event \"x\" or \"server\" tag", 403)
			return
		}
	}

	for _, rg := range bs.RejectGet {
		reject, reason, code := rg(r.Context(), auth, hhash)
		if reject {
			blossomError(w, reason, code)
			return
		}
	}

	var ext string
	if len(spl) == 2 {
		ext = spl[1]
	}

	if len(bs.RedirectGet) > 0 {
		for _, redirect := range bs.RedirectGet {
			redirectURL, code, err := redirect(r.Context(), hhash, ext)
			if err == nil && redirectURL != "" {
				// check that the redirectURL contains the hash of the file
				if ok, _ := regexp.MatchString(`\b`+hhash+`\b`, redirectURL); !ok {
					continue
				}

				// not sure if browsers will cache redirects
				// but it doesn't hurt anyway
				w.Header().Set("ETag", hhash)
				w.Header().Set("Cache-Control", "public, max-age=604800, immutable")
				http.Redirect(w, r, redirectURL, code)
				return
			}
		}
	}

	for _, lb := range bs.LoadBlob {
		reader, _ := lb(r.Context(), hhash)
		if reader != nil {
			// use unix epoch as the time if we can't find the descriptor
			// as described in the http.ServeContent documentation
			t := time.Unix(0, 0)
			descriptor, err := bs.Store.Get(r.Context(), hhash)
			if err == nil && descriptor != nil {
				t = descriptor.Uploaded.Time()
			}
			w.Header().Set("ETag", hhash)
			w.Header().Set("Cache-Control", "public, max-age=604800, immutable")
			name := hhash
			if ext != "" {
				name += "." + ext
			}
			http.ServeContent(w, r, name, t, reader)
			return
		}
	}

	blossomError(w, "file not found", 404)
}

func (bs BlossomServer) handleHasBlob(w http.ResponseWriter, r *http.Request) {
	spl := strings.SplitN(r.URL.Path, ".", 2)
	hhash := spl[0]
	if len(hhash) != 65 {
		blossomError(w, "invalid /<sha256>[.ext] path", 400)
		return
	}
	hhash = hhash[1:]

	bd, err := bs.Store.Get(r.Context(), hhash)
	if err != nil {
		blossomError(w, "failed to query: "+err.Error(), 500)
		return
	}

	if bd == nil {
		blossomError(w, "file not found", 404)
		return
	}
	w.Header().Set("Content-Length", strconv.Itoa(bd.Size))
	w.Header().Set("Accept-Ranges", "bytes")
	w.Header().Set("Content-Type", bd.Type)
}

func (bs BlossomServer) handleList(w http.ResponseWriter, r *http.Request) {
	// check for an authorization tag, if any
	auth, err := readAuthorization(r)
	if err != nil {
		blossomError(w, err.Error(), 400)
		return
	}

	// if there is one, we check if it has the extra requirements
	if auth != nil {
		if auth.Tags.FindWithValue("t", "list") == nil {
			blossomError(w, "invalid \"Authorization\" event \"t\" tag", 403)
			return
		}
	}

	pubkey := r.URL.Path[6:]

	for _, rl := range bs.RejectList {
		reject, reason, code := rl(r.Context(), auth, pubkey)
		if reject {
			blossomError(w, reason, code)
			return
		}
	}

	ch, err := bs.Store.List(r.Context(), pubkey)
	if err != nil {
		blossomError(w, "failed to query: "+err.Error(), 500)
		return
	}

	w.Write([]byte{'['})
	enc := json.NewEncoder(w)
	first := true
	for bd := range ch {
		if !first {
			w.Write([]byte{','})
		} else {
			first = false
		}
		enc.Encode(bd)
	}
	w.Write([]byte{']'})
}

func (bs BlossomServer) handleDelete(w http.ResponseWriter, r *http.Request) {
	auth, err := readAuthorization(r)
	if err != nil {
		blossomError(w, err.Error(), 400)
		return
	}

	if auth != nil {
		if auth.Tags.FindWithValue("t", "delete") == nil {
			blossomError(w, "invalid \"Authorization\" event \"t\" tag", 403)
			return
		}
	}

	spl := strings.SplitN(r.URL.Path, ".", 2)
	hhash := spl[0]
	if len(hhash) != 65 {
		blossomError(w, "invalid /<sha256>[.ext] path", 400)
		return
	}
	hhash = hhash[1:]
	if auth.Tags.FindWithValue("x", hhash) == nil &&
		auth.Tags.FindWithValue("server", bs.ServiceURL) == nil {
		blossomError(w, "invalid \"Authorization\" event \"x\" or \"server\" tag", 403)
		return
	}

	// should we accept this delete?
	for _, rd := range bs.RejectDelete {
		reject, reason, code := rd(r.Context(), auth, hhash)
		if reject {
			blossomError(w, reason, code)
			return
		}
	}

	// delete the entry that links this blob to this author
	if err := bs.Store.Delete(r.Context(), hhash, auth.PubKey); err != nil {
		blossomError(w, "delete of blob entry failed: "+err.Error(), 500)
		return
	}

	// we will actually only delete the file if no one else owns it
	if bd, err := bs.Store.Get(r.Context(), hhash); err == nil && bd == nil {
		for _, del := range bs.DeleteBlob {
			if err := del(r.Context(), hhash); err != nil {
				blossomError(w, "failed to delete blob: "+err.Error(), 500)
				return
			}
		}
	}
}

func (bs BlossomServer) handleReport(w http.ResponseWriter, r *http.Request) {
	var body []byte
	_, err := r.Body.Read(body)
	if err != nil {
		blossomError(w, "can't read request body", 400)
		return
	}

	var evt nostr.Event
	if err := json.Unmarshal(body, &evt); err != nil {
		blossomError(w, "can't parse event", 400)
		return
	}

	if isValid, _ := evt.CheckSignature(); !isValid {
		blossomError(w, "invalid report event is provided", 400)
		return
	}

	if evt.Kind != nostr.KindReporting {
		blossomError(w, "invalid report event is provided", 400)
		return
	}

	for _, rr := range bs.ReceiveReport {
		if err := rr(r.Context(), &evt); err != nil {
			blossomError(w, "failed to receive report: "+err.Error(), 500)
			return
		}
	}
}

func (bs BlossomServer) handleMirror(w http.ResponseWriter, r *http.Request) {
	auth, err := readAuthorization(r)
	if err != nil {
		blossomError(w, "invalid \"Authorization\": "+err.Error(), 404)
		return
	}
	if auth == nil {
		blossomError(w, "missing \"Authorization\" header", 401)
		return
	}
	if auth.Tags.FindWithValue("t", "upload") == nil {
		blossomError(w, "invalid \"Authorization\" event \"t\" tag", 403)
		return
	}

	var body struct {
		URL string `json:"url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		blossomError(w, "invalid request body: "+err.Error(), 400)
		return
	}

	// download the blob
	resp, err := http.Get(body.URL)
	if err != nil {
		blossomError(w, "failed to download blob: "+err.Error(), 400)
		return
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		blossomError(w, "failed to read blob: "+err.Error(), 400)
		return
	}

	// calculate sha256 hash
	hash := sha256.Sum256(b)
	hhash := hex.EncodeToString(hash[:])

	// verify hash matches x tag in auth event
	if auth.Tags.FindWithValue("x", hhash) == nil {
		blossomError(w, "blob hash does not match any \"x\" tag in authorization event", 403)
		return
	}

	// get content type and extension
	var ext string
	contentType := resp.Header.Get("Content-Type")
	if contentType != "" {
		ext = getExtension(contentType)
	} else {
		// Try to detect from URL extension
		if idx := strings.LastIndex(body.URL, "."); idx >= 0 {
			ext = body.URL[idx:]
		}
	}

	// run reject hooks
	for _, ru := range bs.RejectUpload {
		reject, reason, code := ru(r.Context(), auth, len(b), ext)
		if reject {
			blossomError(w, reason, code)
			return
		}
	}

	// create blob descriptor
	bd := BlobDescriptor{
		URL:      bs.ServiceURL + "/" + hhash + ext,
		SHA256:   hhash,
		Size:     len(b),
		Type:     contentType,
		Uploaded: nostr.Now(),
	}

	// store blob metadata
	if err := bs.Store.Keep(r.Context(), bd, auth.PubKey); err != nil {
		blossomError(w, "failed to save metadata: "+err.Error(), 400)
		return
	}

	// store actual blob
	for _, sb := range bs.StoreBlob {
		if err := sb(r.Context(), hhash, b); err != nil {
			blossomError(w, "failed to save blob: "+err.Error(), 500)
			return
		}
	}

	json.NewEncoder(w).Encode(bd)
}

func (bs BlossomServer) handleMedia(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/upload", 307)
	return
}

func (bs BlossomServer) handleNegentropy(w http.ResponseWriter, r *http.Request) {
}
