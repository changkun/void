// Copyright (c) 2021 Changkun Ou <hi@changkun.de>. All Rights Reserved.
// Unauthorized using, copying, modifying and distributing, via any
// medium is strictly prohibited.

package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"

	"changkun.de/x/void/internal/void"
	"golang.design/x/tgstore"
)

// Upload uploads the given file to the void server and returns
// the corresponding file ID for future downloads.
func Upload(fpath string) (r *void.Response, err error) {
	defer func() {
		if err == nil {
			return
		}

		err = fmt.Errorf("upload error: %w", err)
		log.Println(err)
	}()

	u, p := os.Getenv("VOID_USER"), os.Getenv("VOID_PASS")
	if u == "" || p == "" {
		log.Fatal("missing VOID_USER and VOID_PASS")
	}

	_, err = os.Stat(fpath)
	if err != nil {
		return
	}

	var f *os.File
	f, err = os.Open(fpath)
	if err != nil {
		return
	}
	defer f.Close()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	var part io.Writer
	part, err = writer.CreateFormFile("file", filepath.Base(fpath))
	if err != nil {
		writer.Close()
		return
	}

	_, err = io.Copy(part, f)
	if err != nil {
		writer.Close()
		return
	}
	writer.Close()

	var req *http.Request
	req, err = http.NewRequest(http.MethodPost, Endpoint, body)
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.SetBasicAuth(u, p)

	var resp *http.Response
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return
	}

	r = &void.Response{}
	err = json.Unmarshal(data, r)

	switch resp.StatusCode {
	case http.StatusOK:
		return r, nil
	default:
		err = errors.New(r.Message)
		return nil, fmt.Errorf("failed with status code %d: %w", resp.StatusCode, err)
	}
}

const overwrite = "\r\033[1A\033[0K"

// Download tries to download the corresponding file of the given id,
// and stores it to the given destination folder.
func Download(id string) (err error) {
	defer func() {
		if err == nil {
			return
		}

		err = fmt.Errorf("download error: %w", err)
	}()

	var req *http.Request
	req, err = http.NewRequest(http.MethodGet, Endpoint+"?mode=data&id="+id, nil)
	if err != nil {
		return
	}
	req.SetBasicAuth(void.Conf.Auth.Username, void.Conf.Auth.Password)

	var resp *http.Response
	resp, err = (&http.Client{}).Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return
	}
	meta := &void.Metadata{}
	err = json.Unmarshal(b, meta)
	if err != nil {
		return
	}

	switch resp.StatusCode {
	case http.StatusOK:

		var tgf io.ReadSeekCloser
		store := tgstore.New()
		store.BotToken = void.Conf.BotToken
		store.ChatID = void.Conf.ChatID
		tgf, err = store.Download(context.Background(), meta.Key, meta.UploadId)
		if err != nil {
			err = fmt.Errorf("download with error: %w", err)
			return
		}
		defer tgf.Close()

		var f *os.File
		f, err = os.Create(meta.FileName)
		if err != nil {
			return err
		}
		defer f.Close()

		log.Printf("[%d] downloading: %sprogress: 0.00%%", resp.StatusCode, meta.FileName)
		batch := int64(1 << 15)
		for i := int64(0); i < meta.FileSize; i += batch {
			_, err = io.CopyN(f, tgf, int64(batch))
			if err == io.EOF {
				err = nil
				break
			}
			if err != nil {
				return
			}
			log.Printf("[%d] progress: %.2f%%%s", resp.StatusCode, float64(i*100)/float64(meta.FileSize), overwrite)
		}
		log.Println("DONE.                    ")
	default:
		var b []byte
		b, err = io.ReadAll(resp.Body)
		if err != nil {
			return
		}

		r := &void.Response{}
		err = json.Unmarshal(b, r)
		if r.Message == "" {
			r.Message = "internal error"
		}
		log.Printf("[%d]%s\n", resp.StatusCode, r.Message)
	}
	return
}

// Delete deletes a given id from the current database.
func Delete(id string) (err error) {
	defer func() {
		if err == nil {
			return
		}

		err = fmt.Errorf("delete error: %w", err)
	}()

	var req *http.Request
	req, err = http.NewRequest(http.MethodDelete, Endpoint+"?id="+id, nil)
	if err != nil {
		return
	}

	var resp *http.Response
	resp, err = (&http.Client{}).Do(req)
	if err != nil {
		return
	}
	resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		return
	default:
		err = fmt.Errorf("failed with status: %v", resp.StatusCode)
		return
	}
}

// List lists all existing files in the current database.
func List() (files []*void.Metadata, err error) {
	defer func() {
		if err == nil {
			return
		}

		err = fmt.Errorf("list error: %w", err)
	}()
	u, p := os.Getenv("VOID_USER"), os.Getenv("VOID_PASS")
	if u == "" || p == "" {
		log.Fatal("missing VOID_USER and VOID_PASS")
	}

	var req *http.Request
	req, err = http.NewRequest(http.MethodGet, Endpoint+"?mode=data", nil)
	if err != nil {
		return
	}
	req.SetBasicAuth(u, p)

	var resp *http.Response
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	var raw []byte
	raw, err = io.ReadAll(resp.Body)
	if err != nil {
		return
	}

	switch resp.StatusCode {
	case http.StatusOK:
		files = []*void.Metadata{}
		err = json.Unmarshal(raw, &files)
		return
	default:
		err = fmt.Errorf("failed with status: %v", resp.StatusCode)
		return
	}
}
