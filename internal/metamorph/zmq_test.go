package metamorph_test

import (
	"context"
	"github.com/bitcoin-sv/arc/internal/metamorph"
	"github.com/bitcoin-sv/arc/internal/metamorph/bcnet/metamorph_p2p"
	"github.com/bitcoin-sv/arc/internal/metamorph/metamorph_api"
	"github.com/bitcoin-sv/arc/internal/metamorph/mocks"
	"github.com/bitcoin-sv/arc/internal/testdata"
	"github.com/go-zeromq/zmq4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"log/slog"
	"net/url"
	"os"
	"sync"
	"testing"
	"time"
)

var endpoint = "tcp://127.0.0.1:5555"
var zmqEndpointURL, _ = url.Parse(endpoint)
var zmqURLDOWN, _ = url.Parse("tcp://URLDOWN:5556")
var statusMessageCh chan *metamorph_p2p.TxStatusMessage
var zmqMessages = make(chan []string, 10)

// var validTopics = []string{"hashblock", "hashblock2", "hashtx", "discardedfrommempool", "hashtx2", "invalidtx", "rawblock", "rawblock2", "rawtx", "rawtx2", "removedfrommempoolblock"}
var topic = "my-topic"
var message = "Hello, world!"
var msgFromFrames = zmq4.NewMsgFrom([]byte(topic), []byte(message))

const (
	msgMissingInputs = "7b2266726f6d426c6f636b223a2066616c73652c22736f75726365223a2022703270222c2261646472657373223a20223132372e302e302e313a3135323234222c226e6f64654964223a2037303139352c2274786964223a202234616531643230396131616165326134616137303365326164646166393133356634613162316364306438373032303033376561353631396434393566373137222c2273697a65223a203139322c22686578223a2022303130303030303030316133376534386661616133613438353966363233313631353035336534323930396532383734646537643738346666376133303430303030303030303030303030313030303030303662343833303435303232313030653530373933303264636632626336326635326265653862333031626565313666373538396336343934613638383337313065343631633439383334656266333032323037663933663361636563626566626465373965343763653334336330663763653731373866323338313166316262303566356234323763313931323630373431343132313033386662386464386534336664663664353036333339623335623563326234396435303930646538613637343239376437346463333838663766333164646631346666666666666666303133373033303030303030303030303030313937366139313435306331663839393031336364353030363231666131366533613932326334663035616261346164383861633030303030303030222c226973496e76616c6964223a20747275652c22697356616c69646174696f6e4572726f72223a2066616c73652c2269734d697373696e67496e70757473223a20747275652c226973446f75626c655370656e644465746563746564223a2066616c73652c2269734d656d706f6f6c436f6e666c6963744465746563746564223a2066616c73652c2269734e6f6e46696e616c223a2066616c73652c22697356616c69646174696f6e54696d656f75744578636565646564223a2066616c73652c2269735374616e646172645478223a20747275652c2272656a656374696f6e436f6465223a20302c2272656a656374696f6e526561736f6e223a2022222c22636f6c6c6964656457697468223a205b5d2c2272656a656374696f6e54696d65223a2022323032332d31312d31335431333a33393a32365a227d"
	// msgDoubleSpendAttempted has 1 competing tx in the msg
	msgDoubleSpendAttempted = "7b2266726f6d426c6f636b223a2066616c73652c22736f75726365223a2022703270222c2261646472657373223a20226e6f6465323a3138333333222c226e6f64654964223a20312c2274786964223a202238653735616531306638366438613433303434613534633363353764363630643230636462373465323333626534623563393062613735326562646337653838222c2273697a65223a203139312c22686578223a202230313030303030303031313134386239653931646336383232313635306539363861366164613863313531373135656135373864623130376336623563333362363762376636376630323030303030303030366134373330343430323230313863396166396334626634653736383932376263363335363233623434383362656261656334343433396165613838356363666430363163373731636435613032323034613839626531333534613038613539643466316636323235343937366532373466316333333334383334373137363462623936633565393837626539663365343132313033303830373637393438326663343533323461386133326166643832333730646337316365383966373936376536636635646139646430356330366665356137616666666666666666303130613030303030303030303030303030313937366139313434613037363038353032653464646131363662333830343130613633663066653962383830666532383861633030303030303030222c226973496e76616c6964223a20747275652c22697356616c69646174696f6e4572726f72223a2066616c73652c2269734d697373696e67496e70757473223a2066616c73652c226973446f75626c655370656e644465746563746564223a2066616c73652c2269734d656d706f6f6c436f6e666c6963744465746563746564223a20747275652c2269734e6f6e46696e616c223a2066616c73652c22697356616c69646174696f6e54696d656f75744578636565646564223a2066616c73652c2269735374616e646172645478223a20747275652c2272656a656374696f6e436f6465223a203235382c2272656a656374696f6e526561736f6e223a202274786e2d6d656d706f6f6c2d636f6e666c696374222c22636f6c6c6964656457697468223a205b7b2274786964223a202264363461646663653662313035646336626466343735343934393235626630363830326134316130353832353836663333633262313664353337613062376236222c2273697a65223a203139312c22686578223a202230313030303030303031313134386239653931646336383232313635306539363861366164613863313531373135656135373864623130376336623563333362363762376636376630323030303030303030366134373330343430323230376361326162353332623936303130333362316464636138303838353433396366343433666264663262616463656637303964383930616434373661346162353032323032653730666565353935313462313763353635336138313834643730646232646363643062613339623731663730643239386231643939313764333837396663343132313033303830373637393438326663343533323461386133326166643832333730646337316365383966373936376536636635646139646430356330366665356137616666666666666666303130613030303030303030303030303030313937366139313435313335306233653933363037613437616136623161653964343937616336656135366130623132383861633030303030303030227d5d2c2272656a656374696f6e54696d65223a2022323032342d30372d32355431313a30313a35365a227d"
	ZmqFirstTopic           = "hashblock"
)

