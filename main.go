package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"
	"time"

	"github.com/grafov/m3u8"
	"github.com/horsley/svrkit"
	flag "github.com/spf13/pflag"
)

var (
	listen        *string
	download      *string
	outputDir     *string
	useWorkingDir *bool
	forceDownload *bool
	thread        *int
)

func init() {
	thread = flag.IntP("thread", "t", 10, "download thread limit")
	listen = flag.StringP("listen", "l", ":8000", "http server listen address")
	download = flag.StringP("download", "d", "", "m3u8 download address")
	outputDir = flag.StringP("output", "o", "out", "output(download) directory")
	useWorkingDir = flag.BoolP("use-working-dir", "w", false, "use working directory instead of executable directory")
	forceDownload = flag.BoolP("force-download", "f", false, "force download even if file already exist")

	flag.Parse()
}

func main() {
	if !*useWorkingDir {
		svrkit.SwitchPwd()
	}
	go httpServer()

	if *download != "" {
		go doDownload(*download)
	}

	select {}
}

func httpServer() {
	addr := *listen
	if addr[0] == ':' {
		addr = "localhost" + addr
	}
	log.Println("http server entry at http://" + addr + "/")
	log.Println("http server start failed:", http.ListenAndServe(*listen, http.FileServer(http.Dir("."))))
}

func doDownload(url string) {
	err := os.MkdirAll(*outputDir, 0755)
	if err != nil {
		log.Println("create output dir fail:", err)
		return
	}

	m3u8Bin, url, err := m3u8download(url)
	if err != nil {
		log.Println("m3u8 download fail:", err)
		return
	}

	p, listType, err := m3u8.DecodeFrom(bytes.NewReader(m3u8Bin), true)
	if err != nil {
		panic(err)
	}
	switch listType {
	case m3u8.MEDIA:
		mediapl := p.(*m3u8.MediaPlaylist)

		addrChan := setupDownloader(int(mediapl.Count()))
		for i := uint(0); i < mediapl.Count(); i++ {
			s := mediapl.Segments[i]
			if !strings.HasPrefix(s.URI, "http") {
				s.URI = resolveRelativeURL(url, s.URI)
			}
			addrChan <- *s
		}
		close(addrChan)

	case m3u8.MASTER:
		masterpl := p.(*m3u8.MasterPlaylist)

		playlists := make([]string, len(masterpl.Variants))
		for i, v := range masterpl.Variants {
			if !strings.HasPrefix(v.URI, "http") {
				v.URI = resolveRelativeURL(url, v.URI)
			}
			playlists[i] = fmt.Sprintln(i+1, "=>", v.URI, fmt.Sprintf("\ninfo:%+v", v))
		}

		log.Println("m3u8 was a master playlist, choose a media playlist to download:\n" + strings.Join(playlists, "\n"))
	}
}

func setupDownloader(total int) chan<- m3u8.MediaSegment {
	receiver := make(chan m3u8.MediaSegment, *thread)
	go func() {
		token := make(chan bool, *thread)
		for i := 0; i < *thread; i++ {
			token <- true
		}

		var counter int
		var downloadFinishDuration float64
		for s := range receiver {
			<-token

			counter++

			downloadFinishDuration += s.Duration

			log.Println("going to download [", counter, "/", total, "(", prettyDuration(downloadFinishDuration), ")] file:", s.URI)
			go segmentDownload(s.URI, token)
		}

		for i := 0; i < *thread; i++ {
			<-token
		}
		close(token)
		log.Println("all download complete")
	}()
	return receiver
}

func segmentDownload(url string, token chan<- bool) {
	defer func() {
		token <- true
	}()

	saveFile := path.Join(*outputDir, path.Base(url))

	if !*forceDownload {
		if _, err := os.Stat(saveFile); err == nil {
			return //exist
		}
	}

	bin, err := svrkit.HTTPGet(url)
	if err != nil {
		log.Println("download url:", url, "error:", err)
		return
	}
	err = ioutil.WriteFile(saveFile, bin, 0644)
	if err != nil {
		log.Println("url:", url, "save error:", err)
		return
	}
}

func m3u8download(url string) (data []byte, realURL string, err error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()

	data, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		return
	}

	realURL = resp.Request.URL.String()

	err = ioutil.WriteFile(path.Join(*outputDir, path.Base(realURL)), data, 0644)
	if err != nil {
		return
	}
	log.Println("m3u8 save as:", path.Base(realURL))

	return data, resp.Request.URL.String(), nil
}

func resolveRelativeURL(baseURL, newPart string) string {
	u, err := url.Parse(baseURL)
	if err != nil {
		log.Println("resolveRelativeURL err:", err)
		return ""
	}
	if strings.HasPrefix(newPart, "/") {
		u.Path = newPart
	} else {
		u.Path = path.Join(path.Dir(u.Path), newPart)
	}
	return u.String()
}

func prettyDuration(duration float64) string {
	d, _ := time.ParseDuration(fmt.Sprint(duration) + "s")
	return d.String()
}
