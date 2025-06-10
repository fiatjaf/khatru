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
    // (do not use the same database used for the relay events)
	bl.Store = blossom.EventStoreBlobIndexWrapper{Store: blobdb, ServiceURL: bl.ServiceURL}

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

## URL Redirection

Blossom supports redirection to external storage locations when retrieving blobs. This is useful when you want to serve files from a CDN or cloud storage service while keeping Blossom compatibility.

You can implement a custom redirect function. This function should return a string with the redirect URL and an HTTP status code.

Here's an example that redirects to a templated URL:
```go
import "github.com/fiatjaf/khatru/policies"

// ...

bl.RedirectGet = append(bl.RedirectGet, policies.RedirectGet("https://blossom.example.com", http.StatusMovedPermanently))
```

The `RedirectGet` hook will append the blob's SHA256 hash and file extension to the redirect URL.

For example, if the blob's SHA256 hash is `b1674191a88ec5cdd733e4240a81803105dc412d6c6708d53ab94fc248f4f553` and the file extension is `pdf`, the redirect URL will be `https://blossom.exampleserver.com/b1674191a88ec5cdd733e4240a81803105dc412d6c6708d53ab94fc248f4f553.pdf`.

You can also customize the redirect URL by passing `{sha256}` and `{extension}` placeholders in the URL. For example:

```go
bl.RedirectGet = append(bl.RedirectGet, policies.RedirectGet("https://mybucket.myblobstorage.com/{sha256}.{extension}?ref=xxxx", http.StatusFound))
```

If you need more control over the redirect URL, you can implement a custom redirect function from scratch. This function should return a string with the redirect URL and an HTTP status code.

```go
bl.RedirectGet = append(bl.RedirectGet, func(ctx context.Context, sha256 string, ext string) (string, int, error) {
    // generate a custom redirect URL
    cid := IPFSCID(sha256)
    redirectURL := fmt.Sprintf("https://ipfs.io/ipfs/%s/%s.%s", cid, sha256, ext)
    return redirectURL, http.StatusTemporaryRedirect, nil
})
```

This URL must include the sha256 hash somewhere. If you return an empty string `""` as the URL, your redirect call will be ignored and the next one in the chain (if any) will be called.

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

This will store blob metadata as special `kind:24242` events, but you shouldn't have to worry about it as the wrapper handles all the complexity of tracking ownership and managing blob lifecycle. Just avoid reusing the same datastore that is used for the actual relay events unless you know what you're doing.
