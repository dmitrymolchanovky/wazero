package binary

import (
	"bytes"
	"fmt"
	"io"

	"github.com/tetratelabs/wazero/internal/leb128"
	"github.com/tetratelabs/wazero/internal/wasm"
)

func decodeDataSegment(r *bytes.Reader) (*wasm.DataSegment, error) {
	d, _, err := leb128.DecodeUint32(r)
	if err != nil {
		return nil, fmt.Errorf("read memory index: %v", err)
	}

	if d != 0 {
		return nil, fmt.Errorf("invalid memory index: %d", d)
	}

	expr, err := decodeConstantExpression(r)
	if err != nil {
		return nil, fmt.Errorf("read offset expression: %v", err)
	}

	vs, _, err := leb128.DecodeUint32(r)
	if err != nil {
		return nil, fmt.Errorf("get the size of vector: %v", err)
	}

	b := make([]byte, vs)
	if _, err := io.ReadFull(r, b); err != nil {
		return nil, fmt.Errorf("read bytes for init: %v", err)
	}

	return &wasm.DataSegment{
		OffsetExpression: expr,
		Init:             b,
	}, nil
}

func encodeDataSegment(d *wasm.DataSegment) (ret []byte) {
	// Currently multiple memories are not supported.
	ret = append(ret, leb128.EncodeInt32(0)...)
	ret = append(ret, encodeConstantExpression(d.OffsetExpression)...)
	ret = append(ret, leb128.EncodeUint32(uint32(len(d.Init)))...)
	ret = append(ret, d.Init...)
	return
}
