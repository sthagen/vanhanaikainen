package main

import (
	"crypto/sha256"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

const MAX_UPLOAD_SIZE = 1024 * 1024

type Progress struct {
	TotalSize int64
	BytesRead int64
}

func (pr *Progress) Write(p []byte) (n int, err error) {
	n, err = len(p), nil
	pr.BytesRead += int64(n)
	pr.Print()
	return
}

func (pr *Progress) Print() {
	if pr.BytesRead == pr.TotalSize {
		fmt.Println("DONE!")
		return
	}

	fmt.Printf("File upload in progress: %d\n", pr.BytesRead)
}

func IndexHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "text/html")
	http.ServeFile(w, r, "index.html")
}

func uploadHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 32 MB is the default used by FormFile
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// get a reference to the fileHeaders
	files := r.MultipartForm.File["file"]

	name_part := time.Now().UnixNano()
	xml_ext := ""
	xml_path := ""
	for _, fileHeader := range files {
		if fileHeader.Size > MAX_UPLOAD_SIZE {
			http.Error(w, fmt.Sprintf("The source file is too big: %s. Please use files with less than 1MB in size", fileHeader.Filename), http.StatusBadRequest)
			return
		}

		file, err := fileHeader.Open()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		defer file.Close()

		buff := make([]byte, 512)
		_, err = file.Read(buff)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		filetype := http.DetectContentType(buff)
		if filetype != "text/xml; charset=utf-8" {
			message := fmt.Sprintf("The provided file format (%s) is not allowed. Please upload only xml files for now", filetype)
			http.Error(w, message, http.StatusBadRequest)
			return
		}

		_, err = file.Seek(0, io.SeekStart)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		err = os.MkdirAll("./uploads", os.ModePerm)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		name_part = time.Now().UnixNano()
		xml_ext = filepath.Ext(fileHeader.Filename)
		xml_path = fmt.Sprintf("./incoming/%d%s", name_part, xml_ext)
		f, err := os.Create(xml_path)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		defer f.Close()

		pr := &Progress{
			TotalSize: fileHeader.Size,
		}

		_, err = io.Copy(f, io.TeeReader(file, pr))
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		xml_src := fmt.Sprintf("./incoming/%d%s", name_part, xml_ext)

		f, err = os.Open(xml_src)
		if err != nil {
			log.Fatal(err)
		}
		defer f.Close()

		h := sha256.New()
		if _, err := io.Copy(h, f); err != nil {
			log.Fatal(err)
		}
		h_xml_src := fmt.Sprintf("%x", h.Sum(nil))
		h_name := fmt.Sprintf("https://example.com/downloads/app/svg/%s.svg", h_xml_src)

		fmt.Printf("incoming: sha256:%s<-(%s)\n", h_xml_src, xml_src)
		fmt.Printf("Upload of (%s) successful; SVG promised at(%s)", xml_src, h_name)

		fmt.Fprintf(w, "<p>Upload successful, resulting SVG soon at <a href=\"%s\">%s</a></p>", h_name, h_name)

	}
}

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/", IndexHandler)
	mux.HandleFunc("/incoming", uploadHandler)

	if err := http.ListenAndServe(":1234", mux); err != nil {
		log.Fatal(err)
	}
}
