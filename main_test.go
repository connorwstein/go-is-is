package main 

import (
    "testing"
    "fmt"
    "sync"
)

func TestInitInterfaces(t *testing.T) {
    cfg = &Config{lock: sync.Mutex{}, sid: ""}
    initInterfaces()
    fmt.Println(cfg.interfaces[0].routes)
    // TODO: more testing here
}
