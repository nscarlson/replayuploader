package replayuploader

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"mime/multipart"
	"net/http"
	"time"
)

type Uploader interface {
	Upload(filename string, content io.Reader) error
}

type uploader struct {
	config Config
	client *http.Client
}

func (u *uploader) Upload(filename string, content io.Reader) error {
	body := &bytes.Buffer{}

	mpWriter := multipart.NewWriter(body)
	mpWriter.WriteField("hashkey", u.config.Hash)
	mpWriter.WriteField("token", u.config.Token)
	mpWriter.WriteField("upload_method", "linux_uploader")
	mpWriter.WriteField("timestamp", fmt.Sprintf("%v", time.Now().Unix()))
	part, err := mpWriter.CreateFormFile("Filedata", filename)
	if err != nil {
		return err
	}
	_, err = io.Copy(part, content)
	if err != nil {
		return err
	}
	err = mpWriter.Close()
	if err != nil {
		return err
	}

	url := "https://sc2replaystats.com/upload_replay.php"

	req, err := http.NewRequest("POST", url, body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", mpWriter.FormDataContentType())
	//data, _ := httputil.DumpRequestOut(req, true)
	//log.Printf("Full:\n%v", string(data))

	res, err := u.client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	log.Printf("[%v %v] %v", req.Method, url, res.Status)

	resBody, err := ioutil.ReadAll(res.Body)
	if err != nil {
		log.Printf("Error reading response body: %v", err)
	}
	log.Printf("Response: %v", string(resBody))

	if res.StatusCode == 200 || res.StatusCode == 201 || res.StatusCode == 204 {
		return nil
	}

	return errors.New(fmt.Sprintf("Something went wrong, got StatusCode=%v", res.StatusCode))
}

func New(config Config) Uploader {
	client := &http.Client{
		Timeout: time.Second * 5,
	}
	upl := &uploader{
		config: config,
		client: client,
	}
	return &retryingUploader{
		uploader:       upl,
		maxTries:       config.MaxTries,
		backoffSeconds: 15,
	}
}

type retryingUploader struct {
	maxTries       int
	backoffSeconds int
	uploader       Uploader
}

func (u *retryingUploader) Upload(filename string, content io.Reader) error {
	var err error

	for tries := 0; tries < u.maxTries; tries++ {
		err = u.uploader.Upload(filename, content)
		if err != nil {
			sleepTime := time.Duration(tries*u.backoffSeconds) * time.Second
			log.Printf("[%v] Upload of replay='%v' failed: %v", tries, filename, err)
			log.Printf("[%v] Retrying in %vs", tries, sleepTime.Seconds())
			time.Sleep(sleepTime)
			continue
		} else {
			return nil
		}
	}

	return err
}
