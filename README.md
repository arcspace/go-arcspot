## go-librespot


This Go package is an adaption of [librespot-golang](https://github.com/librespot-org/librespot-golang), which itself is an adaption of a [librespot for Rust](https://github.com/librespot-org/librespot) and [librespot-java](https://github.com/librespot-org/librespot-java).  


Why this fork?
  - Offer _librespot_ for Go while departing from the constraints of its predecessor.
  - Refactor its predecessor into proper interfaces that leverage the awesomeness of Go.
  - Focus on core functionality and drop peripheral functionality (e.g. audio conversion, remote control).  For multiple reasons, such non-core functionality should be in a consuming repo, not the core repo.

I will happily support any efforts to merge the work done here with [librespot-golang](https://github.com/librespot-org/librespot-golang), but it will require the cooperation and support of others who agree with the whys above.  Anyone interested, please speak up in a discussion and let's get to work.