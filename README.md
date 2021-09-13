# Messenger
> Baking :baguette_bread:

A tool for libp2p message based protocols. It is a part of an effort to migrate 
[Tendermint](https://github.com/tendermint/tendermint) over [libp2p](https://github.com/libp2p/libp2p).

## Design

* Two persistent streams per peer: inbound, outbound.
  * Technically it is possible to have only one stream per peer for both inbound and outbound traffic,
  however, the logic would be more complicated for case where peers open streams simultaneously. In such case they would
  need to additionally decide whose stream to use. Instead, once connected, each peer opens a 
  stream to the other side and uses it for writing outbound traffic, as well as controls its lifecycle. This method does 
  not produce noticeable overhead, as streams are lightweight.

  * All opened streams are persistent meaning that one opened stream is used during whole lifespan of a messenger, unless 
  no reconnects happened. 
    