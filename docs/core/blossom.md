---
outline: deep
---

# Blossom: Media Storage

Khatru comes with a built-in Blossom HTTP handler that allows you to store and serve media blobs using storage backend you want (filesystem, S3 etc).

## Basic Setup

Here's a minimal example of what you should do to enable it:

```go
func main() {
    relay := khatru.NewRelay()

    // create blossom server with the relay and service URL
    bl := blossom.New(relay, "http://localhost:3334")

    // create a database for keeping track of blob metadata
	bl.Store = blossom.EventStoreBlobIndexWrapper{Store: db, ServiceURL: bl.ServiceURL}

    // implement the required storage functions
    bl.StoreBlob = append(bl.StoreBlob, func(ctx context.Context, sha256 string, body []byte) error {
        // store the blob data somewhere
        return nil
    })
    bl.LoadBlob = append(bl.LoadBlob, func(ctx context.Context, sha256 string) (io.ReadSeeker, error) {
        // load and return the blob data
        return nil, nil
    })
    bl.DeleteBlob = append(bl.DeleteBlob, func(ctx context.Context, sha256 string) error {
        // delete the blob data
        return nil
    })

    http.ListenAndServe(":3334", relay)
}
```

## Storage Backend Integration

You can integrate any storage backend by implementing the three core functions:

- `StoreBlob`: Save the blob data
- `LoadBlob`: Retrieve the blob data
- `DeleteBlob`: Remove the blob data

## Upload Restrictions

You can implement upload restrictions using the `RejectUpload` hook. Here's an example that limits file size and restricts uploads to whitelisted users:

```go
const maxFileSize = 10 * 1024 * 1024 // 10MB

var allowedUsers = map[string]bool{
    "pubkey1": true,
    "pubkey2": true,
}

bl.RejectUpload = append(bl.RejectUpload, func(ctx context.Context, auth *nostr.Event, size int, ext string) (bool, string, int) {
    // check file size
    if size > maxFileSize {
        return true, "file too large", 413
    }

    // check if user is allowed
    if auth == nil || !allowedUsers[auth.PubKey] {
        return true, "unauthorized", 403
    }

    return false, "", 0
})
```

There are other `Reject*` hooks you can also implement, but this is the most important one.

## Tracking blob metadata

Blossom needs a database to keep track of blob metadata in order to know which user owns each blob, for example (and mind you that more than one user might own the same blob so when of them deletes the blob we don't actually delete it because the other user still has a claim to it). The simplest way to do it currently is by relying on a wrapper on top of fake Nostr events over eventstore, which is `EventStoreBlobIndexWrapper`, but other solutions can be used.

```go
db := &badger.BadgerBackend{Path: "/tmp/khatru-badger-blossom-blobstore"}
db.Init()

bl.Store = blossom.EventStoreBlobIndexWrapper{
    Store: db,
    ServiceURL: bl.ServiceURL,
}
```

This will store blob metadata as special `kind:24242` events, but you shouldn't have to worry about it as the wrapper handles all the complexity of tracking ownership and managing blob lifecycle. Jut avoid reusing the same datastore that is used for the actual relay events unless you know what you're doing.
