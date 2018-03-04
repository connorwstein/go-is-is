package main 

import (
    "testing"
    "fmt"
)
func TestAvlTreeSearch(t *testing.T) {
    var root *AvlNode
    root = &AvlNode{key: 10, left: nil, right: nil, height: 1}
    var i uint64
    for i = 0; i < 1000; i++ {
        root = AvlInsert(root, i, i)
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
        root = AvlInsert(root, i, nil)
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
        root = AvlInsert(root, i, nil)
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

func TestGetHeight(t *testing.T) {
    var root *AvlNode
    root = &AvlNode{key: 2, left: nil, right: nil, height: 1}
    root = AvlInsert(root, 10, nil)
    root = AvlInsert(root, 20, nil)
    if GetHeight(root) != 2 {
        t.Fail()
    }
    AvlInsert(root, 30, nil)
    if GetHeight(root) != 3 {
        t.Fail()
    }
}

func TestRightRotate(t *testing.T) {
    var root *AvlNode
    root = &AvlNode{key: 10, left: nil, right: nil, height: 1}
    root = AvlInsert(root, 9, nil)
    root = AvlInsert(root, 8, nil)
    if root.left.key != 8 || root.right.key != 10 || root.key !=9 {
        t.Fail()
    }
}

func TestLeftRotate(t *testing.T) {
    var root *AvlNode
    root = &AvlNode{key: 10, left: nil, right: nil, height: 1}
    root = AvlInsert(root, 11, nil)
    root = AvlInsert(root, 12, nil)
    if root.left.key != 10 || root.right.key != 12 || root.key != 11 {
        t.Fail()
    }
}

func TestLeftRight(t *testing.T) {
    var root *AvlNode
    root = &AvlNode{key: 10, left: nil, right: nil, height: 1}
    root = AvlInsert(root, 5, nil)
    root = AvlInsert(root, 9, nil)
    if root.left.key != 5 || root.right.key != 10 || root.key != 9 {
        t.Fail()
    }
}

func TestRightLeft(t *testing.T) {
    var root *AvlNode
    root = &AvlNode{key: 10, left: nil, right: nil, height: 1}
    root = AvlInsert(root, 15, nil)
    root = AvlInsert(root, 13, nil)
    if root.left.key != 10 || root.right.key != 15 || root.key != 13{
        t.Fail()
    }
}
