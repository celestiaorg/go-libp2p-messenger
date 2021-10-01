# Messenger
Messenger provides a simple API to send arbitrary messages to multiple peers over libp2p based protocols. It's main 
purpose is to bootstrap development for new protocols and decrease boilerplate code.

## Background
The [libp2p library](https://github.com/libp2p/go-libp2p) provides all necessary primitives to build custom fully 
decentralized p2p protocols. Also, its solely stream based for reasons, but in practice, the vast majority of libp2p 
based protocols are message based, with messages being sent over reliable and multiplexed streams. 

Mainly, there are two common pattern for message sending over stream.

### Request/Response
The most popular pattern in libp2p protocols is to send requests over a fresh stream, wait for a response from it with 
a subsequent closage. Using libp2p's bare Stream API is trivial for such case and this also explains why it is the most 
popular pattern.

### 'Fire and Forget'
Another less common pattern in libp2p protocols is sending messages without expecting a response. Once instantiated, a 
protocol establishes a unique and persistent stream with a remote side and only sends messages over it. This pattern
optimizes protocols by removing unnecessary stream negotiation which are expensive mostly for round-trips.

## Motivations
* Provide reusable utility for common use-cases in libp2p protocol development.
  * Even for simple Request/Response pattern, there is some level of boilerplating which can be avoided
    (Examples: [Bitswap](https://github.com/ipfs/go-bitswap/blob/master/network/ipfs_impl.go#L96), 
    [kadDHT](https://github.com/libp2p/go-libp2p-kad-dht/blob/master/crawler/crawler.go#L56))
    > Currently, the Messenger does not provide a Request/Response semantics, and the main focus is 'fire and forget'
  * 'Fire and Forget' is much more complex. It can be implemented in multiple ways, has undesirable edge-cases and 
    requires deeper knowledge of libp2p stack. This also an interesting engineering challenge to solved, but in business
    and rapidly changing environments, it is easier to reuse existing instrument, like Messenger. 
* Lower entrance threshold for new developers exploring decentralization and p2p networking.
* Bootstrap migration of [Tendermint](https://github.com/tendermint/tendermint) over [libp2p](https://github.com/libp2p/go-libp2p).
  
  Tendermint is the first and most popular BFT consensus implementation. It's decentralized and requires p2p networking,
  and the project team through time developed an in-house solution for it. However, they are recently committed to 
  migrate over libp2p.
  
  Tendermint is a towering stack of multiple sub-protocols(or reactors) which rely on patterns explained above. 
  Therefore, Lesser code boilerplating and simple API would be especially handy for such migration.

## API
> NOTE: It is not entirely stable and there are minor plans to change it slightly.

// TODO: Link to godoc.

## Design

// TODO: Link to design doc

