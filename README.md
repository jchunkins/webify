Webify
======

Simply serve your current directory over HTTP.

This is a fork from github.com/goware/webify. It has been updated to run with 
updated dependencies and logging options. Logging defaults to production logging
with Elastic Common Schema with structured logging. Webify can be configured to 
use human readable logs when running in localhost mode (i.e. with environment 
variable `ENV` set to `localhost`). Debugging can also be engaged, which uses 
github.com/golang-cz/devslog for debug logging which provides colorized output
in a terminal. If you're not running in a terminal, then no color is used.

## Install / Usage

```shell
$ go install github.com/jchunkins/webify@latest
```

OR

```shell
$ go run github.com/jchunkins/webify@latest -h
```

run `webify -h` for help

Basic usage:

```shell
$ cd somepath
$ webify
================================================================================
Serving:  /Users/jhunkins/somepath
URL:      http://0.0.0.0:3000
Cache:    off
================================================================================

{"time":"2025-08-01T18:41:57.409183-05:00","level":"INFO","msg":"GET / => HTTP 200 (601.417µs)"}
```

Executing in localhost mode:

```shell
$ ENV=localhost webify
================================================================================
Serving:  /Users/jhunkins/somepath
URL:      http://0.0.0.0:3000
Cache:    off
================================================================================

[18:49:11]  INFO  GET / => HTTP 200 (752.75µs)
```


Specify a directory:

```shell
$ ENV=localhost webify -dir /tmp/test
================================================================================
Serving:  /tmp/test
URL:      http://0.0.0.0:3000
Cache:    off
================================================================================

[19:18:58]  INFO  GET / => HTTP 200 (291.125µs)
```

Debug:

```shell
$ ENV=localhost webify -dir /tmp/test -debug
================================================================================
Serving:  /tmp/test
URL:      http://0.0.0.0:3000
Cache:    off
================================================================================

[20:15:55]  INFO  GET / => HTTP 200 (1.683333ms)
  client.ip                : [::1]:52400
# event.duration           : 1
# http.request.body.bytes  : 0
  http.request.method      : GET
  http.request.referrer    : empty
# http.response.body.bytes : 1157
# http.response.status_code: 200
  http.version             : HTTP/1.1
  log.level                : INFO
* url.domain               : localhost:3000
* url.full                 : http://localhost:3000/
* url.path                 : /
  url.scheme               : http
  user_agent.original      : Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.5 Safari/605.1.15
G headers                  : 
    Accept                   : text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8
    Accept-Encoding          : gzip, deflate
    Accept-Language          : en-US,en;q=0.9
    Connection               : keep-alive
    Priority                 : u=0, i
    Sec-Fetch-Dest           : document
    Sec-Fetch-Mode           : navigate
    Sec-Fetch-Site           : none
    Upgrade-Insecure-Requests: 1
```

Set Log Level:

In this example the page is returned with 200, but the value is not logged since error 
(i.e. 5xx responses only) is set.

```shell
$ ENV=localhost webify -dir /tmp/test -log-level error
================================================================================
Serving:  /tmp/test
URL:      http://0.0.0.0:3000
Cache:    off
================================================================================


```

No banner:

This will not display the banner but will display logs. Can be combined with "silent".

```shell
$ ENV=localhost webify -dir /tmp/test -no-banner
[20:24:58]  INFO  GET / => HTTP 200 (2.524667ms)
```

Silent:

This will prevent logs from being displayed. Can be combined with "no-banner".

```shell
$ ENV=localhost webify -dir /tmp/test -silent
================================================================================
Serving:  /tmp/test
URL:      http://0.0.0.0:3000
Cache:    off
================================================================================


```

No output of any kind:

```shell
webify -dir /tmp/test -silent -no-banner
```

Echo:

Echos back the request body (if provided). If we send a GET and a POST:
```shell
$ curl -L http://localhost:3000/docs
$ curl -L http://localhost:3000 --json '{ "drink": "coffee" }' -H "foo: bar"
```

You end up with this output:
```shell
$ ENV=localhost webify -dir /tmp/test -echo
================================================================================
Serving:  /tmp/test
URL:      http://0.0.0.0:3000
Cache:    off
================================================================================

[20:35:21]  INFO  GET /docs => HTTP 200 (23.584µs)
  http.request.body.content: empty
[20:35:24]  INFO  POST / => HTTP 200 (19.042µs)
  http.request.body.content: { "drink": "coffee" }
```