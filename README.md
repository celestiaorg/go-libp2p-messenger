# Messenger
[![build-img]][build-url]
[![pkg-img]][pkg-url]
[![reportcard-img]][reportcard-url]
[![coverage-img]][coverage-url]

Messenger provides a simple arbitrary message sending API to multiple peers for [libp2p](https://github.com/libp2p/go-libp2p)-based protocols. The main
purpose is to bootstrap development for new protocols and decrease boilerplate code.

## Features
* Simple non-blocking API
* Not a protocol - sends only user's data without message wrapping on the wire
* Not a framework - use it anyhow
* Integrates generics in a clean way
* Almost zero allocations API
* Hides away stream management(re-creation, duplicates, lifecycle)

## Background
The libp2p provides all necessary primitives to build custom fully 
decentralized p2p protocols. Also, it is solely stream-based for reasons, but in practice, the vast majority of libp2p
based protocols are message-based, with messages sent over reliable and multiplexed streams.

Mainly, there are two typical patterns for message sending over streams.

### Request/Response
The most popular pattern in libp2p protocols is sending requests over a fresh stream and waiting for a response with a 
subsequent closing. Using libp2p's bare Stream API is trivial for such a case, which also explains why it is the most 
popular pattern.

### 'Fire and Forget'
Another less common pattern in libp2p protocols is sending messages without expecting a response. Once instantiated, a
protocol establishes a unique and persistent stream with a remote endpoint and only sends messages over it. This pattern
optimizes protocols by removing unnecessary stream negotiations, which are primarily expensive for round trips.

## Motivations
* Provide reusable utility for common use-cases in libp2p protocol development.
  * Even for simple Request/Response pattern, there is some level of boilerplate that can be avoided
    (Examples: [Bitswap](https://github.com/ipfs/go-bitswap/blob/master/network/ipfs_impl.go#L96),
    [kadDHT](https://github.com/libp2p/go-libp2p-kad-dht/blob/master/crawler/crawler.go#L56))
    > Currently, the Messenger does not provide a Request/Response semantics, and the main focus is 'fire and forget'.
  * 'Fire and Forget' is more complex. It can be implemented in multiple ways, has undesirable edge cases, and requires 
    a more profound knowledge of the libp2p stack. Although the pattern is an interesting engineering challenge to solve,
    in business and rapidly changing environments, it is easier to reuse existing instruments, like Messenger.
* Lower entrance threshold for new developers exploring decentralization and p2p networking.
* Bootstrap migration of [Tendermint](https://github.com/tendermint/tendermint) over [libp2p](https://github.com/libp2p/go-libp2p).
  
  Tendermint is the first and most popular BFT consensus implementation, and it is decentralized and requires p2p 
  networking. The project's team, through time, developed an in-house solution for it. However, they are now committed 
  to migration over libp2p.

  Tendermint is a towering stack of multiple sub-protocols(or their terms - reactors) which rely on patterns explained 
  above. Therefore, a lesser code boilerplate and simple API would be convenient for such migration.
  > Technically, Tendermint p2p stack only supports 'fire and forget' pattern, but a few sub-protocols can benefit from
  > migration to Request/Response pattern.
  
## Documentation

See [these docs][pkg-url].

## Contributing

This repo follows [conventional commits](https://www.conventionalcommits.org/en/v1.0.0/), so PR titles should start with `fix:`, `feat:`, `build:`, `chore:`, `ci:`, `docs:`, `style:`, `refactor:`, `perf:`, or `test:`. Include an `!` (e.g. `feat!:`) if the PR includes a breaking change.

PR titles must start the subject with `[high]`, `[medium]`, or `[low]` to indicate the review thoroughness needed. For example: `fix!: [high] handle concurrent stream resets` or `docs: [low] clarify usage`. Include the same tag in the PR description and explain why you chose it.

- `[high]`: Deep review for message framing/serialization, stream handling/concurrency, peer management, security, or other changes with significant risk.
- `[medium]`: Standard review for code changes with limited impact and well-understood behavior.
- `[low]`: Focused review for straightforward docs, tests, or tooling changes with low risk.

Choose the higher level when unsure. A small diff can still need deep review. CI checks the title format; reviewers should confirm the chosen level.

## License

[Apache-2.0](LICENSE).

[build-img]: https://github.com/celestiaorg/go-libp2p-messenger/actions/workflows/go.yml/badge.svg
[build-url]: https://github.com/celestiaorg/go-libp2p-messenger/actions
[pkg-img]: https://pkg.go.dev/badge/celestiaorg/go-libp2p-messenger
[pkg-url]: https://pkg.go.dev/github.com/celestiaorg/go-libp2p-messenger/
[reportcard-img]: https://goreportcard.com/badge/celestiaorg/go-libp2p-messenger
[reportcard-url]: https://goreportcard.com/report/celestiaorg/go-libp2p-messenger
[coverage-img]: https://codecov.io/gh/celestiaorg/go-libp2p-messenger/branch/main/graph/badge.svg
[coverage-url]: https://codecov.io/gh/celestiaorg/go-libp2p-messenger

