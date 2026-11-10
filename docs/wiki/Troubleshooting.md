# Troubleshooting

Check the exact error, then verify the network hop involved. A working public HTTPS endpoint does not prove that the separate raw TCP control connection works.

## The client says nothing is listening on the local port

The client checks `localhost:<port>` before registering a tunnel.

1. Start the local application first.
2. Test it from the client machine with `curl http://localhost:3000` for HTTP, or the appropriate TCP client.
3. Confirm the port argument matches the listener. An application bound only to another host or interface is not a supported target; smuf always dials `localhost`.

## `cannot reach smuf-server`

1. Confirm the server process is running and has not logged a bind error.
2. Verify `SMUF_SERVER` includes the actual control host and port, for example `tunnels.example.com:7000`.
3. Test raw TCP connectivity with `nc -zv tunnels.example.com 7000`.
4. Check host, container, cloud, and provider firewall rules. An HTTP health check to port `8080` does not test this listener.
5. If a proxy is involved, make sure it forwards raw TCP rather than HTTP only.

## Authentication fails

Errors such as `el servidor pide contraseña` and `la contraseña no coincide` mean the token is missing or different.

- Set exactly the same `SMUF_AUTH_TOKEN` on both sides and restart processes after changes.
- Check which configuration source each process loaded. A valid profile `config.json` prevents `.env` from being read.
- Remove accidental quotes from `.env`; the parser treats them as part of the token.
- Do not use `$(openssl rand -hex 32)` inside `.env`. Run that command in the shell and paste the result.
- If server and client wizards ran under the same OS account, one may have overwritten the other's JSON settings.

## A custom subdomain is invalid or already used

Use 1 to 63 lowercase letters, digits, or hyphens, with no leading or trailing hyphen. Active names are first-come, first-served.

The server releases a name when its tunnel closes, but does not reserve it afterward. If a reconnection cannot reclaim the name, choose another or stop the conflicting tunnel.

## The URL changed after reconnecting

This is expected for random HTTP IDs and allocated raw TCP ports. Reconnection creates a new registration. Use `--sub` for an HTTP name you want to request again, and monitor the client output for URL changes.

## `tcp tunnels disabled` or `no tcp ports available`

- Set a valid inclusive range such as `SMUF_TCP_PORT_RANGE=20000-20100` on the server and restart it.
- Check that enough ports are free on the host and not allocated to other active smuf tunnels.
- If the tunnel registers but its public port is unreachable, check Docker and provider port mappings. The supplied Compose example does not publish its configured TCP range.
- Retry an immediate first connection. The current server sends the success response before binding the public listener, leaving a short startup window. If the failure persists, inspect the server log for a listener bind error; the earlier success response does not prove that the bind succeeded.

## The public URL returns `missing tunnel subdomain`

The request's `Host` does not match `SMUF_DOMAIN` or one of its subdomains.

1. Confirm `SMUF_DOMAIN` is the base name, not a URL with a scheme or path.
2. Configure DNS for both the base and wildcard names.
3. Preserve `Host` through every reverse proxy.
4. For a local test without wildcard DNS, set the header explicitly:

```bash
curl -H 'Host: a3f1c9d4e2b8.localhost' http://127.0.0.1:8080/
```

Use an active ID shown by your client or dashboard.

## The URL returns `tunnel unavailable`

A `404` normally means the requested ID is not registered. Check that the client is still connected and that the URL was not replaced after reconnection.

A `502` means the server found a tunnel but could not proxy the request. Inspect server logs and confirm the local application is still accepting connections. The client initially checks a local port, but the application can stop later.

## WebSockets, SSE, or long polling fail through a proxy

Direct smuf HTTP tunnels include upgrade and streaming paths. An external proxy can still buffer responses, strip upgrade headers, change `Host`, or enforce an idle timeout.

Configure that proxy for HTTP/1.1 upgrades, disable buffering for streaming endpoints, preserve the original host, and use timeouts appropriate for long-lived connections. Test the smuf public listener directly to separate proxy behavior from tunnel behavior.

## HTTPS certificates are not issued

1. Confirm both the base domain and wildcard DNS resolve to this server.
2. Confirm public port `80` reaches `SMUF_HTTP_PORT` and public port `443` reaches `SMUF_HTTPS_PORT`.
3. For direct ACME HTTP-01, normally set `SMUF_HTTP_PORT=80`.
4. Check that the service account can create and write `SMUF_ACME_CACHE`.
5. Inspect server logs for ACME errors. `SMUF_ACME_EMAIL` is a contact setting, not proof that DNS or networking is correct.
6. Keep the cache across restarts and avoid sharing a writable cache with unrelated processes.

If TLS terminates at an external provider, also test absolute redirects. smuf's printed scheme is based on its own `SMUF_HTTPS` setting, not on an external proxy's TLS termination.

## Connections are rejected or the handshake closes unexpectedly

The server defaults to five concurrent control connections per observed source IP and 30 new attempts per IP per minute. Each tunnel uses one connection, including each port in a multi-port command.

Reduce concurrent tunnels, wait before retrying, or adjust `SMUF_MAX_CONNS_PER_IP` and `SMUF_CONN_RATE_PER_MIN` deliberately. Clients behind NAT or a TCP proxy can share one bucket. These settings do not limit public visitor traffic.

## The dashboard exposes metadata

This is current behavior, not a configuration error. The base-domain dashboard and `/_smuf/tunnels` have no built-in authentication, and the JSON endpoint permits cross-origin reads. Restrict access at a reverse proxy or network layer if tunnel URLs, local ports, and client IPs must not be public.

## Getting help

Search [existing issues](https://github.com/cosmind-rusu/smuf/issues), then open a [new issue](https://github.com/cosmind-rusu/smuf/issues/new) with reproduction steps and logs. Include the version or commit, OS, architecture, deployment topology, and relevant configuration with tokens and private addresses removed.