func TestZMQ(t *testing.T) {
	testCases := []struct {
		name                  string
		eventTopic            string
		eventMsg              string
		expectedStatus        metamorph_api.Status
		expectedStatusesCount int // double spend will return multiple statuses for all competing txs
	}{
		{
			name:                  "invalidtx - missing inputs",
			eventTopic:            "invalidtx",
			eventMsg:              msgMissingInputs,
			expectedStatus:        metamorph_api.Status_SEEN_IN_ORPHAN_MEMPOOL,
			expectedStatusesCount: 1,
		},
		{
			name:                  "invalidtx - double spend",
			eventTopic:            "invalidtx",
			eventMsg:              msgDoubleSpendAttempted,
			expectedStatus:        metamorph_api.Status_DOUBLE_SPEND_ATTEMPTED,
			expectedStatusesCount: 2, // one for the competing tx
		},
		{
			name:                  "hashtx2 - valid",
			eventTopic:            "hashtx2",
			eventMsg:              testdata.TX1Hash.String(),
			expectedStatus:        metamorph_api.Status_ACCEPTED_BY_NETWORK,
			expectedStatusesCount: 1,
		},
	}

	for _, tc := range testCases {
		// given
		mockedZMQ := &mocks.ZMQIMock{
			SubscribeFunc: func(s string, stringsCh chan []string) error {
				if s != tc.eventTopic {
					return nil
				}
				event := make([]string, 0)
				event = append(event, tc.eventTopic)
				event = append(event, tc.eventMsg)
				event = append(event, "2459")
				stringsCh <- event
				return nil
			},
		}

		statuses := make(chan *metamorph_p2p.TxStatusMessage, tc.expectedStatusesCount)

		zmqURL, err := url.Parse("https://some-url.com")
		require.NoError(t, err)

		sut, err := metamorph.NewZMQ(zmqURL, statuses, mockedZMQ, slog.Default())
		require.NoError(t, err)

		// when
		cleanup, err := sut.Start()
		require.NoError(t, err)
		defer cleanup()

		// then
		var status *metamorph_p2p.TxStatusMessage
		sCounter := 0
		for i := 0; i < tc.expectedStatusesCount; i++ {
			select {
			case status = <-statuses:
				sCounter++
				assert.Equal(t, tc.expectedStatus, status.Status)
			case <-time.After(time.Second):
				t.Fatal("timed out waiting for status")
			}
		}

		assert.Equal(t, tc.expectedStatusesCount, sCounter)
	}
}

func TestZMQDoubleSpend(t *testing.T) {
	// given
	mockedZMQ := &mocks.ZMQIMock{
		SubscribeFunc: func(s string, stringsCh chan []string) error {
			if s != "invalidtx" {
				return nil
			}
			event := make([]string, 0)
			event = append(event, "invalidtx")
			event = append(event, msgDoubleSpendAttempted)
			event = append(event, "2459")
			stringsCh <- event
			return nil
		},
	}

	numberOfMsgs := 2
	hashes := []string{"8e75ae10f86d8a43044a54c3c57d660d20cdb74e233be4b5c90ba752ebdc7e88", "d64adfce6b105dc6bdf475494925bf06802a41a0582586f33c2b16d537a0b7b6"}

	statuses := make(chan *metamorph_p2p.TxStatusMessage, numberOfMsgs)

	zmqURL, err := url.Parse("https://some-url.com")
	require.NoError(t, err)

	sut, err := metamorph.NewZMQ(zmqURL, statuses, mockedZMQ, slog.Default())
	require.NoError(t, err)

	// when
	cleanup, err := sut.Start()
	require.NoError(t, err)
	defer cleanup()

	// then
	var status *metamorph_p2p.TxStatusMessage
	sCounter := 0
	for i := 0; i < numberOfMsgs; i++ {
		select {
		case status = <-statuses:
			sCounter++
			assert.Equal(t, metamorph_api.Status_DOUBLE_SPEND_ATTEMPTED, status.Status)
			assert.Equal(t, hashes[sCounter-1], status.Hash.String())
			t.Logf("hash: %s", status.Hash.String())
		case <-time.After(time.Second):
			t.Fatal("timed out waiting for status")
		}
	}

	assert.Equal(t, numberOfMsgs, sCounter)
}

