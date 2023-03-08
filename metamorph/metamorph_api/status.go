package metamorph_api

import "github.com/glycerine/zebrapack/msgp"

func (x *Status) EncodeMsg(en *msgp.Writer) error {
	return en.WriteInt32(int32(*x))
}

func (x *Status) DecodeMsg(dc *msgp.Reader) error {
	xx, err := dc.ReadInt32()
	*x = Status(xx)
	return err
}

func (x Status) Msgsize() int {
	return msgp.Int32Size
}
