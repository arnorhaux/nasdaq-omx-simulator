// NASDAQ OMX simulator that replays ITCH files over UDP.
package main

import (
	"bytes"
	"code.google.com/p/go.net/ipv4"
	"encoding/binary"
	"fmt"
	"net"
	"os"
	"syscall"
	"unsafe"
)

func main() {
	filename := os.Args[1]
	fmt.Printf("NASDAQ OMX simulator\n\n")
	fmt.Printf("=> Replaying file %s...\n", filename)
	fi, err := os.Stat(filename)
	if err != nil {
		panic(err)
	}
	file, err := os.Open(filename)
	if err != nil {
		panic(err)
	}
	mmap, err := syscall.Mmap(int(file.Fd()), 0, int(fi.Size()), syscall.PROT_READ, syscall.MAP_PRIVATE)
	if err != nil {
		panic(err)
	}
	defer syscall.Munmap(mmap)

	c, err := net.ListenPacket("udp4", "233.74.125.41:31041")
	if err != nil {
		panic(err)
	}
	defer c.Close()

	ifi, err := net.InterfaceByName("lo")
	if err != nil {
		panic(err)
	}

	dst, err := net.ResolveUDPAddr("udp4", "233.74.125.41:31041")
	if err != nil {
		panic(err)
	}

	p := ipv4.NewPacketConn(c)

	if err := p.JoinGroup(ifi, dst); err != nil {
		panic(err)
	}
	if err := p.SetMulticastInterface(ifi); err != nil {
		panic(err)
	}
	if err := p.SetMulticastLoopback(true); err != nil {
		panic(err)
	}

	seqNo := uint32(0)
	offset := 0
	for {
		if offset >= int(fi.Size()) {
			break
		}
		ch := mmap[offset]
		seqNo += 1
		buf := new(bytes.Buffer)
		header := MoldUDPHeader{
			SequenceNumber: seqNo,
			MessageCount:   1,
		}
		err := binary.Write(buf, binary.LittleEndian, header)
		if err != nil {
			panic(err)
		}
		if ch != 0x0d {
			size := 0
			switch ch {
			case 'T':
				size = int(unsafe.Sizeof(Seconds{}))
			case 'M':
				size = int(unsafe.Sizeof(Milliseconds{}))
			case 'O':
				size = int(unsafe.Sizeof(MarketSegmentState{}))
			case 'S':
				size = int(unsafe.Sizeof(SystemEvent{}))
			case 'R':
				size = int(unsafe.Sizeof(OrderBookDirectory{}))
			case 'H':
				size = int(unsafe.Sizeof(OrderBookTradingAction{}))
			case 'A':
				size = int(unsafe.Sizeof(AddOrder{}))
			case 'F':
				size = int(unsafe.Sizeof(AddOrderMPID{}))
			case 'E':
				size = int(unsafe.Sizeof(OrderExecuted{}))
			case 'C':
				size = int(unsafe.Sizeof(OrderExecutedWithPrice{}))
			case 'X':
				size = int(unsafe.Sizeof(OrderCancel{}))
			case 'D':
				size = int(unsafe.Sizeof(OrderDelete{}))
			case 'P':
				size = int(unsafe.Sizeof(Trade{}))
			case 'Q':
				size = int(unsafe.Sizeof(CrossTrade{}))
			case 'B':
				size = int(unsafe.Sizeof(BrokenTrade{}))
			case 'I':
				size = int(unsafe.Sizeof(NOII{}))
			default:
				fmt.Printf("Unknown message type: '%c' (%x)\n", ch, ch)
				os.Exit(1)
			}
			msgBlock := MoldUDPMessageBlock{
				MessageLength: uint16(size),
			}
			err := binary.Write(buf, binary.LittleEndian, msgBlock)
			if err != nil {
				panic(err)
			}
			buf.Write(mmap[offset : offset+size])
			p.WriteTo(buf.Bytes(), nil, dst)
			offset += size
		}
		// SoupFILE end of message marker CR/LF:
		offset += 2
	}
}
