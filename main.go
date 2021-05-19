package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha1"
	"encoding/binary"
	"fmt"
	log "github.com/sirupsen/logrus"
	"net"
	"net/http"
	"net/url"
	"os/signal"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"os"

	"github.com/pkg/errors"

	"github.com/IncSW/go-bencode"
)

func init() {
	log.SetLevel(log.TraceLevel)
}

type torrent struct {
	Filename string

	InfoHash   [20]byte
	PeerId     [20]byte
	Port       string
	Uploaded   string
	Downloaded string
	Left       string

	Announce     string        `bencode:"announce"`
	AnnounceList []interface{} `bencode:"announce-list"`
	//info interface{} `bencode:"info"`
}

type announce_list struct {
	List []a_list
}

type a_list struct {
	Annouces []announce
}

type announce string

type connectReq struct {
	connection_id  uint64 //0x41727101980
	action         uint32
	transaction_id uint32
}

func (c connectReq) toPacket() packet {
	msg := make([]byte, 16)
	binary.BigEndian.PutUint64(msg[:8], c.connection_id)
	binary.BigEndian.PutUint32(msg[8:12], c.action)
	binary.BigEndian.PutUint32(msg[12:16], c.transaction_id)
	return msg
}

type connectRsp struct {
	action         uint32
	transaction_id uint32
	connection_id  uint64
}

type packet []byte

func (pkt packet) toConnectResponse() (connectRsp, error) {
	var rsp connectRsp

	if len(pkt) < 16 {
		return rsp, errors.New("response less than 16")
	}

	rsp.action = binary.BigEndian.Uint32(pkt[:4])
	rsp.transaction_id = binary.BigEndian.Uint32(pkt[4:8])
	rsp.connection_id = binary.BigEndian.Uint64(pkt[8:16])
	return rsp, nil
}

const Port = 6881

func main() {
	trt := &torrent{Filename: "/home/hxm/Downloads/ubuntu-21.04-desktop-amd64.iso.torrent"}

	data, err := os.ReadFile(trt.Filename)
	//data, err := os.ReadFile("/u01/downloads/City.on.a.Hill.S02E04.Overtime.White.And.Overtime.Stupid.1080p.AMZN.WEBRip.DDP5.1.x264-NTb[rartv]-[rarbg.to].torrent")
	//f, err := os.Open("/u01/downloads/City.on.a.Hill.S02E04.Overtime.White.And.Overtime.Stupid.1080p.AMZN.WEBRip.DDP5.1.x264-NTb[rartv]-[rarbg.to].torrent")
	if err != nil {
		fmt.Printf("%+v", err)
		os.Exit(1)
	}
	//defer f.Close()

	//t:= &torrent{}

	d, err := bencode.Unmarshal(data)
	if err != nil {
		fmt.Printf("%+v")
	}

	//torrent:= t.(*torrent)
	//fmt.Println(torrent.Announce)

	/*if err :=  bencode.Unmarshal(f, t);err!=nil {
		fmt.Printf("%+v", err)
		os.Exit(1)
	}*/

	//fmt.Println(t.Announce)

	/*for _, list := range t.AnnounceList.List {
		for _, v :=range list.Annouces {
			fmt.Println(v)
		}
	}*/

	trtMap := d.(map[string]interface{})

	ann := string(trtMap["announce"].([]byte))
	log.Debugf("announce %s\n", ann)

	trt.Announce = ann

	//trkList := trt["announce-list"].([]interface{})

	info := trtMap["info"]
	infoData, err := bencode.Marshal(info)
	if err != nil {
		fmt.Printf("%+v", err)
		os.Exit(1)
	}
	trt.InfoHash = sha1.Sum(infoData)

	_, err = rand.Read(trt.PeerId[:])
	if err != nil {
		log.Error(err)
		os.Exit(1)
	}

	trt.Port = strconv.Itoa(Port)
	trt.Uploaded = "0"
	trt.Downloaded = "0"

	length := info.(map[string]interface{})["length"]
	trt.Left = strconv.Itoa(int(length.(int64)))

	/*n := 0
	timeout := time.Duration(60*(2 << n))*time.Second
	for _, t := range trackers {
		if t.url.Scheme == "udp" {
			conn, err := net.DialTimeout("udp", t.url.Host, timeout)
			if err != nil {
				fmt.Printf("+v", err)
				os.Exit(1)
			}
			defer conn.Close()

			creq := &connectReq{connection_id: 0x41727101980, action: 0, transaction_id: uint32(rand.Int31())}

			n, err := conn.Write(creq.toPacket())

			if err != nil {
				fmt.Println("write conn error", err)
				os.Exit(1)
			}
			if n != 16 {
				fmt.Println("err, connectReq write no 16", n)
				os.Exit(1)
			}

			fmt.Println("connect sent")

			pkt := packet(make([]byte, 16))
			n, err = conn.Read(pkt)
			if n != 16 {
				fmt.Println("err, connect response read less than 16", n)
				os.Exit(1)
			}
			if err != nil {
				fmt.Println("read conn error", err)
				os.Exit(1)
			}
		}

	}*/

	var sigs = make(chan os.Signal, 1)
	signal.Notify(sigs, os.Interrupt, syscall.SIGTERM, syscall.SIGQUIT)
	defer signal.Stop(sigs)

	ctx, cancel := context.WithCancel(context.Background())

	trk := &tracker{rawurl: ann, stage: StageConnect}
	trk.ctx = ctx
	trk.terminate = cancel

	/*var trackers []tracker
	for _, v := range trkList {
		tl := v.([]interface{})
		for _, v1 := range tl {
			t := tracker{rawurl: string(v1.([]byte))}
			url, err := url.Parse(t.rawurl)
			if err != nil {
				fmt.Println(err)
				os.Exit(1)
			}
			t.url = url

			t.stage = StageConnect
			t.ctx = ctx
			t.terminate = cancel

			trackers = append(trackers, t)
		}
	}*/

	var wg sync.WaitGroup

	go func() {
		wg.Add(1)
		trk.run(trt)
		wg.Done()
	}()

	loop:
		for {
			sig := <- sigs

			switch sig {
			case syscall.SIGQUIT:
				buf := make([]byte, 1<<10)
				stacklen := runtime.Stack(buf, true)
				log.Debugf("=== received SIGQUIT ===\n*** goroutine dump...\n%s\n*** end\n", buf[:stacklen])
			default:
				// shutdown
				log.Infof("terminating ...")
				trk.terminate()
				wg.Wait()
				break loop
			}
		}
}

