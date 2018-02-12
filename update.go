// Receive LSPs and flood them, also
// generate our own LSPs
// The update db should be an AVL tree where the keys are LSP IDs the values contain
// the actual LSP, let's use just a slice for now
package main

func update_input(intf *Intf) {
    // Don't do anything if there is no adjacency up on this interface 
    // Use a channel to signal that the adjacency as come up
    // recv_frame(intf.name) blocks until something available on that interface
}

func update(intf *Intf) {
    // Send out LSP updates via intf
    // send_frame(payload, intf.name)
}

func generate_local_lsp() {
    // Create a local_lsp based on the information stored in the adjacency database
    // Changes the adjacency information triggers this
}



