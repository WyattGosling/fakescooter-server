Fake E-Scooter Sharing Service: Server

Just a dummy app used to practice. In particular, to practice SQL.

# Related Repositories

While the server can be used by making REST calls against it. It is intended to
be used with one of the following frontends. Currently there is only one.

* iOS: [fakescooter-app](https://github.com/WyattGosling/fakescooter-app)

# Usage

```bash
go get
go build
./server
```

There is a included [.restbook](https://github.com/WyattGosling/fakescooter-server/blob/main/api_tests.restbook).
To use this, you will need the [REST Book](https://marketplace.visualstudio.com/items?itemName=tanhakabir.rest-book)
extension for Visual Studio Code. The restbook simply contains some pre-made
requests that can be sent to the server. Other REST UIs like Postman or curl
could be used as well.
