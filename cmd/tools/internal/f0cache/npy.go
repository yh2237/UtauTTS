package f0cache

import (
	"encoding/binary"
	"fmt"
	"io"
	"strings"
)

// WriteNPYは既存キャッシュに合わせてヘッダを16または64バイト境界にそろえる。
func WriteNPY(w io.Writer, x []float64, alignment int) error {
	if alignment != 16 && alignment != 64 {
		return fmt.Errorf("unsupported NPY header alignment %d", alignment)
	}
	header := "{'descr': '<f8', 'fortran_order': False, 'shape': (" + fmt.Sprint(len(x)) + ",), }"
	pad := alignment - (10+len(header)+1)%alignment
	header += strings.Repeat(" ", pad) + "\n"
	pre := append([]byte("\x93NUMPY\x01\x00"), byte(len(header)), byte(len(header)>>8))
	pre = append(pre, header...)
	if _, err := w.Write(pre); err != nil {
		return err
	}
	return binary.Write(w, binary.LittleEndian, x)
}
