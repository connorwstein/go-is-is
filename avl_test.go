package main

import (
	"fmt"
	"testing"
)

type DummyData struct {
	data int
}

func (d DummyData) String() string {
	return fmt.Sprintf("%d", d.data)
}

func TestAvlGetAll(t *testing.T) {
	// Should produce an in-order traversal
	var root *AvlNode
	root = &AvlNode{key: 10, left: nil, right: nil, height: 1, data: &DummyData{}}
	root = AvlInsert(root, 20, &DummyData{data: 20}, false)
	root = AvlInsert(root, 30, &DummyData{data: 30}, false)
	nodes := AvlGetAll(root)
	if nodes[0].key != 10 && nodes[1].key != 20 && nodes[2].key != 30 {
		t.Fail()
	}
}

func TestAvlInsertDup(t *testing.T) {
	// Should only have one element
	var root *AvlNode
	root = &AvlNode{key: 10, left: nil, right: nil, height: 1, data: &DummyData{}}
	root = AvlInsert(root, 10, &DummyData{data: 10}, false)
	root = AvlInsert(root, 10, &DummyData{data: 10}, false)
	nodes := AvlGetAll(root)
	if len(nodes) != 1 {
		t.Fail()
	}
}

func TestAvlSearch(t *testing.T) {
	var root *AvlNode
	root = &AvlNode{key: 10, left: nil, right: nil, height: 1}
	var i uint64
	for i = 0; i < 1000; i++ {
		root = AvlInsert(root, i, &DummyData{data: int(i)}, false)
	}
	tmp1 := AvlSearch(root, 578)
	tmp2 := AvlSearch(root, 1200)
	if tmp1.String() != "578" || tmp2 != nil {
		t.Fail()
	}
}

func TestAvlInsert(t *testing.T) {
	var root *AvlNode
	root = &AvlNode{key: 10, left: nil, right: nil, height: 1, data: &DummyData{}}
	var i uint64
	for i = 0; i < 1000; i++ {
		root = AvlInsert(root, i, nil, false)
	}
	// Check balanced
	b := GetBalanceFactor(root)
	if b > 1 || b < -1 {
		t.Fail()
	}
}

func TestAvlDelete(t *testing.T) {
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
	if root.left.key != 8 || root.right.key != 10 || root.key != 9 {
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
	if root.left.key != 10 || root.right.key != 15 || root.key != 13 {
		t.Fail()
	}
}

func TestAvlOverwrite(t *testing.T) {
	var root *AvlNode
	root = &AvlNode{key: 10, left: nil, right: nil, height: 1}
	root = AvlInsert(root, 12, &DummyData{data: 10}, false)
	root = AvlInsert(root, 15, &DummyData{data: 10}, false)
	root = AvlInsert(root, 15, &DummyData{data: 12}, true)
	nodes := AvlGetAll(root)
	if len(nodes) != 3 || nodes[2].data.String() != "12" {
		t.Fail()
	}
}
