package processor

func packRPCMessage(name string, data []byte) []byte {
	nameLen := len(name)
	dataLen := len(data)
	buf := make([]byte, nameLen+dataLen+2)
	buf[0] = byte(nameLen >> 8)
	buf[1] = byte(nameLen)
	copy(buf[2:], name)
	copy(buf[nameLen+2:], data)
	return buf
}

func unpackRPCMessage(buf []byte) (string, []byte) {
	nameLen := int(buf[0])<<8 | int(buf[1])
	name := string(buf[2 : 2+nameLen])
	data := buf[2+nameLen:]
	return name, data
}
