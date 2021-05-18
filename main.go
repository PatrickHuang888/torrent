package main

import (
	"context"
	"encoding/binary"
	log "github.com/sirupsen/logrus"
	"net/url"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"os"

	"github.com/pkg/errors"
)

type torrent struct {
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

func main() {
	//f, err := os.ReadFile("/u01/downloads/City.on.a.Hill.S02E04.Overtime.White.And.Overtime.Stupid.1080p.AMZN.WEBRip.DDP5.1.x264-NTb[rartv]-[rarbg.to].torrent")
	//f, err := os.Open("/u01/downloads/City.on.a.Hill.S02E04.Overtime.White.And.Overtime.Stupid.1080p.AMZN.WEBRip.DDP5.1.x264-NTb[rartv]-[rarbg.to].torrent")
	/*if err != nil {
		fmt.Printf("%+v", err)
		os.Exit(1)
	}*/
	//defer f.Close()

	//t:= &torrent{}

	/*d, err := bencode.Unmarshal(f)
	if err != nil {
		fmt.Printf("%+v")
	}*/

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

	/*t := d.(map[string]interface{})

	ann := string(t["announce"].([]byte))
	fmt.Println(ann)

	trkList := t["announce-list"].([]interface{})

	var trackers []tracker

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
			trackers = append(trackers, t)
		}
	}

	info := t["info"]
	infoData, err := bencode.Marshal(info)
	if err != nil {
		fmt.Printf("%+v", err)
		os.Exit(1)
	}
	infHash := sha1.Sum(infoData)
	fmt.Println(infHash)*/

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

	var terminate = make(chan os.Signal, 1)
	signal.Notify(terminate, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(terminate)

	ctx, cancel := context.WithCancel(context.Background())

	var t tracker
	t.stage = StageConnect
	t.ctx = ctx
	t.terminate = cancel

	var wg sync.WaitGroup

	go func() {
		wg.Add(1)
		t.run()
		wg.Done()
	}()

	select {
	case <-terminate:
		// shutdown
		log.Infof("terminating ...")
		t.terminate()
		wg.Wait()
	}
}

type tracker struct {
	stage int

	ctx       context.Context
	terminate context.CancelFunc

	rawurl string
	url    *url.URL
}

func (t *tracker) connect(ctx context.Context, attempt int) error {
	c := make(chan error, 1)

	log.Infof("connect %d", attempt)

	/*if t.url.Scheme == "udp" {
		conn, err := net.Dial("udp", t.url.Host)
		if err != nil {
			return errors.WithStack(err)
		}
		defer conn.Close()
	}*/

	go func() {
		time.Sleep(15 * time.Second)
		c <- nil

		t.stage = StageFinished
	}()

	var err error

	select {
	case err = <-c:
		log.Infof("connect %d exit", attempt)

	case <-ctx.Done():
		log.Infof("connect %d canceled", attempt)
	}

	return err
}

const StageConnect = 2
const StageFinished = 10
const MaxAttempt = 5

func (t *tracker) run() {
	log.Info("tracker run")

	n := 0
	//timeout := time.Duration(60*(2<<n)) * time.Second

	attempt := 0

	c := make(chan struct{})

	loop := true
	for loop {

		timeout := time.Duration(5*(2<<n)) * time.Second

		var wg sync.WaitGroup

		ctx, cancel := context.WithCancel(t.ctx)

		go func() {
			wg.Add(1)
			defer wg.Done()

			if t.stage == StageConnect {
				if err := t.connect(ctx, attempt); err != nil {
					log.Error(err)
					t.terminate()
				}
			}

			if t.stage == StageFinished {
				log.Infof("stage finished")
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
