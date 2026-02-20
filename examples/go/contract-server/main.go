package main

import (
	"crypto/sha256"
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	contractsDir := flag.String("dir", "./contracts", "Directory of CUE contract files")
	addr := flag.String("addr", ":26861", "Listen address")
	service := flag.String("service", "billing", "Service name")
	domain := flag.String("domain", "billing", "Domain subdirectory to serve")
	flag.Parse()

	srv := &contractServer{
		dir:     *contractsDir,
		service: *service,
		domain:  *domain,
	}

	http.HandleFunc("GET /.well-known/covenant", srv.handleDiscovery)
	http.HandleFunc("GET /contracts/", srv.handleFile)

	log.Printf("Contract server listening on %s (dir: %s)", *addr, *contractsDir)
	log.Fatal(http.ListenAndServe(*addr, nil))
}

type contractServer struct {
	dir     string
	service string
	domain  string
}

func (s *contractServer) handleDiscovery(w http.ResponseWriter, r *http.Request) {
	files, etag, err := s.listFiles()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	disc := map[string]any{
		"version":       "1.0",
		"service":       s.service,
		"description":   fmt.Sprintf("%s domain contracts", s.service),
		"contract_etag": etag,
		"persona":       "customer",
		"contracts": map[string]any{
			"files": files,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	json.NewEncoder(w).Encode(disc)
}

func (s *contractServer) handleFile(w http.ResponseWriter, r *http.Request) {
	// Strip /contracts/ prefix and resolve to filesystem path.
	rel := strings.TrimPrefix(r.URL.Path, "/contracts/")
	abs := filepath.Join(s.dir, rel)

	// Prevent path traversal.
	if !strings.HasPrefix(abs, filepath.Clean(s.dir)) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	data, err := os.ReadFile(abs)
	if err != nil {
		if os.IsNotExist(err) {
			http.NotFound(w, r)
		} else {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}

	w.Header().Set("Content-Type", "text/x-cue")
	w.Header().Set("Cache-Control", "public, max-age=60")
	w.Write(data)
}

// listFiles returns the /contracts/... URLs for all .cue files in the domain
// subdirectory, along with a content-based ETag.
func (s *contractServer) listFiles() ([]string, string, error) {
	domainDir := filepath.Join(s.dir, s.domain)
	h := sha256.New()
	var files []string

	err := filepath.WalkDir(domainDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".cue") {
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		h.Write(data)

		// Convert abs path to a /contracts/... URL.
		rel, err := filepath.Rel(s.dir, path)
		if err != nil {
			return err
		}
		files = append(files, "/contracts/"+filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		return nil, "", err
	}

	etag := fmt.Sprintf("%x", h.Sum(nil))[:12]
	return files, etag, nil
}
