package metamorph

import (
	"fmt"
	"testing"
)

func TestSomethingThatUsesZMQI(t *testing.T) {

	// make and configure a mocked ZMQI
	mockedZMQI := &ZMQIMock{
		SubscribeFunc: func(s string, stringsCh chan []string) error {
			stringsCh <- []string{mda}
			return nil
		},
	}

	statuses := make(chan *PeerTxMessage, 1)
	zmq := NewZMQ(nil, statuses)
	zmq.Start(mockedZMQI)
	status := <-statuses
	fmt.Println(status)
}
