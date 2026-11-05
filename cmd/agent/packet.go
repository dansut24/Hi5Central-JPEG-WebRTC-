package main

import "encoding/binary"

const chunkSize = 12 * 1024
const packetHeaderSize = 12

// Header: frameID uint32, chunkIndex uint16, chunkCount uint16, JPEG byte length uint32.
// Values use little-endian encoding.
func framePackets(frameID uint32, jpeg []byte) [][]byte {
    count := (len(jpeg) + chunkSize - 1) / chunkSize
    if count == 0 || count > 65535 { return nil }
    packets := make([][]byte, 0, count)
    for i := 0; i < count; i++ {
        from := i*chunkSize
        to := from + chunkSize
        if to > len(jpeg) { to = len(jpeg) }
        p := make([]byte, packetHeaderSize+to-from)
        binary.LittleEndian.PutUint32(p[:4], frameID)
        binary.LittleEndian.PutUint16(p[4:6], uint16(i))
        binary.LittleEndian.PutUint16(p[6:8], uint16(count))
        binary.LittleEndian.PutUint32(p[8:12], uint32(len(jpeg)))
        copy(p[12:], jpeg[from:to])
        packets = append(packets,p)
    }
    return packets
}
