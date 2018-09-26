package main

import (
	"bytes"
	"testing"
)

func TestParseTLVs(t *testing.T) {
	rawBytes := []byte{0x01, 0x02, 0x01, 0x01} // Single TLV
	tlv := parseTLVs(rawBytes, 0)
	t.Log("TLV: %v", tlv)
	if !bytes.Equal(tlv.valueTLV, []byte{0x01, 0x01}) {
		t.Fail()
	}
	// Multi-TLV
	rawBytes = []byte{0x01, 0x02, 0x01, 0x01, 0x02, 0x02, 0x01, 0x01, 0x03, 0x02, 0x01, 0x01} // Single TLV
	multiTLV := parseTLVs(rawBytes, 0)
	t.Logf("TLV: %v %v %v", multiTLV, multiTLV.nextTLV, multiTLV.nextTLV.nextTLV)
	if !(bytes.Equal(multiTLV.valueTLV, []byte{0x01, 0x01}) &&
		bytes.Equal(multiTLV.nextTLV.valueTLV, []byte{0x01, 0x01}) &&
		bytes.Equal(multiTLV.nextTLV.nextTLV.valueTLV, []byte{0x01, 0x01})) {
		t.Fail()
	}
}
