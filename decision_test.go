package main 

import (
    "testing"
    "fmt"
)

func TestDecisionSPF(t *testing.T) {
    // Build a sample update database
    UpdateDBInit()
    fmt.Println(UpdateDB)
}
