# rewritecap #

[![GoDoc](https://godoc.org/github.com/jordan2175/rewritecap?status.png)](https://godoc.org/github.com/jordan2175/rewritecap)

A tool for rebasing a PCAP file, editing layer2 and layer3 addresses, and updating ARP packets. PCAP-ng files are not currently supported. This tool will accommodate 802.1Q tagged frames and Q-in-Q double tagged frames. The timestamp changes allow you to rebase the PCAP file to a new date without changing the actual time of day or the inter-frame gaps.  

I wrote this using Go (golang) v1.4

For command line flags run, ./rewritecap --help  

## Installation ##

```
go get github.com/jordan2175/rewritecap
go install rewritecap
```

## Usage ##

[See GoDoc](http://godoc.org/github.com/jordan2175/rewritecap) for
documentation and examples.

## Example ##

```
./rewritecap --help
./rewritecap -f test.pcap -n test2.pacp -y 2016 -m 3 -d 10
./rewritecap -f test.pcap -n test2.pcap --ip4 10.0.2.32 --ip4new 2.2.2.2 --mac 68:A8:6D:18:36:92 --macnew 22:33:44:55:66:77 
```

## Contributing ##

Contributions welcome! Please fork the repository and open a pull request
with your changes or send me a diff patch file.

## License ##

This is free software, licensed under the Apache License, Version 2.0.