func TestNewZMQHandler(t *testing.T) {

	// Test Case
	// Given I want to test metamorph handler
	// When I have a ZMQ publisher up
	// Then I want to make sure the handler can
	// subscribe and unsubscribe to all topics
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{}))
	var handler *metamorph.ZMQHandler
	var zmq *metamorph.ZMQ
	var wg sync.WaitGroup
	defer wg.Wait()
	handler = metamorph.NewZMQHandler(context.Background(), zmqEndpointURL, logger)
	assert.NotNil(t, handler)
	zmq, err := metamorph.NewZMQ(zmqEndpointURL, statusMessageCh, handler, logger)
	close, err := zmq.Start()
	if err != nil {
		logger.Error("failed to create ZMQ: %v")
	}
	logger.Info("Listening to ZMQ", slog.String("host", zmqEndpointURL.Hostname()), slog.String("port", zmqEndpointURL.Port()))
	wg.Add(1)
	go func() {

		err = handler.Subscribe("notvalid", zmqMessages)
		require.Error(t, err)

		err = handler.Unsubscribe("notvalid", zmqMessages)
		require.Error(t, err)

		err = handler.Subscribe("hashblock", zmqMessages)
		require.NoError(t, err)

		err = handler.Unsubscribe("hashblock", zmqMessages)
		require.NoError(t, err)

		err = handler.Subscribe("discardedfrommempool", zmqMessages)
		require.NoError(t, err)

		err = handler.Unsubscribe("discardedfrommempool", zmqMessages)
		require.NoError(t, err)

		err = handler.Subscribe("hashtx2", zmqMessages)
		require.NoError(t, err)

		err = handler.Unsubscribe("hashtx2", zmqMessages)
		require.NoError(t, err)

		err = handler.Subscribe("invalidtx", zmqMessages)
		require.NoError(t, err)

		err = handler.Unsubscribe("invalidtx", zmqMessages)
		require.NoError(t, err)
		wg.Done()
	}()
	wg.Wait()
	defer close()

}

func TestNewZMQHandlerStop(t *testing.T) {

	NewZMQHandlerStopPanic(t)

}

func NewZMQHandlerStopPanic(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{}))
	var handler *metamorph.ZMQHandler
	for i := 0; i < 5; i++ {
		handler = metamorph.NewZMQHandler(context.Background(), zmqEndpointURL, logger)
		assert.NotNil(t, handler)

	}

	defer func() {
		if r := recover(); r != nil {
			logger.Info("Recovered:", r)
		}
	}()

	panic("This is a panic")
}

func TestZMQHandlerNoErrorServiceDown(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	// Test Case
	// Given I want to test metamorph handler
	// When I am using a URL that is not valid or is down
	// Then I want to make sure that the appropriate error arises

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{}))
	handler := metamorph.NewZMQHandler(ctx, zmqURLDOWN, logger)
	zmqObj, err := metamorph.NewZMQ(zmqURLDOWN, statusMessageCh, handler, logger)
	if err != nil {
		logger.Error("failed to create ZMQ: %v")
	} else {
		logger.Info("No error was raised using a made up URL")
	}
	cleanup, err := zmqObj.Start()
	require.NoError(t, err)
	defer cleanup()

	err = handler.Subscribe(ZmqFirstTopic, zmqMessages)
	require.NoError(t, err)

	err = handler.Unsubscribe(ZmqFirstTopic, nil)
	require.NoError(t, err)
}

func TestZMQ4SendReceiveMessage(t *testing.T) {
	// Create a new subscriber socket.
	sub := zmq4.NewSub(context.Background())
	defer sub.Close()

	// Create a new publisher socket.
	pub := zmq4.NewPub(context.Background())
	defer pub.Close()

	//Set sub to subscribe mode
	if err := sub.SetOption(zmq4.OptionSubscribe, topic); err != nil {
		t.Fatalf("Subscription failed: %v", err)
	}

	// Bind the publisher to a port.
	if err := sub.Listen(endpoint); err != nil {
		t.Fatalf("Sub Dial failed: %v", err)
	}
	if err := pub.Dial(endpoint); err != nil {
		t.Fatalf("Pub Dial failed: %v", err)
	}
	var wg sync.WaitGroup
	defer wg.Wait()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	// Send a message to a specific topic
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			_ = pub.Send(msgFromFrames)
			if ctx.Err() != nil {
				return
			}
			time.Sleep(1 * time.Millisecond)
		}
	}()
	msg, err := sub.Recv()
	if err != nil {
		t.Fatalf("Recv failed: %v", err)
	}
	if string(msg.Frames[0]) != topic {
		t.Fatalf("invalid message received: got '%s', wanted '%s'", msg.Frames[0], message)
	}
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{}))
	logger.Info(string(msg.Frames[1]))
	err = sub.SetOption(zmq4.OptionUnsubscribe, topic)
	if err != nil {
		t.Fatalf("Unsubscribe failed: %v", err)
	}
}
