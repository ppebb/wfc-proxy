# wfc-proxy

Simple reverse proxy to sit in front of
[wfc-server](https://github.com/WiiLink24/wfc-server) and properly forward
malformed DWC requests.  
  
This allows wfc-server to use port 80 alongside Nginx and other reverse proxies
which do not properly handle DWC requests.

## Usage

`./wfc-proxy -c [config file]`

See config-example.yml. The following options are available.

| Field         | Description                                                                                               |
| ------------- | --------------------------------------------------------------------------------------------------------- |
| localip       | The local IP to bind to                                                                                   |
| port          | The local port to bind to                                                                                 |
| hostdomain    | The root domain WFC Server is hosted on. Requests to other domains will all be sent to the default remote |
| wfcremote     | The remote to send wfc traffic to                                                                         |
| defaultremote | The remote to send all other traffic to                                                                   |
