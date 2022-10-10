package handler

import (
	"github.com/taal/mapi/bitcoin"
	"github.com/taal/mapi/config"
)

type Options func(c *handlerOptions)

type (
	handler struct {
		options *handlerOptions
	}

	handlerOptions struct {
		minerID string
		nodes   []*bitcoin.Node
	}
)

// WithNode sets a single node to use in calls
func WithNode(node *bitcoin.Node) Options {
	return func(o *handlerOptions) {
		o.nodes = []*bitcoin.Node{
			node,
		}
	}
}

// WithNodes sets multiple nodes to use in calls
func WithNodes(nodes []bitcoin.Node) Options {
	return func(o *handlerOptions) {
		o.nodes = []*bitcoin.Node{}
		for _, node := range nodes {
			addNode := node
			o.nodes = append(o.nodes, &addNode)
		}
	}
}

// WithBitcoinRestServer sets a single node to use in calls
func WithBitcoinRestServer(server string) Options {
	return func(o *handlerOptions) {
		node := bitcoin.NewRest(server)
		o.nodes = []*bitcoin.Node{
			&node,
		}
	}
}

// WithBitcoinRestServers sets multiple nodes to use in calls
func WithBitcoinRestServers(servers []string) Options {
	return func(o *handlerOptions) {
		o.nodes = []*bitcoin.Node{}
		for _, nodeEndpoint := range servers {
			node := bitcoin.NewRest(nodeEndpoint)
			o.nodes = append(o.nodes, &node)
		}
	}
}

// WithBitcoinRPCServer sets a single node to use in calls
func WithBitcoinRPCServer(config *config.RPCClientConfig) Options {
	return func(o *handlerOptions) {
		node := bitcoin.NewRPC(config)
		o.nodes = []*bitcoin.Node{
			&node,
		}
	}
}

// WithBitcoinRPCSServers sets multiple rpc servers to use in calls
func WithBitcoinRPCSServers(servers []string) Options {
	return func(o *handlerOptions) {
		o.nodes = []*bitcoin.Node{}
		for _, nodeEndpoint := range servers {
			node := bitcoin.NewRest(nodeEndpoint)
			o.nodes = append(o.nodes, &node)
		}
	}
}

// WithMinerID sets the miner ID to use in all calls
func WithMinerID(minerID string) Options {
	return func(o *handlerOptions) {
		o.minerID = minerID
	}
}
