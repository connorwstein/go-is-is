package main

import (
    "fmt"
    "time"
    "os"
    "os/signal"
    "syscall"
    "sync"
)

var wg sync.WaitGroup
var l1_lan_hello_dst []byte

type Adjacency struct {
    intf string
    state string // Can be NEW, INITIALIZING or UP
}

func hello_send() {
    for {
        send_frame(l1_lan_hello_dst, "eth0")
        time.Sleep(2000 * time.Millisecond)
    }
}
func hello_recv() {
    for {
        recv_frame("eth0")
    }
}

func cleanup() {
    fmt.Println("cleanup")
}

func main() {
    // Initialize global variable
    l1_lan_hello_dst = []byte{0x01, 0x80, 0xc2, 0x00, 0x00, 0x14}
    c := make(chan os.Signal, 2)
    signal.Notify(c, os.Interrupt, syscall.SIGTERM)
    go func() {
        <-c
        cleanup()
        os.Exit(1)
    }()
	// Start a couple go routines to communicate with other nodes
	// to establish adjacencies 
    // Multicast mac address used in IS-IS hellos
    wg.Add(1)
    go hello_send()
    wg.Add(1)
    go hello_recv()
    wg.Wait()
}


