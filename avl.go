package main

import (
       "fmt"
)

type AvlNode struct {
	key int
	left *AvlNode 
	right *AvlNode
    height int
}

func Max(a, b int) int {
    if a > b {
        return a
    }
    return b
}

func GetHeight(node *AvlNode) int {
    if node == nil {
        return 0
    }
    return node.height
}

func GetBalanceFactor(node *AvlNode) int {
    // Determine the balance factor of this node
    // height of left subtree - height of right subtree
    if node == nil {
        return 0
    }
    return GetHeight(node.left) - GetHeight(node.right)
}

func RotateRight(node *AvlNode) *AvlNode {
    // Ex:   z            y 
    //      /           /  \
    //     y     -->   x    z 
    //   /  \              /
    //  x    T1           T1
    // node is z
    y := node.left 
    T1 := y.right
    y.right = node
    node.left = T1
    node.height = Max(GetHeight(node.left), GetHeight(node.right)) + 1
    y.height = Max(GetHeight(y.left), GetHeight(y.right)) + 1
    return y
}

func RotateLeft(node *AvlNode) *AvlNode {
    // Ex: z                  y
    //      \                / \
    //       y     -->      z   x
    //     /  \              \
    //    T1   x             T1
    y := node.right
    T1 := y.left
    y.left = node
    node.right = T1
    node.height = Max(GetHeight(node.left), GetHeight(node.right)) + 1
    y.height = Max(GetHeight(y.left), GetHeight(y.right)) + 1
    return y
}

func AvlInsert(root *AvlNode, key int) *AvlNode {
    // Standard binary search tree insert, but we also rebalance as we go
    if root == nil {
        return &AvlNode{key: key, left: nil, right: nil, height: 1}
    }
    if key < root.key {
        root.left = AvlInsert(root.left, key)
    } else {
        root.right = AvlInsert(root.right, key)
    }
    // Update height of the ancestor node
    // Get the balance factor of this ancestor node (height left subtree - height right subtree)
    // Using balance factor, do the necessary tree rotations
    root.height = 1 + Max(GetHeight(root.left), GetHeight(root.right))
    bf := GetBalanceFactor(root) 
    // bf 2 means left tree is greater than the right tree by 2 
    // if the key to be inserted is also less than the left child's key
    // This is the left-left case
    if bf > 1 && key < root.left.key {
        // Handle left-left rotation
        return RotateRight(root)
    } 
    if bf > 1 && key > root.left.key { 
        // Handle left-right rotation
        root.left = RotateLeft(root.left)
        return RotateRight(root)
    }
    if bf < -1 && key > root.right.key { 
        // Handle right-right rotation
        return RotateLeft(root)
    }
    if bf < -1 && key < root.right.key { 
        // Handle right-left rotation
        root.right = RotateRight(root.right)
        return RotateLeft(root)
    }
    return root // Return the unchanged pointer
}

func AvlPrint(node *AvlNode){
    // Find a way to pretty print the nodes
    if node != nil {
        AvlPrint(node.left) 
        fmt.Printf("Key: %v Height: %v Left: %v Right: %v\n", node.key, node.height, node.left, node.right)
        AvlPrint(node.right)
    }
}
