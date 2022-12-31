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

const VERBOSE = true

const (
	RELEASED uint8 = 0
	WANTED         = 1
	HELD           = 2
)

type msg struct {
	id      int32
	lamport uint64
}

type peer struct {
	RA.UnimplementedRAServer
	id    int32
	mutex sync.Mutex
	// max 255 other peers
	replies uint8
	// fire updates when we hold the channel
	held chan bool
	// for 3 states, this is a waste.. however, it is neglible on the scale we are at
	state   uint8
	lamport uint64
	// some gRPC calls use empty, prevent making a new one each time
	empty RA.Empty
	// same as above, except for replies
	idmsg RA.Id
	// queued "messages" get appended here, FIFO, in actuality we just store the id of the
	// peer instance we wish to 'gRPC.Reply' to, and the lamport timestamp for updating own lamport
	queue []msg
	// fire update on this channel, when we need to send messages in our queue
	reply   chan bool
	clients map[int32]RA.RAClient
	ctx     context.Context
}

func main() {
	arg1, _ := strconv.ParseInt(os.Args[1], 10, 32)
	ownPort := int32(arg1) + 8000

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	p := &peer{
		id:      ownPort,
		clients: make(map[int32]RA.RAClient),
		replies: 0,
		held:    make(chan bool),
		ctx:     ctx,
		state:   RELEASED,
		lamport: 1,
		empty:   RA.Empty{},
		idmsg:   RA.Id{Id: ownPort},
		reply:   make(chan bool),
	}

	// Create listener tcp on port ownPort
	list, err := net.Listen("tcp", fmt.Sprintf(":%v", ownPort))
	if err != nil {
		log.Fatalf("Could not listen to port %v: %v", ownPort, err)
	}

	f := setLog(ownPort)
	defer f.Close()

	grpcServer := grpc.NewServer()
	RA.RegisterRAServer(grpcServer, p)

	go func() {
		if err := grpcServer.Serve(list); err != nil {
			log.Fatalf("Failed to serve gRPC server over port %v: %v", ownPort, err)
		}
	}()

	for i := 0; i < 3; i++ {
		port := 8000 + int32(i)

		if port == ownPort {
			continue
		}

		var conn *grpc.ClientConn
		if VERBOSE {
			log.Printf("Dialing to %v\n", port)
		}
		conn, err := grpc.Dial(fmt.Sprintf(":%v", port), grpc.WithInsecure(), grpc.WithBlock())
		if err != nil {
			log.Fatalf("Could not dial to port %v: %v", port, err)
		}
		defer conn.Close()
		c := RA.NewRAClient(conn)
		p.clients[port] = c
	}
	if VERBOSE {
		log.Printf("Dialing done\n")
	}
	// as seen from log message above - this is because if we do not sleep, then the first (or second) client will cause
	// invalid control flow in the different gRPC functions on last client, since it might not have an active connection to the one dialing it yet
	// - the simplest solution, is to just wait a little, rather than inquiring each client about whether or not it has N-clients dialed, like ourselves
	time.Sleep(5 * time.Second)

	// start our queue loop, it will (when told to) - send messages that have been delayed
	go func() {
		for {
			// wait for update
			<-p.reply
			p.mutex.Lock()
			for _, msg := range p.queue {
				// update our lamport to the max found value in queue
				if msg.lamport > p.lamport {
					p.lamport = msg.lamport
				}
				if VERBOSE {
					log.Printf("Replying to %v\n", msg.id)
				}
				p.clients[msg.id].Reply(p.ctx, &p.idmsg)
			}
			// see previous comment, increment highest found
			p.lamport++
			p.queue = nil
			p.state = RELEASED
			p.mutex.Unlock()
		}
	}()

	rand.Seed(time.Now().UnixNano() / int64(ownPort))
	for {
		if rand.Intn(100) == 42 {
			p.mutex.Lock()
			p.state = WANTED
			p.mutex.Unlock()
			p.enter()
			<-p.held
			log.Printf("Entered critical section\n")
			// sleep for 5 seconds in critical section
			time.Sleep(5 * time.Second)
			log.Printf("Leaving critical section\n")
			p.reply <- true
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func (p *peer) Request(ctx context.Context, req *RA.Info) (*RA.Empty, error) {
	p.mutex.Lock()
	if p.state == HELD || (p.state == WANTED && p.LessThan(req.Id, req.Lamport)) {
		if VERBOSE {
			log.Printf("Request | %v is waiting\n", req.Id)
		}
		p.queue = append(p.queue, msg{id: req.Id, lamport: req.Lamport})
	} else {
		if req.Lamport > p.lamport {
			p.lamport = req.Lamport
		}
		p.lamport++
		go func() {
			// sleep a little, to make sure that the other clients have time to add their messages to the queue
			time.Sleep(5 * time.Millisecond)
			if VERBOSE {
				log.Printf("Request | %v is allowed\n", req.Id)
			}
			p.clients[req.Id].Reply(p.ctx, &p.idmsg)
		}()
	}
	p.mutex.Unlock()
	return &p.empty, nil
}

func (p *peer) LessThan(id int32, lamport uint64) bool {
	if p.lamport < lamport {
		return true
	} else if p.lamport > lamport {
		return false
	}
	if p.id < id {
		return true
	}
	return false
}

func (p *peer) Reply(ctx context.Context, req *RA.Id) (*RA.Empty, error) {
	if VERBOSE {
		log.Printf("Reply | %v replied\n", req.Id)
	}
	p.mutex.Lock()
	p.replies++
	if p.replies >= 3-1 {
		p.state = HELD
		p.replies = 0
		p.mutex.Unlock()
		p.held <- true
	} else {
		p.mutex.Unlock()
	}

	return &p.empty, nil
}

func (p *peer) enter() {
	log.Printf("Enter | Requesting\n")
	info := &RA.Info{Id: p.id, Lamport: p.lamport}
	for id, client := range p.clients {
		_, err := client.Request(p.ctx, info)
		if err != nil {
			log.Printf(" Enter | Failed to request from %v: %v\n", id, err)
		}
		if VERBOSE {
			log.Printf("Enter | Requested %v\n", id)
		}
	}
}

func setLog(port int32) *os.File {
	// Clears the log.txt file when a new server is started
	filename := fmt.Sprintf("log%d.txt", port)
	if err := os.Truncate(filename, 0); err != nil {
		log.Printf("Error truncating file: %v", err)
	}

	// This connects to the log file/changes the output of the log informaiton to the log.txt file.
	f, err := os.OpenFile(filename, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		log.Fatalf("Error opening file: %v", err)
	}
	// print to both file and console
	mw := io.MultiWriter(os.Stdout, f)
	log.SetOutput(mw)
	return f
}
