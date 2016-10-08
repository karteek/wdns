## wdns

wdns is a simple DNS relay server that can do wildcard DNS for IP Addresses.
Like [xip.io](http://xip.io/), but with a small change. This supports hyphens in addition to dots.

When run with `int.example.com` as argument, it will resolve

    192-168-1-1.int.example.com to 192.168.1.1
    192.168.1.1.int.example.com to 192.168.1.1
    foo.10-1-2-3.int.example.com to 10.1.2.3


This supports only requests for A records (for wild card DNS), and any domain that's doesn't match *.int.example.com
gets resolved as per Google DNS Server 8.8.8.8

