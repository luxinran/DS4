package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net"
	"os"
	"strconv"
	"sync"
	"time"

	RA "github.com/luxinran/DS4/grpc"
	"google.golang.org/grpc"
)

const INITPORT int32 = 8000

const (
	RELEASED uint8 = 0
	WANTED         = 1
	HELD           = 2
)

type msg struct {
	id      int32
	lamport uint64
}

type node struct {
	RA.UnimplementedRAServer
	id      int32
	mutex   sync.Mutex
	replies uint8
	// channel used to signal that we have entered the critical section
	held    chan bool
	state   uint8
	lamport uint64
	// clients is a map of all clients we have dialed, and their respective gRPC client
	empty RA.Empty
	// idmsg is used to send our id to other peers
	idmsg RA.Id
	// queue of messages to be sent
	queue   []msg
	reply   chan bool
	clients map[int32]RA.RAClient
	ctx     context.Context
}

func main() {
	arg1, _ := strconv.ParseInt(os.Args[1], 10, 32)
	_port := int32(arg1) + INITPORT

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// create a new node
	_n := &node{
		id:      _port,
		clients: make(map[int32]RA.RAClient),
		replies: 0,
		held:    make(chan bool),
		ctx:     ctx,
		state:   RELEASED,
		lamport: 1,
		empty:   RA.Empty{},
		idmsg:   RA.Id{Id: _port},
		reply:   make(chan bool),
	}

	portlist, err := net.Listen("tcp", fmt.Sprintf(":%v", _port))
	if err != nil {
		log.Fatalf("Could not listen on port: %v", err)
	}

	logfile := setLog(_port)
	defer logfile.Close()

	grpcServer := grpc.NewServer()
	RA.RegisterRAServer(grpcServer, _n)

	// connect to all other nodes
	go func() {
		if err := grpcServer.Serve(portlist); err != nil {
			log.Fatalf("Failed to serve: %v", err)
		}
	}()

	for i := 0; i < 4; i++ {
		port := INITPORT + int32(i)

		if port == _port {
			continue
		}

		var conn *grpc.ClientConn
		conn, err := grpc.Dial(fmt.Sprintf(":%v", port), grpc.WithInsecure(), grpc.WithBlock())
		if err != nil {
			log.Fatalf("Dial failed: %s", err)
		}
		defer conn.Close()
		_c := RA.NewRAClient(conn)
		_n.clients[port] = _c
	}

	time.Sleep(5 * time.Second)

	// start the main loop
	go func() {
		for {
			<-_n.reply
			_n.mutex.Lock()
			for _, msg := range _n.queue {

				if msg.lamport > _n.lamport {
					_n.lamport = msg.lamport
				}
				_n.clients[msg.id].Reply(_n.ctx, &_n.idmsg)
			}

			_n.lamport++
			_n.queue = nil
			_n.state = RELEASED
			_n.mutex.Unlock()
		}
	}()

	rand.Seed(time.Now().UnixNano() / int64(_port))
	for {

		if rand.Intn(100) == 42 {
			_n.mutex.Lock()
			_n.state = WANTED
			_n.mutex.Unlock()
			_n.enter()

			<-_n.held
			log.Printf("+ Entered critical section\n")
			time.Sleep(5 * time.Second)
			log.Printf("- Leaving critical section\n")
			_n.reply <- true
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// Request is called by other nodes to request access to the critical section
func (_n *node) Request(ctx context.Context, req *RA.Info) (*RA.Empty, error) {
	_n.mutex.Lock()

	if _n.state == WANTED {
		if _n.lamport < req.Lamport || _n.id < req.Id {
			_n.queue = append(_n.queue, msg{id: req.Id, lamport: req.Lamport})
		}
	} else if _n.state == HELD {
		_n.queue = append(_n.queue, msg{id: req.Id, lamport: req.Lamport})
	} else {
		if req.Lamport > _n.lamport {
			_n.lamport = req.Lamport
		}
		_n.lamport++
		go func() {
			time.Sleep(10 * time.Millisecond)
			_n.clients[req.Id].Reply(_n.ctx, &_n.idmsg)
		}()
	}
	_n.mutex.Unlock()
	return &_n.empty, nil
}

// Reply is called by other nodes to reply to our request
func (_n *node) Reply(ctx context.Context, req *RA.Id) (*RA.Empty, error) {
	_n.mutex.Lock()
	_n.replies++
	if _n.replies >= 3 {
		_n.state = HELD
		_n.replies = 0
		_n.mutex.Unlock()
		_n.held <- true
	} else {
		_n.mutex.Unlock()
	}

	return &_n.empty, nil
}

// enter is called to enter the critical section
func (_n *node) enter() {
	log.Printf("Enter | Seeking critical section access")
	info := &RA.Info{Id: _n.id, Lamport: _n.lamport}
	for id, client := range _n.clients {
		_, err := client.Request(_n.ctx, info)
		if err != nil {
			log.Printf("something went wrong with id %v\n", id)
		}
	}
}

// setLog sets up the log file
func setLog(port int32) *os.File {
	filename := fmt.Sprintf("node(%v)-log.txt", port)
	if err := os.Truncate(filename, 0); err != nil {
		log.Printf("Failed to truncate: %v\n", err)
	}

	logfile, err := os.OpenFile(filename, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		log.Fatalf("Error opening file: %v", err)
	}
	mw := io.MultiWriter(os.Stdout, logfile)
	log.SetOutput(mw)
	return logfile
}
