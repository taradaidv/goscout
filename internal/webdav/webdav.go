package webdav

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/pkg/sftp"
	"golang.org/x/net/webdav"
)

var (
	isMacOS = runtime.GOOS == "darwin"
)

func errorString(err error) string {
	if err != nil {
		return err.Error()
	}
	return ""
}

func Mount(sftpClient *sftp.Client) string {
	fs := webdav.NewMemFS()
	ls := webdav.NewMemLS()

	dav := &webdav.Handler{
		Prefix:     "",
		FileSystem: fs,
		LockSystem: ls,
		Logger: func(r *http.Request, err error) {
			log.Println("Logger ::: ", r.Method, r.URL.Path, errorString(err))
		},
	}

	serverReady := make(chan struct{})

	var addr string

	go func() {
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			log.Fatalf("Error starting TCP listener: %v", err)
		}
		defer listener.Close()

		addr = listener.Addr().String()

		mux := http.NewServeMux()
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			if isMacOS {
				for _, segment := range strings.Split(r.URL.Path, "/") {
					if segment == ".DS_Store" || strings.HasPrefix(segment, "._") {
						return
					}
				}
			}
			switch r.Method {
			case "PROPFIND":
				if err := updateDirectory(fs, sftpClient, r.URL.Path); err != nil {
					log.Printf("Failed to update directory: %v", err)
				}
			case "GET":
				if err := downloadFile(sftpClient, fs, r.URL.Path); err != nil {
					http.Error(w, "This is an experimental feature that starts WebDAV on the localhost without creating local folders, meaning the file system is in-memory and available as long as GoScout is running.\nExample for macOS:\nHit CMD+K in Finder, enter the address below with ANY creds.\n\nhttp://"+addr, http.StatusInternalServerError)
					return
				}
			case "POST", "PUT":
				uploadFile(sftpClient, r.URL.Path, r)
			case "DELETE":
				sftpClient.Remove(r.URL.Path)
			case "LOCK":
				lockResource(r.URL.Path)
				return
			case "UNLOCK":
				unlockResource(r.URL.Path)
			case "MKCOL":
				sftpClient.Mkdir(r.URL.Path)
			case "MOVE":
				destination := r.Header.Get("Destination")
				if destination == "" {
					http.Error(w, "Destination header is required", http.StatusBadRequest)
					return
				}
				if err := moveResource(sftpClient, r.URL.Path, destination); err != nil {
					http.Error(w, "Failed to move resource", http.StatusInternalServerError)
					return
				}
			}
			dav.ServeHTTP(w, r)
		})

		log.Printf("Server started %v", addr)
		close(serverReady)
		log.Fatal(http.Serve(listener, mux))
	}()

	<-serverReady

	return addr
}

var locks = make(map[string]bool)

func moveResource(sftpClient *sftp.Client, source, destination string) error {
	if err := sftpClient.Rename(source, destination); err != nil {
		return err
	}
	return nil
}

func lockResource(path string) error {
	if locks[path] {
		return fmt.Errorf("resource already locked")
	}
	locks[path] = true
	return nil
}

func unlockResource(path string) {
	delete(locks, path)
}

func uploadFile(sftpClient *sftp.Client, path string, r *http.Request) error {
	dstFile, err := sftpClient.Create(path)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	done := make(chan error)
	go func() {
		_, err := io.Copy(dstFile, r.Body)
		done <- err
	}()

	return <-done
}

func updateDirectory(fs webdav.FileSystem, sftpClient *sftp.Client, path string) error {

	entries, _ := sftpClient.ReadDir(path)

	for _, entry := range entries {
		relPath := filepath.Join(path, entry.Name())
		if entry.IsDir() {
			if isMacOS {
				//TODO .metadata_never_index
			}

			if err := fs.Mkdir(context.Background(), relPath, os.FileMode(0755)); err != nil {
				log.Printf("Error creating directory: %s - %s", err, relPath)
			}
		} else {
			dstFile, err := fs.OpenFile(context.Background(), relPath, os.O_RDWR|os.O_CREATE, os.FileMode(0644))
			if err != nil {
				return err
			}
			dstFile.Close()
		}
	}

	return nil
}

func downloadFile(sftpClient *sftp.Client, fs webdav.FileSystem, path string) error {
	srcFile, err := sftpClient.Open(path)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := fs.OpenFile(context.Background(), path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, os.FileMode(0644))
	if err != nil {
		return err
	}
	defer dstFile.Close()

	done := make(chan error)
	go func() {
		_, err := io.Copy(dstFile, srcFile)
		done <- err
	}()

	return <-done
}
