# Tunnel - A naive HTTP tunnel implementation in go

> Note: Still in development

## High level design

### Legend

* FP : Forward proxy
* RP : Reverse proxy
* LTS : Local HTTP server

Request cycle

```
UserClient --request-> FP --forward->  RP
                                       ↓
                                    forward
                                       ↓
                                      LTS      
                                       ↓
                                    response
                                       ↓
UserClient <-response-- FP <-forward-- RP
```

## Deploy forward proxy node

Without authentication enabled (Not recommended)

```
tunnel listen --port PORT --host HOST
```

With authentication enables

```
tunnel listen --port PORT --host HOST --uima
```

Here `--uima` flag tells the forward proxy to use the In-Memory authentication server. When running the proxy using this flag, the proxy server will generate a RSA public key file, you will need this RSA public key file to generate and revoke authentication tokens.

By default this key will be stored in the working directory where the proxy was deployed and you'll see a log lie this

```
FP: 09:19:47 Using in-memory authentication server; RSA public key for authentication is stored in key.pub
```

You can store this key in whatever directory you like using `PROXY_PUBLIC_KEY_FILE` environment variable

```
$ PROXY_PUBLIC_KEY_FILE=/path/to/key.pub tunnel listen --port PORT --host HOST --uima
(...)
FP: 09:19:47 Using in-memory authentication server; RSA public key for authentication is stored in /path/to/key.pub
```

> I'm planning to implement support for authentication using third party auth services

## Generating authentication token

```
tunnel generate-key --key PUBKEY-FILE --proxy PROXY-ADDRESS
```

## Revoking authentication token

```
tunnel revoke-key --id TOKEN-ID --key PUBKEY-FILE --proxy PROXY-ADDRESS
```

## Creating a tunnel

If the proxy has the auth disabled

```
tunnel forward --port PORT --proxy PROXY-ADDRESS
```

else

```
tunnel forward --port PORT --proxy PROXY-ADDRESS --key AUTH-TOKEN
```

## DNS Setup

Add a AAA record with wildcard character as the subdomain for the DNS pointing to the proxy server. For example if your using `tunnel.example.com` as proxy address, add a AAA record which looks like `*.tunnel.example.com`