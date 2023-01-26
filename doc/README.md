# ARC
> Transaction processor for Bitcoin

## Overview

ARC is a transaction processor for Bitcoin. It consists of four microservices: [API](#API), [Metamorph](#Metamorph), [BlockTx](#BlockTx) and [Callbacker](#Callbacker), which are all described below.

All the microservices are written in Go, and use grpc and protobufs for internal communications.

All the microservices are designed to be horizontally scalable, and can be deployed on a single machine or on multiple machines. Each one has been programmed with a store interface and various databases can be used to store data. The default store is sqlite3, but any database that implements the store interface can be used.

```plantuml
@startuml
hide footbox
skinparam ParticipantPadding 15
skinparam BoxPadding 10

actor "client" as tx

box api server
participant handler
participant auth
participant validator
end box

box metamorph
participant grpc
participant worker
database "transient\nstore" as store
participant "peer\nserver" as peer
participant "zmq\nlistener" as zmq
participant "activity\nqueue" as aqueue
end box

database "bitcoin\nnetwork" as bsv


title Submit transaction via P2P

tx -> handler ++: extended\nraw tx

handler -> auth ++: apikey
return

handler -> validator ++: tx
return success

handler -> grpc ++: tx

    grpc -> worker ++: tx
      worker -> store++: register txid
      worker -> store: tx
    return STORED

    worker -> peer: txid

    peer -> bsv: inv txid
    peer -> store: ANNOUNCED

    store -> worker: ANNOUNCED


    bsv -> peer++: getdata txid
      peer -> store: REQUESTED
      store -> worker: REQUESTED
      peer -> store ++ : get tx
      return raw tx
      
    return tx
      
    peer -> store: SENT
    
    store -> worker: SENT


  
    bsv -> zmq: txid
    zmq -> store: ACCEPTED

    store -> worker: ACCEPTED

    worker -> aqueue: save activity
    return ACCEPTED


grpc -> grpc: wait for ACCEPTED\nor TIMEOUT

return ACCEPTED



return ACCEPTED

@enduml
```

## Extended format

For optimal performance, ARC uses a custom format for transactions. This format is called the extended format, and is a superset of the raw transaction format. The extended format includes the satoshis and scriptPubKey for each input, which makes it possible to validate the transaction without having to download the parent transactions.

The extended format has been described in detail in [BIP-239](./BIP-239.MD).

## Settings

## Microservices

### API

API is a REST API for the ARC.

### Metamorph

#### Metamorph stores

### BlockTx

BlockTx is a microservice that is responsible for processing blocks mined on the Bitcoin network, and for propagating the status of transactions to all Metamorphs connected to the server.

#### BlockTx stores

### Callbacker

Callbacker is a very simple microservice that is responsible for sending callbacks to clients when a transaction has been accepted by the Bitcoin network. To register a callback, the client must add the `X-Callback-Url` header to the request. The callbacker will then send a POST request to the URL specified in the header, with the transaction ID in the body. See the [API documentation](#API) for more information.

#### Callbacker stores

## Client Libraries

### Go

WIP

### Javascript

WIP