type tracker struct {
	stage int

	ctx       context.Context
	terminate context.CancelFunc

	rawurl string
	url    *url.URL

	conn net.Conn
}

func (t *tracker) getRequest(trt *torrent) string {
	var buf strings.Builder

	buf.WriteString("?info_hash=")
	buf.WriteString(url.QueryEscape(string(trt.InfoHash[:])))

	buf.WriteString("&peer_id=")
	buf.WriteString(url.QueryEscape(string(trt.PeerId[:])))

	buf.WriteString("&port=")
	buf.WriteString(url.QueryEscape(trt.Port))

	buf.WriteString("&uploaded=")
	buf.WriteString(url.QueryEscape(trt.Uploaded))

	buf.WriteString("&downloaded=")
	buf.WriteString(url.QueryEscape(trt.Downloaded))

	buf.WriteString("&left=")
	buf.WriteString(url.QueryEscape(trt.Left))
	return buf.String()
}

func (t *tracker) connect(ctx context.Context, attempt int, trt *torrent) error {
	c := make(chan error, 1)

	log.Debugf("connect %d", attempt)

	go func() {

		url := t.rawurl + t.getRequest(trt)
		resp, err := http.Get(url)

		if err != nil {
			log.Error(errors.WithStack(err))
			c <- err
			return
		}

		if resp.StatusCode!=200 {
			err = errors.New(resp.Status)
			c <- err
			return
		}

		buf := &bytes.Buffer{}
		if _, err := buf.ReadFrom(resp.Body);err!=nil {
			c <- errors.WithStack(err)
			return
		}
		defer resp.Body.Close()

		um, err:= bencode.Unmarshal(buf.Bytes())
		if err!=nil {
			c <- errors.WithStack(err)
			return
		}

		unMap := um.(map[string]interface{})
		v, ok := unMap["failure reason"]
		if ok {
			c <- errors.New(v.(string))
			return
		}

		peersListI := unMap["peers"].([]interface{})
		log.Infof("got %d peers", len(peersListI))
		for i, v := range peersListI {
			peerMap := v.(map[string]interface{})
			peerIP := string(peerMap["ip"].([]byte))
			fmt.Println(peerIP)
			_ =  int(peerMap["port"].(int64))

			if i==len(peersListI)-1 {
				log.Tracef("peers finished")
			}
		}



		t.stage= StageFinished

		c <- nil

		/*if t.url.Scheme == "udp" {
			conn, err := net.Dial("udp", t.url.Host)
			if err != nil {
				c <- errors.WithStack(err)
			}
			t.conn = conn
		}*/

	}()

	var err error
	select {
	case err = <-c:
		log.Tracef("connect %d exit", attempt)

	case <-ctx.Done():
		log.Tracef("connect %d canceled", attempt)
	}

	return err
}

func (t *tracker) close() {
	t.conn.Close()
}

const StageConnect = 2
const StageFinished = 10
const MaxAttempt = 5

func (t *tracker) run(trt *torrent) {
	log.Infof("tracker %s run", t.rawurl)

	n := 0
	//timeout := time.Duration(60*(2<<n)) * time.Second

	attempt := 0

	c := make(chan struct{})

	loop := true
	for loop {

		timeout := time.Duration(60*(2<<n)) * time.Second

		var wg sync.WaitGroup

		ctx, cancel := context.WithCancel(t.ctx)

		go func() {
			wg.Add(1)
			defer wg.Done()

			if t.stage == StageConnect {
				if err := t.connect(ctx, attempt, trt); err != nil {
					log.Error(err)
					t.terminate()
				}
			}

			if t.stage == StageFinished {
				log.Tracef("stage finished")
				loop = false
				close(c)
			}

		}()

		select {
		case <-time.After(timeout):
			log.Infof("time out %s", timeout.String())
			cancel()
			wg.Wait()

			n++
			attempt++
			if attempt >= MaxAttempt {
				loop = false
			}

		case <-c:
			log.Infof("tracker finished")

		case <-t.ctx.Done():
			loop = false
			log.Infof("tracker cancel")
		}

	}

	log.Infof("tracker exit run")

}
