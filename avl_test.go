package main 

import (
    "testing"
    "fmt"
)

func TestAvlIterativePrint(t *testing.T) {
    var root *AvlNode
    root = &AvlNode{key: 10, left: nil, right: nil, height: 1, data: nil}
    root = AvlInsert(root, 20, 20, false) 
    root = AvlInsert(root, 30, 30, false) 
    stack := make([]*AvlNode, 0)
    current := root
    done := false
    for ! done {
        if current != nil {
            stack = append(stack, current)
            current = current.left
        } else {
            if len(stack) != 0 {
                // Pop an item off the stack
                current = stack[len(stack) -1]
                fmt.Printf("LSP: %v\n", current) 
                stack = stack[:len(stack) -1]
                current = current.right
            } else {
                done = true
            }
        }
    }
}

func TestAvlTreeInsertDup(t *testing.T) {
    var root *AvlNode
    root = &AvlNode{key: 10, left: nil, right: nil, height: 1, data: nil}
    root = AvlInsert(root, 10, 10, false) 
    root = AvlInsert(root, 10, 10, false) 
    AvlPrint(root) 
}

func TestAvlTreeSearch(t *testing.T) {
    var root *AvlNode
    root = &AvlNode{key: 10, left: nil, right: nil, height: 1}
    var i uint64
    for i = 0; i < 1000; i++ {
        root = AvlInsert(root, i, i, false)
    }
    tmp1 := AvlSearch(root, 578)
    tmp2 := AvlSearch(root, 1200)
    fmt.Println(tmp1.(uint64), tmp2)
    if tmp1.(uint64) != 578 || tmp2 != nil {
        t.Fail()
    }
}

func TestAvlTreeInsert(t *testing.T) {
    var root *AvlNode
    root = &AvlNode{key: 10, left: nil, right: nil, height: 1}
    var i uint64
    for i = 0; i < 1000; i++ {
        root = AvlInsert(root, i, nil, false)
    }
//     AvlPrint(root)
    // Check balanced 
    b := GetBalanceFactor(root)
    fmt.Println("Balance factor is:", b)
    if b > 1 || b < -1 {
        t.Fail()
    }
}

func TestAvlTreeDelete(t *testing.T) {
    var root *AvlNode
    root = &AvlNode{key: 10, left: nil, right: nil, height: 1}
    var i uint64
    for i = 0; i < 1000; i++ {
        root = AvlInsert(root, i, nil, false)
    }
    // Should have roughly 0-499 in left subtree and 500-999 in right subtree
    // Try deleting all nodes from 999-500 (worst case scenario)
    for i = 999; i > 499; i-- {
        root = AvlDelete(root, i)
    }
    b := GetBalanceFactor(root)
    fmt.Println("Balance factor is:", b)
    if b > 1 || b < -1 {
        t.Fail()
    }
}

func TestAvlGetHeight(t *testing.T) {
    var root *AvlNode
    root = &AvlNode{key: 2, left: nil, right: nil, height: 1}
    root = AvlInsert(root, 10, nil, false)
    root = AvlInsert(root, 20, nil, false)
    if GetHeight(root) != 2 {
        t.Fail()
    }
    AvlInsert(root, 30, nil, false)
    if GetHeight(root) != 3 {
        t.Fail()
    }
}

func TestAvlRightRotate(t *testing.T) {
    var root *AvlNode
    root = &AvlNode{key: 10, left: nil, right: nil, height: 1}
    root = AvlInsert(root, 9, nil, false)
    root = AvlInsert(root, 8, nil, false)
    if root.left.key != 8 || root.right.key != 10 || root.key !=9 {
        t.Fail()
    }
}

func TestAvlLeftRotate(t *testing.T) {
    var root *AvlNode
    root = &AvlNode{key: 10, left: nil, right: nil, height: 1}
    root = AvlInsert(root, 11, nil, false)
    root = AvlInsert(root, 12, nil, false)
    if root.left.key != 10 || root.right.key != 12 || root.key != 11 {
        t.Fail()
    }
}

func TestAvlLeftRight(t *testing.T) {
    var root *AvlNode
    root = &AvlNode{key: 10, left: nil, right: nil, height: 1}
    root = AvlInsert(root, 5, nil, false)
    root = AvlInsert(root, 9, nil, false)
    if root.left.key != 5 || root.right.key != 10 || root.key != 9 {
        t.Fail()
    }
}

func TestAvlRightLeft(t *testing.T) {
    var root *AvlNode
    root = &AvlNode{key: 10, left: nil, right: nil, height: 1}
    root = AvlInsert(root, 15, nil, false)
    root = AvlInsert(root, 13, nil, false)
    if root.left.key != 10 || root.right.key != 15 || root.key != 13{
        t.Fail()
    }
}

func TestAvlOverwrite(t *testing.T) {
    var root *AvlNode
    root = &AvlNode{key: 10, left: nil, right: nil, height: 1}
    root = AvlInsert(root, 12, 10, false)
    root = AvlInsert(root, 15, 10, false)
    root = AvlInsert(root, 15, 12, true)
    AvlPrint(root)
}
