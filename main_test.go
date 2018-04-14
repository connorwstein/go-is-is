package main 

import (
    "testing"
    "github.com/golang/glog"
    "sync"
)

func TestInitInterfaces(t *testing.T) {
    cfg = &Config{lock: sync.Mutex{}, sid: ""}
    initInterfaces()
    glog.V(2).Infof("%v", cfg.interfaces[0].routes)
    // TODO: more testing here
}
