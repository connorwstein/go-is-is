package main 

import (
    "testing"
)

func TestAvlTreeInsert(t *testing.T) {
    var root *AvlNode
    root = &AvlNode{key: 10, left: nil, right: nil, height: 1}
    root = AvlInsert(root, 20)
    root = AvlInsert(root, 30)
    root = AvlInsert(root, 40)
    root = AvlInsert(root, 50)
    // Check balanced 
    if GetBalanceFactor(root) > 1 || GetBalanceFactor(root) < -1 {
        t.Fail()
    }
}

func TestGetHeight(t *testing.T) {
    var root *AvlNode
    root = &AvlNode{key: 2, left: nil, right: nil, height: 1}
    root = AvlInsert(root, 10)
    root = AvlInsert(root, 20)
    if GetHeight(root) != 2 {
        t.Fail()
    }
    AvlInsert(root, 30)
    if GetHeight(root) != 3 {
        t.Fail()
    }
}

func TestRightRotate(t *testing.T) {
    var root *AvlNode
    root = &AvlNode{key: 10, left: nil, right: nil, height: 1}
    root = AvlInsert(root, 9)
    root = AvlInsert(root, 8)
    if root.left.key != 8 || root.right.key != 10 || root.key !=9 {
        t.Fail()
    }
}

func TestLeftRotate(t *testing.T) {
    var root *AvlNode
    root = &AvlNode{key: 10, left: nil, right: nil, height: 1}
    root = AvlInsert(root, 11)
    root = AvlInsert(root, 12)
    if root.left.key != 10 || root.right.key != 12 || root.key != 11 {
        t.Fail()
    }
}

func TestLeftRight(t *testing.T) {
    var root *AvlNode
    root = &AvlNode{key: 10, left: nil, right: nil, height: 1}
    root = AvlInsert(root, 5)
    root = AvlInsert(root, 9)
    if root.left.key != 5 || root.right.key != 10 || root.key != 9 {
        t.Fail()
    }
}

func TestRightLeft(t *testing.T) {
    var root *AvlNode
    root = &AvlNode{key: 10, left: nil, right: nil, height: 1}
    root = AvlInsert(root, 15)
    root = AvlInsert(root, 13)
    if root.left.key != 10 || root.right.key != 15 || root.key != 13{
        t.Fail()
    }
}
