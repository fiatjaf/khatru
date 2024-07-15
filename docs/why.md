# Why `khatru`?

If you want to craft a relay that isn't completely dumb, but it's supposed to

* have custom own policies for accepting events;
* handle requests for stored events using data from multiple sources;
* require users to authenticate for some operations and not for others;
* and other stuff.

`khatru` provides a simple framework for creating your custom relay without having to reimplement it all from scratch or hack into other relay codebases.
