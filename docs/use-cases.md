# Use cases

`khatru` is being used today in the real world by

* [pyramid](https://github.com/github-tijlxyz/khatru-pyramid), a relay with a invite-based whitelisting system similar to [lobste.rs](https://lobste.rs)
* [triflector](https://github.com/coracle-social/triflector), a relay which enforces authentication based on custom policy
* [countries](https://git.fiatjaf.com/countries), a relay that stores and serves content differently according to the country of the reader or writer
* [jingle](https://github.com/fiatjaf/jingle), a simple relay that exposes part of `khatru`'s configuration options to JavaScript code supplied by the user that is interpreted at runtime
* [njump](https://git.njump.me/njump), a Nostr gateway to the web that also serves its cached content in a relay interface
* [song](https://git.fiatjaf.com/song), a personal git server that comes with an embedded relay dedicated to dealing with [NIP-34](https://nips.nostr.com/34) git-related Nostr events
* [relay29](https://github.com/fiatjaf/relay29), a relay that powers most of the [NIP-29](https://nips.nostr.com/29) Nostr groups ecosystem
* [fiatjaf.com](https://fiatjaf.com), a personal website that serves the same content as HTML but also as Nostr events.
* [gm-relay](https://github.com/ptrio42/gm-relay), a relay that only accepts GM notes once a day.

## Other possible use cases

Other possible use cases, still not developed, include:

* Bridges: `khatru` was initially developed to serve as an RSS-to-Nostr bridge server that would fetch RSS feeds on demand in order to serve them to Nostr clients. Other similar use cases could fit.
* Paid relays: Nostr has multiple relays that charge for write-access currently, but there are many other unexplored ways to make this scheme work: charge per each note, charge per month, charge per month per note, have different payment methods, and so on.
* Other whitelisting schemes: _pyramid_ implements a cool inviting scheme for granting access to the relay, same for _triflector_, but there are infinite other possibilities of other ways to grant access to people to an exclusive or community relay.
* Just-in-time content generation: instead of storing a bunch of signed JSON and serving that to clients, there could be relays that store data in a more compact format and turn it into Nostr events at the time they receive a request from a Nostr client -- or relays that do some kind of live data generation based on who is connected, not storing anything.
* Community relays: some internet communities may want relays that restrict writing or browsing of content only to its members, essentially making it a closed group -- or it could be closed for outsiders to write, but public for them to read and vice-versa.
* Automated moderation schemes: relays that are owned by a group (either a static or a dynamic group) can rely on signals from their members, like mutes or reports, to decide what content to allow in its domains and what to disallow, making crowdfunded moderation easy.
* Curation: in the same way as community relays can deal with unwanted content, they can also perform curation based on signals from their members (for example, if a member of the relay likes some note from someone that is outside the relay that note can be fetched and stored), creating a dynamic relay that can be browsed by anyone that share the same interests as that community.
* Local relays: a relay that can be only browsed by people using the WiFi connection of some event or some building, serving as a way to share temporary or restricted content that only interests people sharing that circumstance.
* Cool experiments: relays that only allow one note per user per day, relays that require proof-of-work on event ids], relays that require engagement otherwise you get kicked, relays that return events in different ordering, relays that impose arbitrary funny rules on notes in order for them to be accepted (i.e. they must contain the word "poo"), I don't know!
