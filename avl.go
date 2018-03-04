package main

import (
       "fmt"
)

type AvlNode struct {
	key uint64
	left *AvlNode 
	right *AvlNode
    data interface{}
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

func AvlInsert(root *AvlNode, key uint64, data interface{}) *AvlNode {
    // Standard binary search tree insert, but we also rebalance as we go
    if root == nil {
        return &AvlNode{key: key, left: nil, right: nil, height: 1, data: data}
    }
    if key < root.key {
        root.left = AvlInsert(root.left, key, data)
    } else {
        root.right = AvlInsert(root.right, key, data)
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

func AvlDelete(root *AvlNode, key uint64) *AvlNode {
    if root == nil {
        return root // Tree already empty
    }
    if key < root.key {
        root.left = AvlDelete(root.left, key)
    } else if key > root.key {
        root.right = AvlDelete(root.right, key)
    } else {
        // Found the node to be deleted
        // 3 cases to handle: no children, one child or two children
        // Property of BSTs which is good to know: the predecessor is always the left subtree's right-most child
        // and the successor is always the right subtree's left-most child
        if root.left == nil && root.right == nil {
            root = nil
        } else if root.left == nil {
            root = root.right
        } else if root.right == nil {
            root = root.left
        } else {
            // Two children case
            var tmp *AvlNode = root.right
            for tmp.left != nil {
                tmp = tmp.left
            }
            root.key = tmp.key
            root.right = AvlDelete(root.right, tmp.key) 
        }
    }
    if root == nil {
        return root
    }
    root.height = 1 + Max(GetHeight(root.left), GetHeight(root.right))
    bf := GetBalanceFactor(root) 
    if bf > 1 && GetBalanceFactor(root.left) >= 0 {
        // Handle left-left rotation
        return RotateRight(root)
    } 
    if bf > 1 && GetBalanceFactor(root.left) < 0 { 
        // Handle left-right rotation
        root.left = RotateLeft(root.left)
        return RotateRight(root)
    }
    if bf < -1 && GetBalanceFactor(root.right) <= 0 { 
        // Handle right-right rotation
        return RotateLeft(root)
    }
    if bf < -1 && GetBalanceFactor(root.right) > 0 { 
        // Handle right-left rotation
        root.right = RotateRight(root.right)
        return RotateLeft(root)
    }
    return root // Return the unchanged pointer
}

func AvlSearch(root *AvlNode, key uint64) interface{} {
    // Same as a normal binary search tree search
    // If the key is greater than root.key, search right subtree, smaller - search left subtree
    if root == nil {
        // Unable to find key
        return nil
    }
    if root.key == key {
        return root.data
    } else if root.key > key {
        return AvlSearch(root.left, key)
    } else {
        return AvlSearch(root.right, key)
    }
}

func AvlUpdate(root *AvlNode, key uint64, newData interface {}) {
    // Find the key, replace the data
    if root == nil {
        // Unable to find key
        return
    }
    if root.key == key {
        root.data = newData
    } else if root.key > key {
        AvlUpdate(root.left, key, newData)
    } else {
        AvlUpdate(root.right, key, newData)
    }
}

func AvlPrint(node *AvlNode){
    // Find a way to pretty print the nodes
    if node != nil {
        AvlPrint(node.left) 
        fmt.Printf("Key: %v Height: %v Left: %v Right: %v\n", node.key, node.height, node.left, node.right)
        AvlPrint(node.right)
    }
}
