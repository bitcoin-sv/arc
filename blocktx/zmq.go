package blocktx

import (
	"github.com/ordishs/go-bitcoin"
	"github.com/ordishs/gocore"
)

type ZMQ struct {
	processor *Processor
	logger    *gocore.Logger

	host string
	port int
}

func NewZMQ(processor *Processor, host string, port int) *ZMQ {
	return &ZMQ{
		processor: processor,
		logger:    gocore.Log("zmq"),
		host:      host,
		port:      port,
	}
}

func (z *ZMQ) Start() {
	z.logger.Infof("Listening to ZMQ on %s:%d", z.host, z.port)
	zmq := bitcoin.NewZMQ(z.host, z.port, z.logger)

	ch := make(chan []string)

	go func() {
		for c := range ch {
			switch c[0] {
			case "hashblock":
				z.processor.ProcessBlock(c[1])
			default:
				z.logger.Info("Unhandled ZMQ message", c)
			}
		}
	}()

	if err := zmq.Subscribe("hashblock", ch); err != nil {
		z.logger.Fatal(err)
	}

}
