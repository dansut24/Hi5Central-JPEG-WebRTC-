package main

import ("bytes";"encoding/binary";"testing")

func TestFramePackets(t *testing.T) {
  input:=bytes.Repeat([]byte{0xab},26*1024)
  packets:=framePackets(42,input)
  if len(packets)!=3 { t.Fatalf("got %d chunks",len(packets)) }
  out:=make([]byte,0,len(input))
  for i,p:=range packets {
    if binary.LittleEndian.Uint32(p[:4])!=42 ||
       int(binary.LittleEndian.Uint16(p[4:6]))!=i ||
       int(binary.LittleEndian.Uint16(p[6:8]))!=3 ||
       int(binary.LittleEndian.Uint32(p[8:12]))!=len(input) {
       t.Fatal("invalid JPEG frame packet")
    }
    out=append(out,p[12:]...)
  }
  if !bytes.Equal(input,out) {t.Fatal("JPEG roundtrip mismatch")}
}
