// just thrown something together for now..
package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"log"
	"net"
	"time"

	"github.com/davecgh/go-spew/spew"
	"golang.org/x/net/ipv4"
)

type Microbeat uint64

// Timeline represents a timeline packet.
type Timeline struct {
	Version      uint8
	Session      uint32
	MsgType      uint8
	MsgSubType   uint8
	ClientID     uint64
	Framerate    uint32
	BeatDuration time.Duration // beat length
	ElapsedMB    Microbeat     // relative to last start
	Elapsed      time.Duration // relative to session 	// ElapsedMS    uint64    // relative to session
}

func (t Timeline) String() string {

	spew.Config.DisableMethods = true
	return fmt.Sprintf(`
struct: %s
bpm: %.2f
position: %d
session elapsed: %s
`,
		spew.Sdump(&t),
		t.BPM(),
		t.ElapsedMB,
		t.Elapsed.String(),
	)

}

func (t *Timeline) BPM() float64 {
	return float64(time.Minute) / float64(t.BeatDuration)
}

func main() {

	nif, err := net.InterfaceByName("ens135")
	if err != nil {
		panic(err)
	}
	group := net.IPv4(224, 76, 78, 75)

	c, err := net.ListenPacket("udp4", "0.0.0.0:20808")
	if err != nil {
		panic(err)
	}
	defer c.Close()

	p := ipv4.NewPacketConn(c)
	if err := p.JoinGroup(nif, &net.UDPAddr{IP: group}); err != nil {
		panic(err)
	}

	// just a slightly larger than the known packet size
	b := make([]byte, 128)

	for {
		n, cm, src, err := p.ReadFrom(b)
		_ = cm  // ignore for now
		_ = src // ignore for now
		if err != nil {
			panic(err)
		}
		if n != 82 {
			log.Printf("unexpected packet length: %d", n)
			continue
		}

		t, err := parseTimelinePacket(b)
		if err != nil {
			log.Fatalln(err)
		}

		log.Println(t.String())
		// if cm != nil {
		// 	if cm.Dst.IsMulticast() {
		// 		if cm.Dst.Equal(group) {
		// 			log.Printf("%d %v", n, src)

		// 		} else {
		// 			// unknown group, discard
		// 			continue
		// 		}

		// 	}
		// }

	}
}

func parseTimelinePacket(b []byte) (*Timeline, error) {
	// spew.Dump(b)
	t := &Timeline{}
	r := bytes.NewBuffer(b)
	// en := binary.LittleEndian
	en := binary.BigEndian

	header := r.Next(6)
	if !bytes.Equal(header, []byte("_asdp_")) {
		return nil, fmt.Errorf("invalid header \n %s", spew.Sdump(header))
	}

	{
		ch, n, err := r.ReadRune()
		if err != nil {
			return nil, fmt.Errorf("cannot read version prefix rune: %v", err)
		}
		if n != 1 || ch != 'v' {
			return nil, fmt.Errorf("invalid version prefix (%d)%c", n, ch)
		}
		if err := binary.Read(r, en, &t.Version); err != nil {
			return nil, fmt.Errorf("cannot read version number")
		}
	}

	if err := binary.Read(r, en, &t.MsgType); err != nil {
		return nil, fmt.Errorf("cannot read X: %v", err)
	}

	if err := binary.Read(r, en, &t.MsgSubType); err != nil {
		return nil, fmt.Errorf("cannot read X: %v", err)
	}

	r.Next(2) // maybe padding

	if err := binary.Read(r, en, &t.ClientID); err != nil {
		return nil, fmt.Errorf("cannot read X: %v", err)
	}

	tmln := r.Next(4)
	if !bytes.Equal(tmln, []byte("tmln")) {
		return nil, fmt.Errorf("expected tmln, got %v", string(tmln))
	}

	if err := binary.Read(r, en, &t.Framerate); err != nil {
		return nil, fmt.Errorf("cannot read X: %v", err)
	}

	var microseconds uint64

	if err := binary.Read(r, en, &microseconds); err != nil {
		return nil, fmt.Errorf("cannot read X: %v", err)
	}
	t.BeatDuration = time.Duration(microseconds) * time.Microsecond

	if err := binary.Read(r, en, &t.ElapsedMB); err != nil {
		return nil, fmt.Errorf("cannot read X: %v", err)
	}

	var elapsedMicroseconds uint64
	if err := binary.Read(r, en, &elapsedMicroseconds); err != nil {
		return nil, fmt.Errorf("cannot read X: %v", err)
	}
	t.Elapsed = time.Duration(elapsedMicroseconds) * time.Microsecond

	sess := r.Next(4)
	if !bytes.Equal(sess, []byte("sess")) {
		return nil, fmt.Errorf("expected sess, got %v", string(sess))
	}

	return t, nil
}
