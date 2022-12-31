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

type msg struct {
	// id of the client that sent the message
	id int32
	// lamport timestamp of the message
	lamport uint64
}

type peerNode struct {
	RA.UnimplementedRAServer
	id int32
	// mutex to protect the state of the peer
	mutex   sync.Mutex
	replies uint8
	// channel to signal that we have entered the critical section
	held chan bool
	// state of the peer, 0 = idle, 1 = wanted, 2 = waiting to critical section
	// this is used to determine if we are allowed to enter the critical section
	// or if we should wait until we are allowed to enter
	state   uint8
	lamport uint64
	empty   RA.Empty
	idmsg   RA.Id
	queue   []msg
	reply   chan bool
	clients map[int32]RA.RAClient
	ctx     context.Context
}

func main() {
	arg1, _ := strconv.ParseInt(os.Args[1], 10, 32)
	nodePort := int32(arg1) + 8000
	log.Printf("Starting node on port %v", nodePort)

	// create a new peer and start it up on port nodePort (8000 + arg1)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	node := &peerNode{
		id:      nodePort,
		clients: make(map[int32]RA.RAClient),
		replies: 0,
		held:    make(chan bool),
		ctx:     ctx,
		state:   0,
		lamport: 1,
		empty:   RA.Empty{},
		idmsg:   RA.Id{Id: nodePort},
		reply:   make(chan bool),
	}

	// Create listener tcp on port nodePort
	list, err := net.Listen("tcp", fmt.Sprintf(":%v", nodePort))
	if err != nil {
		log.Fatalf("Could not listen to port %v: %v", nodePort, err)
	}

	file := Log(nodePort)
	defer file.Close()

	grpcServer := grpc.NewServer()
	RA.RegisterRAServer(grpcServer, node)

	// start the gRPC server
	go func() {
		if err := grpcServer.Serve(list); err != nil {
			log.Fatalf("Failed to serve gRPC server over port %v: %v", nodePort, err)
		}
	}()

	// connect to all other nodes
	for i := 0; i < 3; i++ {
		port := 8000 + int32(i)

		if port == nodePort {
			continue
		}

		var conn *grpc.ClientConn
		log.Printf("Dialing to %v\n", port)
		conn, err := grpc.Dial(fmt.Sprintf(":%v", port), grpc.WithInsecure(), grpc.WithBlock())
		if err != nil {
			log.Fatalf("Could not dial to port %v: %v", port, err)
		}
		defer conn.Close()
		client := RA.NewRAClient(conn)
		node.clients[port] = client
	}
	log.Printf("Dialing done\n")
	// sleep for 1 seconds to make sure that all clients have been connected
	time.Sleep(1 * time.Second)

	// start the main loop
	// this loop will wait for a reply from a node and then reply to all nodes in the queue
	go func() {
		for {
			// wait for update
			<-node.reply
			node.mutex.Lock()
			for _, msg := range node.queue {
				// update our lamport to the max found value in queue
				if msg.lamport > node.lamport {
					node.lamport = msg.lamport
				}
				log.Printf("Replying to %v\n", msg.id)
				node.clients[msg.id].Reply(node.ctx, &node.idmsg)
			}
			// see previous comment, increment highest found
			node.lamport++
			node.queue = nil
			node.state = 0
			node.mutex.Unlock()
		}
	}()
	// this loop will send a request to a random node every 100ms
	// if the random number is 22, the node will enter the critical section
	// and then leave it after 1 second
	rand.Seed(time.Now().UnixNano() / int64(nodePort))
	for {
		// send request to random node
		if rand.Intn(100) == 22 {
			node.mutex.Lock()
			node.state = 1
			node.mutex.Unlock()
			node.Enter()
			<-node.held
			log.Printf("Entered critical section\n")
			// sleep for 1 seconds in critical section
			time.Sleep(1 * time.Second)
			log.Printf("Leaving critical section\n")
			node.reply <- true
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// request is called by a client to request access to the critical section
// the client will wait until it is allowed to enter the critical section
// and then send a reply to all nodes in the queue
func (node *peerNode) Request(ctx context.Context, req *RA.Info) (*RA.Empty, error) {
	node.mutex.Lock()
	// check if we are allowed to enter the critical section
	if node.state == 2 || (node.state == 1 && node.lamport < req.Lamport) || (node.state == 1 && node.lamport == req.Lamport && node.id < req.Id) {
		log.Printf("Request- %v is waiting\n", req.Id)
		node.queue = append(node.queue, msg{id: req.Id, lamport: req.Lamport})
	} else {
		if req.Lamport > node.lamport {
			node.lamport = req.Lamport
		}
		node.lamport++
		go func() {
			// sleep a little, to make sure that the other clients have time to add their messages to the queue
			time.Sleep(5 * time.Millisecond)
			log.Printf("Request- %v is allowed\n", req.Id)
			node.clients[req.Id].Reply(node.ctx, &node.idmsg)
		}()
	}
	node.mutex.Unlock()
	return &node.empty, nil
}

// reply is called by a client to reply to a request
func (node *peerNode) Reply(ctx context.Context, req *RA.Id) (*RA.Empty, error) {
	log.Printf("Reply- %v replied\n", req.Id)
	node.mutex.Lock()
	node.replies++
	if node.replies >= 3-1 {
		node.state = 2
		node.replies = 0
		node.mutex.Unlock()
		node.held <- true
	} else {
		node.mutex.Unlock()
	}

	return &node.empty, nil
}

// enter is called by the main loop to request access to the critical section
func (node *peerNode) Enter() {
	log.Printf("Enter | Requesting\n")
	info := &RA.Info{Id: node.id, Lamport: node.lamport}
	for id, client := range node.clients {
		_, err := client.Request(node.ctx, info)
		if err != nil {
			log.Printf(" Enter- Failed to request from %v: %v\n", id, err)
		}
		log.Printf("Enter- Requested %v\n", id)
	}
}

// Log sets up the log file for the server
func Log(port int32) *os.File {
	filename := fmt.Sprintf("log%d.txt", port)
	if err := os.Truncate(filename, 0); err != nil {
		log.Printf("Error truncating file: %v", err)
	}
	myfile, err := os.OpenFile(filename, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		log.Fatalf("Error opening file: %v", err)
	}
	write := io.MultiWriter(os.Stdout, myfile)
	log.SetOutput(write)
	return myfile
}
